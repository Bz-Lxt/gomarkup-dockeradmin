package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func (s *Server) health(c *gin.Context) {
	dockerStatus := "degraded"
	if s.dm.Available() {
		dockerStatus = "connected"
	}
	respondData(c, http.StatusOK, gin.H{
		"status":           "ok",
		"version":          s.cfg.Version,
		"docker":           dockerStatus,
		"uptime_sec":       int(time.Since(s.startedAt).Seconds()),
		"collect_interval": s.cfg.CollectInterval.String(),
	})
}

func (s *Server) metricsCurrent(c *gin.Context) {
	snap, ok := s.col.Latest()
	if !ok {
		respondErr(c, http.StatusServiceUnavailable, "no_data", "采集器尚未产生数据，请稍后重试", nil)
		return
	}
	respondData(c, http.StatusOK, snap)
}

func (s *Server) metricsHistory(c *gin.Context) {
	minutes := 30
	if v := c.Query("minutes"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 1 || parsed > 720 {
			respondErr(c, http.StatusBadRequest, "invalid_param", "minutes 须为 1-720 的整数", nil)
			return
		}
		minutes = parsed
	}
	respondData(c, http.StatusOK, s.col.History(time.Duration(minutes)*time.Minute))
}
