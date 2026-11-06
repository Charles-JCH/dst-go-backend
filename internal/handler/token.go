package handler

import (
	"github.com/gin-gonic/gin"
	"strings"
)

// BearerToken 从 Authorization 请求头取出 Bearer 令牌，格式不对时返回 false
func BearerToken(c *gin.Context) (string, bool) {
	parts := strings.Fields(c.GetHeader("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	return parts[1], true
}
