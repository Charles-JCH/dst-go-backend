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

// Update 更新存档：返回配置写入任务，写入成功后存档才更新
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

	resp, err := h.dstService.Update(c.Request.Context(), c.GetUint64("userID"), dstID, &req)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}

// WorldOptions 地面和洞穴可修改的世界设置目录
// [GET] /api/v1/dst/world-options
func (h *Handler) WorldOptions(c *gin.Context) {
	result.Success(c, h.dstService.WorldOptions(c.Request.Context()))
}

// SaveWorld 保存地面或洞穴的世界设置，只存平台，返回存档
// [PUT] /api/v1/dst/:id/world
func (h *Handler) SaveWorld(c *gin.Context) {
	dstID, ok := handler.PathID(c, "存档ID")
	if !ok {
		return
	}

	var req request.SaveDstWorldReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数无效")
		return
	}

	resp, err := h.dstService.SaveWorld(c.Request.Context(), c.GetUint64("userID"), dstID, &req)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}

// ApplyWorld 把已保存的世界设置写入服务器，返回配置写入任务
// [POST] /api/v1/dst/:id/world/apply
func (h *Handler) ApplyWorld(c *gin.Context) {
	dstID, ok := handler.PathID(c, "存档ID")
	if !ok {
		return
	}

	resp, err := h.dstService.ApplyWorld(c.Request.Context(), c.GetUint64("userID"), dstID)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}

// Start 启动存档，返回启动任务
// [POST] /api/v1/dst/:id/start
func (h *Handler) Start(c *gin.Context) {
	dstID, ok := handler.PathID(c, "存档ID")
	if !ok {
		return
	}

	resp, err := h.dstService.Start(c.Request.Context(), c.GetUint64("userID"), dstID)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}

// Stop 停止存档，返回停止任务
// [POST] /api/v1/dst/:id/stop
func (h *Handler) Stop(c *gin.Context) {
	dstID, ok := handler.PathID(c, "存档ID")
	if !ok {
		return
	}

	resp, err := h.dstService.Stop(c.Request.Context(), c.GetUint64("userID"), dstID)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}

// Status 查看存档运行状态，返回任务；任务成功后存档状态按实际更新
// [POST] /api/v1/dst/:id/status
func (h *Handler) Status(c *gin.Context) {
	dstID, ok := handler.PathID(c, "存档ID")
	if !ok {
		return
	}

	resp, err := h.dstService.Status(c.Request.Context(), c.GetUint64("userID"), dstID)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}

// Delete 删除存档：返回删除任务，清理成功后记录才删除；recordOnly=true 时只删除记录，data 为 null
// [DELETE] /api/v1/dst/:id?recordOnly=false
func (h *Handler) Delete(c *gin.Context) {
	dstID, ok := handler.PathID(c, "存档ID")
	if !ok {
		return
	}

	var req request.DeleteDstReq
	if err := c.ShouldBindQuery(&req); err != nil {
		result.FailWithMsg(c, 400, "查询参数无效")
		return
	}

	resp, err := h.dstService.Delete(c.Request.Context(), c.GetUint64("userID"), dstID, &req)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}
