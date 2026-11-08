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
	LogLevel                logger.LogLevel
	SlowThreshold           time.Duration
	IsRecordNotFoundIgnored bool
}

func NewGormLogger(logLevel string, slowThreshold time.Duration, isRecordNotFoundIgnored bool) *GormLogger {
	return &GormLogger{
		LogLevel:                parseGormLogLevel(logLevel),
		SlowThreshold:           slowThreshold,
		IsRecordNotFoundIgnored: isRecordNotFoundIgnored,
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
	case err != nil && l.LogLevel >= logger.Error && !l.isExpected(err):
		level = slog.LevelError
		message = "sql 执行失败"
	case l.SlowThreshold > 0 && elapsed > l.SlowThreshold && l.LogLevel >= logger.Warn:
		level = slog.LevelWarn
		message = "sql 慢查询"
	case l.LogLevel >= logger.Info:
		level = slog.LevelInfo
		message = "sql 执行"
	default:
		return
	}

	if !slog.Default().Enabled(ctx, level) {
		return
	}

	sql, rows := fc()

	attrs := []slog.Attr{
		slog.Float64("durationMs", float64(elapsed)/float64(time.Millisecond)),
		slog.Int64("rows", rows),
		slog.String("sql", sql),
	}

	if level == slog.LevelError {
		attrs = append(attrs, slog.Any("error", err))
	}
	if level == slog.LevelWarn {
		attrs = append(attrs, slog.Float64("slowThresholdMs", float64(l.SlowThreshold)/float64(time.Millisecond)))
	}

	slog.LogAttrs(ctx, level, message, attrs...)
}

func (l *GormLogger) ParamsFilter(_ context.Context, sql string, _ ...interface{}) (string, []interface{}) {
	return sql, nil
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

// isExpected 业务上预期的错误不记录：唯一约束冲突由 service 转成 409，查无记录按配置忽略
func (l *GormLogger) isExpected(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	return l.IsRecordNotFoundIgnored && errors.Is(err, gorm.ErrRecordNotFound)
}
