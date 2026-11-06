package gameenv

import (
	"game-panel/internal/result"
	"game-panel/internal/service/gameenv"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	gameEnvService gameenv.Service
}

func NewHandler(gameEnvService gameenv.Service) *Handler {
	return &Handler{gameEnvService: gameEnvService}
}

// List 查询可部署的游戏环境
// [GET] /api/v1/game-env
func (h *Handler) List(c *gin.Context) {
	list, err := h.gameEnvService.List(c.Request.Context())
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, list)
}
