package router

import (
	"game-panel/internal/agent_manager"
	authhandler "game-panel/internal/handler/v1/auth"
	"game-panel/internal/handler/v1/dst"
	"game-panel/internal/handler/v1/server"
	"game-panel/internal/handler/v1/task"
	"game-panel/internal/middleware"
	authservice "game-panel/internal/service/auth"
	"github.com/gin-gonic/gin"
)

func RegisterV1Routes(
	r *gin.Engine,
	authService authservice.Service,
	authHandler *authhandler.Handler,
	serverHandler *server.Handler,
	dstHandler *dst.Handler,
	taskHandler *task.Handler,
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
		serverGroup := protected.Group("/server")
		{
			serverGroup.POST("", serverHandler.Create)
			serverGroup.GET("", serverHandler.List)
			serverGroup.GET("/:id", serverHandler.GetByID)
			serverGroup.PUT("/:id", serverHandler.Update)
			serverGroup.DELETE("/:id", serverHandler.Delete)
		}
		dstGroup := protected.Group("/dst")
		{
			dstGroup.POST("", dstHandler.Create)
			dstGroup.GET("", dstHandler.List)
			dstGroup.GET("/:id", dstHandler.GetByID)
			dstGroup.PUT("/:id", dstHandler.Update)
			dstGroup.DELETE("/:id", dstHandler.Delete)
		}
		taskGroup := protected.Group("/tasks")
		{
			taskGroup.GET("", taskHandler.List)
			taskGroup.GET("/:id", taskHandler.GetByID)
		}
	}

	agentGroup := v1Group.Group("/agent")
	{
		agentGroup.GET("/ws", agentHandler.ServeWS)
	}
}
