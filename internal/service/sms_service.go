package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"game-panel/internal/apperr"
	"game-panel/internal/model/request"
	"game-panel/internal/platform/captcha"
	"game-panel/internal/platform/sms"
	"game-panel/internal/repository/cache"
	"log/slog"
	"math/big"
	"time"
)

type SMSService interface {
	SendCode(ctx context.Context, clientIP string, req *request.SendSMSCodeReq) error
	VerifyCode(ctx context.Context, phone, code string) (bool, error)
}

type smsService struct {
	captcha   captcha.Captcha
	smsCache  cache.SMSCache
	smsClient sms.Client
	smsLimit  cache.SMSLimit
}

func NewSMSService(
	captcha captcha.Captcha,
	smsCache cache.SMSCache,
	smsClient sms.Client,
	smsLimit cache.SMSLimit,
) SMSService {
	return &smsService{
		captcha:   captcha,
		smsCache:  smsCache,
		smsClient: smsClient,
		smsLimit:  smsLimit,
	}
}

func (s *smsService) SendCode(ctx context.Context, clientIP string, req *request.SendSMSCodeReq) error {
	if clientIP == "" {
		return apperr.NewBizError(400, "无法获取客户端 IP")
	}

	// 校验并销毁图形验证码
	if !s.captcha.Verify(req.CaptchaId, req.CaptchaCode, true) {
		return apperr.NewBizError(400, "图形验证码错误或已过期")
	}

	// 生成 6 位纯数字随机验证码
	number, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		slog.ErrorContext(ctx, "生成短信验证码失败", "错误", err)
		return apperr.NewBizError(500, "短信服务暂时不可用")
	}
	code := fmt.Sprintf("%06d", number.Int64()+100000)

	// 一次检查并预占四项额度
	reservation, reason, err := s.smsLimit.Reserve(
		ctx,
		clientIP,
		req.Phone,
		cache.SMSLimitPolicy{
			IPMinute:    10,
			PhoneMinute: 1,
			IPDay:       50,
			PhoneDay:    5,
		},
	)
	if err != nil {
		slog.ErrorContext(ctx, "检查短信发送额度失败", "错误", err)
		return apperr.NewBizError(503, "短信服务暂时不可用")
	}
	switch reason {
	case cache.SMSLimitAllowed:
		// 所有额度均允许，继续发送。
	case cache.SMSLimitIPMinute:
		return apperr.NewBizError(429, "当前 IP 发送过于频繁，请一分钟后再试")
	case cache.SMSLimitPhoneMinute:
		return apperr.NewBizError(429, "该手机号发送过于频繁，请一分钟后再试")
	case cache.SMSLimitIPDay:
		return apperr.NewBizError(429, "当前 IP 今日短信次数已用完")
	case cache.SMSLimitPhoneDay:
		return apperr.NewBizError(429, "该手机号今日短信次数已用完")
	default:
		slog.ErrorContext(ctx, "短信限流返回未知结果", "原因", reason)
		return apperr.NewBizError(503, "短信服务暂时不可用")
	}

	if err := s.smsCache.SetCode(ctx, req.Phone, code, 5*time.Minute); err != nil {
		// 尚未调用短信服务，可以退还本次额度。
		if releaseErr := s.smsLimit.Release(ctx, reservation); releaseErr != nil {
			slog.ErrorContext(ctx, "退还短信发送额度失败", "错误", releaseErr)
		}
		slog.ErrorContext(ctx, "保存短信验证码失败", "错误", err)
		return apperr.NewBizError(503, "短信服务暂时不可用")
	}

	// 调用短信客户端发送验证码
	if err := s.smsClient.SendSMS(ctx, req.Phone, code); err != nil {
		slog.ErrorContext(ctx, "短信发送未能确认成功", "错误", err)

		// 可能已经发送成功，保留验证码和已占用额度。
		return apperr.NewBizError(503, "短信发送结果暂未确认，请留意短信，稍后再试")
	}

	return nil
}

func (s *smsService) VerifyCode(ctx context.Context, phone, code string) (bool, error) {
	// 限流
	valid, err := s.smsCache.ConsumeCode(ctx, phone, code)
	if errors.Is(err, cache.ErrSMSCodeAttemptsExceeded) {
		return false, apperr.NewBizError(400, "验证码已连续输错三次，请重新发送")
	}

	if err != nil {
		slog.ErrorContext(ctx, "校验短信验证码失败", "错误", err)
		return false, apperr.NewBizError(503, "短信服务暂时不可用")
	}

	return valid, nil
}
