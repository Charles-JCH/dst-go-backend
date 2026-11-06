package bootstrap

import (
	"fmt"
	"game-panel/internal/config"
	"game-panel/internal/model/entity"
	"game-panel/internal/platform/database"
	"game-panel/internal/platform/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
	"log/slog"
	"time"
)

// entities 需要自动迁移的实体，新增实体在这里追加
var entities = []any{
	&entity.User{},
	&entity.GameEnv{},
	&entity.Server{},
	&entity.Dst{},
	&entity.Task{},
}

func InitSQLite(cfg config.SQLiteConfig) (*gorm.DB, error) {
	db, err := database.NewSQLite(&database.SQLiteConfig{
		DBPath:          cfg.DBPath,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.ConnMaxIdleTime,
		Logger:          newGormLogger(cfg.LogLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("sqlite 初始化失败: %w", err)
	}
	if err := migrate(db); err != nil {
		return nil, err
	}
	return db, nil
}

// InitMySQL 预留，迁移到 MySQL 时替换 InitSQLite
func InitMySQL(cfg config.MySQLConfig) (*gorm.DB, error) {
	db, err := database.NewMySQL(&database.MySQLConfig{
		Host:            cfg.Host,
		Port:            cfg.Port,
		User:            cfg.User,
		Password:        cfg.Password,
		Database:        cfg.Database,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.ConnMaxIdleTime,
		Logger:          newGormLogger(cfg.LogLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("mysql 初始化失败: %w", err)
	}
	if err := migrate(db); err != nil {
		return nil, err
	}
	return db, nil
}

// newGormLogger 创建 GORM 日志器，慢查询阈值 200ms，忽略记录不存在错误
func newGormLogger(level string) gormlogger.Interface {
	return logger.NewGormLogger(level, 200*time.Millisecond, true)
}

// migrate 自动迁移表结构并写入初始数据，失败时关闭连接
func migrate(db *gorm.DB) error {
	err := db.AutoMigrate(entities...)
	if err == nil {
		err = seedGameEnv(db)
	}
	if err == nil {
		return nil
	}
	if sqlDB, dbErr := db.DB(); dbErr == nil {
		if closeErr := sqlDB.Close(); closeErr != nil {
			slog.Error("迁移失败后数据库关闭失败", "error", closeErr)
		}
	}
	return fmt.Errorf("数据库迁移失败: %w", err)
}

// seedGameEnv 写入系统支持部署的游戏环境
func seedGameEnv(db *gorm.DB) error {
	envs := []entity.GameEnv{
		{Code: "dst", Name: "饥荒联机版"},
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&envs).Error
}
