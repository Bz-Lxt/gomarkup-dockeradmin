package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port            int
	CollectInterval time.Duration
	RetentionWindow time.Duration
	LogLevel        string
	DataDir         string
	Version         string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:            8080,
		CollectInterval: 2 * time.Second,
		RetentionWindow: time.Hour,
		LogLevel:        "info",
		DataDir:         "./data",
		Version:         "1.0.0",
	}

	if v := os.Getenv("PORT"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil || p < 1 || p > 65535 {
			return nil, fmt.Errorf("invalid PORT %q: must be 1-65535", v)
		}
		cfg.Port = p
	}
	if v := os.Getenv("COLLECT_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < time.Second || d > 5*time.Minute {
			return nil, fmt.Errorf("invalid COLLECT_INTERVAL %q: must be 1s-5m", v)
		}
		cfg.CollectInterval = d
	}
	if v := os.Getenv("RETENTION_WINDOW"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d < time.Minute || d > 24*time.Hour {
			return nil, fmt.Errorf("invalid RETENTION_WINDOW %q: must be 1m-24h", v)
		}
		cfg.RetentionWindow = d
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		switch v {
		case "debug", "info", "warn", "error":
			cfg.LogLevel = v
		default:
			return nil, fmt.Errorf("invalid LOG_LEVEL %q: must be debug|info|warn|error", v)
		}
	}
	if v := os.Getenv("DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	return cfg, nil
}
