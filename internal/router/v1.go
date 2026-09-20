package router

import (
	"game-panel/internal/handler/v1"
	"game-panel/internal/middleware"
	"game-panel/internal/service"
	"github.com/gin-gonic/gin"
)

func RegisterV1Routes(
	r *gin.Engine,
	authHandler *v1.AuthHandler,
	authService service.AuthService,
) {
	v1Group := r.Group("/api/v1")

	authGroup := v1Group.Group("/auth")
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

	protected := v1Group.Group("")
	protected.Use(middleware.Auth(authService))
	{
		protected.GET("/auth/profile", authHandler.GetProfile)
	}
}
