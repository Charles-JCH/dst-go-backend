package task

import (
	"game-panel/internal/model/request"
	"game-panel/internal/result"
	"game-panel/internal/service/task"
	"github.com/gin-gonic/gin"
	"strconv"
)

type Handler struct {
	taskService task.Service
}

func NewHandler(taskService task.Service) *Handler {
	return &Handler{taskService: taskService}
}

// List 分页查询
// [GET] /api/v1/tasks
func (h *Handler) List(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		return
	}

	var req request.ListTaskReq
	if err := c.ShouldBindQuery(&req); err != nil {
		result.FailWithMsg(c, 400, "查询参数无效")
		return
	}

	resp, err := h.taskService.List(c.Request.Context(), userID, &req)
	if err != nil {
		result.Fail(c, err)
		return
	}

	result.Success(c, resp)
}

// GetByID 查询任务详情
// GET /api/v1/tasks/:id
func (h *Handler) GetByID(c *gin.Context) {
	userID, ok := h.getUserID(c)
	if !ok {
		return
	}

	taskID, err := strconv.ParseUint(c.Param("id"), 10, 63)
	if err != nil || taskID == 0 {
		result.FailWithMsg(c, 400, "任务ID无效")
		return
	}

	resp, err := h.taskService.GetByID(c.Request.Context(), userID, taskID)
	if err != nil {
		result.Fail(c, err)
		return
	}

	result.Success(c, resp)
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
