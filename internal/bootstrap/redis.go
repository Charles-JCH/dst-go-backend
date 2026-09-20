package bootstrap

import (
	"fmt"
	"game-panel/internal/config"
	pkgRedis "game-panel/internal/platform/redis"
	"github.com/redis/go-redis/v9"
)

func InitRedis(cfg config.RedisConfig) (*redis.Client, error) {
	rdb, err := pkgRedis.NewRedisClient(&pkgRedis.Config{
		Host:         cfg.Host,
		Port:         cfg.Port,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	if err != nil {
		return nil, fmt.Errorf("redis 初始化失败: %w", err)
	}

	return rdb, nil
}
