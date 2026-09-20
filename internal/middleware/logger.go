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
			slog.Int("状态码", status),
			slog.Duration("耗时", cost),
			slog.String("客户端IP", clientIP),
			slog.String("请求方法", method),
			slog.String("请求路径", path),
			slog.String("查询参数", rawQuery),
		}

		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("错误", c.Errors.String()))
		}

		// 根据 HTTP 状态码分级输出日志
		switch {
		case status >= http.StatusInternalServerError:
			slog.LogAttrs(ctx, slog.LevelError, "HTTP Request", attrs...)
		case status >= http.StatusBadRequest:
			slog.LogAttrs(ctx, slog.LevelWarn, "HTTP Request", attrs...)
		default:
			slog.LogAttrs(ctx, slog.LevelInfo, "HTTP Request", attrs...)
		}
	}
}
