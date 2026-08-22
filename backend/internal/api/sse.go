package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"dockeradmin/internal/model"
)

func sseHeaders(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
}

func streamJSON[T any](c *gin.Context, ch <-chan T, initial *T) {
	sseHeaders(c)
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	write := func(v T) bool {
		data, err := json.Marshal(v)
		if err != nil {
			return true
		}
		if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", data); err != nil {
			return false
		}
		c.Writer.Flush()
		return true
	}

	if initial != nil && !write(*initial) {
		return
	}
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case v, ok := <-ch:
			if !ok || !write(v) {
				return
			}
		case <-heartbeat.C:
			if _, err := io.WriteString(c.Writer, ": ping\n\n"); err != nil {
				return
			}
			c.Writer.Flush()
		}
	}
}

func (s *Server) streamMetrics(c *gin.Context) {
	ch := s.col.Subscribe()
	defer s.col.Unsubscribe(ch)
	var initial *model.MetricSnapshot
	if snap, ok := s.col.Latest(); ok {
		initial = &snap
	}
	streamJSON(c, ch, initial)
}

func (s *Server) streamContainers(c *gin.Context) {
	if !s.dm.Available() {
		respondErr(c, http.StatusServiceUnavailable, "docker_unavailable", "Docker 不可用（降级模式）", nil)
		return
	}
	ch := s.dm.Subscribe()
	defer s.dm.Unsubscribe(ch)
	var initial *[]model.ContainerInfo
	if list, ok := s.dm.Latest(); ok {
		initial = &list
	}
	streamJSON(c, ch, initial)
}
