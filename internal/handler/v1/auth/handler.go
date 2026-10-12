package auth

import (
	"game-panel/internal/model/request"
	"game-panel/internal/result"
	"game-panel/internal/service/auth"
	"github.com/gin-gonic/gin"
	"strings"
	"time"
)

const (
	RefreshTokenCookieName = "refresh_token"
	RefreshTokenCookiePath = "/api/v1/auth"
)

type Handler struct {
	authService   auth.Service
	refreshExpire time.Duration
	cookieSecure  bool
}

func NewHandler(authService auth.Service, refreshExpire time.Duration, cookieSecure bool) *Handler {
	return &Handler{
		authService:   authService,
		refreshExpire: refreshExpire,
		cookieSecure:  cookieSecure,
	}
}

// GetCaptcha 获取图形验证码
// [GET] /api/v1/auth/captcha
func (h *Handler) GetCaptcha(c *gin.Context) {
	resp, err := h.authService.GenerateCaptcha(c.Request.Context())
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}

// SendSMSCode 发送短信验证码
// [POST] /api/v1/auth/sms-code
func (h *Handler) SendSMSCode(c *gin.Context) {
	var req request.SendSMSCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数无效")
		return
	}
	if err := h.authService.SendSMSCode(c.Request.Context(), c.ClientIP(), &req); err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, nil)
}

// Register 注册
// [POST] /api/v1/auth/register
func (h *Handler) Register(c *gin.Context) {
	var req request.RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数无效")
		return
	}
	resp, refreshToken, err := h.authService.Register(c.Request.Context(), &req)
	if err != nil {
		result.Fail(c, err)
		return
	}
	h.setRefreshTokenCookie(c, refreshToken)
	result.Success(c, resp)
}

// Login 密码登录
// [POST] /api/v1/auth/login
func (h *Handler) Login(c *gin.Context) {
	var req request.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数无效")
		return
	}
	resp, refreshToken, err := h.authService.Login(c.Request.Context(), &req)
	if err != nil {
		result.Fail(c, err)
		return
	}
	h.setRefreshTokenCookie(c, refreshToken)
	result.Success(c, resp)
}

// LoginBySMS 短信免密登录
// [POST] /api/v1/auth/login/sms
func (h *Handler) LoginBySMS(c *gin.Context) {
	var req request.LoginBySMSReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数无效")
		return
	}
	resp, refreshToken, err := h.authService.LoginBySMS(c.Request.Context(), &req)
	if err != nil {
		result.Fail(c, err)
		return
	}
	h.setRefreshTokenCookie(c, refreshToken)
	result.Success(c, resp)
}

// RefreshToken 刷新 AccessToken
// [POST] /api/v1/auth/refresh
func (h *Handler) RefreshToken(c *gin.Context) {
	refreshToken, err := c.Cookie(RefreshTokenCookieName)
	if err != nil || refreshToken == "" {
		result.FailWithMsg(c, 401, "刷新凭证缺失")
		return
	}
	newAccessToken, newRefreshToken, err := h.authService.RefreshToken(c.Request.Context(), refreshToken)
	if err != nil {
		result.Fail(c, err)
		return
	}
	h.setRefreshTokenCookie(c, newRefreshToken)
	result.Success(c, gin.H{
		"accessToken": newAccessToken,
	})
}

// Logout 退出登录
// [POST] /api/v1/auth/logout
func (h *Handler) Logout(c *gin.Context) {
	parts := strings.Fields(c.GetHeader("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		result.FailWithMsg(c, 400, "登录凭证缺失或格式错误")
		return
	}
	accessToken := parts[1]
	refreshToken, err := c.Cookie(RefreshTokenCookieName)
	if err != nil {
		refreshToken = ""
	}
	if err = h.authService.Logout(c.Request.Context(), accessToken, refreshToken); err != nil {
		result.Fail(c, err)
		return
	}
	h.clearRefreshTokenCookie(c)
	result.Success(c, nil)
}

// VerifyResetCode 重置密码第一步
// [POST] /api/v1/auth/password/verify
func (h *Handler) VerifyResetCode(c *gin.Context) {
	var req request.VerifyResetCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数无效")
		return
	}
	resetToken, err := h.authService.VerifyResetCode(c.Request.Context(), &req)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, gin.H{
		"resetToken": resetToken,
	})
}

// ResetPassword 重置密码第二步
// [POST] /api/v1/auth/password/reset
func (h *Handler) ResetPassword(c *gin.Context) {
	var req request.ResetPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数无效")
		return
	}
	if err := h.authService.ResetPassword(c.Request.Context(), &req); err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, nil)
}

// GetProfile 获取当前登录用户的信息
// [GET] /api/v1/auth/profile
func (h *Handler) GetProfile(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		result.FailWithMsg(c, 401, "未授权")
		return
	}
	userID, ok := userIDVal.(uint64)
	if !ok {
		result.FailWithMsg(c, 401, "用户标识无效")
		return
	}
	user, err := h.authService.GetProfile(c.Request.Context(), userID)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, user)
}

// setRefreshTokenCookie 设置 HttpOnly Cookie
func (h *Handler) setRefreshTokenCookie(c *gin.Context, refreshToken string) {
	c.SetCookie(
		RefreshTokenCookieName,
		refreshToken,
		int(h.refreshExpire/time.Second),
		RefreshTokenCookiePath,
		"",
		h.cookieSecure,
		true,
	)
}

// clearRefreshTokenCookie 清除 Cookie
func (h *Handler) clearRefreshTokenCookie(c *gin.Context) {
	c.SetCookie(
		RefreshTokenCookieName,
		"",
		-1,
		RefreshTokenCookiePath,
		"",
		h.cookieSecure,
		true,
	)
}
