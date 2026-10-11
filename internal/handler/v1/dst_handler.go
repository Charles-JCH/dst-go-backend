package v1

import (
	"game-panel/internal/model/request"
	"game-panel/internal/result"
	"game-panel/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
)

type DstHandler struct {
	dstService service.DstService
}

func NewDstHandler(dstService service.DstService) *DstHandler {
	return &DstHandler{dstService: dstService}
}

// Create 创建存档
// [POST] /api/v1/dst
func (h *DstHandler) Create(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		return
	}

	var req request.CreateDstReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数无效")
		return
	}

	resp, err := h.dstService.Create(c.Request.Context(), userID, &req)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}

// GetByID 查看存档
// [GET] /api/v1/dst/:id
func (h *DstHandler) GetByID(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		return
	}

	dstID, ok := h.getDstID(c)
	if !ok {
		return
	}

	resp, err := h.dstService.GetByID(c.Request.Context(), userID, dstID)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}

// List 分页查询
// [GET] /api/v1/dst?serverID=1&page=1&pageSize=20
func (h *DstHandler) List(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		return
	}

	var req request.ListDstReq
	if err := c.ShouldBindQuery(&req); err != nil {
		result.FailWithMsg(c, 400, "查询参数无效")
		return
	}

	resp, err := h.dstService.List(c.Request.Context(), userID, &req)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}

// Update 更新存档
// [PUT] /api/v1/dst/:id
func (h *DstHandler) Update(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		return
	}

	dstID, ok := h.getDstID(c)
	if !ok {
		return
	}

	var req request.UpdateDstReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数无效")
		return
	}

	if err := h.dstService.Update(c.Request.Context(), userID, dstID, &req); err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, nil)
}

// Delete 删除存档
// [DELETE] /api/v1/dst/:id
func (h *DstHandler) Delete(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		return
	}

	dstID, ok := h.getDstID(c)
	if !ok {
		return
	}

	if err := h.dstService.Delete(c.Request.Context(), userID, dstID); err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, nil)
}

func (h *DstHandler) getUserID(c *gin.Context) (uint64, bool) {
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

func (h *DstHandler) getDstID(c *gin.Context) (uint64, bool) {
	dstID, err := strconv.ParseUint(c.Param("id"), 10, 63)
	if err != nil || dstID == 0 {
		result.FailWithMsg(c, 400, "存档ID无效")
		return 0, false
	}
	return dstID, true
}
