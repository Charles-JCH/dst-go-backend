package handler

import (
	"game-panel/internal/result"
	"github.com/gin-gonic/gin"
	"strconv"
)

// PathID 解析路径参数
func PathID(c *gin.Context, name string) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 63)
	if err != nil || id == 0 {
		result.FailWithMsg(c, 400, name+"无效")
		return 0, false
	}
	return id, true
}
