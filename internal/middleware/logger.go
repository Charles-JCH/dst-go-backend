package middleware

import (
	"github.com/gin-gonic/gin"
	"log/slog"
	"net/http"
	"time"
)

// Logger HTTP 请求访问日志中间件
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		path := c.Request.URL.Path
		rawQuery := c.Request.URL.RawQuery

		c.Next()

		// 组装请求参数
		cost := time.Since(startTime)
		status := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()
		ctx := c.Request.Context()

		// 构造日志输出字段
		attrs := []slog.Attr{
			slog.Int("status", status),
			slog.Float64("durationMs", float64(cost)/float64(time.Millisecond)),
			slog.String("clientIP", clientIP),
			slog.String("method", method),
			slog.String("path", path),
			slog.String("query", rawQuery),
		}

		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("error", c.Errors.String()))
		}

		// 根据 HTTP 状态码分级输出日志
		switch {
		case status >= http.StatusInternalServerError:
			slog.LogAttrs(ctx, slog.LevelError, "http 请求", attrs...)
		case status >= http.StatusBadRequest:
			slog.LogAttrs(ctx, slog.LevelWarn, "http 请求", attrs...)
		default:
			slog.LogAttrs(ctx, slog.LevelInfo, "http 请求", attrs...)
		}
	}
}
