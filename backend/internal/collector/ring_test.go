package collector

import (
	"testing"
	"time"

	"dockeradmin/internal/model"
)

func snapAt(t time.Time) model.MetricSnapshot {
	return model.MetricSnapshot{Ts: t}
}

func TestRingAddAndLatest(t *testing.T) {
	r := NewRing(time.Hour)
	if _, ok := r.Latest(); ok {
		t.Fatal("empty ring should have no latest")
	}
	now := time.Now()
	r.Add(snapAt(now.Add(-2 * time.Second)))
	r.Add(snapAt(now.Add(-1 * time.Second)))
	r.Add(snapAt(now))

	latest, ok := r.Latest()
	if !ok || !latest.Ts.Equal(now) {
		t.Fatalf("latest ts = %v, want %v", latest.Ts, now)
	}
	if r.Len() != 3 {
		t.Fatalf("len = %d, want 3", r.Len())
	}
}

func TestRingWindowTrim(t *testing.T) {
	r := NewRing(10 * time.Second)
	now := time.Now()
	r.Add(snapAt(now.Add(-time.Hour))) // 超出窗口
	r.Add(snapAt(now.Add(-5 * time.Second)))
	r.Add(snapAt(now))

	if r.Len() != 2 {
		t.Fatalf("len = %d, want 2 (过期样本应被裁剪)", r.Len())
	}
}

func TestRingSince(t *testing.T) {
	r := NewRing(time.Hour)
	now := time.Now()
	r.Add(snapAt(now.Add(-30 * time.Minute)))
	r.Add(snapAt(now.Add(-5 * time.Minute)))
	r.Add(snapAt(now))

	got := r.Since(10 * time.Minute)
	if len(got) != 2 {
		t.Fatalf("Since(10m) = %d 条, want 2", len(got))
	}
	if got[0].Ts.Before(now.Add(-10 * time.Minute)) {
		t.Fatal("Since 返回了窗口外的样本")
	}
}

func TestRingConcurrentAccess(t *testing.T) {
	r := NewRing(time.Minute)
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			r.Add(snapAt(time.Now()))
		}
		close(done)
	}()
	for i := 0; i < 100; i++ {
		_ = r.Since(time.Minute)
		_, _ = r.Latest()
	}
	<-done
}
