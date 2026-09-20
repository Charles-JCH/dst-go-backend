package logger

import (
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log/slog"
	"strings"
	"time"
)

type GormLogger struct {
	LogLevel                  logger.LogLevel
	SlowThreshold             time.Duration
	IgnoreRecordNotFoundError bool
}

func NewGormLogger(logLevel string, slowThreshold time.Duration, ignoreRecordNotFound bool) *GormLogger {
	return &GormLogger{
		LogLevel:                  parseGormLogLevel(logLevel),
		SlowThreshold:             slowThreshold,
		IgnoreRecordNotFoundError: ignoreRecordNotFound,
	}
}

func (l *GormLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

func (l *GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		slog.InfoContext(ctx, fmt.Sprintf(msg, data...))
	}
}

func (l *GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		slog.WarnContext(ctx, fmt.Sprintf(msg, data...))
	}
}

func (l *GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		slog.ErrorContext(ctx, fmt.Sprintf(msg, data...))
	}
}

func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)

	var level slog.Level
	var message string

	switch {
	case err != nil && l.LogLevel >= logger.Error && (!l.IgnoreRecordNotFoundError || !errors.Is(err, gorm.ErrRecordNotFound)):
		level = slog.LevelError
		message = "SQL 执行异常"
	case l.SlowThreshold > 0 && elapsed > l.SlowThreshold && l.LogLevel >= logger.Warn:
		level = slog.LevelWarn
		message = "慢查询"
	case l.LogLevel >= logger.Info:
		level = slog.LevelInfo
		message = "执行 SQL"
	default:
		return
	}

	if !slog.Default().Enabled(ctx, level) {
		return
	}

	sql, rows := fc()

	attrs := []slog.Attr{
		slog.Duration("耗时", elapsed),
		slog.Int64("rows", rows),
		slog.String("sql", sql),
	}

	if level == slog.LevelError {
		attrs = append(attrs, slog.Any("错误", err))
	}
	if level == slog.LevelWarn {
		attrs = append(attrs, slog.Duration("慢查询", l.SlowThreshold))
	}

	slog.LogAttrs(ctx, level, message, attrs...)
}

func parseGormLogLevel(level string) logger.LogLevel {
	switch strings.ToLower(level) {
	case "silent":
		return logger.Silent
	case "error":
		return logger.Error
	case "warn", "warning":
		return logger.Warn
	case "info":
		return logger.Info
	default:
		return logger.Info
	}
}
