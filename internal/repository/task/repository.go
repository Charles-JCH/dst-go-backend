package task

import (
	"context"
	"errors"
	"game-panel/internal/model/entity"
	repo "game-panel/internal/repository"
	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, task *entity.Task) error
	GetByID(ctx context.Context, userID uint64, taskID uint64) (*entity.Task, error)
	List(ctx context.Context, userID uint64, query ListQuery) ([]entity.Task, int64, error)
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

func (r *repository) getDB(ctx context.Context) *gorm.DB {
	return repo.GetDBFromCtx(ctx, r.db)
}

func (r *repository) Create(ctx context.Context, task *entity.Task) error {
	return r.getDB(ctx).Create(task).Error
}

func (r *repository) GetByID(ctx context.Context, userID uint64, taskID uint64) (*entity.Task, error) {
	var task entity.Task

	err := r.getDB(ctx).
		Where("id = ? AND user_id = ?", taskID, userID).
		First(&task).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &task, nil
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
