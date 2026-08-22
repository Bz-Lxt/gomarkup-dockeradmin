package alert

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"dockeradmin/internal/model"
)

// 数据源回调（依赖注入，避免 import 环）
type Sources struct {
	LatestSystem     func() (model.MetricSnapshot, bool)
	LatestContainers func() ([]model.ContainerInfo, bool)
}

type ruleState struct {
	breachSince time.Time // 零值 = 未越限
	active      bool      // 当前处于告警态
	lastFired   time.Time
}

// Engine 告警引擎：随采集节拍评估全部启用规则。
// 语义：持续越限达 DurationSec 才触发；触发后进入冷却期；回落且 active 时可发恢复通知。
type Engine struct {
	store    *Store
	notifier *Notifier
	sources  Sources
	log      *slog.Logger

	mu     sync.Mutex
	states map[string]*ruleState
}

func NewEngine(store *Store, notifier *Notifier, sources Sources, log *slog.Logger) *Engine {
	return &Engine{
		store:    store,
		notifier: notifier,
		sources:  sources,
		log:      log,
		states:   make(map[string]*ruleState),
	}
}

// Run 以采集间隔为节拍评估（独立于采集循环，解耦）。
func (e *Engine) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.Evaluate(time.Now())
		}
	}
}

// Evaluate 对所有启用规则执行一轮判定（导出以便测试）。
func (e *Engine) Evaluate(now time.Time) {
	snap, sysOK := e.sources.LatestSystem()
	containers, ctrOK := e.sources.LatestContainers()

	for _, rule := range e.store.List() {
		if !rule.Enabled {
			e.resetState(rule.ID)
			continue
		}
		value, ok := resolveValue(rule, snap, sysOK, containers, ctrOK)
		if !ok {
			continue // 目标不存在（如容器消失）：不判越限，保持原状态
		}
		e.evalRule(rule, value, now)
	}
}

func resolveValue(rule model.AlertRule, snap model.MetricSnapshot, sysOK bool, containers []model.ContainerInfo, ctrOK bool) (float64, bool) {
	switch rule.Metric {
	case model.MetricCPUPercent:
		if sysOK {
			return snap.CPU.Percent, true
		}
	case model.MetricMemPercent:
		if sysOK {
			return snap.Mem.Percent, true
		}
	case model.MetricDiskPercent:
		if sysOK && len(snap.Disk) > 0 {
			diskPct := snap.Disk[0].Percent
			for _, d := range snap.Disk[1:] {
				if d.Percent < diskPct {
					diskPct = d.Percent
				}
			}
			return diskPct, true
		}
	case model.MetricNetRxBps, model.MetricNetTxBps:
		if sysOK {
			var total float64
			for _, n := range snap.Net {
				if n.Iface == "lo" {
					continue
				}
				if rule.Metric == model.MetricNetRxBps {
					total += n.RxBps
				} else {
					total += n.TxBps
				}
			}
			return total, true
		}
	case model.MetricContainerCPUPct, model.MetricContainerMemPct:
		if ctrOK {
			for _, c := range containers {
				if c.Name == rule.Target || c.ID == rule.Target {
					if rule.Metric == model.MetricContainerCPUPct {
						return c.CPUPercent, true
					}
					return c.MemPercent, true
				}
			}
		}
	}
	return 0, false
}

func (e *Engine) evalRule(rule model.AlertRule, value float64, now time.Time) {
	breached := model.Compare(rule.Op, value, rule.Threshold)

	e.mu.Lock()
	st, ok := e.states[rule.ID]
	if !ok {
		st = &ruleState{}
		e.states[rule.ID] = st
	}

	if breached {
		if st.breachSince.IsZero() {
			st.breachSince = now
		}
		durationMet := now.Sub(st.breachSince) >= time.Duration(rule.DurationSec)*time.Second
		cooldownOver := st.lastFired.IsZero() || now.Sub(st.lastFired) >= time.Duration(rule.CooldownSec)*time.Second
		// 触发条件：持续达标 且（新告警且冷却已过）或（持续越限的重复通知，cooldown>0 时按冷却期节流；
		// cooldown=0 表示「每次越限事件只通知一次」，避免每拍轰炸）
		if durationMet && cooldownOver && (!st.active || rule.CooldownSec > 0) {
			st.active = true
			st.lastFired = now
			e.mu.Unlock()
			e.fire(rule, value, "fired", now)
			return
		}
		e.mu.Unlock()
		return
	}

	// 未越限：清除越限计时；若处于告警态且开启恢复通知 → 发恢复
	st.breachSince = time.Time{}
	if st.active && rule.NotifyRecovery {
		st.active = false
		e.mu.Unlock()
		e.fire(rule, value, "recovered", now)
		return
	}
	st.active = false
	e.mu.Unlock()
}

func (e *Engine) resetState(ruleID string) {
	e.mu.Lock()
	delete(e.states, ruleID)
	e.mu.Unlock()
}

func (e *Engine) fire(rule model.AlertRule, value float64, kind string, now time.Time) {
	payload := model.WebhookPayload{
		RuleID:    rule.ID,
		RuleName:  rule.Name,
		Metric:    string(rule.Metric),
		Target:    rule.Target,
		Op:        string(rule.Op),
		Threshold: rule.Threshold,
		Value:     value,
		Kind:      kind,
		FiredAt:   now,
	}

	event := model.AlertEvent{
		RuleID:    rule.ID,
		RuleName:  rule.Name,
		Metric:    string(rule.Metric),
		Target:    rule.Target,
		Value:     value,
		Threshold: rule.Threshold,
		Op:        string(rule.Op),
		Kind:      kind,
		FiredAt:   now,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result := e.notifier.Send(ctx, rule.WebhookURL, payload)
	event.WebhookStatus = result.Status
	event.WebhookError = result.Err
	e.store.AddEvent(event)

	if result.Err != "" {
		e.log.Warn("alert webhook failed", "rule", rule.Name, "kind", kind, "err", result.Err)
	} else {
		e.log.Info("alert webhook sent", "rule", rule.Name, "kind", kind, "value", value, "status", result.Status)
	}
}
