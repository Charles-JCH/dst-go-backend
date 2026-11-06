package gameenv

import (
	"context"
	"game-panel/internal/model/entity"
	repo "game-panel/internal/repository"
	"gorm.io/gorm"
)

type Repository interface {
	List(ctx context.Context) ([]entity.GameEnv, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) List(ctx context.Context) ([]entity.GameEnv, error) {
	var envs []entity.GameEnv
	err := repo.GetDBFromCtx(ctx, r.db).Order("id ASC").Find(&envs).Error
	return envs, err
}
