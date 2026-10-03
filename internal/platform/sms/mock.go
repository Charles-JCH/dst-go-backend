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
	slog.InfoContext(ctx, "模拟短信发送", "phone", phone, "code", code)
	return nil
}
