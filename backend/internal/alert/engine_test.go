package alert

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"dockeradmin/internal/logger"
	"dockeradmin/internal/model"
)

type fakeSources struct {
	snap       model.MetricSnapshot
	sysOK      bool
	containers []model.ContainerInfo
	ctrOK      bool
}

func (f *fakeSources) toSources() Sources {
	return Sources{
		LatestSystem:     func() (model.MetricSnapshot, bool) { return f.snap, f.sysOK },
		LatestContainers: func() ([]model.ContainerInfo, bool) { return f.containers, f.ctrOK },
	}
}

func setup(t *testing.T) (*Store, *httptest.Server, *int32) {
	t.Helper()
	store, err := NewStore(filepath.Join(t.TempDir(), "alerts.json"), logger.New("error"))
	if err != nil {
		t.Fatal(err)
	}
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		body, _ := io.ReadAll(r.Body)
		var p model.WebhookPayload
		if err := json.Unmarshal(body, &p); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return store, srv, &calls
}

func cpuRule(url string) model.AlertRule {
	return model.AlertRule{
		Name: "t", Metric: model.MetricCPUPercent, Op: model.OpGt,
		Threshold: 80, DurationSec: 30, CooldownSec: 300,
		Enabled: true, WebhookURL: url, NotifyRecovery: true,
	}
}

func TestEngine_NoBreach_NoEvent(t *testing.T) {
	store, srv, calls := setup(t)
	fs := &fakeSources{snap: model.MetricSnapshot{CPU: model.CPUMetric{Percent: 50}}, sysOK: true}
	e := NewEngine(store, NewNotifier(logger.New("error")), fs.toSources(), logger.New("error"))

	rule := cpuRule(srv.URL)
	if _, err := store.Create(&rule); err != nil {
		t.Fatal(err)
	}
	e.Evaluate(time.Now())
	e.Evaluate(time.Now().Add(time.Minute))
	if got := len(store.Events(10)); got != 0 {
		t.Fatalf("未越限不应触发，events = %d", got)
	}
	if *calls != 0 {
		t.Fatalf("webhook 不应被调用，calls = %d", *calls)
	}
}

func TestEngine_DurationGate(t *testing.T) {
	store, srv, calls := setup(t)
	fs := &fakeSources{snap: model.MetricSnapshot{CPU: model.CPUMetric{Percent: 95}}, sysOK: true}
	e := NewEngine(store, NewNotifier(logger.New("error")), fs.toSources(), logger.New("error"))

	rule := cpuRule(srv.URL)
	if _, err := store.Create(&rule); err != nil {
		t.Fatal(err)
	}

	t0 := time.Now()
	e.Evaluate(t0)                       // 越限开始
	e.Evaluate(t0.Add(10 * time.Second)) // 持续 10s < 30s
	if len(store.Events(10)) != 0 {
		t.Fatal("持续时间未达不应触发")
	}
	e.Evaluate(t0.Add(31 * time.Second)) // 持续 31s ≥ 30s → 触发
	events := store.Events(10)
	if len(events) != 1 || events[0].Kind != "fired" {
		t.Fatalf("持续时间到达应触发 fired，events = %v", events)
	}
	if *calls != 1 {
		t.Fatalf("webhook 应调用 1 次，实际 %d", *calls)
	}
	if events[0].WebhookStatus != 200 {
		t.Fatalf("webhook_status = %d, want 200", events[0].WebhookStatus)
	}
	if events[0].Value != 95 {
		t.Fatalf("event value = %v, want 95", events[0].Value)
	}
}

func TestEngine_Cooldown(t *testing.T) {
	store, srv, calls := setup(t)
	fs := &fakeSources{snap: model.MetricSnapshot{CPU: model.CPUMetric{Percent: 95}}, sysOK: true}
	e := NewEngine(store, NewNotifier(logger.New("error")), fs.toSources(), logger.New("error"))

	rule := cpuRule(srv.URL)
	rule.DurationSec = 0 // 立即触发，聚焦冷却期语义
	if _, err := store.Create(&rule); err != nil {
		t.Fatal(err)
	}

	t0 := time.Now()
	e.Evaluate(t0) // 越限 → 立即触发（calls=1）
	if *calls != 1 {
		t.Fatalf("首次越限应触发，calls = %d", *calls)
	}
	// 抖动场景：恢复（calls=2, recovered）→ 冷却期内再次越限 → 不得触发
	fs.snap.CPU.Percent = 40
	e.Evaluate(t0.Add(10 * time.Second))
	if *calls != 2 {
		t.Fatalf("恢复应通知，calls = %d, want 2", *calls)
	}
	fs.snap.CPU.Percent = 95
	e.Evaluate(t0.Add(20 * time.Second)) // 距 lastFired 20s < 300s 冷却期
	if *calls != 2 {
		t.Fatalf("冷却期内抖动不应重复触发，calls = %d, want 2", *calls)
	}
	// 冷却期过后仍越限 → 再次触发
	e.Evaluate(t0.Add(301 * time.Second))
	if *calls != 3 {
		t.Fatalf("冷却期过后持续越限应再次触发，calls = %d, want 3", *calls)
	}
}

func TestEngine_Recovery(t *testing.T) {
	store, srv, _ := setup(t)
	fs := &fakeSources{snap: model.MetricSnapshot{CPU: model.CPUMetric{Percent: 95}}, sysOK: true}
	e := NewEngine(store, NewNotifier(logger.New("error")), fs.toSources(), logger.New("error"))

	rule := cpuRule(srv.URL)
	if _, err := store.Create(&rule); err != nil {
		t.Fatal(err)
	}

	t0 := time.Now()
	e.Evaluate(t0)                       // 建立越限起点
	e.Evaluate(t0.Add(31 * time.Second)) // 持续达标 → fired
	fs.snap.CPU.Percent = 40             // 回落
	e.Evaluate(t0.Add(32 * time.Second)) // recovered

	events := store.Events(10)
	if len(events) != 2 {
		t.Fatalf("应有 fired + recovered 两条事件，实际 %d", len(events))
	}
	if events[0].Kind != "recovered" || events[1].Kind != "fired" {
		t.Fatalf("事件顺序/类型错误: %v", events)
	}
}

func TestEngine_DisabledRule(t *testing.T) {
	store, srv, calls := setup(t)
	fs := &fakeSources{snap: model.MetricSnapshot{CPU: model.CPUMetric{Percent: 95}}, sysOK: true}
	e := NewEngine(store, NewNotifier(logger.New("error")), fs.toSources(), logger.New("error"))

	rule := cpuRule(srv.URL)
	rule.Enabled = false
	if _, err := store.Create(&rule); err != nil {
		t.Fatal(err)
	}
	e.Evaluate(time.Now())
	e.Evaluate(time.Now().Add(time.Hour))
	if *calls != 0 {
		t.Fatal("停用规则不应触发")
	}
}

func TestEngine_ContainerMetric(t *testing.T) {
	store, srv, calls := setup(t)
	fs := &fakeSources{
		sysOK: false, ctrOK: true,
		containers: []model.ContainerInfo{
			{Name: "web", State: "running", CPUPercent: 150},
			{Name: "db", State: "running", CPUPercent: 10},
		},
	}
	e := NewEngine(store, NewNotifier(logger.New("error")), fs.toSources(), logger.New("error"))

	rule := model.AlertRule{
		Name: "容器CPU", Metric: model.MetricContainerCPUPct, Target: "web", Op: model.OpGt,
		Threshold: 100, DurationSec: 0, CooldownSec: 0,
		Enabled: true, WebhookURL: srv.URL,
	}
	if _, err := store.Create(&rule); err != nil {
		t.Fatal(err)
	}
	e.Evaluate(time.Now())
	if *calls != 1 {
		t.Fatalf("目标容器越限应触发，calls = %d", *calls)
	}
	// 目标容器消失 → 不触发也不 panic
	fs.containers = fs.containers[1:]
	e.Evaluate(time.Now().Add(time.Minute))
	if *calls != 1 {
		t.Fatal("容器消失后不应再次触发")
	}
}

func TestStore_PersistAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "alerts.json")
	store, err := NewStore(path, logger.New("error"))
	if err != nil {
		t.Fatal(err)
	}
	rule := cpuRule("http://example.com/hook")
	created, err := store.Create(&rule)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Fatal("创建后应有 ID")
	}

	// 重载
	store2, err := NewStore(path, logger.New("error"))
	if err != nil {
		t.Fatal(err)
	}
	list := store2.List()
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("持久化重载失败: %v", list)
	}

	// 更新 + 删除
	rule.Threshold = 90
	if _, found, err := store2.Update(created.ID, &rule); err != nil || !found {
		t.Fatalf("更新失败 found=%v err=%v", found, err)
	}
	if got, _ := store2.Get(created.ID); got.Threshold != 90 {
		t.Fatal("更新未生效")
	}
	if found, err := store2.Delete(created.ID); err != nil || !found {
		t.Fatalf("删除失败 found=%v err=%v", found, err)
	}
	if len(store2.List()) != 0 {
		t.Fatal("删除后应为空")
	}
}

func TestStore_SkipInvalidPersisted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "alerts.json")
	// 写入一条合法 + 一条非法（缺 webhook_url）
	content := `[
	  {"id":"ok1","name":"a","metric":"cpu_percent","op":">","threshold":80,"webhook_url":"http://x.com/h"},
	  {"id":"bad1","name":"b","metric":"cpu_percent","op":">","threshold":80}
	]`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(path, logger.New("error"))
	if err != nil {
		t.Fatal(err)
	}
	list := store.List()
	if len(list) != 1 || list[0].ID != "ok1" {
		t.Fatalf("非法持久化条目应被跳过: %v", list)
	}
}
