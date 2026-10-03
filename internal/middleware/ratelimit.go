package middleware

import (
	"game-panel/internal/platform/redis"
	"game-panel/internal/result"
	"github.com/gin-gonic/gin"
	"log/slog"
	"time"
)

// RateLimit 全局限流
func RateLimit(limiter redis.Limiter, limit int64, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		const key = "ratelimit:global"
		allowed, err := limiter.Allow(c.Request.Context(), key, limit, window)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "限流检查失败", "error", err)
			result.FailWithMsg(c, 503, "服务器繁忙")
			c.Abort()
			return
		}

		if !allowed {
			result.FailWithMsg(c, 503, "服务器繁忙")
			c.Abort()
			return
		}

		c.Next()
	}
}
