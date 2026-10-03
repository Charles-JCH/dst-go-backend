package repository

import (
	"context"
	"errors"
	"game-panel/internal/model/entity"
	"gorm.io/gorm"
)

type UserRepository interface {
	Create(ctx context.Context, user *entity.User) error
	GetByID(ctx context.Context, userID uint64) (*entity.User, error)
	GetByUsername(ctx context.Context, username string) (*entity.User, error)
	GetByPhone(ctx context.Context, phone string) (*entity.User, error)
	GetByEmail(ctx context.Context, email string) (*entity.User, error)
	Update(ctx context.Context, userID uint64, username string) error
	UpdatePassword(ctx context.Context, userID uint64, passwordHash string, tokenVersion uint64) (bool, error)
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) getDB(ctx context.Context) *gorm.DB {
	return GetDBFromCtx(ctx, r.db)
}

// Create 创建新用户
func (r *userRepository) Create(ctx context.Context, user *entity.User) error {
	return r.getDB(ctx).Create(user).Error
}

// GetByID 根据用户 ID 查询
func (r *userRepository) GetByID(ctx context.Context, userID uint64) (*entity.User, error) {
	var user entity.User
	err := r.getDB(ctx).Where("id = ?", userID).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByUsername 根据用户名查询
func (r *userRepository) GetByUsername(ctx context.Context, username string) (*entity.User, error) {
	var user entity.User
	err := r.getDB(ctx).Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByPhone 根据手机号查询
func (r *userRepository) GetByPhone(ctx context.Context, phone string) (*entity.User, error) {
	var user entity.User
	err := r.getDB(ctx).Where("phone = ?", phone).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByEmail 根据邮箱查询
func (r *userRepository) GetByEmail(ctx context.Context, email string) (*entity.User, error) {
	var user entity.User
	err := r.getDB(ctx).Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Update 更新用户名
func (r *userRepository) Update(ctx context.Context, userID uint64, username string) error {
	result := r.getDB(ctx).
		Model(&entity.User{}).
		Where("id = ?", userID).
		Update("username", username)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// UpdatePassword 更新用户密码并递增令牌版本
func (r *userRepository) UpdatePassword(ctx context.Context, userID uint64, passwordHash string, tokenVersion uint64) (bool, error) {
	result := r.getDB(ctx).
		Model(&entity.User{}).
		Where("id = ? AND token_version = ?", userID, tokenVersion).
		Updates(map[string]any{
			"password":      passwordHash,
			"token_version": gorm.Expr("token_version + 1"),
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}
