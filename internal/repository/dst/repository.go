package dst

import (
	"context"
	"errors"
	"game-panel/internal/model/entity"
	repo "game-panel/internal/repository"
	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, dst *entity.Dst) error
	GetByID(ctx context.Context, userID uint64, dstID uint64) (*entity.Dst, error)
	List(ctx context.Context, userID uint64, serverID uint64, page int, pageSize int) ([]entity.Dst, int64, error)
	Update(ctx context.Context, userID uint64, dstID uint64, updates map[string]any) error
	Delete(ctx context.Context, userID uint64, dstID uint64) error
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

func (r *repository) Create(ctx context.Context, dst *entity.Dst) error {
	return r.getDB(ctx).Create(dst).Error
}

func (r *repository) GetByID(ctx context.Context, userID uint64, dstID uint64) (*entity.Dst, error) {
	var dst entity.Dst
	err := r.getDB(ctx).
		Where("id = ? AND user_id = ?", dstID, userID).
		First(&dst).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &dst, nil
}

func (r *repository) List(ctx context.Context, userID uint64, serverID uint64, page int, pageSize int) ([]entity.Dst, int64, error) {
	var total int64
	err := r.getDB(ctx).
		Model(&entity.Dst{}).
		Where("user_id = ? AND server_id = ?", userID, serverID).
		Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	dsts := make([]entity.Dst, 0)
	err = r.getDB(ctx).
		Where("user_id = ? AND server_id = ?", userID, serverID).
		Order("slot ASC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&dsts).Error
	if err != nil {
		return nil, 0, err
	}
	return dsts, total, nil
}

func (r *repository) Update(ctx context.Context, userID uint64, dstID uint64, updates map[string]any) error {
	result := r.getDB(ctx).
		Model(&entity.Dst{}).
		Where("id = ? AND user_id = ?", dstID, userID).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	dst, err := r.GetByID(ctx, userID, dstID)
	if err != nil {
		return err
	}
	if dst == nil {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *repository) Delete(ctx context.Context, userID uint64, dstID uint64) error {
	result := r.getDB(ctx).
		Where("id = ? AND user_id = ?", dstID, userID).
		Delete(&entity.Dst{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
