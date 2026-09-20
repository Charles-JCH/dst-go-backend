package bootstrap

import (
	"fmt"
	"game-panel/internal/config"
	"game-panel/internal/platform/logger"
	"log/slog"
)

func InitLogger(cfg config.LogConfig, env string) (*slog.Logger, error) {
	addSource := env != "prod"

	l, err := logger.NewLogger(&logger.Config{
		Level:      cfg.Level,
		Format:     cfg.Format,
		ToConsole:  cfg.ToConsole,
		ToFile:     cfg.ToFile,
		Dir:        cfg.Dir,
		Filename:   cfg.Filename,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
		AddSource:  addSource,
	})
	if err != nil {
		return nil, fmt.Errorf("初始化日志失败: %w", err)
	}

	slog.SetDefault(l)

	return l, nil
}
