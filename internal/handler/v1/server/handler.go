package server

import (
	"game-panel/internal/model/request"
	"game-panel/internal/result"
	"game-panel/internal/service/server"
	"github.com/gin-gonic/gin"
	"strconv"
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
	userID, ok := h.getUserID(c)
	if !ok {
		return
	}

	var req request.CreateServerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数无效")
		return
	}

	resp, err := h.serverService.Create(c.Request.Context(), userID, &req)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}

// GetByID 查看服务器
// [GET] /api/v1/server/:id
func (h *Handler) GetByID(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		return
	}

	serverID, ok := h.getServerID(c)
	if !ok {
		return
	}

	resp, err := h.serverService.GetByID(c.Request.Context(), userID, serverID)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}

// List 分页查询
// [GET] /api/v1/server?page=1&pageSize=20
func (h *Handler) List(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		return
	}

	var req request.ListServerReq
	if err := c.ShouldBindQuery(&req); err != nil {
		result.FailWithMsg(c, 400, "分页参数无效")
		return
	}

	resp, err := h.serverService.List(c.Request.Context(), userID, &req)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}

// Update 更新服务器
// [PUT] /api/v1/server/:id
func (h *Handler) Update(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		return
	}

	serverID, ok := h.getServerID(c)
	if !ok {
		return
	}

	var req request.UpdateServerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数无效")
		return
	}

	if err := h.serverService.Update(c.Request.Context(), userID, serverID, &req); err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, nil)
}

// Delete 删除服务器
// [DELETE] /api/v1/server/:id
func (h *Handler) Delete(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		return
	}

	serverID, ok := h.getServerID(c)
	if !ok {
		return
	}

	if err := h.serverService.Delete(c.Request.Context(), userID, serverID); err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, nil)
}

func (h *Handler) getUserID(c *gin.Context) (uint64, bool) {
	value, exists := c.Get("userID")
	if !exists {
		result.FailWithMsg(c, 401, "未授权")
		return 0, false
	}

	userID, ok := value.(uint64)
	if !ok || userID == 0 {
		result.FailWithMsg(c, 401, "用户标识无效")
		return 0, false
	}
	return userID, true
}

func (h *Handler) getServerID(c *gin.Context) (uint64, bool) {
	serverID, err := strconv.ParseUint(c.Param("id"), 10, 63)
	if err != nil || serverID == 0 {
		result.FailWithMsg(c, 400, "服务器ID无效")
		return 0, false
	}
	return serverID, true
}
