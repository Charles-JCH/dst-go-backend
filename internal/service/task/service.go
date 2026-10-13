package task

import (
	"context"
	"game-panel/internal/apperr"
	"game-panel/internal/model/entity"
	"game-panel/internal/model/request"
	"game-panel/internal/model/response"
	"game-panel/internal/repository/task"
	"log/slog"
)

type Service interface {
	GetByID(ctx context.Context, userID uint64, taskID uint64) (*response.TaskResp, error)
	List(ctx context.Context, userID uint64, req *request.ListTaskReq) (*response.TaskListResp, error)
}

type service struct {
	taskRepo task.Repository
}

func NewService(taskRepo task.Repository) Service {
	return &service{taskRepo: taskRepo}
}

func (s *service) GetByID(ctx context.Context, userID uint64, taskID uint64) (*response.TaskResp, error) {
	task, err := s.taskRepo.GetByID(ctx, userID, taskID)
	if err != nil {
		slog.ErrorContext(ctx, "任务查询失败", "userID", userID, "taskID", taskID, "error", err)
		return nil, apperr.NewBizError(500, "任务查询失败")
	}
	if task == nil {
		return nil, apperr.NewBizError(404, "任务不存在")
	}
	return response.ToTaskResp(task), nil
}

func (s *service) List(ctx context.Context, userID uint64, req *request.ListTaskReq) (*response.TaskListResp, error) {
	if req == nil ||
		req.Page < 1 || req.Page > 1000000 ||
		req.PageSize < 1 || req.PageSize > 100 {
		return nil, apperr.NewBizError(400, "分页参数无效")
	}

	if len(req.Type) > 64 || len(req.ResourceType) > 64 {
		return nil, apperr.NewBizError(400, "查询参数无效")
	}

	switch req.Status {
	case "",
		entity.TaskStatusPending,
		entity.TaskStatusRunning,
		entity.TaskStatusSucceeded,
		entity.TaskStatusFailed:
	default:
		return nil, apperr.NewBizError(400, "任务状态无效")
	}

	if req.ResourceID != 0 && req.ResourceType == "" {
		return nil, apperr.NewBizError(400, "指定资源ID时必须指定资源类型")
	}

	query := task.ListQuery{
		Page:         req.Page,
		PageSize:     req.PageSize,
		Type:         req.Type,
		Status:       req.Status,
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
	}

	tasks, total, err := s.taskRepo.List(ctx, userID, query)
	if err != nil {
		slog.ErrorContext(ctx, "任务列表查询失败", "userID", userID, "error", err)
		return nil, apperr.NewBizError(500, "任务列表查询失败")
	}

	list := make([]response.TaskResp, 0, len(tasks))
	for i := range tasks {
		list = append(list, *response.ToTaskResp(&tasks[i]))
	}

	return &response.TaskListResp{
		List:     list,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}
