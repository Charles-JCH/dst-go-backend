package database

import (
	"fmt"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"path/filepath"
	"time"
)

type SQLiteConfig struct {
	DBPath          string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	Logger          logger.Interface
}

func NewSQLite(cfg *SQLiteConfig) (*gorm.DB, error) {
	dir := filepath.Dir(cfg.DBPath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("数据库目录创建失败 path=%q: %w", dir, err)
	}

	// 开启 WAL 模式
	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)", cfg.DBPath)

	// 打开 SQLite 连接
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:         cfg.Logger,
		TranslateError: true,
	})
	if err != nil {
		return nil, fmt.Errorf("sqlite 数据库打开失败: %w", err)
	}

	// 获取底层 sql.DB
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("数据库连接池获取失败: %w", err)
	}

	// 连接池配置
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	return db, nil
}
