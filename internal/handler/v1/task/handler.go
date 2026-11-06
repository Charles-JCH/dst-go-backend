package task

import (
	"game-panel/internal/handler"
	"game-panel/internal/model/request"
	"game-panel/internal/result"
	"game-panel/internal/service/task"
	"github.com/gin-gonic/gin"
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
	var req request.ListTaskReq
	if err := c.ShouldBindQuery(&req); err != nil {
		result.FailWithMsg(c, 400, "查询参数无效")
		return
	}

	resp, err := h.taskService.List(c.Request.Context(), c.GetUint64("userID"), &req)
	if err != nil {
		result.Fail(c, err)
		return
	}

	result.Success(c, resp)
}

// GetByID 查询任务详情
// GET /api/v1/tasks/:id
func (h *Handler) GetByID(c *gin.Context) {
	taskID, ok := handler.PathID(c, "任务ID")
	if !ok {
		return
	}

	resp, err := h.taskService.GetByID(c.Request.Context(), c.GetUint64("userID"), taskID)
	if err != nil {
		result.Fail(c, err)
		return
	}

	result.Success(c, resp)
}
