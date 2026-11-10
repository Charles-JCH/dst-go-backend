package router

import (
	"github.com/gin-gonic/gin"
)

func registerV1Routes(r *gin.Engine, h Handlers, rateLimit, auth gin.HandlerFunc) {
	v1 := r.Group("/api/v1")
	public := v1.Group("", rateLimit)

	authGroup := public.Group("/auth")
	{
		authGroup.GET("/captcha", h.Auth.GetCaptcha)
		authGroup.POST("/sms-code", h.Auth.SendSMSCode)
		authGroup.POST("/register", h.Auth.Register)
		authGroup.POST("/login", h.Auth.Login)
		authGroup.POST("/login/sms", h.Auth.LoginBySMS)
		authGroup.POST("/refresh", h.Auth.RefreshToken)
		authGroup.POST("/logout", h.Auth.Logout)
		authGroup.POST("/password/verify", h.Auth.VerifyResetCode)
		authGroup.POST("/password/reset", h.Auth.ResetPassword)
	}

	protected := public.Group("", auth)
	{
		protected.GET("/auth/profile", h.Auth.GetProfile)
		protected.GET("/game-env", h.GameEnv.List)

		serverGroup := protected.Group("/server")
		{
			serverGroup.POST("", h.Server.Create)
			serverGroup.GET("", h.Server.List)
			serverGroup.GET("/:id", h.Server.GetByID)
			serverGroup.PUT("/:id", h.Server.Update)
			serverGroup.DELETE("/:id", h.Server.Delete)
			serverGroup.POST("/:id/agent/install", h.Server.InstallAgent)
			serverGroup.GET("/:id/metrics", h.Server.ListMetrics)
			serverGroup.POST("/:id/deploy", h.Server.DeployGame)
		}

		dstGroup := protected.Group("/dst")
		{
			dstGroup.POST("/init", h.Dst.Init)
			dstGroup.POST("", h.Dst.Create)
			dstGroup.GET("", h.Dst.List)
			dstGroup.GET("/:id", h.Dst.GetByID)
			dstGroup.PUT("/:id", h.Dst.Update)
			dstGroup.DELETE("/:id", h.Dst.Delete)
			dstGroup.POST("/:id/start", h.Dst.Start)
			dstGroup.POST("/:id/stop", h.Dst.Stop)
		}

		taskGroup := protected.Group("/tasks")
		{
			taskGroup.GET("", h.Task.List)
			taskGroup.GET("/active", h.Task.ListActive)
			taskGroup.GET("/:id", h.Task.GetByID)
			taskGroup.GET("/:id/logs", h.Task.ListLogs)
		}

		protected.POST("/ws/ticket", h.Auth.IssueWSTicket)
	}

	// 浏览器实时事件：连接后用一次性 ticket 认证，不走 Authorization 头
	public.GET("/ws", h.Realtime.ServeWS)

	// Agent 使用自己的令牌认证，不走用户鉴权和 IP 限流
	agentGroup := v1.Group("/agent")
	{
		agentGroup.GET("/ws", h.Agent.ServeWS)
	}
}
