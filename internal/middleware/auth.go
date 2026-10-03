package middleware

import (
	"game-panel/internal/result"
	"game-panel/internal/service"
	"github.com/gin-gonic/gin"
	"strings"
)

// Auth JWT 鉴权中间件
func Auth(authService service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			result.FailWithMsg(c, 401, "登录凭证缺失")
			c.Abort()
			return
		}

		// 按 Bearer 前缀切割
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			result.FailWithMsg(c, 401, "登录凭证格式错误")
			c.Abort()
			return
		}

		identity, err := authService.Authenticate(c.Request.Context(), parts[1])
		if err != nil {
			result.Fail(c, err)
			c.Abort()
			return
		}

		// 注入 Gin Context
		c.Set("userID", identity.UserID)
		c.Set("username", identity.Username)
		c.Set("role", identity.Role)

		c.Next()
	}
}
