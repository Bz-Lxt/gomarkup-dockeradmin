package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"dockeradmin/internal/alert"
	"dockeradmin/internal/api"
	"dockeradmin/internal/collector"
	"dockeradmin/internal/config"
	"dockeradmin/internal/dockermon"
	"dockeradmin/internal/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}
	log := logger.New(cfg.LogLevel)

	// 宿主路径探测（须在首次采集前设置 HOST_PROC）
	rootfs := collector.DetectHostPaths(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	col := collector.New(cfg.CollectInterval, cfg.RetentionWindow, rootfs, log)
	go col.Run(ctx)

	dm := dockermon.NewMonitor(ctx, log)
	go dm.Run(ctx, cfg.CollectInterval)

	store, err := alert.NewStore(filepath.Join(cfg.DataDir, "alerts.json"), log)
	if err != nil {
		log.Error("alert store init failed", "err", err)
		os.Exit(1)
	}
	engine := alert.NewEngine(store, alert.NewNotifier(log), alert.Sources{
		LatestSystem:     col.Latest,
		LatestContainers: dm.Latest,
	}, log)
	go engine.Run(ctx, cfg.CollectInterval)

	router := api.NewRouter(cfg, log, col, dm, store)
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("dockeradmin started",
			"port", cfg.Port,
			"interval", cfg.CollectInterval,
			"retention", cfg.RetentionWindow,
			"docker", dm.Available(),
			"version", cfg.Version,
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "err", err)
	}
	log.Info("bye")
}
