package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"time"
)

const (
	tokenBlacklistPrefix = "token_blacklist:"
	resetTokenPrefix     = "reset_token:"
)

type ResetTokenData struct {
	UserID       uint64 `json:"userID"`
	TokenVersion uint64 `json:"tokenVersion"`
}

type AuthCache interface {
	// 黑名单
	AddTokenToBlacklist(ctx context.Context, token string, ttl time.Duration) error
	IsTokenInBlacklist(ctx context.Context, token string) (bool, error)
	ConsumeRefreshToken(ctx context.Context, token string, ttl time.Duration) (bool, error)

	// 重置密码
	SetResetToken(ctx context.Context, token string, userID uint64, tokenVersion uint64, ttl time.Duration) error
	GetResetToken(ctx context.Context, token string) (*ResetTokenData, error)
	DelResetToken(ctx context.Context, token string) error
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
		return false, errors.New("刷新凭证为空")
	}
	if ttl <= 0 {
		return false, errors.New("刷新凭证已过期")
	}

	key := tokenBlacklistPrefix + token

	consumed, err := c.client.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		return false, fmt.Errorf("刷新凭证消费失败: %w", err)
	}

	return consumed, nil
}

func (c *authCache) SetResetToken(ctx context.Context, token string, userID uint64, tokenVersion uint64, ttl time.Duration) error {
	data, err := json.Marshal(ResetTokenData{
		UserID:       userID,
		TokenVersion: tokenVersion,
	})
	if err != nil {
		return fmt.Errorf("密码重置凭证序列化失败: %w", err)
	}

	key := resetTokenPrefix + token
	return c.client.Set(ctx, key, string(data), ttl).Err()
}

func (c *authCache) GetResetToken(ctx context.Context, token string) (*ResetTokenData, error) {
	key := resetTokenPrefix + token

	val, err := c.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("密码重置凭证读取失败: %w", err)
	}

	var data ResetTokenData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, err
	}

	return &data, nil
}

func (c *authCache) DelResetToken(
	ctx context.Context,
	token string,
) error {
	key := resetTokenPrefix + token
	return c.client.Del(ctx, key).Err()
}
