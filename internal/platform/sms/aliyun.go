package sms

import (
	"context"
	"encoding/json"
	"fmt"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	"github.com/alibabacloud-go/tea/dara"
	"log/slog"

	dysmsapi "github.com/alibabacloud-go/dysmsapi-20170525/v5/client"
)

type Config struct {
	AccessKeyId     string
	AccessKeySecret string
	SignName        string
	TemplateCode    string
}

type aliyunClient struct {
	client       *dysmsapi.Client
	signName     string
	templateCode string
}

func NewAliyunSMSClient(cfg Config) (Client, error) {
	client, err := dysmsapi.NewClient(&openapi.Config{
		AccessKeyId:     dara.String(cfg.AccessKeyId),
		AccessKeySecret: dara.String(cfg.AccessKeySecret),
		Endpoint:        dara.String("dysmsapi.aliyuncs.com"),
		RegionId:        dara.String("cn-hangzhou"),
	})
	if err != nil {
		return nil, fmt.Errorf("创建阿里云短信客户端失败: %w", err)
	}

	return &aliyunClient{
		client:       client,
		signName:     cfg.SignName,
		templateCode: cfg.TemplateCode,
	}, nil
}

// SendSMS 发送短信验证码
func (c *aliyunClient) SendSMS(ctx context.Context, phone string, code string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// 参数名 code 对应短信模板中的 ${code}。
	templateParam, err := json.Marshal(map[string]string{"code": code})
	if err != nil {
		return fmt.Errorf("序列化短信模板参数失败: %w", err)
	}

	request := &dysmsapi.SendSmsRequest{
		PhoneNumbers:  dara.String(phone),
		SignName:      dara.String(c.signName),
		TemplateCode:  dara.String(c.templateCode),
		TemplateParam: dara.String(string(templateParam)),
	}

	runtime := &dara.RuntimeOptions{
		ConnectTimeout: dara.Int(3000),
		ReadTimeout:    dara.Int(5000),

		// 发送接口不具备幂等性，禁止自动重试，避免重复发送。
		Autoretry: dara.Bool(false),
	}

	response, err := c.client.SendSmsWithOptions(request, runtime)
	if err != nil {
		return fmt.Errorf("调用阿里云短信接口失败: %w", err)
	}

	if response == nil || response.Body == nil {
		return fmt.Errorf("阿里云短信接口返回空响应")
	}

	body := response.Body
	if dara.StringValue(body.Code) != "OK" {
		return fmt.Errorf(
			"阿里云短信提交失败: code=%s, message=%s, request_id=%s",
			dara.StringValue(body.Code),
			dara.StringValue(body.Message),
			dara.StringValue(body.RequestId),
		)
	}

	slog.InfoContext(ctx, "阿里云短信已受理", "request_id", dara.StringValue(body.RequestId), "biz_id", dara.StringValue(body.BizId))

	return nil
}
