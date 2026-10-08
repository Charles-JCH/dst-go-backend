package v1

import (
	"game-panel/internal/model/request"
	"game-panel/internal/result"
	"game-panel/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
)

type ServerHandler struct {
	serverService service.ServerService
}

func NewServerHandler(serverService service.ServerService) *ServerHandler {
	return &ServerHandler{
		serverService: serverService,
	}
}

// Create 创建服务器
// [POST] /api/v1/servers
func (h *ServerHandler) Create(c *gin.Context) {
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

// GetByID 获取服务器详情
// [GET] /api/v1/servers/:id
func (h *ServerHandler) GetByID(c *gin.Context) {
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

// List 分页查询当前用户的服务器
// [GET] /api/v1/servers?page=1&pageSize=20
func (h *ServerHandler) List(c *gin.Context) {
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

// Update 更新服务器，密码为空时保留原密码
// [PUT] /api/v1/servers/:id
func (h *ServerHandler) Update(c *gin.Context) {
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
// [DELETE] /api/v1/servers/:id
func (h *ServerHandler) Delete(c *gin.Context) {
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

func (h *ServerHandler) getUserID(c *gin.Context) (uint64, bool) {
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

func (h *ServerHandler) getServerID(c *gin.Context) (uint64, bool) {
	serverID, err := strconv.ParseUint(c.Param("id"), 10, 63)
	if err != nil || serverID == 0 {
		result.FailWithMsg(c, 400, "服务器ID无效")
		return 0, false
	}
	return serverID, true
}
