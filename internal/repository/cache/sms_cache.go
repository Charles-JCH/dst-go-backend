package cache

import (
	"context"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"time"
)

const smsCodePrefix = "sms_code:"

var ErrSMSCodeAttemptsExceeded = errors.New(
	"验证码错误次数已达上限，请重新发送",
)

type SMSCache interface {
	SetCode(ctx context.Context, phone string, code string, ttl time.Duration) error
	ConsumeCode(ctx context.Context, phone string, code string) (bool, error)
	DelCode(ctx context.Context, phone string) error
}

type smsCache struct {
	client *redis.Client
}

func NewSMSCache(client *redis.Client) SMSCache {
	return &smsCache{client: client}
}

var setCodeLua = redis.NewScript(`
redis.call('HSET', KEYS[1], 'code', ARGV[1], 'attempts', 0)
redis.call('PEXPIRE', KEYS[1], ARGV[2])
return 1
`)

var consumeCodeLua = redis.NewScript(`
local cachedCode = redis.call("HGET", KEYS[1], 'code')

if not cachedCode then
	return 0
end

if cachedCode == ARGV[1] then
	redis.call('DEL', KEYS[1])
	return 1
end

local attempts = redis.call('HINCRBY', KEYS[1], 'attempts', 1)
if attempts >= 3 then
	redis.call('DEL', KEYS[1])
	return -1
end

return 0
`)

func (c *smsCache) SetCode(ctx context.Context, phone, code string, ttl time.Duration) error {
	if ttl.Milliseconds() <= 0 {
		return fmt.Errorf("验证码有效期必须至少为一毫秒")
	}

	key := smsCodePrefix + phone
	err := setCodeLua.Run(ctx, c.client, []string{key}, code, ttl.Milliseconds()).Err()
	if err != nil {
		return fmt.Errorf("保存短信验证码失败: %w", err)
	}
	return nil
}

func (c *smsCache) ConsumeCode(ctx context.Context, phone string, code string) (bool, error) {
	key := smsCodePrefix + phone
	result, err := consumeCodeLua.Run(ctx, c.client, []string{key}, code).Int64()
	if err != nil {
		return false, fmt.Errorf("消费短信验证码失败: %w", err)
	}
	if result == -1 {
		return false, ErrSMSCodeAttemptsExceeded
	}
	return result == 1, nil
}

func (c *smsCache) DelCode(ctx context.Context, phone string) error {
	key := smsCodePrefix + phone
	return c.client.Del(ctx, key).Err()
}
