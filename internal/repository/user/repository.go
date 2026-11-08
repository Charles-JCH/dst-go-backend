package user

import (
	"context"
	"errors"
	"game-panel/internal/model/entity"
	repo "game-panel/internal/repository"
	"gorm.io/gorm"
)

var ErrTokenVersionConflict = errors.New("用户令牌版本已变更")

type Repository interface {
	Create(ctx context.Context, user *entity.User) error
	GetByID(ctx context.Context, userID uint64) (*entity.User, error)
	GetByUsername(ctx context.Context, username string) (*entity.User, error)
	GetByPhone(ctx context.Context, phone string) (*entity.User, error)
	UpdatePassword(ctx context.Context, userID uint64, passwordHash string, tokenVersion uint64) error
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// Create 创建新用户
func (r *repository) Create(ctx context.Context, user *entity.User) error {
	return repo.WrapError(r.getDB(ctx).Create(user).Error)
}

// GetByID 根据用户 ID 查询
func (r *repository) GetByID(ctx context.Context, userID uint64) (*entity.User, error) {
	return repo.First[entity.User](r.getDB(ctx), "id = ?", userID)
}

// GetByUsername 根据用户名查询
func (r *repository) GetByUsername(ctx context.Context, username string) (*entity.User, error) {
	return repo.First[entity.User](r.getDB(ctx), "username = ?", username)
}

// GetByPhone 根据手机号查询
func (r *repository) GetByPhone(ctx context.Context, phone string) (*entity.User, error) {
	return repo.First[entity.User](r.getDB(ctx), "phone = ?", phone)
}

// UpdatePassword 更新用户密码并递增令牌版本
func (r *repository) UpdatePassword(ctx context.Context, userID uint64, passwordHash string, tokenVersion uint64) error {
	result := r.getDB(ctx).
		Model(&entity.User{}).
		Where("id = ? AND token_version = ?", userID, tokenVersion).
		Updates(map[string]any{
			"password":      passwordHash,
			"token_version": gorm.Expr("token_version + 1"),
		})
	if result.Error != nil {
		return repo.WrapError(result.Error)
	}
	if result.RowsAffected > 0 {
		return nil
	}
	user, err := r.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return repo.ErrNotFound
	}
	return ErrTokenVersionConflict
}

func (r *repository) getDB(ctx context.Context) *gorm.DB {
	return repo.GetDBFromCtx(ctx, r.db)
}
