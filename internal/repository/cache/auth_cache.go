package cache

import (
	"context"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"strconv"
	"time"
)

const (
	tokenBlacklistPrefix = "token_blacklist:"
	resetTokenPrefix     = "reset_token:"
)

type AuthCache interface {
	// 黑名单
	AddTokenToBlacklist(ctx context.Context, token string, ttl time.Duration) error
	IsTokenInBlacklist(ctx context.Context, token string) (bool, error)
	ConsumeRefreshToken(ctx context.Context, token string, ttl time.Duration) (bool, error)

	// 重置密码
	SetResetToken(ctx context.Context, token string, userId uint64, ttl time.Duration) error
	ConsumeResetToken(ctx context.Context, token string) (userId uint64, err error)
}

type authCache struct {
	client *redis.Client
}

func NewAuthCache(client *redis.Client) AuthCache {
	return &authCache{client: client}
}

func (c *authCache) AddTokenToBlacklist(ctx context.Context, token string, ttl time.Duration) error {
	key := tokenBlacklistPrefix + token
	return c.client.Set(ctx, key, "1", ttl).Err()
}

func (c *authCache) IsTokenInBlacklist(ctx context.Context, token string) (bool, error) {
	key := tokenBlacklistPrefix + token
	exists, err := c.client.Exists(ctx, key).Result()
	return exists > 0, err
}

func (c *authCache) ConsumeRefreshToken(ctx context.Context, token string, ttl time.Duration) (bool, error) {
	if token == "" {
		return false, errors.New("refreshToken 不能为空")
	}
	if ttl <= 0 {
		return false, errors.New("refreshToken 剩余有效期必须大于 0")
	}

	key := tokenBlacklistPrefix + token

	consumed, err := c.client.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("消费 refreshToken 失败: %w", err)
	}

	return consumed, nil
}

func (c *authCache) SetResetToken(ctx context.Context, token string, userId uint64, ttl time.Duration) error {
	key := resetTokenPrefix + token
	return c.client.Set(ctx, key, userId, ttl).Err()
}

func (c *authCache) ConsumeResetToken(ctx context.Context, token string) (userId uint64, err error) {
	key := resetTokenPrefix + token
	val, err := c.client.GetDel(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	userId, err = strconv.ParseUint(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("解析用户ID失败: %w", err)
	}
	return userId, nil
}
