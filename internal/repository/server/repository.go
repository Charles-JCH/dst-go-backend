package server

import (
	"context"
	"errors"
	"game-panel/internal/model/entity"
	repo "game-panel/internal/repository"
	"gorm.io/gorm"
)

type Repository interface {
	ExistsByID(ctx context.Context, serverID uint64) (bool, error)
	Create(ctx context.Context, server *entity.Server) error
	GetByID(ctx context.Context, userID uint64, serverID uint64) (*entity.Server, error)
	List(ctx context.Context, userID uint64, page int, pageSize int) ([]entity.Server, int64, error)
	Update(ctx context.Context, userID uint64, serverID uint64, updates map[string]any) error
	Delete(ctx context.Context, userID uint64, serverID uint64) error
}

var ErrServerHasDst = errors.New("服务器仍有关联存档")

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) ExistsByID(ctx context.Context, serverID uint64) (bool, error) {
	var count int64
	err := r.getDB(ctx).
		Model(&entity.Server{}).
		Where("id = ?", serverID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *repository) getDB(ctx context.Context) *gorm.DB {
	return repo.GetDBFromCtx(ctx, r.db)
}

func (r *repository) Create(ctx context.Context, server *entity.Server) error {
	return r.getDB(ctx).Create(server).Error
}

func (r *repository) GetByID(ctx context.Context, userID uint64, serverID uint64) (*entity.Server, error) {
	var server entity.Server
	err := r.getDB(ctx).
		Where("id = ? AND user_id = ?", serverID, userID).
		First(&server).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &server, nil
}

func (r *repository) List(ctx context.Context, userID uint64, page int, pageSize int) ([]entity.Server, int64, error) {
	var total int64
	err := r.getDB(ctx).
		Model(&entity.Server{}).
		Where("user_id = ?", userID).
		Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	servers := make([]entity.Server, 0)
	err = r.getDB(ctx).
		Where("user_id = ?", userID).
		Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&servers).Error
	if err != nil {
		return nil, 0, err
	}
	return servers, total, nil
}

func (r *repository) Update(ctx context.Context, userID uint64, serverID uint64, updates map[string]any) error {
	result := r.getDB(ctx).
		Model(&entity.Server{}).
		Where("id = ? AND user_id = ?", serverID, userID).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	server, err := r.GetByID(ctx, userID, serverID)
	if err != nil {
		return err
	}
	if server == nil {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *repository) Delete(ctx context.Context, userID uint64, serverID uint64) error {
	db := r.getDB(ctx)
	dsts := db.Model(&entity.Dst{}).
		Select("1").
		Where("server_id = ?", serverID)
	result := db.
		Where("id = ? AND user_id = ?", serverID, userID).
		Where("NOT EXISTS (?)", dsts).
		Delete(&entity.Server{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	server, err := r.GetByID(ctx, userID, serverID)
	if err != nil {
		return err
	}
	if server == nil {
		return gorm.ErrRecordNotFound
	}
	return ErrServerHasDst
}
