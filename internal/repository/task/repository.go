package task

import (
	"context"
	"errors"
	"game-panel/internal/model/entity"
	repo "game-panel/internal/repository"
	"gorm.io/gorm"
	"time"
)

// ErrStatusConflict 任务当前状态不允许这次流转（已被处理过），调用方直接忽略即可
var ErrStatusConflict = errors.New("任务状态已变更")

type Repository interface {
	Create(ctx context.Context, task *entity.Task) error
	GetByID(ctx context.Context, userID uint64, taskID uint64) (*entity.Task, error)
	List(ctx context.Context, userID uint64, query ListQuery) ([]entity.Task, int64, error)
	ListActive(ctx context.Context, userID uint64) ([]entity.Task, error)

	// 以下为系统内部使用，不带 user_id 条件
	FindByID(ctx context.Context, taskID uint64) (*entity.Task, error)
	FindByCommandID(ctx context.Context, commandID string) (*entity.Task, error)
	ListPending(ctx context.Context, taskType string) ([]entity.Task, error)
	Start(ctx context.Context, taskID uint64) error
	UpdateStage(ctx context.Context, taskID uint64, stage string) error
	Finish(ctx context.Context, taskID uint64, status string, errorMessage string) error
	FailRunning(ctx context.Context, errorMessage string) (int64, error)
	FailPendingCommands(ctx context.Context, errorMessage string) (int64, error)

	CreateLog(ctx context.Context, log *entity.TaskLog) error
	ListLogs(ctx context.Context, taskID uint64, afterID uint64, limit int) ([]entity.TaskLog, error)
}

type ListQuery struct {
	Page         int
	PageSize     int
	Type         string
	Status       string
	ResourceType string
	ResourceID   uint64
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, task *entity.Task) error {
	return repo.WrapError(r.getDB(ctx).Create(task).Error)
}

func (r *repository) GetByID(ctx context.Context, userID uint64, taskID uint64) (*entity.Task, error) {
	return repo.First[entity.Task](r.getDB(ctx), "id = ? AND user_id = ?", taskID, userID)
}

func (r *repository) List(ctx context.Context, userID uint64, query ListQuery) ([]entity.Task, int64, error) {
	db := r.getDB(ctx).
		Model(&entity.Task{}).
		Where("user_id = ?", userID)

	if query.Type != "" {
		db = db.Where("type = ?", query.Type)
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	if query.ResourceType != "" {
		db = db.Where("resource_type = ?", query.ResourceType)
	}
	if query.ResourceID != 0 {
		db = db.Where("resource_id = ?", query.ResourceID)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tasks []entity.Task
	err := db.
		Order("id DESC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&tasks).Error

	if err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

// ListActive 查询用户待执行和执行中的任务；进行中的任务受 ActiveKey 唯一约束，数量很少，不分页
func (r *repository) ListActive(ctx context.Context, userID uint64) ([]entity.Task, error) {
	var tasks []entity.Task
	err := r.getDB(ctx).
		Where("user_id = ? AND status IN ?", userID, []string{entity.TaskStatusPending, entity.TaskStatusRunning}).
		Order("id DESC").
		Find(&tasks).Error
	return tasks, err
}

func (r *repository) FindByID(ctx context.Context, taskID uint64) (*entity.Task, error) {
	return repo.First[entity.Task](r.getDB(ctx), "id = ?", taskID)
}

func (r *repository) FindByCommandID(ctx context.Context, commandID string) (*entity.Task, error) {
	return repo.First[entity.Task](r.getDB(ctx), "command_id = ?", commandID)
}

func (r *repository) ListPending(ctx context.Context, taskType string) ([]entity.Task, error) {
	var tasks []entity.Task
	err := r.getDB(ctx).
		Where("type = ? AND status = ?", taskType, entity.TaskStatusPending).
		Order("id ASC").
		Find(&tasks).Error
	return tasks, err
}

// Start pending → running；任务不是 pending 时返回 ErrStatusConflict
func (r *repository) Start(ctx context.Context, taskID uint64) error {
	return r.transit(ctx, taskID, []string{entity.TaskStatusPending}, map[string]any{
		"status":     entity.TaskStatusRunning,
		"started_at": time.Now(),
	})
}

// UpdateStage 更新执行步骤，只对执行中的任务生效
func (r *repository) UpdateStage(ctx context.Context, taskID uint64, stage string) error {
	return r.transit(ctx, taskID, []string{entity.TaskStatusRunning}, map[string]any{"stage": stage})
}

// Finish pending / running → succeeded / failed，并释放 ActiveKey；任务已结束时返回 ErrStatusConflict。
// 允许从 pending 直接结束：命令下发失败、Agent 拒绝执行时任务还没开始
func (r *repository) Finish(ctx context.Context, taskID uint64, status string, errorMessage string) error {
	return r.transit(ctx, taskID, []string{entity.TaskStatusPending, entity.TaskStatusRunning}, map[string]any{
		"status":        status,
		"error_message": errorMessage,
		"active_key":    nil,
		"finished_at":   time.Now(),
	})
}

// FailRunning 把所有执行中的任务标记为失败，用于平台重启后清理中断的任务
func (r *repository) FailRunning(ctx context.Context, errorMessage string) (int64, error) {
	result := r.getDB(ctx).
		Model(&entity.Task{}).
		Where("status = ?", entity.TaskStatusRunning).
		Updates(map[string]any{
			"status":        entity.TaskStatusFailed,
			"error_message": errorMessage,
			"active_key":    nil,
			"finished_at":   time.Now(),
		})
	return result.RowsAffected, repo.WrapError(result.Error)
}

// FailPendingCommands 把待执行的 Agent 命令任务标记为失败：平台重启后已不知道命令是否发出
func (r *repository) FailPendingCommands(ctx context.Context, errorMessage string) (int64, error) {
	result := r.getDB(ctx).
		Model(&entity.Task{}).
		Where("status = ? AND command_id <> ''", entity.TaskStatusPending).
		Updates(map[string]any{
			"status":        entity.TaskStatusFailed,
			"error_message": errorMessage,
			"active_key":    nil,
			"finished_at":   time.Now(),
		})
	return result.RowsAffected, repo.WrapError(result.Error)
}

func (r *repository) CreateLog(ctx context.Context, log *entity.TaskLog) error {
	return repo.WrapError(r.getDB(ctx).Create(log).Error)
}

// ListLogs 按 ID 升序读取 afterID 之后的日志，用于分批加载
func (r *repository) ListLogs(ctx context.Context, taskID uint64, afterID uint64, limit int) ([]entity.TaskLog, error) {
	var logs []entity.TaskLog
	err := r.getDB(ctx).
		Where("task_id = ? AND id > ?", taskID, afterID).
		Order("id ASC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

func (r *repository) getDB(ctx context.Context) *gorm.DB {
	return repo.GetDBFromCtx(ctx, r.db)
}

// transit 只在任务处于 from 中的状态时更新，保证重复执行不会把状态改乱
func (r *repository) transit(ctx context.Context, taskID uint64, from []string, updates map[string]any) error {
	result := r.getDB(ctx).
		Model(&entity.Task{}).
		Where("id = ? AND status IN ?", taskID, from).
		Updates(updates)
	if result.Error != nil {
		return repo.WrapError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrStatusConflict
	}
	return nil
}
