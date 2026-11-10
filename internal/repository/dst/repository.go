package dst

import (
	"context"
	"encoding/json"
	"game-panel/internal/model/entity"
	repo "game-panel/internal/repository"
	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, dst *entity.Dst) error
	GetByID(ctx context.Context, userID uint64, dstID uint64) (*entity.Dst, error)
	List(ctx context.Context, userID uint64, serverID uint64, page int, pageSize int) ([]entity.Dst, int64, error)
	Update(ctx context.Context, userID uint64, dstID uint64, fields UpdateFields) error
	Delete(ctx context.Context, userID uint64, dstID uint64) error
}

type UpdateFields struct {
	Name         string
	Description  *string
	MaxPlayers   int
	Mods         *[]string
	Password     *string
	ClusterToken *string
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(ctx context.Context, dst *entity.Dst) error {
	return repo.WrapError(r.getDB(ctx).Create(dst).Error)
}

func (r *repository) GetByID(ctx context.Context, userID uint64, dstID uint64) (*entity.Dst, error) {
	return repo.First[entity.Dst](r.getDB(ctx), "id = ? AND user_id = ?", dstID, userID)
}

func (r *repository) List(ctx context.Context, userID uint64, serverID uint64, page int, pageSize int) ([]entity.Dst, int64, error) {
	db := r.getDB(ctx).Model(&entity.Dst{}).Where("user_id = ?", userID)
	if serverID != 0 {
		db = db.Where("server_id = ?", serverID)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var dsts []entity.Dst
	if err := db.
		Order("server_id ASC, slot ASC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&dsts).Error; err != nil {
		return nil, 0, err
	}
	return dsts, total, nil
}

func (r *repository) Update(ctx context.Context, userID uint64, dstID uint64, fields UpdateFields) error {
	updates := map[string]any{
		"name":        fields.Name,
		"max_players": fields.MaxPlayers,
	}
	if fields.Description != nil {
		updates["description"] = *fields.Description
	}
	if fields.Mods != nil {
		mods := *fields.Mods
		if mods == nil {
			mods = []string{}
		}
		modsJSON, err := json.Marshal(mods)
		if err != nil {
			return err
		}
		updates["mods"] = string(modsJSON)
	}
	if fields.Password != nil {
		updates["password"] = *fields.Password
	}
	if fields.ClusterToken != nil {
		updates["cluster_token"] = *fields.ClusterToken
	}
	result := r.getDB(ctx).
		Model(&entity.Dst{}).
		Where("id = ? AND user_id = ?", dstID, userID).
		Updates(updates)
	if result.Error != nil {
		return repo.WrapError(result.Error)
	}
	if result.RowsAffected > 0 {
		return nil
	}
	dst, err := r.GetByID(ctx, userID, dstID)
	if err != nil {
		return err
	}
	if dst == nil {
		return repo.ErrNotFound
	}
	return nil
}

func (r *repository) Delete(ctx context.Context, userID uint64, dstID uint64) error {
	result := r.getDB(ctx).
		Where("id = ? AND user_id = ?", dstID, userID).
		Delete(&entity.Dst{})
	if result.Error != nil {
		return repo.WrapError(result.Error)
	}
	if result.RowsAffected == 0 {
		return repo.ErrNotFound
	}
	return nil
}

func (r *repository) getDB(ctx context.Context) *gorm.DB {
	return repo.GetDBFromCtx(ctx, r.db)
}
