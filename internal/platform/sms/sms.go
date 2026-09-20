package sms

import (
	"context"
)

type Client interface {
	SendSMS(ctx context.Context, phone string, code string) error
}
