package alert_test

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"dockeradmin/internal/alert"
	"dockeradmin/internal/model"
)

func TestDiskAlertTriggersWhenAnyMountExceedsThreshold(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	store, err := alert.NewStore(filepath.Join(t.TempDir(), "alerts.json"), log)
	if err != nil {
		t.Fatal(err)
	}
	rule := model.AlertRule{
		Name:       "disk capacity",
		Metric:     model.MetricDiskPercent,
		Op:         model.OpGt,
		Threshold:  80,
		Enabled:    true,
		WebhookURL: "http://localhost:bad/hook",
	}
	if _, err := store.Create(&rule); err != nil {
		t.Fatal(err)
	}

	snapshot := model.MetricSnapshot{Disk: []model.DiskMetric{
		{Mount: "/", Percent: 45},
		{Mount: "/data", Percent: 92},
	}}
	engine := alert.NewEngine(store, alert.NewNotifier(log), alert.Sources{
		LatestSystem:     func() (model.MetricSnapshot, bool) { return snapshot, true },
		LatestContainers: func() ([]model.ContainerInfo, bool) { return nil, false },
	}, log)

	engine.Evaluate(time.Now())

	events := store.Events(10)
	if len(events) != 1 || events[0].Kind != "fired" || events[0].Value != 92 {
		t.Fatalf("alert events = %+v, want one fired event with value 92", events)
	}
}
