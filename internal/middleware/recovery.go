package middleware

import (
	"game-panel/internal/result"
	"github.com/gin-gonic/gin"
	"log/slog"
	"runtime/debug"
)

// Recovery 自定义 Panic 异常恢复中间件
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 获取堆栈信息
				stack := string(debug.Stack())

				// 打印堆栈信息
				slog.ErrorContext(
					c.Request.Context(),
					"系统异常崩溃",
					slog.Any("错误信息", err),
					slog.String("堆栈信息", stack),
				)

				// 返回统一格式的系统错误信息
				result.FailWithMsg(c, 500, "服务器内部错误")
				c.Abort()
			}
		}()
		c.Next()
	}
}
