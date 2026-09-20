package sms

import (
	"context"
	"log/slog"
)

type mockSMSClient struct{}

func NewMockSMSClient() Client {
	return &mockSMSClient{}
}

func (m *mockSMSClient) SendSMS(ctx context.Context, phone, code string) error {
	slog.InfoContext(ctx, "【模拟短信】发送验证码通知", "手机号", phone, "验证码", code)
	return nil
}
