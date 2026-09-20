package bootstrap

import (
	"fmt"
	"game-panel/internal/config"
	"game-panel/internal/handler/v1"
	"game-panel/internal/platform/captcha"
	"game-panel/internal/platform/crypto"
	"game-panel/internal/platform/jwt"
	"game-panel/internal/platform/redis"
	"game-panel/internal/platform/sms"
	"game-panel/internal/repository"
	"game-panel/internal/repository/cache"
	"game-panel/internal/router"
	"game-panel/internal/service"
	"log/slog"
)

func Run(configPath string) error {
	// 初始化配置
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 初始化日志
	if _, err := InitLogger(cfg.Log, cfg.App.Env); err != nil {
		return err
	}

	// 初始化 SQLite
	sqlite, err := InitSQLite(cfg.SQLite)
	if err != nil {
		return err
	}
	sqlDB, err := sqlite.DB()
	if err != nil {
		return fmt.Errorf("获取数据库连接池失败: %w", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			slog.Error("关闭 SQLite 失败", "错误", err)
		}
	}()

	// 初始化 Redis
	rdb, err := InitRedis(cfg.Redis)
	if err != nil {
		return err
	}
	defer func() {
		if err := rdb.Close(); err != nil {
			slog.Error("关闭 Redis 失败", "错误", err)
		}
	}()

	// 基础组件
	limiter := redis.NewLimiter(rdb)
	captchaManager := captcha.NewCaptcha(captcha.Config(cfg.Captcha), rdb)
	jwtManager := jwt.NewTokenManager(jwt.Config(cfg.JWT))
	passwordHasher := crypto.NewPasswordHasher()

	var smsClient sms.Client
	switch cfg.SMS.Provider {
	case "aliyun":
		smsClient, err = sms.NewAliyunSMSClient(sms.Config(cfg.SMS.Aliyun))
		if err != nil {
			return err
		}
	case "mock":
		smsClient = sms.NewMockSMSClient()
	default:
		return fmt.Errorf("不支持的短信服务商: %q", cfg.SMS.Provider)
	}

	// repository
	userRepo := repository.NewUserRepository(sqlite)
	txManager := repository.NewTransaction(sqlite)
	authCache := cache.NewAuthCache(rdb)
	smsCache := cache.NewSMSCache(rdb)
	smsLimit := cache.NewSMSLimit(rdb)

	// service
	captchaService := service.NewCaptchaService(captchaManager)
	smsService := service.NewSMSService(captchaManager, smsCache, smsClient, smsLimit)
	authService := service.NewAuthService(captchaService, smsService, jwtManager, authCache, userRepo, txManager, passwordHasher)

	// handler
	authHandler := v1.NewAuthHandler(authService, cfg.JWT.RefreshExpire)

	// 注册路由
	engine := router.InitRouter(cfg.Server, limiter, authHandler, authService)

	return RunHTTPServer(cfg.Server, engine)
}
