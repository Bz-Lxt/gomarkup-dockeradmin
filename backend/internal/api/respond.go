package api

import (
	"time"

	"github.com/gin-gonic/gin"

	"dockeradmin/internal/model"
)

func respondData(c *gin.Context, status int, v any) {
	c.JSON(status, gin.H{"data": v})
}

func respondErr(c *gin.Context, status int, code, msg string, details []model.FieldError) {
	body := gin.H{"error": gin.H{"code": code, "message": msg}}
	if len(details) > 0 {
		body["error"].(gin.H)["details"] = details
	}
	c.JSON(status, body)
}

func (s *Server) accessLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		s.log.Debug("http", "method", c.Request.Method, "path", c.Request.URL.Path,
			"status", c.Writer.Status(), "dur", time.Since(start).Round(time.Microsecond))
	}
}
