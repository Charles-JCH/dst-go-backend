package middleware

import (
	"game-panel/internal/handler"
	"game-panel/internal/result"
	"game-panel/internal/service/auth"
	"github.com/gin-gonic/gin"
)

// Auth JWT 鉴权中间件
func Auth(authService auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := handler.BearerToken(c)
		if !ok {
			result.FailWithMsg(c, 401, "登录凭证缺失或格式错误")
			c.Abort()
			return
		}

		identity, err := authService.Authenticate(c.Request.Context(), token)
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
