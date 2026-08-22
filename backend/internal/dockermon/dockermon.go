package dockermon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"golang.org/x/sync/errgroup"

	"dockeradmin/internal/model"
)

// Monitor 封装 Docker Engine API；socket 不可用时 cli 为 nil（降级模式）。
type Monitor struct {
	cli *client.Client
	log *slog.Logger

	mu      sync.RWMutex
	latest  []model.ContainerInfo
	subs    map[chan []model.ContainerInfo]struct{}
	sockURL string
}

// socket 探测顺序：DOCKER_HOST（SDK 默认）→ Linux 惯例 → macOS Docker Desktop 挂载点
var socketCandidates = []string{
	"/var/run/docker.sock",
	"/run/host-docker.sock",
}

func NewMonitor(ctx context.Context, log *slog.Logger) *Monitor {
	m := &Monitor{log: log, subs: make(map[chan []model.ContainerInfo]struct{})}

	if os.Getenv("DOCKER_HOST") != "" {
		if cli, err := newClient(ctx, ""); err == nil {
			m.cli = cli
			m.sockURL = "DOCKER_HOST"
			return m
		}
		log.Warn("DOCKER_HOST set but ping failed, falling back to socket candidates")
	}
	for _, path := range socketCandidates {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		cli, err := newClient(ctx, "unix://"+path)
		if err != nil {
			log.Debug("socket candidate failed", "path", path, "err", err)
			continue
		}
		m.cli = cli
		m.sockURL = path
		log.Info("docker connected", "socket", path)
		return m
	}
	log.Warn("no docker socket available, running in DEGRADED mode (system metrics only)")
	return m
}

func newClient(ctx context.Context, host string) (*client.Client, error) {
	opts := []client.Opt{client.WithAPIVersionNegotiation()}
	if host != "" {
		opts = append(opts, client.WithHost(host))
	} else {
		opts = append(opts, client.FromEnv)
	}
	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := cli.Ping(pingCtx); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return cli, nil
}

func (m *Monitor) Available() bool { return m.cli != nil }

func (m *Monitor) SocketURL() string { return m.sockURL }

// Run 周期采集容器列表并广播（与系统指标同节奏）。
func (m *Monitor) Run(ctx context.Context, interval time.Duration) {
	if !m.Available() {
		return
	}
	m.refresh(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.refresh(ctx)
		}
	}
}

func (m *Monitor) Subscribe() chan []model.ContainerInfo {
	ch := make(chan []model.ContainerInfo, 4)
	m.mu.Lock()
	m.subs[ch] = struct{}{}
	m.mu.Unlock()
	return ch
}

func (m *Monitor) Unsubscribe(ch chan []model.ContainerInfo) {
	m.mu.Lock()
	delete(m.subs, ch)
	m.mu.Unlock()
}

func (m *Monitor) Latest() ([]model.ContainerInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.latest, m.latest != nil
}

func (m *Monitor) publish(list []model.ContainerInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latest = list
	for ch := range m.subs {
		select {
		case ch <- list:
		default:
		}
	}
}

func (m *Monitor) refresh(ctx context.Context) {
	list, err := m.List(ctx)
	if err != nil {
		m.log.Debug("container list refresh failed", "err", err)
		return
	}
	m.publish(list)
}

func (m *Monitor) List(ctx context.Context) ([]model.ContainerInfo, error) {
	if !m.Available() {
		return nil, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	summaries, err := m.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	out := make([]model.ContainerInfo, len(summaries))
	for i, s := range summaries {
		out[i] = summaryToInfo(s)
	}

	// 并发采集运行中容器的 stats（限流 8，防大量容器时打爆 daemon）
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(8)
	for i := range out {
		if out[i].State != "running" {
			continue
		}
		g.Go(func() error {
			stats, err := m.oneShotStats(gctx, out[i].ID)
			if err != nil {
				m.log.Debug("container stats failed", "id", out[i].ID[:12], "err", err)
				return nil // 单容器失败不拖垮整列
			}
			out[i].CPUPercent = stats.cpuPercent
			out[i].MemUsed = stats.memUsed
			out[i].MemLimit = stats.memLimit
			out[i].MemPercent = stats.memPercent
			out[i].NetRx = stats.netRx
			out[i].NetTx = stats.netTx
			return nil
		})
	}
	_ = g.Wait()

	sort.Slice(out, func(i, j int) bool {
		if (out[i].State == "running") != (out[j].State == "running") {
			return out[i].State == "running"
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func summaryToInfo(s container.Summary) model.ContainerInfo {
	name := ""
	if len(s.Names) > 0 {
		name = strings.TrimPrefix(s.Names[0], "/")
	}
	created := time.Unix(s.Created, 0)
	var uptime float64
	if s.State == "running" {
		uptime = time.Since(created).Seconds()
	}
	return model.ContainerInfo{
		ID:        s.ID,
		Name:      name,
		Image:     s.Image,
		State:     s.State,
		Status:    s.Status,
		CreatedAt: created,
		UptimeSec: uptime,
	}
}

type containerStats struct {
	cpuPercent float64
	memUsed    uint64
	memLimit   uint64
	memPercent float64
	netRx      uint64
	netTx      uint64
}

func (m *Monitor) oneShotStats(ctx context.Context, id string) (containerStats, error) {
	var result containerStats
	sctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	resp, err := m.cli.ContainerStatsOneShot(sctx, id)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	var st container.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return result, fmt.Errorf("decode stats: %w", err)
	}

	// CPU% = Δcpu / Δsystem × online_cpus × 100（Contract Gate 已验证字段；首帧 precpu 为零值时守卫）
	cpuDelta := float64(st.CPUStats.CPUUsage.TotalUsage) - float64(st.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(st.CPUStats.SystemUsage) - float64(st.PreCPUStats.SystemUsage)
	if cpuDelta > 0 && sysDelta > 0 {
		online := float64(st.CPUStats.OnlineCPUs)
		if online == 0 {
			online = float64(len(st.CPUStats.CPUUsage.PercpuUsage))
		}
		if online > 0 {
			result.cpuPercent = round2(cpuDelta / sysDelta * online * 100)
		}
	}
	result.memUsed = st.MemoryStats.Usage
	result.memLimit = st.MemoryStats.Limit
	if st.MemoryStats.Limit > 0 {
		result.memPercent = round2(float64(st.MemoryStats.Usage) / float64(st.MemoryStats.Limit) * 100)
	}
	for _, n := range st.Networks {
		result.netRx += n.RxBytes
		result.netTx += n.TxBytes
	}
	return result, nil
}

func (m *Monitor) Detail(ctx context.Context, id string) (model.ContainerDetail, error) {
	if !m.Available() {
		return model.ContainerDetail{}, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	ins, err := m.cli.ContainerInspect(ctx, id)
	if err != nil {
		return model.ContainerDetail{}, err
	}

	image := ""
	if ins.Config != nil {
		image = ins.Config.Image
	}
	info := model.ContainerInfo{
		ID:    ins.ID,
		Name:  strings.TrimPrefix(ins.Name, "/"),
		Image: image,
	}
	if created, err := time.Parse(time.RFC3339Nano, ins.Created); err == nil {
		info.CreatedAt = created
	}
	if ins.State != nil {
		info.State = ins.State.Status
		info.Status = ins.State.Status
		if ins.State.Running {
			if started, err := time.Parse(time.RFC3339Nano, ins.State.StartedAt); err == nil {
				info.UptimeSec = time.Since(started).Seconds()
			}
			if stats, err := m.oneShotStats(ctx, id); err == nil {
				info.CPUPercent, info.MemUsed, info.MemLimit, info.MemPercent = stats.cpuPercent, stats.memUsed, stats.memLimit, stats.memPercent
				info.NetRx, info.NetTx = stats.netRx, stats.netTx
			}
		}
	}

	d := model.ContainerDetail{ContainerInfo: info}
	// 端口：去重 IPv4/IPv6 重复项
	seenPorts := make(map[string]bool)
	if ins.NetworkSettings != nil {
		for portProto, bindings := range ins.NetworkSettings.Ports {
			for _, b := range bindings {
				s := fmt.Sprintf("%s:%s/%s", b.HostPort, portProto.Port(), portProto.Proto())
				if !seenPorts[s] {
					seenPorts[s] = true
					d.Ports = append(d.Ports, s)
				}
			}
		}
	}
	sort.Strings(d.Ports)
	for _, mp := range ins.Mounts {
		mode := "rw"
		if !mp.RW {
			mode = "ro"
		}
		d.Mounts = append(d.Mounts, fmt.Sprintf("%s → %s (%s)", mp.Source, mp.Destination, mode))
	}
	// 环境变量预览：敏感键脱敏（server 记忆：敏感字段禁止明文）
	if ins.Config != nil {
		for i, e := range ins.Config.Env {
			if i >= 5 {
				break
			}
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 && isSensitiveKey(parts[0]) {
				e = parts[0] + "=****"
			}
			d.EnvPreview = append(d.EnvPreview, e)
		}
	}
	return d, nil
}

func isSensitiveKey(k string) bool {
	k = strings.ToUpper(k)
	for _, kw := range []string{"KEY", "TOKEN", "PASS", "SECRET", "PWD"} {
		if strings.Contains(k, kw) {
			return true
		}
	}
	return false
}

// Logs 返回最近 tail 行日志（stdcopy 解复用 8 字节帧头，Contract Gate 已验证）。
func (m *Monitor) Logs(ctx context.Context, id string, tail int) (string, error) {
	if !m.Available() {
		return "", ErrUnavailable
	}
	if tail < 1 || tail > 1000 {
		tail = 100
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()

	reader, err := m.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", tail),
	})
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var sb strings.Builder
	sb.Grow(64 * 1024)
	if _, err := stdcopy.StdCopy(&sb, &sb, reader); err != nil && err != io.EOF {
		return "", fmt.Errorf("demux logs: %w", err)
	}
	return sb.String(), nil
}

type Action string

const (
	ActionStart   Action = "start"
	ActionStop    Action = "stop"
	ActionRestart Action = "restart"
)

func (m *Monitor) Action(ctx context.Context, id string, action Action) error {
	if !m.Available() {
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	timeout := 10
	var err error
	switch action {
	case ActionStart:
		err = m.cli.ContainerStart(ctx, id, container.StartOptions{})
	case ActionStop:
		err = m.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout})
	case ActionRestart:
		err = m.cli.ContainerRestart(ctx, id, container.StopOptions{Timeout: &timeout})
	default:
		return fmt.Errorf("unknown action %q", action)
	}
	return err
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
