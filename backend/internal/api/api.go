package api

import (
	"log/slog"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"dockeradmin/internal/alert"
	"dockeradmin/internal/collector"
	"dockeradmin/internal/config"
	"dockeradmin/internal/dockermon"
	"dockeradmin/internal/model"
)

type Server struct {
	cfg       *config.Config
	log       *slog.Logger
	col       *collector.Collector
	dm        *dockermon.Monitor
	store     *alert.Store
	startedAt time.Time

	receiptsMu sync.Mutex
	receipts   []model.WebhookReceipt
	receiptSeq int64
}

func NewRouter(cfg *config.Config, log *slog.Logger, col *collector.Collector, dm *dockermon.Monitor, store *alert.Store) *gin.Engine {
	if cfg.LogLevel != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	s := &Server{cfg: cfg, log: log, col: col, dm: dm, store: store, startedAt: time.Now()}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(s.accessLog())

	api := r.Group("/api")
	{
		api.GET("/health", s.health)
		api.GET("/metrics/current", s.metricsCurrent)
		api.GET("/metrics/history", s.metricsHistory)
		api.GET("/stream/metrics", s.streamMetrics)
		api.GET("/containers", s.containerList)
		api.GET("/containers/:id", s.containerDetail)
		api.GET("/containers/:id/logs", s.containerLogs)
		api.POST("/containers/:id/start", s.containerAction(dockermon.ActionStart))
		api.POST("/containers/:id/stop", s.containerAction(dockermon.ActionStop))
		api.POST("/containers/:id/restart", s.containerAction(dockermon.ActionRestart))
		api.GET("/stream/containers", s.streamContainers)
		api.GET("/alert-rules", s.ruleList)
		api.POST("/alert-rules", s.ruleCreate)
		api.PUT("/alert-rules/:id", s.ruleUpdate)
		api.DELETE("/alert-rules/:id", s.ruleDelete)
		api.GET("/alert-events", s.eventList)
		api.POST("/mock/webhook", s.mockWebhook)
		api.GET("/mock/webhook/receipts", s.mockReceipts)
	}

	s.registerStatic(r)
	return r
}
