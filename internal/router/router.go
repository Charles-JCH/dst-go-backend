package router

import (
	"fmt"
	"game-panel/internal/agentmanager"
	"game-panel/internal/config"
	"game-panel/internal/handler/v1/auth"
	"game-panel/internal/handler/v1/dst"
	"game-panel/internal/handler/v1/gameenv"
	"game-panel/internal/handler/v1/server"
	"game-panel/internal/handler/v1/task"
	"game-panel/internal/middleware"
	"game-panel/internal/platform/redis"
	authservice "game-panel/internal/service/auth"
	"github.com/gin-gonic/gin"
	"time"
)

// Handlers 汇总所有接口的 handler
type Handlers struct {
	Auth    *auth.Handler
	GameEnv *gameenv.Handler
	Server  *server.Handler
	Dst     *dst.Handler
	Task    *task.Handler
	Agent   *agentmanager.Handler
}

func InitRouter(cfg config.ServerConfig, limiter redis.Limiter, authService authservice.Service, h Handlers) (*gin.Engine, error) {
	gin.SetMode(cfg.Mode)

	r := gin.New()
	if err := r.SetTrustedProxies([]string{"127.0.0.1"}); err != nil {
		return nil, fmt.Errorf("可信代理设置失败: %w", err)
	}

	// 全局中间件
	r.Use(
		middleware.Trace(),
		middleware.Logger(),
		middleware.Recovery(),
		middleware.CORS(cfg.AllowedOrigins),
	)

	registerV1Routes(r, h,
		middleware.RateLimit(limiter, 20, time.Second),
		middleware.Auth(authService),
	)
	return r, nil
}
