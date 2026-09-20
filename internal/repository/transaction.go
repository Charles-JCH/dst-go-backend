package repository

import (
	"context"
	"gorm.io/gorm"
)

type txKey struct{}

type Transaction interface {
	ExecTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type gormTransaction struct {
	db *gorm.DB
}

func NewTransaction(db *gorm.DB) Transaction {
	return &gormTransaction{db: db}
}

func (t *gormTransaction) ExecTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, txKey{}, tx)
		return fn(txCtx)
	})
}

func GetDBFromCtx(ctx context.Context, defaultDB *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return defaultDB.WithContext(ctx)
}
