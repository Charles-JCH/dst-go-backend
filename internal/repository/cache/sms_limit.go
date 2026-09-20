package cache

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"time"
)

type SMSLimitReason int

const (
	SMSLimitAllowed SMSLimitReason = iota
	SMSLimitIPMinute
	SMSLimitPhoneMinute
	SMSLimitIPDay
	SMSLimitPhoneDay
)

type SMSLimitPolicy struct {
	IPMinute    int64
	PhoneMinute int64
	IPDay       int64
	PhoneDay    int64
}

type SMSReservation struct {
	id   string
	keys []string
}

type SMSLimit interface {
	Reserve(ctx context.Context, ip, phone string, policy SMSLimitPolicy) (*SMSReservation, SMSLimitReason, error)
	Release(ctx context.Context, reservation *SMSReservation) error
}

type smsLimit struct {
	client *redis.Client
}

func NewSMSLimit(client *redis.Client) SMSLimit {
	return &smsLimit{client: client}
}

// 四个 Key 依次对应：IP分钟、手机号分钟、IP当天、手机号当天
var reserveSMSLua = redis.NewScript(`
local serverTime = redis.call('TIME')
local now = tonumber(serverTime[1]) * 1000 + math.floor(tonumber(serverTime[2]) / 1000)

local offset = 8 * 60 * 60 * 1000
local day = 24 * 60 * 60 * 1000
local dayStart = math.floor((now + offset) / day) * day - offset
local dayEnd = dayStart + day

for i = 1, 2 do
	redis.call('ZREMRANGEBYSCORE', KEYS[i], '-inf', now - 60000)
end

for i = 3, 4 do
	redis.call('ZREMRANGEBYSCORE', KEYS[i], '-inf', '(' .. dayStart)
end

for i = 1, 4 do
	if redis.call('ZCARD', KEYS[i]) >= tonumber(ARGV[i]) then
		return i
	end
end

for i = 1, 4 do
	redis.call('ZADD', KEYS[i], now, ARGV[5])
	if i <= 2 then
		redis.call('PEXPIRE', KEYS[i], 60000)
	else
		redis.call('PEXPIREAT', KEYS[i], dayEnd)
	end
end

return 0
`)

// 退还本次预占
var releaseSMSLua = redis.NewScript(`
for i = 1, #KEYS do
	redis.call('ZREM', KEYS[i], ARGV[1])
end
return 1
`)

func (l *smsLimit) Reserve(ctx context.Context, ip, phone string, policy SMSLimitPolicy) (*SMSReservation, SMSLimitReason, error) {
	if policy.IPMinute <= 0 || policy.PhoneMinute <= 0 ||
		policy.IPDay <= 0 || policy.PhoneDay <= 0 {
		return nil, SMSLimitAllowed, fmt.Errorf("短信限流额度必须大于零")
	}

	keys := []string{
		"sms:limit:ip:minute:" + ip,
		"sms:limit:phone:minute:" + phone,
		"sms:limit:ip:day:" + ip,
		"sms:limit:phone:day:" + phone,
	}
	id := uuid.NewString()

	reason, err := reserveSMSLua.Run(
		ctx,
		l.client,
		keys,
		policy.IPMinute,
		policy.PhoneMinute,
		policy.IPDay,
		policy.PhoneDay,
		id,
	).Int64()
	if err != nil {
		return nil, SMSLimitAllowed, fmt.Errorf("检查短信发送额度失败: %w", err)
	}
	if reason != 0 {
		return nil, SMSLimitReason(reason), nil
	}

	return &SMSReservation{id: id, keys: keys}, SMSLimitAllowed, nil
}

func (l *smsLimit) Release(ctx context.Context, reservation *SMSReservation) error {
	if reservation == nil {
		return nil
	}

	cleanupCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx), 3*time.Second,
	)
	defer cancel()

	if err := releaseSMSLua.Run(cleanupCtx, l.client, reservation.keys, reservation.id).Err(); err != nil {
		return fmt.Errorf("退还短信发送额度失败: %w", err)
	}
	return nil
}
