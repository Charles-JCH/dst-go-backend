package captcha

import (
	"context"
	"github.com/mojocn/base64Captcha"
	"github.com/redis/go-redis/v9"
	"time"
)

type Config struct {
	Expire    time.Duration
	ImgWidth  int
	ImgHeight int
	Length    int
	MaxSkew   float64
	DotCount  int
}

type Captcha interface {
	Generate() (id string, b64s string, err error)
	Verify(ctx context.Context, id string, answer string, clear bool) (bool, error)
}

type captcha struct {
	driver *base64Captcha.DriverDigit
	store  *RedisStore
}

func NewCaptcha(cfg Config, rdb *redis.Client) Captcha {
	store := NewRedisStore(rdb, "captcha:", cfg.Expire)

	// 图形驱动
	driver := base64Captcha.NewDriverDigit(
		cfg.ImgHeight,
		cfg.ImgWidth,
		cfg.Length,
		cfg.MaxSkew,
		cfg.DotCount,
	)

	return &captcha{
		driver: driver,
		store:  store,
	}
}

// Generate 生成验证码
// base64Captcha 的 Generate 和 Store.Set 都不接收 ctx，因此这里没有 ctx 参数，写入 Redis 时使用 context.Background()
func (c *captcha) Generate() (id string, b64s string, err error) {
	cp := base64Captcha.NewCaptcha(c.driver, c.store)
	id, b64s, _, err = cp.Generate()
	return id, b64s, err
}

// Verify 校验验证码，绕过 base64Captcha 直接读取 store，以便带上 ctx 并把 Redis 错误返回给调用方
func (c *captcha) Verify(ctx context.Context, id string, answer string, clear bool) (bool, error) {
	value, err := c.store.GetWithContext(ctx, id, clear)
	if err != nil {
		return false, err
	}
	return value != "" && value == answer, nil
}
