package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/docker/docker/errdefs"
	"github.com/gin-gonic/gin"

	"dockeradmin/internal/dockermon"
)

func (s *Server) containerList(c *gin.Context) {
	list, err := s.dm.List(c.Request.Context())
	if err != nil {
		s.dockerErr(c, err)
		return
	}
	respondData(c, http.StatusOK, list)
}

func (s *Server) containerDetail(c *gin.Context) {
	d, err := s.dm.Detail(c.Request.Context(), c.Param("id"))
	if err != nil {
		s.dockerErr(c, err)
		return
	}
	respondData(c, http.StatusOK, d)
}

func (s *Server) containerLogs(c *gin.Context) {
	tail := 100
	if v := c.Query("tail"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 1 && parsed <= 1000 {
			tail = parsed
		}
	}
	lines, err := s.dm.Logs(c.Request.Context(), c.Param("id"), tail)
	if err != nil {
		s.dockerErr(c, err)
		return
	}
	respondData(c, http.StatusOK, gin.H{"lines": lines})
}

func (s *Server) containerAction(action dockermon.Action) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := strings.TrimSpace(c.Param("id"))
		if id == "" {
			respondErr(c, http.StatusBadRequest, "invalid_param", "容器 ID 不能为空", nil)
			return
		}
		if err := s.dm.Action(c.Request.Context(), id, action); err != nil {
			s.dockerErr(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func (s *Server) dockerErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, dockermon.ErrUnavailable):
		respondErr(c, http.StatusServiceUnavailable, "docker_unavailable", "Docker 不可用（降级模式）：未检测到 docker.sock", nil)
	case errdefs.IsNotFound(err):
		respondErr(c, http.StatusNotFound, "not_found", "容器不存在", nil)
	case errdefs.IsConflict(err):
		respondErr(c, http.StatusConflict, "conflict", "容器状态冲突", nil)
	case errdefs.IsNotModified(err):
		c.Status(http.StatusNoContent)
	default:
		s.log.Warn("docker api error", "err", err)
		respondErr(c, http.StatusInternalServerError, "docker_error", "Docker 操作失败", nil)
	}
}
