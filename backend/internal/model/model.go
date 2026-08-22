package model

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// ---------- 系统指标 ----------

type CPUMetric struct {
	Percent float64   `json:"percent"`
	PerCore []float64 `json:"per_core"`
}

type MemMetric struct {
	Total   uint64  `json:"total"`
	Used    uint64  `json:"used"`
	Percent float64 `json:"percent"`
}

type DiskMetric struct {
	Mount   string  `json:"mount"`
	Total   uint64  `json:"total"`
	Used    uint64  `json:"used"`
	Percent float64 `json:"percent"`
}

type NetMetric struct {
	Iface string  `json:"iface"`
	RxBps float64 `json:"rx_bps"`
	TxBps float64 `json:"tx_bps"`
}

type MetricSnapshot struct {
	Ts    time.Time    `json:"ts"`
	CPU   CPUMetric    `json:"cpu"`
	Mem   MemMetric    `json:"mem"`
	Disk  []DiskMetric `json:"disk"`
	Net   []NetMetric  `json:"net"`
	Load  [3]float64   `json:"load"`
	Procs int          `json:"procs"`
}

// ---------- 容器 ----------

type ContainerInfo struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Image      string    `json:"image"`
	State      string    `json:"state"`
	Status     string    `json:"status"`
	CPUPercent float64   `json:"cpu_percent"`
	MemUsed    uint64    `json:"mem_used"`
	MemLimit   uint64    `json:"mem_limit"`
	MemPercent float64   `json:"mem_percent"`
	NetRx      uint64    `json:"net_rx"`
	NetTx      uint64    `json:"net_tx"`
	CreatedAt  time.Time `json:"created_at"`
	UptimeSec  float64   `json:"uptime_sec"`
}

type ContainerDetail struct {
	ContainerInfo
	Ports      []string `json:"ports"`
	Mounts     []string `json:"mounts"`
	EnvPreview []string `json:"env_preview"`
}

// ---------- 告警 ----------

type MetricKind string

const (
	MetricCPUPercent      MetricKind = "cpu_percent"
	MetricMemPercent      MetricKind = "mem_percent"
	MetricDiskPercent     MetricKind = "disk_percent"
	MetricNetRxBps        MetricKind = "net_rx_bps"
	MetricNetTxBps        MetricKind = "net_tx_bps"
	MetricContainerCPUPct MetricKind = "container_cpu_percent"
	MetricContainerMemPct MetricKind = "container_mem_percent"
)

func (k MetricKind) IsContainer() bool {
	return k == MetricContainerCPUPct || k == MetricContainerMemPct
}

// 阈值上下界（schema 驱动，前后端一致）
func (k MetricKind) ThresholdRange() (min, max float64) {
	switch k {
	case MetricCPUPercent, MetricMemPercent, MetricDiskPercent, MetricContainerMemPct:
		return 0, 100
	case MetricContainerCPUPct:
		return 0, 1e6 // 容器 CPU 可超 100%（多核）
	default: // 网络速率
		return 0, 1e12
	}
}

type AlertOp string

const (
	OpGt AlertOp = ">"
	OpGe AlertOp = ">="
	OpLt AlertOp = "<"
	OpLe AlertOp = "<="
)

type AlertRule struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Metric         MetricKind `json:"metric"`
	Target         string     `json:"target"`
	Op             AlertOp    `json:"op"`
	Threshold      float64    `json:"threshold"`
	DurationSec    int        `json:"duration_sec"`
	CooldownSec    int        `json:"cooldown_sec"`
	Enabled        bool       `json:"enabled"`
	WebhookURL     string     `json:"webhook_url"`
	NotifyRecovery bool       `json:"notify_recovery"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type AlertEvent struct {
	ID            string    `json:"id"`
	RuleID        string    `json:"rule_id"`
	RuleName      string    `json:"rule_name"`
	Metric        string    `json:"metric"`
	Target        string    `json:"target"`
	Value         float64   `json:"value"`
	Threshold     float64   `json:"threshold"`
	Op            string    `json:"op"`
	Kind          string    `json:"kind"` // fired | recovered
	WebhookStatus int       `json:"webhook_status"`
	WebhookError  string    `json:"webhook_error"`
	FiredAt       time.Time `json:"fired_at"`
}

// WebhookReceipt Mock Webhook 接收记录
type WebhookReceipt struct {
	ID         int64     `json:"id"`
	ReceivedAt time.Time `json:"received_at"`
	Payload    string    `json:"payload"`
}

type WebhookPayload struct {
	RuleID    string    `json:"rule_id"`
	RuleName  string    `json:"rule_name"`
	Metric    string    `json:"metric"`
	Target    string    `json:"target"`
	Op        string    `json:"op"`
	Threshold float64   `json:"threshold"`
	Value     float64   `json:"value"`
	Kind      string    `json:"kind"`
	FiredAt   time.Time `json:"fired_at"`
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidateRule 结构完整性校验（全局记忆：反序列化必须校验字段存在性/类型/边界）
func ValidateRule(r *AlertRule) []FieldError {
	var errs []FieldError
	add := func(field, msg string) { errs = append(errs, FieldError{Field: field, Message: msg}) }

	if strings.TrimSpace(r.Name) == "" {
		add("name", "规则名称不能为空")
	}
	switch r.Metric {
	case MetricCPUPercent, MetricMemPercent, MetricDiskPercent,
		MetricNetRxBps, MetricNetTxBps, MetricContainerCPUPct, MetricContainerMemPct:
	default:
		add("metric", fmt.Sprintf("未知指标类型 %q", r.Metric))
	}
	switch r.Op {
	case OpGt, OpGe, OpLt, OpLe:
	default:
		add("op", fmt.Sprintf("未知运算符 %q", r.Op))
	}
	if r.Metric != "" {
		min, max := r.Metric.ThresholdRange()
		if r.Threshold < min || r.Threshold > max {
			add("threshold", fmt.Sprintf("阈值范围为 %v ~ %v", min, max))
		}
	}
	if r.DurationSec < 0 || r.DurationSec >= 86400 {
		add("duration_sec", "持续时间须为 0 ~ 86400 秒")
	}
	if r.CooldownSec < 0 || r.CooldownSec > 86400 {
		add("cooldown_sec", "冷却期须为 0 ~ 86400 秒")
	}
	u := strings.TrimSpace(r.WebhookURL)
	if u == "" {
		add("webhook_url", "Webhook URL 不能为空")
	} else if parsed, err := url.Parse(u); err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		add("webhook_url", "仅支持合法的 http/https URL")
	}
	if r.Metric.IsContainer() && strings.TrimSpace(r.Target) == "" {
		add("target", "容器类指标必须填写目标容器名称")
	}
	return errs
}

// Compare 应用运算符
func Compare(op AlertOp, value, threshold float64) bool {
	switch op {
	case OpGt:
		return value > threshold
	case OpGe:
		return value >= threshold
	case OpLt:
		return value < threshold
	case OpLe:
		return value <= threshold
	}
	return false
}
