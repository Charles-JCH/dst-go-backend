package repository

import (
	"context"
	"errors"
	"game-panel/internal/model/entity"
	"gorm.io/gorm"
)

type ServerRepository interface {
	ExistsByID(ctx context.Context, serverID uint64) (bool, error)
	Create(ctx context.Context, server *entity.Server) error
	GetByID(ctx context.Context, userID uint64, serverID uint64) (*entity.Server, error)
	List(ctx context.Context, userID uint64, page int, pageSize int) ([]entity.Server, int64, error)
	Update(ctx context.Context, userID uint64, serverID uint64, updates map[string]any) error
	Delete(ctx context.Context, userID uint64, serverID uint64) error
}

var ErrServerHasDst = errors.New("服务器仍有关联存档")

type serverRepository struct {
	db *gorm.DB
}

func NewServerRepository(db *gorm.DB) ServerRepository {
	return &serverRepository{db: db}
}

func (r *serverRepository) ExistsByID(ctx context.Context, serverID uint64) (bool, error) {
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

func (r *serverRepository) getDB(ctx context.Context) *gorm.DB {
	return GetDBFromCtx(ctx, r.db)
}

func (r *serverRepository) Create(ctx context.Context, server *entity.Server) error {
	return r.getDB(ctx).Create(server).Error
}

func (r *serverRepository) GetByID(ctx context.Context, userID uint64, serverID uint64) (*entity.Server, error) {
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

func (r *serverRepository) List(ctx context.Context, userID uint64, page int, pageSize int) ([]entity.Server, int64, error) {
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

func (r *serverRepository) Update(ctx context.Context, userID uint64, serverID uint64, updates map[string]any) error {
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

func (r *serverRepository) Delete(ctx context.Context, userID uint64, serverID uint64) error {
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
