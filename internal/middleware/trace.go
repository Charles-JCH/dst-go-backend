package middleware

import (
	"game-panel/internal/platform/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"regexp"
)

// traceIDPattern 客户端传入的 traceID 只接受字母、数字、- 和 _，最长 64 位，防止日志注入和超长内容
var traceIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Trace 链路追踪中间件
func Trace() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 读取请求头，不合法时重新生成
		traceID := c.GetHeader(logger.TraceHeaderKey)
		if !traceIDPattern.MatchString(traceID) {
			traceID = uuid.NewString()
		}

		// 将 TraceID 注入 context.Context
		ctx := logger.WithTraceID(c.Request.Context(), traceID)
		c.Request = c.Request.WithContext(ctx)

		// 设置响应头
		c.Header(logger.TraceHeaderKey, traceID)
		c.Next()
	}
}
