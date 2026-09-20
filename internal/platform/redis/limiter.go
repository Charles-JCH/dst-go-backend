package redis

import (
	"context"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"time"
)

type Limiter interface {
	Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error)
}

type limiter struct {
	client *redis.Client
}

func NewLimiter(client *redis.Client) Limiter {
	return &limiter{client: client}
}

var rateLimitLua = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]

redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window)

local currentRequests = redis.call('ZCARD', key)
if currentRequests >= limit then
	return 0
end

redis.call('ZADD', key, now, member)
redis.call('EXPIRE', key, math.ceil(window / 1000))
return 1
`)

// Allow 判断是否允许当前请求，允许时记录本次请求并占用一个额度
func (l *limiter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
	// 执行 Lua 脚本
	result, err := rateLimitLua.Run(
		ctx,
		l.client,
		[]string{key},
		time.Now().UnixMilli(),
		window.Milliseconds(),
		limit,
		uuid.NewString(),
	).Int64()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}
