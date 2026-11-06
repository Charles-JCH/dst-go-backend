package redis

import (
	"context"
	"github.com/redis/go-redis/v9"
	"time"
)

var rateLimitLua = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 or redis.call('PTTL', KEYS[1]) == -1 then
	redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
if count > tonumber(ARGV[2]) then
	return 0
end
return 1
`)

type Limiter interface {
	Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error)
}

type limiter struct {
	client *redis.Client
}

func NewLimiter(client *redis.Client) Limiter {
	return &limiter{client: client}
}

// Allow 判断当前请求是否在窗口额度内，每次调用占用一个额度
func (l *limiter) Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error) {
	result, err := rateLimitLua.Run(ctx, l.client, []string{key}, window.Milliseconds(), limit).Int64()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}
