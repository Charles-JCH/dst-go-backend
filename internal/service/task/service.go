package task

import (
	"context"
	"game-panel/internal/apperr"
	"game-panel/internal/model/request"
	"game-panel/internal/model/response"
	taskrepo "game-panel/internal/repository/task"
	"log/slog"
)

type Service interface {
	GetByID(ctx context.Context, userID uint64, taskID uint64) (*response.TaskResp, error)
	List(ctx context.Context, userID uint64, req *request.ListTaskReq) (*response.TaskListResp, error)
	ListActive(ctx context.Context, userID uint64) ([]response.TaskResp, error)
	ListLogs(ctx context.Context, userID uint64, taskID uint64, req *request.ListTaskLogReq) ([]response.TaskLogResp, error)
}

type service struct {
	taskRepo taskrepo.Repository
}

func NewService(taskRepo taskrepo.Repository) Service {
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
	if req.ResourceID != 0 && req.ResourceType == "" {
		return nil, apperr.NewBizError(400, "指定资源ID时必须指定资源类型")
	}

	query := taskrepo.ListQuery{
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

// ListActive 查询待执行和执行中的任务，用于页面顶部的后台任务列表
func (s *service) ListActive(ctx context.Context, userID uint64) ([]response.TaskResp, error) {
	tasks, err := s.taskRepo.ListActive(ctx, userID)
	if err != nil {
		slog.ErrorContext(ctx, "进行中任务查询失败", "userID", userID, "error", err)
		return nil, apperr.NewBizError(500, "任务列表查询失败")
	}
	list := make([]response.TaskResp, 0, len(tasks))
	for i := range tasks {
		list = append(list, *response.ToTaskResp(&tasks[i]))
	}
	return list, nil
}

// ListLogs 分批查询任务日志，afterID 为上一批最后一条的 ID
func (s *service) ListLogs(ctx context.Context, userID uint64, taskID uint64, req *request.ListTaskLogReq) ([]response.TaskLogResp, error) {
	t, err := s.taskRepo.GetByID(ctx, userID, taskID)
	if err != nil {
		slog.ErrorContext(ctx, "任务查询失败", "userID", userID, "taskID", taskID, "error", err)
		return nil, apperr.NewBizError(500, "任务日志查询失败")
	}
	if t == nil {
		return nil, apperr.NewBizError(404, "任务不存在")
	}

	logs, err := s.taskRepo.ListLogs(ctx, taskID, req.AfterID, req.Limit)
	if err != nil {
		slog.ErrorContext(ctx, "任务日志查询失败", "userID", userID, "taskID", taskID, "error", err)
		return nil, apperr.NewBizError(500, "任务日志查询失败")
	}
	list := make([]response.TaskLogResp, 0, len(logs))
	for i := range logs {
		list = append(list, *response.ToTaskLogResp(&logs[i]))
	}
	return list, nil
}
