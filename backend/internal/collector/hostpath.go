package collector

import (
	"log/slog"
	"os"
)

// DetectHostPaths 探测宿主挂载并配置 gopsutil 环境（须在首次采集前调用）。
// 返回 rootfs 前缀（无宿主根挂载时为 ""）。
func DetectHostPaths(log *slog.Logger) string {
	if st, err := os.Stat("/host/proc/stat"); err == nil && !st.IsDir() {
		if err := os.Setenv("HOST_PROC", "/host/proc"); err == nil {
			log.Info("host /proc detected, collecting HOST system metrics")
		}
	}
	if st, err := os.Stat("/rootfs"); err == nil && st.IsDir() {
		log.Info("host rootfs detected at /rootfs, collecting HOST disk metrics")
		return "/rootfs"
	}
	log.Info("no host mounts detected, collecting container/VM metrics (expected on macOS/Windows)")
	return ""
}
