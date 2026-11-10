package dst

import (
	"game-panel/internal/handler"
	"game-panel/internal/model/request"
	"game-panel/internal/result"
	"game-panel/internal/service/dst"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	dstService dst.Service
}

func NewHandler(dstService dst.Service) *Handler {
	return &Handler{dstService: dstService}
}

// Init 新增存档第一步：初始化默认存档，返回初始化任务
// [POST] /api/v1/dst/init
func (h *Handler) Init(c *gin.Context) {
	var req request.InitDstReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数无效")
		return
	}

	resp, err := h.dstService.Init(c.Request.Context(), c.GetUint64("userID"), &req)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}

// Create 新增存档第二步：写入配置文件，返回创建任务；任务成功后存档才保存
// [POST] /api/v1/dst
func (h *Handler) Create(c *gin.Context) {
	var req request.CreateDstReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数无效")
		return
	}

	resp, err := h.dstService.Create(c.Request.Context(), c.GetUint64("userID"), &req)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}

// GetByID 查看存档
// [GET] /api/v1/dst/:id
func (h *Handler) GetByID(c *gin.Context) {
	dstID, ok := handler.PathID(c, "存档ID")
	if !ok {
		return
	}

	resp, err := h.dstService.GetByID(c.Request.Context(), c.GetUint64("userID"), dstID)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}

// List 分页查询
// [GET] /api/v1/dst?serverID=1&page=1&pageSize=20（serverID 可不传，查询全部服务器）
func (h *Handler) List(c *gin.Context) {
	var req request.ListDstReq
	if err := c.ShouldBindQuery(&req); err != nil {
		result.FailWithMsg(c, 400, "查询参数无效")
		return
	}

	resp, err := h.dstService.List(c.Request.Context(), c.GetUint64("userID"), &req)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}

// Update 更新存档
// [PUT] /api/v1/dst/:id
func (h *Handler) Update(c *gin.Context) {
	dstID, ok := handler.PathID(c, "存档ID")
	if !ok {
		return
	}

	var req request.UpdateDstReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数无效")
		return
	}

	if err := h.dstService.Update(c.Request.Context(), c.GetUint64("userID"), dstID, &req); err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, nil)
}

// Delete 删除存档
// [DELETE] /api/v1/dst/:id
func (h *Handler) Delete(c *gin.Context) {
	dstID, ok := handler.PathID(c, "存档ID")
	if !ok {
		return
	}

	if err := h.dstService.Delete(c.Request.Context(), c.GetUint64("userID"), dstID); err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, nil)
}
