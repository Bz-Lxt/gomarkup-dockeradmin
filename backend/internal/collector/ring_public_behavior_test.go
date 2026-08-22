package collector_test

import (
	"testing"
	"time"

	"dockeradmin/internal/collector"
	"dockeradmin/internal/model"
)

func TestRingHistoryIsIsolatedFromCallerMutation(t *testing.T) {
	ring := collector.NewRing(time.Hour)
	ring.Add(model.MetricSnapshot{Ts: time.Now(), Procs: 20})

	history := ring.Since(time.Minute)
	if len(history) != 1 {
		t.Fatalf("最近 1 分钟应返回 1 个快照，实际返回 %d 个", len(history))
	}
	history[0].Procs = 999

	again := ring.Since(time.Minute)
	if len(again) != 1 || again[0].Procs != 20 {
		t.Fatalf("修改查询结果污染了采集器中的原始快照: %+v", again)
	}
}
