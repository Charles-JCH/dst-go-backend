package captcha

import (
	"context"
	"errors"
	"github.com/redis/go-redis/v9"
	"log/slog"
	"time"
)

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

// Set 存储验证码
func (s *RedisStore) Set(id string, value string) error {
	ctx := context.Background()
	key := s.prefix + id
	return s.client.Set(ctx, key, value, s.expire).Err()
}

// Get 获取验证码
func (s *RedisStore) Get(id string, clear bool) string {
	ctx := context.Background()
	key := s.prefix + id
	var val string
	var err error

	if clear {
		val, err = s.client.GetDel(ctx, key).Result()
	} else {
		val, err = s.client.Get(ctx, key).Result()
	}

	if errors.Is(err, redis.Nil) {
		return ""
	}
	if err != nil {
		slog.ErrorContext(ctx, "读取图形验证码失败", "错误", err)
		return ""
	}
	return val
}

// Verify 校验验证码
func (s *RedisStore) Verify(id, answer string, clear bool) bool {
	v := s.Get(id, clear)
	return v != "" && v == answer
}
