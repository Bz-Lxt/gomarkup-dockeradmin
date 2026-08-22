package collector

import (
	"sync"
	"time"

	"dockeradmin/internal/model"
)

// Ring 时间窗环形缓冲：按时间窗保留快照，零值不可用（必须 NewRing）
type Ring struct {
	mu     sync.RWMutex
	buf    []model.MetricSnapshot
	window time.Duration
}

func NewRing(window time.Duration) *Ring {
	// 预分配：窗口/2s 采样 + 余量（golang-patterns：已知容量预分配）
	capHint := int(window/(2*time.Second)) * 2
	return &Ring{buf: make([]model.MetricSnapshot, 0, max(capHint, 64)), window: window}
}

func (r *Ring) Add(s model.MetricSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := s.Ts.Add(-r.window)
	// buf 按时间递增，二分找裁剪点
	idx := 0
	for idx < len(r.buf) && r.buf[idx].Ts.Before(cutoff) {
		idx++
	}
	if idx > 0 {
		// 避免底层数组无限增长：超出容量 2 倍时重建
		if idx > cap(r.buf)/2 {
			nb := make([]model.MetricSnapshot, len(r.buf)-idx, cap(r.buf))
			copy(nb, r.buf[idx:])
			r.buf = nb
		} else {
			r.buf = r.buf[idx:]
		}
	}
	r.buf = append(r.buf, s)
}

func (r *Ring) Since(window time.Duration) []model.MetricSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cutoff := time.Now().Add(-window)
	idx := 0
	for idx < len(r.buf) && r.buf[idx].Ts.Before(cutoff) {
		idx++
	}
	return r.buf[idx:]
}

func (r *Ring) Latest() (model.MetricSnapshot, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.buf) == 0 {
		return model.MetricSnapshot{}, false
	}
	return r.buf[len(r.buf)-1], true
}

func (r *Ring) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.buf)
}
