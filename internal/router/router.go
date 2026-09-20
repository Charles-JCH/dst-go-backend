package router

import (
	"game-panel/internal/config"
	"game-panel/internal/handler/v1"
	"game-panel/internal/middleware"
	"game-panel/internal/platform/redis"
	"game-panel/internal/service"
	"github.com/gin-gonic/gin"
	"time"
)

func InitRouter(
	cfg config.ServerConfig,
	limiter redis.Limiter,
	authHandler *v1.AuthHandler,
	authService service.AuthService,
) *gin.Engine {
	gin.SetMode(cfg.Mode)
	r := gin.New()

	if err := r.SetTrustedProxies([]string{"127.0.0.1"}); err != nil {
		panic(err)
	}

	// 挂载全局中间件
	r.Use(middleware.Trace())
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())
	r.Use(middleware.CORS(cfg.AllowedOrigins))

	// 每秒最多请求 100 次
	r.Use(middleware.RateLimit(limiter, 100, time.Second))

	// 注册路由
	RegisterV1Routes(r, authHandler, authService)

	return r
}
