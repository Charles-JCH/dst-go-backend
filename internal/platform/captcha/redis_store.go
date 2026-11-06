package captcha

import (
	"context"
	"errors"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"time"
)

// RedisStore 实现 base64Captcha.Store 接口，把验证码保存到 Redis
type RedisStore struct {
	client *redis.Client
	prefix string
	expire time.Duration
}

func NewRedisStore(client *redis.Client, prefix string, expire time.Duration) *RedisStore {
	return &RedisStore{
		client: client,
		prefix: prefix,
		expire: expire,
	}
}

// Set 存储验证码（base64Captcha.Store 接口方法，接口不带 ctx）
func (s *RedisStore) Set(id string, value string) error {
	ctx := context.Background()
	key := s.prefix + id
	return s.client.Set(ctx, key, value, s.expire).Err()
}

// Get 获取验证码（base64Captcha.Store 接口方法，接口不带 ctx 且无法返回错误）
func (s *RedisStore) Get(id string, clear bool) string {
	ctx := context.Background()

	val, err := s.GetWithContext(ctx, id, clear)
	if err != nil {
		// 接口无法返回错误，只能在这里记录；业务代码走 GetWithContext，由 service 记录
		slog.ErrorContext(ctx, "图形验证码读取失败", "error", err)
		return ""
	}

	return val
}

// GetWithContext 带 ctx 读取验证码，clear 为 true 时读取后删除；不存在返回空字符串
func (s *RedisStore) GetWithContext(ctx context.Context, id string, clear bool) (string, error) {
	key := s.prefix + id
	var val string
	var err error

	if clear {
		val, err = s.client.GetDel(ctx, key).Result()
	} else {
		val, err = s.client.Get(ctx, key).Result()
	}

	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

// Verify 校验验证码（base64Captcha.Store 接口方法，业务代码请用 captcha.Verify）
func (s *RedisStore) Verify(id, answer string, clear bool) bool {
	v := s.Get(id, clear)
	return v != "" && v == answer
}
