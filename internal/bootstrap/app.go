package bootstrap

import (
	"fmt"
	"game-panel/internal/agentmanager"
	"game-panel/internal/config"
	authhandler "game-panel/internal/handler/v1/auth"
	dsthandler "game-panel/internal/handler/v1/dst"
	gameenvhandler "game-panel/internal/handler/v1/gameenv"
	serverhandler "game-panel/internal/handler/v1/server"
	taskhandler "game-panel/internal/handler/v1/task"
	"game-panel/internal/platform/captcha"
	"game-panel/internal/platform/crypto"
	"game-panel/internal/platform/jwt"
	"game-panel/internal/platform/redis"
	"game-panel/internal/platform/sms"
	"game-panel/internal/repository"
	"game-panel/internal/repository/cache"
	dstrepo "game-panel/internal/repository/dst"
	gameenvrepo "game-panel/internal/repository/gameenv"
	serverrepo "game-panel/internal/repository/server"
	taskrepo "game-panel/internal/repository/task"
	"game-panel/internal/repository/user"
	"game-panel/internal/router"
	"game-panel/internal/service/agentevent"
	authservice "game-panel/internal/service/auth"
	captchaservice "game-panel/internal/service/captcha"
	dstservice "game-panel/internal/service/dst"
	gameenvservice "game-panel/internal/service/gameenv"
	serverservice "game-panel/internal/service/server"
	smsservice "game-panel/internal/service/sms"
	taskservice "game-panel/internal/service/task"
	"game-panel/internal/validator"
	"log/slog"
)

func Run(configPath string) error {
	// 初始化配置
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("配置加载失败: %w", err)
	}

	// 初始化日志
	if _, err := InitLogger(cfg.Log, cfg.App.Env); err != nil {
		return err
	}

	// 初始化 AES 加密组件
	encryptor, err := crypto.NewAESEncryptor(cfg.Crypto.AESKey)
	if err != nil {
		return fmt.Errorf("AES 加密组件初始化失败: %w", err)
	}

	// 初始化 SQLite
	sqlite, err := InitSQLite(cfg.SQLite)
	if err != nil {
		return err
	}
	sqlDB, err := sqlite.DB()
	if err != nil {
		return fmt.Errorf("数据库连接池获取失败: %w", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			slog.Error("sqlite 关闭失败", "error", err)
		}
	}()

	// 初始化 Redis
	rdb, err := InitRedis(cfg.Redis)
	if err != nil {
		return err
	}
	defer func() {
		if err := rdb.Close(); err != nil {
			slog.Error("redis 关闭失败", "error", err)
		}
	}()

	// 基础组件
	limiter := redis.NewLimiter(rdb)
	captchaManager := captcha.NewCaptcha(captcha.Config(cfg.Captcha), rdb)
	jwtManager := jwt.NewTokenManager(jwt.Config(cfg.JWT))
	passwordHasher := crypto.NewPasswordHasher()
	agentHub := agentmanager.NewHub()

	// 初始化 SMS
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
		return fmt.Errorf("不支持短信服务商 provider=%q", cfg.SMS.Provider)
	}

	// repository
	userRepo := user.NewRepository(sqlite)
	gameEnvRepo := gameenvrepo.NewRepository(sqlite)
	serverRepo := serverrepo.NewRepository(sqlite)
	dstRepo := dstrepo.NewRepository(sqlite)
	taskRepo := taskrepo.NewRepository(sqlite)
	txManager := repository.NewTransaction(sqlite)
	authCache := cache.NewAuthCache(rdb)
	smsCache := cache.NewSMSCache(rdb)
	smsLimit := cache.NewSMSLimit(rdb)

	agentService := agentmanager.NewService(agentHub)

	// service
	captchaService := captchaservice.NewService(captchaManager)
	smsService := smsservice.NewService(captchaManager, smsCache, smsClient, smsLimit, cache.SMSLimitPolicy(cfg.SMS.Limit))
	authService := authservice.NewService(captchaService, smsService, jwtManager, authCache, userRepo, txManager, passwordHasher)
	gameEnvService := gameenvservice.NewService(gameEnvRepo)
	serverService := serverservice.NewService(serverRepo, taskRepo, txManager, encryptor, agentHub)
	dstService := dstservice.NewService(dstRepo, serverRepo, txManager, encryptor, agentService)
	taskService := taskservice.NewService(taskRepo)

	// handler
	authHandler := authhandler.NewHandler(authService, cfg.JWT.RefreshExpire, cfg.Server.CookieSecure)
	gameEnvHandler := gameenvhandler.NewHandler(gameEnvService)
	serverHandler := serverhandler.NewHandler(serverService)
	dstHandler := dsthandler.NewHandler(dstService)
	taskHandler := taskhandler.NewHandler(taskService)

	// agent
	agentAuth := agentmanager.NewAuth(jwtManager, serverRepo)
	agentEvents := agentevent.NewService()
	agentService.OnAck = agentEvents.OnAck
	agentService.OnLog = agentEvents.OnLog
	agentService.OnResult = agentEvents.OnResult
	agentService.OnInterrupted = agentEvents.OnInterrupted
	agentHandler := agentmanager.NewHandler(agentHub, agentAuth, agentService)

	// 注册参数校验规则
	if err := validator.Register(); err != nil {
		return fmt.Errorf("参数校验规则注册失败: %w", err)
	}

	// 注册路由
	engine, err := router.InitRouter(cfg.Server, limiter, authService, router.Handlers{
		Auth:    authHandler,
		GameEnv: gameEnvHandler,
		Server:  serverHandler,
		Dst:     dstHandler,
		Task:    taskHandler,
		Agent:   agentHandler,
	})
	if err != nil {
		return err
	}

	return RunHTTPServer(cfg.Server, engine)
}
