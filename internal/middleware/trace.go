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
		traceId := c.GetHeader(logger.TraceHeaderKey)

		if traceId == "" {
			traceId = uuid.NewString()
		}

		// 将 TraceId 注入 context.Context
		ctx := logger.WithTraceId(c.Request.Context(), traceId)
		c.Request = c.Request.WithContext(ctx)

		// 设置响应头
		c.Header(logger.TraceHeaderKey, traceId)
		c.Next()
	}
}
