package repository

import (
	"errors"
	"gorm.io/gorm"
)

var (
	ErrNotFound  = errors.New("记录不存在")
	ErrDuplicate = errors.New("记录已存在")
)

// WrapError 把 gorm 的错误翻译成 repository 自己的错误
func WrapError(err error) error {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return ErrNotFound
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return ErrDuplicate
	default:
		return err
	}
}
