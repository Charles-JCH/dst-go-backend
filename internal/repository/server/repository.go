package server

import (
	"context"
	"errors"
	"game-panel/internal/model/entity"
	repo "game-panel/internal/repository"
	"gorm.io/gorm"
)

var ErrServerHasDst = errors.New("服务器仍有关联存档")

type Repository interface {
	AgentTokenVersion(ctx context.Context, serverID uint64) (uint64, bool, error)
	Create(ctx context.Context, server *entity.Server) error
	GetByID(ctx context.Context, userID uint64, serverID uint64) (*entity.Server, error)
	List(ctx context.Context, userID uint64, page int, pageSize int) ([]entity.Server, int64, error)
	Update(ctx context.Context, userID uint64, serverID uint64, fields UpdateFields) error
	Delete(ctx context.Context, userID uint64, serverID uint64) error
}

type UpdateFields struct {
	Name     string
	IP       string
	Port     int
	Username string
	Password *string
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) AgentTokenVersion(ctx context.Context, serverID uint64) (uint64, bool, error) {
	server, err := repo.First[entity.Server](r.getDB(ctx).Select("agent_token_version"), "id = ?", serverID)
	if err != nil || server == nil {
		return 0, false, err
	}
	return server.AgentTokenVersion, true, nil
}

func (r *repository) Create(ctx context.Context, server *entity.Server) error {
	return repo.WrapError(r.getDB(ctx).Create(server).Error)
}

func (r *repository) GetByID(ctx context.Context, userID uint64, serverID uint64) (*entity.Server, error) {
	return repo.First[entity.Server](r.getDB(ctx), "id = ? AND user_id = ?", serverID, userID)
}

func (r *repository) List(ctx context.Context, userID uint64, page int, pageSize int) ([]entity.Server, int64, error) {
	db := r.getDB(ctx).
		Model(&entity.Server{}).
		Where("user_id = ?", userID)
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var servers []entity.Server
	err := db.
		Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&servers).Error
	return servers, total, err
}

func (r *repository) Update(ctx context.Context, userID uint64, serverID uint64, fields UpdateFields) error {
	updates := map[string]any{
		"name":     fields.Name,
		"ip":       fields.IP,
		"port":     fields.Port,
		"username": fields.Username,
	}
	if fields.Password != nil {
		updates["password"] = *fields.Password
	}
	result := r.getDB(ctx).
		Model(&entity.Server{}).
		Where("id = ? AND user_id = ?", serverID, userID).
		Updates(updates)
	if result.Error != nil {
		return repo.WrapError(result.Error)
	}
	if result.RowsAffected > 0 {
		return nil
	}
	server, err := r.GetByID(ctx, userID, serverID)
	if err != nil {
		return err
	}
	if server == nil {
		return repo.ErrNotFound
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
		return repo.WrapError(result.Error)
	}
	if result.RowsAffected > 0 {
		return nil
	}
	server, err := r.GetByID(ctx, userID, serverID)
	if err != nil {
		return err
	}
	if server == nil {
		return repo.ErrNotFound
	}
	return ErrServerHasDst
}

func (r *repository) getDB(ctx context.Context) *gorm.DB {
	return repo.GetDBFromCtx(ctx, r.db)
}
