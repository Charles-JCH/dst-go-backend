package captcha

import (
	"context"
	"game-panel/internal/apperr"
	"game-panel/internal/model/response"
	"game-panel/internal/platform/captcha"
	"log/slog"
)

type Service interface {
	GenerateCaptcha(ctx context.Context) (*response.GetCaptchaResp, error)
	Verify(ctx context.Context, id string, answer string, clear bool) (bool, error)
}

type service struct {
	captcha captcha.Captcha
}

func NewService(cp captcha.Captcha) Service {
	return &service{captcha: cp}
}

// GenerateCaptcha 获取图形验证码
func (s *service) GenerateCaptcha(ctx context.Context) (*response.GetCaptchaResp, error) {
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

func (s *service) Verify(ctx context.Context, id string, answer string, clear bool) (bool, error) {
	return s.captcha.Verify(ctx, id, answer, clear)
}
