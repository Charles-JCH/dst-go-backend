package server

import (
	"game-panel/internal/handler"
	"game-panel/internal/model/request"
	"game-panel/internal/result"
	"game-panel/internal/service/server"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	serverService server.Service
}

func NewHandler(serverService server.Service) *Handler {
	return &Handler{serverService: serverService}
}

// Create 创建服务器
// [POST] /api/v1/server
func (h *Handler) Create(c *gin.Context) {
	var req request.CreateServerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数无效")
		return
	}

	resp, err := h.serverService.Create(c.Request.Context(), c.GetUint64("userID"), &req)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}

// GetByID 查看服务器
// [GET] /api/v1/server/:id
func (h *Handler) GetByID(c *gin.Context) {
	serverID, ok := handler.PathID(c, "服务器ID")
	if !ok {
		return
	}

	resp, err := h.serverService.GetByID(c.Request.Context(), c.GetUint64("userID"), serverID)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}

// List 分页查询
// [GET] /api/v1/server?page=1&pageSize=20
func (h *Handler) List(c *gin.Context) {
	var req request.ListServerReq
	if err := c.ShouldBindQuery(&req); err != nil {
		result.FailWithMsg(c, 400, "分页参数无效")
		return
	}

	resp, err := h.serverService.List(c.Request.Context(), c.GetUint64("userID"), &req)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}

// Update 更新服务器
// [PUT] /api/v1/server/:id
func (h *Handler) Update(c *gin.Context) {
	serverID, ok := handler.PathID(c, "服务器ID")
	if !ok {
		return
	}

	var req request.UpdateServerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数无效")
		return
	}

	if err := h.serverService.Update(c.Request.Context(), c.GetUint64("userID"), serverID, &req); err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, nil)
}

// Delete 删除服务器
// [DELETE] /api/v1/server/:id
func (h *Handler) Delete(c *gin.Context) {
	serverID, ok := handler.PathID(c, "服务器ID")
	if !ok {
		return
	}

	if err := h.serverService.Delete(c.Request.Context(), c.GetUint64("userID"), serverID); err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, nil)
}

// InstallAgent 重新安装 Agent
// [POST] /api/v1/server/:id/agent/install
func (h *Handler) InstallAgent(c *gin.Context) {
	serverID, ok := handler.PathID(c, "服务器ID")
	if !ok {
		return
	}

	resp, err := h.serverService.InstallAgent(c.Request.Context(), c.GetUint64("userID"), serverID)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}
