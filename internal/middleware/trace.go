package middleware

import (
	"game-panel/internal/platform/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Trace 链路追踪中间件
func Trace() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 读取请求头
		traceID := c.GetHeader(logger.TraceHeaderKey)

		if traceID == "" {
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
