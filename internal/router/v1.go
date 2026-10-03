package router

import (
	"game-panel/internal/agent_manager"
	"game-panel/internal/handler/v1"
	"game-panel/internal/middleware"
	"game-panel/internal/service"
	"github.com/gin-gonic/gin"
)

func RegisterV1Routes(
	r *gin.Engine,
	authService service.AuthService,
	authHandler *v1.AuthHandler,
	agentHandler *agent_manager.Handler,
	rateLimit gin.HandlerFunc,
) {
	v1Group := r.Group("/api/v1")
	rateLimitGroup := v1Group.Group("", rateLimit)
	authGroup := rateLimitGroup.Group("/auth")
	{
		authGroup.GET("/captcha", authHandler.GetCaptcha)
		authGroup.POST("/sms-code", authHandler.SendSMSCode)
		authGroup.POST("/register", authHandler.Register)
		authGroup.POST("/login", authHandler.Login)
		authGroup.POST("/login/sms", authHandler.LoginBySMS)
		authGroup.POST("/refresh", authHandler.RefreshToken)
		authGroup.POST("/logout", authHandler.Logout)
		authGroup.POST("/password/verify", authHandler.VerifyResetCode)
		authGroup.POST("/password/reset", authHandler.ResetPassword)
	}
	protected := rateLimitGroup.Group("")
	protected.Use(middleware.Auth(authService))
	{
		protected.GET("/auth/profile", authHandler.GetProfile)
	}

	agentGroup := v1Group.Group("/agent")
	{
		agentGroup.GET("/ws", agentHandler.ServeWS)
	}
}
