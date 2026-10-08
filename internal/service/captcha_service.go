package service

import (
	"context"
	"game-panel/internal/apperr"
	"game-panel/internal/model/response"
	"game-panel/internal/platform/captcha"
	"log/slog"
)

type CaptchaService interface {
	GenerateCaptcha(ctx context.Context) (*response.GetCaptchaResp, error)
	Verify(ctx context.Context, id string, answer string, clear bool) (bool, error)
}

type captchaService struct {
	captcha captcha.Captcha
}

func NewCaptchaService(cp captcha.Captcha) CaptchaService {
	return &captchaService{captcha: cp}
}

// GenerateCaptcha 获取图形验证码
func (s *captchaService) GenerateCaptcha(ctx context.Context) (*response.GetCaptchaResp, error) {
	id, b64s, err := s.captcha.Generate()
	if err != nil {
		slog.ErrorContext(ctx, "图形验证码生成失败", "error", err)
		return nil, apperr.NewBizError(500, "图形验证码生成失败")
	}
	return &response.GetCaptchaResp{
		CaptchaID:  id,
		CaptchaImg: b64s,
	}, nil
}

func (s *captchaService) Verify(ctx context.Context, id string, answer string, clear bool) (bool, error) {
	return s.captcha.Verify(ctx, id, answer, clear)
}
