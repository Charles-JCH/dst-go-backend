package v1

import (
	"errors"
	"game-panel/internal/apperr"
	"game-panel/internal/model/request"
	"game-panel/internal/result"
	"game-panel/internal/service"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
	"time"
)

const (
	RefreshTokenCookieName = "refresh_token"
	RefreshTokenCookiePath = "/api/v1/auth"
)

type AuthHandler struct {
	authService   service.AuthService
	refreshExpire time.Duration
}

func NewAuthHandler(authService service.AuthService, refreshExpire time.Duration) *AuthHandler {
	return &AuthHandler{
		authService:   authService,
		refreshExpire: refreshExpire,
	}
}

// GetCaptcha 获取图形验证码
// [GET] /api/v1/auth/captcha
func (h *AuthHandler) GetCaptcha(c *gin.Context) {
	resp, err := h.authService.GenerateCaptcha(c.Request.Context())
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, resp)
}

// SendSMSCode 发送短信验证码
// [POST] /api/v1/auth/sms-code
func (h *AuthHandler) SendSMSCode(c *gin.Context) {
	var req request.SendSMSCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数不合法")
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
func (h *AuthHandler) Register(c *gin.Context) {
	var req request.RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数不合法")
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

// Login 账号密码登录
// [POST] /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req request.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数不合法")
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
func (h *AuthHandler) LoginBySMS(c *gin.Context) {
	var req request.LoginBySMSReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数不合法")
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
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	refreshToken, err := c.Cookie(RefreshTokenCookieName)
	if err != nil || refreshToken == "" {
		result.FailWithMsg(c, 401, "未找到有效的刷新凭证")
		return
	}
	newAccessToken, newRefreshToken, err := h.authService.RefreshToken(c.Request.Context(), refreshToken)
	if err != nil {
		var bizErr *apperr.BizError
		if errors.As(err, &bizErr) && (bizErr.Code == http.StatusUnauthorized || bizErr.Code == http.StatusForbidden) {
			h.clearRefreshTokenCookie(c)
		}
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
func (h *AuthHandler) Logout(c *gin.Context) {
	parts := strings.Fields(c.GetHeader("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		result.FailWithMsg(c, 400, "退出登录需要携带 Authorization: Bearer <accessToken>")
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
func (h *AuthHandler) VerifyResetCode(c *gin.Context) {
	var req request.VerifyResetCodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数不合法")
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
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req request.ResetPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		result.FailWithMsg(c, 400, "请求参数不合法")
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
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userIdVal, exists := c.Get("userId")
	if !exists {
		result.FailWithMsg(c, 401, "未授权的访问")
		return
	}
	userId, ok := userIdVal.(uint64)
	if !ok {
		result.FailWithMsg(c, 401, "用户标识无效")
		return
	}
	user, err := h.authService.GetProfile(c.Request.Context(), userId)
	if err != nil {
		result.Fail(c, err)
		return
	}
	result.Success(c, user)
}

// setRefreshTokenCookie 设置 HttpOnly Cookie
func (h *AuthHandler) setRefreshTokenCookie(c *gin.Context, refreshToken string) {
	c.SetCookie(
		RefreshTokenCookieName,
		refreshToken,
		int(h.refreshExpire/time.Second),
		RefreshTokenCookiePath,
		"",
		false,
		true,
	)
}

// clearRefreshTokenCookie 清除 Cookie
func (h *AuthHandler) clearRefreshTokenCookie(c *gin.Context) {
	c.SetCookie(
		RefreshTokenCookieName,
		"",
		-1,
		RefreshTokenCookiePath,
		"",
		false,
		true,
	)
}
