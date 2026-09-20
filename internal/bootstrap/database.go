package bootstrap

import (
	"fmt"
	"game-panel/internal/config"
	"game-panel/internal/model/entity"
	"game-panel/internal/platform/database"
	"gorm.io/gorm"
	"log/slog"
	"time"
)

func InitSQLite(cfg config.SQLiteConfig) (*gorm.DB, error) {
	db, err := database.NewSQLite(&database.SQLiteConfig{
		DBPath:          cfg.DBPath,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.ConnMaxIdleTime,
		LogLevel:        cfg.LogLevel,
	})

	if err != nil {
		return nil, fmt.Errorf("SQLite 初始化失败: %w", err)
	}

	// 自动迁移
	err = db.AutoMigrate(
		&entity.User{},
	)
	if err != nil {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			if closeErr := sqlDB.Close(); closeErr != nil {
				slog.Error("迁移失败后关闭数据库失败", "错误", closeErr)
			}
		}
		return nil, fmt.Errorf("数据库自动迁移失败: %w", err)
	}

	return db, nil
}

func InitMySQL(cfg *database.MySQLConfig) (*gorm.DB, error) {
	db, err := database.NewMySQL(&database.MySQLConfig{
		Host:            cfg.Host,
		Port:            cfg.Port,
		User:            cfg.User,
		Password:        cfg.Password,
		Database:        cfg.Database,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 10 * time.Minute,
		LogLevel:        cfg.LogLevel,
	})

	if err != nil {
		return nil, fmt.Errorf("MySQL 初始化失败: %w", err)
	}

	// 自动迁移
	err = db.AutoMigrate(
		&entity.User{},
	)
	if err != nil {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			if closeErr := sqlDB.Close(); closeErr != nil {
				slog.Error("迁移失败后关闭数据库失败", "错误", closeErr)
			}
		}
		return nil, fmt.Errorf("数据库自动迁移失败: %w", err)
	}

	return db, nil
}
