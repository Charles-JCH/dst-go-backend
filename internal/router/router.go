package router

import (
	"game-panel/internal/agent_manager"
	"game-panel/internal/config"
	"game-panel/internal/handler/v1"
	"game-panel/internal/middleware"
	"game-panel/internal/platform/redis"
	"game-panel/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"regexp"
	"time"
)

func InitRouter(
	cfg config.ServerConfig,
	limiter redis.Limiter,
	authService service.AuthService,
	authHandler *v1.AuthHandler,
	agentHandler *agent_manager.Handler,
) *gin.Engine {
	gin.SetMode(cfg.Mode)

	phonePattern := regexp.MustCompile(`^1[3-9][0-9]{9}$`)
	validatorEngine := binding.Validator.Engine().(*validator.Validate)

	if err := validatorEngine.RegisterValidation("cnphone", func(fl validator.FieldLevel) bool {
		return phonePattern.MatchString(fl.Field().String())
	}); err != nil {
		panic(err)
	}

	r := gin.New()

	if err := r.SetTrustedProxies([]string{"127.0.0.1"}); err != nil {
		panic(err)
	}

	// 挂载全局中间件
	r.Use(middleware.Trace())
	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())
	r.Use(middleware.CORS(cfg.AllowedOrigins))

	// 注册路由
	RegisterV1Routes(r, authService, authHandler, agentHandler, middleware.RateLimit(limiter, 100, time.Second))

	return r
}
