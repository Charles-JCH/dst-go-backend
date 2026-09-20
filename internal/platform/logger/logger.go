package logger

import (
	"fmt"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Level      string
	Format     string
	ToConsole  bool
	ToFile     bool
	Dir        string
	Filename   string
	MaxSize    int
	MaxBackups int
	MaxAge     int
	Compress   bool
	AddSource  bool
}

func NewLogger(cfg *Config) (*slog.Logger, error) {
	// 解析日志级别
	level := parseLevel(cfg.Level)

	// 配置日志输出目标
	var writers []io.Writer

	// 控制台输出
	if cfg.ToConsole {
		writers = append(writers, os.Stdout)
	}

	// 文件输出
	if cfg.ToFile && cfg.Filename != "" {
		if err := os.MkdirAll(cfg.Dir, os.ModePerm); err != nil {
			return nil, fmt.Errorf("创建日志目录失败 [%s]: %w", cfg.Dir, err)
		}
		logFilePath := filepath.Join(cfg.Dir, cfg.Filename)
		// 配置 lumberjack 日志切割
		fileWriter := &lumberjack.Logger{
			Filename:   logFilePath,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
		}
		writers = append(writers, fileWriter)
	}

	var writer io.Writer
	if len(writers) == 0 {
		writer = io.Discard
	} else {
		writer = io.MultiWriter(writers...)
	}

	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					return slog.String(slog.TimeKey, t.Format("2006-01-02 15:04:05.000"))
				}
			}
			return a
		},
	}

	// 配置格式化
	var baseHandler slog.Handler
	if strings.ToLower(cfg.Format) == "json" {
		baseHandler = slog.NewJSONHandler(writer, opts)
	} else {
		baseHandler = slog.NewTextHandler(writer, opts)
	}

	// 包装 Handler
	handler := NewTraceHandler(baseHandler)
	logger := slog.New(handler)

	return logger, nil
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
