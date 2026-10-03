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
		return apperr.NewBizError(400, "客户端 ip 缺失")
	}

	// 校验并销毁图形验证码
	valid, err := s.captcha.Verify(ctx, req.CaptchaID, req.CaptchaCode, true)
	if err != nil {
		slog.ErrorContext(ctx, "图形验证码读取失败", "error", err)
		return apperr.NewBizError(503, "图形验证码服务不可用")
	}
	if !valid {
		return apperr.NewBizError(400, "图形验证码错误或已过期")
	}

	// 生成 6 位纯数字随机验证码
	number, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		slog.ErrorContext(ctx, "短信验证码生成失败", "error", err)
		return apperr.NewBizError(500, "短信服务不可用")
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
		slog.ErrorContext(ctx, "短信发送额度检查失败", "error", err)
		return apperr.NewBizError(503, "短信服务不可用")
	}
	switch reason {
	case cache.SMSLimitAllowed:
		// 所有额度均允许，继续发送。
	case cache.SMSLimitIPMinute:
		return apperr.NewBizError(429, "ip 短信发送过于频繁")
	case cache.SMSLimitPhoneMinute:
		return apperr.NewBizError(429, "手机号短信发送过于频繁")
	case cache.SMSLimitIPDay:
		return apperr.NewBizError(429, "ip 今日短信次数已用完")
	case cache.SMSLimitPhoneDay:
		return apperr.NewBizError(429, "手机号今日短信次数已用完")
	default:
		slog.ErrorContext(ctx, "短信限流结果未知", "reason", reason)
		return apperr.NewBizError(503, "短信服务不可用")
	}

	if err := s.smsCache.SetCode(ctx, req.Phone, code, 5*time.Minute); err != nil {
		// 尚未调用短信服务，可以退还本次额度。
		if releaseErr := s.smsLimit.Release(ctx, reservation); releaseErr != nil {
			slog.ErrorContext(ctx, "短信发送额度退还失败", "error", releaseErr)
		}
		slog.ErrorContext(ctx, "短信验证码保存失败", "error", err)
		return apperr.NewBizError(503, "短信服务不可用")
	}

	// 调用短信客户端发送验证码
	if err := s.smsClient.SendSMS(ctx, req.Phone, code); err != nil {
		slog.ErrorContext(ctx, "短信发送结果未知", "error", err)

		// 可能已经发送成功，保留验证码和已占用额度。
		return apperr.NewBizError(503, "短信发送结果未知")
	}

	return nil
}

func (s *smsService) VerifyCode(ctx context.Context, phone, code string) (bool, error) {
	// 限流
	valid, err := s.smsCache.ConsumeCode(ctx, phone, code)
	if errors.Is(err, cache.ErrSMSCodeAttemptsExceeded) {
		return false, apperr.NewBizError(400, "短信验证码已连续输错 3 次")
	}

	if err != nil {
		slog.ErrorContext(ctx, "短信验证码校验失败", "error", err)
		return false, apperr.NewBizError(503, "短信服务不可用")
	}

	return valid, nil
}
