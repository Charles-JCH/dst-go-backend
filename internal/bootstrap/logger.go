package bootstrap

import (
	"fmt"
	"game-panel/internal/config"
	"game-panel/internal/platform/logger"
	"log/slog"
)

func InitLogger(cfg config.LogConfig, env string) (*slog.Logger, error) {
	isSourceEnabled := env != "prod"

	l, err := logger.NewLogger(&logger.Config{
		Level:             cfg.Level,
		Format:            cfg.Format,
		IsConsoleEnabled:  cfg.IsConsoleEnabled,
		IsFileEnabled:     cfg.IsFileEnabled,
		Dir:               cfg.Dir,
		Filename:          cfg.Filename,
		MaxSize:           cfg.MaxSize,
		MaxBackups:        cfg.MaxBackups,
		MaxAge:            cfg.MaxAge,
		IsCompressEnabled: cfg.IsCompressEnabled,
		IsSourceEnabled:   isSourceEnabled,
	})
	if err != nil {
		return nil, fmt.Errorf("日志初始化失败: %w", err)
	}

	slog.SetDefault(l)

	return l, nil
}
