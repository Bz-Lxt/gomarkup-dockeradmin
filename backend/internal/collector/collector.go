package collector

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"

	"dockeradmin/internal/model"
)

// Collector 周期采集系统指标，存环形缓冲并向订阅者广播。
type Collector struct {
	interval time.Duration
	ring     *Ring
	log      *slog.Logger

	rootfs string // 宿主根挂载点（/rootfs），空表示非宿主模式

	mu   sync.Mutex
	subs map[chan model.MetricSnapshot]struct{}

	prevNet   map[string][2]uint64 // iface -> [rx, tx] 累计值
	prevNetAt time.Time
}

func New(interval, window time.Duration, rootfs string, log *slog.Logger) *Collector {
	return &Collector{
		interval: interval,
		ring:     NewRing(window),
		log:      log,
		rootfs:   rootfs,
		subs:     make(map[chan model.MetricSnapshot]struct{}),
		prevNet:  make(map[string][2]uint64),
	}
}

func (c *Collector) Run(ctx context.Context) {
	c.sampleOnce(ctx) // 立即采一帧，避免启动空窗
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.sampleOnce(ctx)
		}
	}
}

func (c *Collector) Subscribe() chan model.MetricSnapshot {
	ch := make(chan model.MetricSnapshot, 8)
	c.mu.Lock()
	c.subs[ch] = struct{}{}
	c.mu.Unlock()
	return ch
}

func (c *Collector) Unsubscribe(ch chan model.MetricSnapshot) {
	c.mu.Lock()
	delete(c.subs, ch)
	c.mu.Unlock()
}

func (c *Collector) History(window time.Duration) []model.MetricSnapshot {
	return c.ring.Since(window)
}

func (c *Collector) Latest() (model.MetricSnapshot, bool) {
	return c.ring.Latest()
}

func (c *Collector) publish(s model.MetricSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for ch := range c.subs {
		select {
		case ch <- s:
		default: // 慢消费者丢帧，防阻塞采集循环
		}
	}
}

func (c *Collector) sampleOnce(ctx context.Context) {
	s := model.MetricSnapshot{Ts: time.Now()}

	if pct, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(pct) > 0 {
		s.CPU.Percent = round2(pct[0])
	}
	if perCore, err := cpu.PercentWithContext(ctx, 0, true); err == nil {
		s.CPU.PerCore = make([]float64, len(perCore))
		for i, v := range perCore {
			s.CPU.PerCore[i] = round2(v)
		}
	}
	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		s.Mem = model.MemMetric{Total: vm.Total, Used: vm.Used, Percent: round2(vm.UsedPercent)}
	}
	s.Disk = c.sampleDisk(ctx)
	s.Net = c.sampleNet(ctx)
	if avg, err := load.AvgWithContext(ctx); err == nil {
		s.Load = [3]float64{round2(avg.Load1), round2(avg.Load5), round2(avg.Load15)}
	}
	if pids, err := process.PidsWithContext(ctx); err == nil {
		s.Procs = len(pids)
	}

	c.ring.Add(s)
	c.publish(s)
}

var allowedFstypes = map[string]bool{
	"ext2": true, "ext3": true, "ext4": true, "xfs": true, "btrfs": true,
	"zfs": true, "apfs": true, "hfs": true, "vfat": true, "exfat": true,
	"ntfs": true, "fuseblk": true, "overlay": true,
}

func (c *Collector) sampleDisk(ctx context.Context) []model.DiskMetric {
	parts, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		c.log.Debug("disk partitions failed", "err", err)
		return nil
	}
	seen := make(map[string]model.DiskMetric) // device -> 指标（去重 bind 挂载，保留最短挂载点）
	for _, p := range parts {
		if !allowedFstypes[p.Fstype] && !strings.HasPrefix(p.Device, "/dev/") {
			continue
		}
		if prev, ok := seen[p.Device]; ok && len(prev.Mount) <= len(p.Mountpoint) {
			continue
		}
		usage, err := disk.UsageWithContext(ctx, c.rootfs+p.Mountpoint)
		if err != nil || usage.Total == 0 {
			continue
		}
		seen[p.Device] = model.DiskMetric{
			Mount:   p.Mountpoint,
			Total:   usage.Total,
			Used:    usage.Used,
			Percent: round2(usage.UsedPercent),
		}
	}
	out := make([]model.DiskMetric, 0, len(seen))
	for _, m := range seen {
		out = append(out, m)
	}
	// 非宿主模式且无物理分区（如 macOS VM 容器内仅 overlay）→ 退化为根路径
	if len(out) == 0 {
		if usage, err := disk.UsageWithContext(ctx, c.rootfs+"/"); err == nil && usage.Total > 0 {
			out = append(out, model.DiskMetric{Mount: "/", Total: usage.Total, Used: usage.Used, Percent: round2(usage.UsedPercent)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Mount < out[j].Mount })
	return out
}

func (c *Collector) sampleNet(ctx context.Context) []model.NetMetric {
	counters, err := net.IOCountersWithContext(ctx, true)
	if err != nil {
		c.log.Debug("net io counters failed", "err", err)
		return nil
	}
	now := time.Now()
	elapsed := now.Sub(c.prevNetAt).Seconds()
	out := make([]model.NetMetric, 0, len(counters))
	for _, ct := range counters {
		var rx, tx float64
		if prev, ok := c.prevNet[ct.Name]; ok && elapsed > 0 {
			if ct.BytesRecv >= prev[0] {
				rx = float64(ct.BytesRecv-prev[0]) / elapsed
			}
			if ct.BytesSent >= prev[1] {
				tx = float64(ct.BytesSent-prev[1]) / elapsed
			}
		}
		c.prevNet[ct.Name] = [2]uint64{ct.BytesRecv, ct.BytesSent}
		out = append(out, model.NetMetric{Iface: ct.Name, RxBps: round2(rx), TxBps: round2(tx)})
	}
	c.prevNetAt = now
	sort.Slice(out, func(i, j int) bool { return out[i].Iface < out[j].Iface })
	return out
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
