package repository

import (
	"errors"
	"gorm.io/gorm"
)

// First 按条件查询单条记录，记录不存在时返回 nil, nil
func First[T any](db *gorm.DB, query string, args ...any) (*T, error) {
	var record T
	err := db.Where(query, args...).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}
