package service

import (
	"context"
	"game-panel/internal/apperr"
	"game-panel/internal/model/response"
	"game-panel/internal/platform/captcha"
	"log/slog"
)

type CaptchaService interface {
	GenerateCaptcha(ctx context.Context) (response.GetCaptchaResp, error)
	Verify(ctx context.Context, id string, answer string, clear bool) bool
}

type captchaService struct {
	captcha captcha.Captcha
}

func NewCaptchaService(cp captcha.Captcha) CaptchaService {
	return &captchaService{captcha: cp}
}

// GenerateCaptcha 获取图形验证码
func (s *captchaService) GenerateCaptcha(ctx context.Context) (response.GetCaptchaResp, error) {
	id, b64s, err := s.captcha.Generate()
	if err != nil {
		slog.ErrorContext(ctx, "生成图形验证码失败", "错误", err)
		return response.GetCaptchaResp{}, apperr.NewBizError(500, "图形验证码生成失败")
	}
	return response.GetCaptchaResp{
		CaptchaId:  id,
		CaptchaImg: b64s,
	}, nil
}

func (s *captchaService) Verify(_ context.Context, id string, answer string, clear bool) bool {
	return s.captcha.Verify(id, answer, clear)
}
