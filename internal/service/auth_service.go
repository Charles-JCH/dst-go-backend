package service

import (
	"context"
	"errors"
	"fmt"
	"game-panel/internal/apperr"
	"game-panel/internal/model/entity"
	"game-panel/internal/model/request"
	"game-panel/internal/model/response"
	"game-panel/internal/platform/crypto"
	"game-panel/internal/platform/jwt"
	"game-panel/internal/repository"
	"game-panel/internal/repository/cache"
	"github.com/google/uuid"
	"log/slog"
	"time"
)

type AuthService interface {
	Authenticate(ctx context.Context, accessToken string) (response.Identity, error)
	GenerateCaptcha(ctx context.Context) (response.GetCaptchaResp, error)
	SendSMSCode(ctx context.Context, clientIP string, req *request.SendSMSCodeReq) error
	Register(ctx context.Context, req *request.RegisterReq) (response.LoginResp, string, error)
	Login(ctx context.Context, req *request.LoginReq) (response.LoginResp, string, error)
	LoginBySMS(ctx context.Context, req *request.LoginBySMSReq) (response.LoginResp, string, error)
	RefreshToken(ctx context.Context, refreshToken string) (string, string, error)
	Logout(ctx context.Context, accessToken string, refreshToken string) error
	VerifyResetCode(ctx context.Context, req *request.VerifyResetCodeReq) (string, error)
	ResetPassword(ctx context.Context, req *request.ResetPasswordReq) error
	GetProfile(ctx context.Context, userId uint64) (response.UserVO, error)
}

type authService struct {
	captchaService CaptchaService
	smsService     SMSService
	jwtManager     jwt.TokenManager
	authCache      cache.AuthCache
	userRepo       repository.UserRepository
	tx             repository.Transaction
	hasher         crypto.PasswordHasher
}

func NewAuthService(
	captchaService CaptchaService,
	smsService SMSService,
	jwtManager jwt.TokenManager,
	authCache cache.AuthCache,
	userRepo repository.UserRepository,
	tx repository.Transaction,
	hasher crypto.PasswordHasher,

) AuthService {
	return &authService{
		captchaService: captchaService,
		smsService:     smsService,
		jwtManager:     jwtManager,
		authCache:      authCache,
		userRepo:       userRepo,
		tx:             tx,
		hasher:         hasher,
	}
}

func (s *authService) Authenticate(ctx context.Context, accessToken string) (response.Identity, error) {
	if accessToken == "" {
		return response.Identity{}, apperr.NewBizError(401, "未携带登录凭证")
	}

	// 校验 accessToken
	claims, err := s.jwtManager.ParseToken(accessToken, false)
	if err != nil {
		return response.Identity{}, apperr.NewBizError(401, "登录凭证无效或已过期，请重新登录")
	}

	// 是否注销
	isBlacklisted, err := s.authCache.IsTokenInBlacklist(ctx, accessToken)
	if err != nil {
		slog.ErrorContext(ctx, "查询登录凭证黑名单失败", "错误", err)
		return response.Identity{}, apperr.NewBizError(500, "身份认证服务暂时不可用")
	}
	if isBlacklisted {
		return response.Identity{}, apperr.NewBizError(401, "登录凭证已注销，请重新登录")
	}

	// 查询用户
	user, err := s.userRepo.GetById(ctx, claims.UserId)
	if err != nil {
		slog.ErrorContext(ctx, "认证时查询用户失败", "用户ID", claims.UserId, "错误", err)
		return response.Identity{}, apperr.NewBizError(500, "身份认证服务暂时不可用")
	}
	if user == nil || user.Status != 1 ||
		user.TokenVersion != claims.TokenVersion {
		return response.Identity{}, apperr.NewBizError(401, "登录状态已失效，请重新登录")
	}

	return response.Identity{
		UserId:   user.Id,
		Username: user.Username,
		Role:     user.Role,
	}, nil
}

// GenerateCaptcha 获取图形验证码
func (s *authService) GenerateCaptcha(ctx context.Context) (response.GetCaptchaResp, error) {
	return s.captchaService.GenerateCaptcha(ctx)
}

// SendSMSCode 发送短信验证码
func (s *authService) SendSMSCode(ctx context.Context, clientIP string, req *request.SendSMSCodeReq) error {
	return s.smsService.SendCode(ctx, clientIP, req)
}

// Register 用户账号注册
func (s *authService) Register(ctx context.Context, req *request.RegisterReq) (response.LoginResp, string, error) {
	// 校验短信验证码
	valid, err := s.smsService.VerifyCode(ctx, req.Phone, req.SMSCode)
	if err != nil {
		return response.LoginResp{}, "", err
	}
	if !valid {
		return response.LoginResp{}, "", apperr.NewBizError(400, "短信验证码错误或已过期")
	}

	// 密码哈希
	hashedPassword, err := s.hasher.HashPassword(req.Password)
	if err != nil {
		slog.ErrorContext(ctx, "密码哈希失败", "错误", err)
		return response.LoginResp{}, "", apperr.NewBizError(500, "密码哈希失败")
	}
	newUser := &entity.User{
		Username:     req.Username,
		Password:     hashedPassword,
		Phone:        req.Phone,
		Role:         "user",
		Status:       1,
		TokenVersion: 1,
	}
	err = s.tx.ExecTx(ctx, func(txCtx context.Context) error {
		existUser, err := s.userRepo.GetByUsername(txCtx, req.Username)
		if err != nil {
			return fmt.Errorf("查询用户名失败: %w", err)
		}
		if existUser != nil {
			return apperr.NewBizError(409, "用户名已被注册")
		}
		existUser, err = s.userRepo.GetByPhone(txCtx, req.Phone)
		if err != nil {
			return fmt.Errorf("查询手机号失败: %w", err)
		}
		if existUser != nil {
			return apperr.NewBizError(409, "手机号已被注册")
		}
		if err := s.userRepo.Create(txCtx, newUser); err != nil {
			return fmt.Errorf("创建用户失败: %w", err)
		}
		return nil
	})
	if err != nil {
		var bizErr *apperr.BizError
		if errors.As(err, &bizErr) {
			return response.LoginResp{}, "", bizErr
		}

		slog.ErrorContext(ctx, "创建注册用户失败", "错误", err)
		return response.LoginResp{}, "", apperr.NewBizError(500, "用户注册失败")
	}

	// 签发 accessToken 和 refreshToken
	accessToken, err := s.jwtManager.GenerateAccessToken(newUser.Id, newUser.Username, newUser.Role, newUser.TokenVersion)
	if err != nil {
		slog.ErrorContext(ctx, "签发 accessToken 失败", "错误", err)
		return response.LoginResp{}, "", apperr.NewBizError(500, "签发 accessToken 失败")
	}
	refreshToken, err := s.jwtManager.GenerateRefreshToken(newUser.Id, newUser.Username, newUser.Role, newUser.TokenVersion)
	if err != nil {
		slog.ErrorContext(ctx, "签发 refreshToken 失败", "错误", err)
		return response.LoginResp{}, "", apperr.NewBizError(500, "签发 refreshToken 失败")
	}
	return response.LoginResp{AccessToken: accessToken, UserId: newUser.Id, Username: newUser.Username, Role: newUser.Role}, refreshToken, nil
}

// Login 用户登录逻辑
func (s *authService) Login(ctx context.Context, req *request.LoginReq) (response.LoginResp, string, error) {
	// 校验图形验证码
	if !s.captchaService.Verify(ctx, req.CaptchaId, req.CaptchaCode, true) {
		return response.LoginResp{}, "", apperr.NewBizError(400, "图形验证码错误或已过期")
	}

	// 查询用户
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		slog.ErrorContext(ctx, "查询用户失败", "用户名", req.Username, "错误", err)
		return response.LoginResp{}, "", apperr.NewBizError(500, "用户登录失败")
	}

	// 校验密码
	if user == nil || !s.hasher.ComparePassword(user.Password, req.Password) {
		return response.LoginResp{}, "", apperr.NewBizError(401, "用户名或密码错误")
	}

	// 校验账号状态
	if user.Status != 1 {
		return response.LoginResp{}, "", apperr.NewBizError(403, "账号已被禁用")
	}

	// 签发 accessToken 和 refreshToken
	accessToken, err := s.jwtManager.GenerateAccessToken(user.Id, user.Username, user.Role, user.TokenVersion)
	if err != nil {
		slog.ErrorContext(ctx, "签发 accessToken 失败", "错误", err)
		return response.LoginResp{}, "", apperr.NewBizError(500, "签发 accessToken 失败")
	}
	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.Id, user.Username, user.Role, user.TokenVersion)
	if err != nil {
		slog.ErrorContext(ctx, "签发 refreshToken 失败", "错误", err)
		return response.LoginResp{}, "", apperr.NewBizError(500, "签发 refreshToken 失败")
	}

	return response.LoginResp{AccessToken: accessToken, UserId: user.Id, Username: user.Username, Role: user.Role}, refreshToken, nil
}

func (s *authService) LoginBySMS(ctx context.Context, req *request.LoginBySMSReq) (response.LoginResp, string, error) {
	// 校验短信验证码
	valid, err := s.smsService.VerifyCode(ctx, req.Phone, req.SMSCode)
	if err != nil {
		return response.LoginResp{}, "", err
	}
	if !valid {
		return response.LoginResp{}, "", apperr.NewBizError(400, "短信验证码错误或已过期")
	}

	// 查询用户
	user, err := s.userRepo.GetByPhone(ctx, req.Phone)
	if err != nil {
		slog.ErrorContext(ctx, "查询用户失败", "手机号", req.Phone, "错误", err)
		return response.LoginResp{}, "", apperr.NewBizError(500, "用户登录失败")
	}
	if user == nil {
		return response.LoginResp{}, "", apperr.NewBizError(404, "该手机号尚未注册")
	}

	// 校验账号状态
	if user.Status != 1 {
		return response.LoginResp{}, "", apperr.NewBizError(403, "账号已被禁用")
	}

	// 签发 accessToken 和 refreshToken
	accessToken, err := s.jwtManager.GenerateAccessToken(user.Id, user.Username, user.Role, user.TokenVersion)
	if err != nil {
		slog.ErrorContext(ctx, "签发 accessToken 失败", "错误", err)
		return response.LoginResp{}, "", apperr.NewBizError(500, "签发 accessToken 失败")
	}
	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.Id, user.Username, user.Role, user.TokenVersion)
	if err != nil {
		slog.ErrorContext(ctx, "签发 refreshToken 失败", "错误", err)
		return response.LoginResp{}, "", apperr.NewBizError(500, "签发 refreshToken 失败")
	}
	return response.LoginResp{AccessToken: accessToken, UserId: user.Id, Username: user.Username, Role: user.Role}, refreshToken, nil
}

// RefreshToken 刷新 AccessToken、RefreshToken
func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (string, string, error) {
	if refreshToken == "" {
		return "", "", apperr.NewBizError(401, "刷新凭证不能为空")
	}

	// 解析 refreshToken
	claims, err := s.jwtManager.ParseToken(refreshToken, true)
	if err != nil {
		return "", "", apperr.NewBizError(401, "刷新凭证无效或已过期")
	}
	if claims.ExpiresAt == nil {
		return "", "", apperr.NewBizError(401, "刷新凭证无效")
	}

	// 查询用户是否存在
	user, err := s.userRepo.GetById(ctx, claims.UserId)
	if err != nil {
		slog.ErrorContext(ctx, "查询用户失败", "用户ID", claims.UserId, "错误", err)
		return "", "", apperr.NewBizError(500, "身份认证服务暂时不可用")
	}
	if user == nil {
		return "", "", apperr.NewBizError(401, "用户不存在")
	}
	if claims.TokenVersion != user.TokenVersion {
		return "", "", apperr.NewBizError(401, "登录状态已失效，请重新登录")
	}

	// 校验用户状态
	if user.Status != 1 {
		return "", "", apperr.NewBizError(403, "账号已被禁用")
	}

	// refreshToken 剩余时间
	remainingTTL := time.Until(claims.ExpiresAt.Time)
	if remainingTTL <= 0 {
		return "", "", apperr.NewBizError(401, "刷新凭证已过期")
	}

	// 签发 accessToken 和 refreshToken
	newAccessToken, err := s.jwtManager.GenerateAccessToken(user.Id, user.Username, user.Role, user.TokenVersion)
	if err != nil {
		slog.ErrorContext(ctx, "签发 accessToken 失败", "错误", err)
		return "", "", apperr.NewBizError(500, "签发 accessToken 失败")
	}
	newRefreshToken, err := s.jwtManager.GenerateRefreshToken(user.Id, user.Username, user.Role, user.TokenVersion)
	if err != nil {
		slog.ErrorContext(ctx, "签发 refreshToken 失败", "错误", err)
		return "", "", apperr.NewBizError(500, "签发 refreshToken 失败")
	}

	// 消费旧 refreshToken
	consumed, err := s.authCache.ConsumeRefreshToken(ctx, refreshToken, remainingTTL)
	if err != nil {
		slog.ErrorContext(ctx, "消费刷新凭证失败", "用户ID", user.Id, "错误", err)
		return "", "", apperr.NewBizError(500, "身份认证服务暂时不可用")
	}
	if !consumed {
		return "", "", apperr.NewBizError(401, "刷新凭证已失效")
	}
	return newAccessToken, newRefreshToken, nil
}

// Logout 登出
func (s *authService) Logout(ctx context.Context, accessToken string, refreshToken string) error {
	if err := s.revokeToken(ctx, refreshToken, true); err != nil {
		slog.ErrorContext(ctx, "注销刷新凭证失败", "错误", err)
		return apperr.NewBizError(503, "退出登录失败，请稍后重试")
	}
	if err := s.revokeToken(ctx, accessToken, false); err != nil {
		slog.ErrorContext(ctx, "注销访问凭证失败", "错误", err)
		return apperr.NewBizError(503, "退出登录失败，请稍后重试")
	}
	return nil
}

func (s *authService) revokeToken(ctx context.Context, token string, isRefresh bool) error {
	if token == "" {
		return nil
	}
	claims, err := s.jwtManager.ParseToken(token, isRefresh)
	if err != nil {
		return nil
	}
	if claims.ExpiresAt == nil {
		return nil
	}
	remainingTTL := time.Until(claims.ExpiresAt.Time)
	if remainingTTL <= 0 {
		return nil
	}
	return s.authCache.AddTokenToBlacklist(ctx, token, remainingTTL)
}

// VerifyResetCode 重置密码第一步
func (s *authService) VerifyResetCode(ctx context.Context, req *request.VerifyResetCodeReq) (string, error) {
	valid, err := s.smsService.VerifyCode(ctx, req.Phone, req.SMSCode)
	if err != nil {
		return "", err
	}
	if !valid {
		return "", apperr.NewBizError(400, "短信验证码错误或已过期")
	}

	// 查询用户
	user, err := s.userRepo.GetByPhone(ctx, req.Phone)
	if err != nil {
		slog.ErrorContext(ctx, "查询用户失败", "手机号", req.Phone, "错误", err)
		return "", apperr.NewBizError(500, "密码重置服务暂时不可用")
	}
	if user == nil {
		return "", apperr.NewBizError(404, "该手机号尚未注册")
	}

	// 生成 resetToken
	resetToken := uuid.NewString()
	if err := s.authCache.SetResetToken(ctx, resetToken, user.Id, 10*time.Minute); err != nil {
		slog.ErrorContext(ctx, "保存密码重置凭证失败", "用户ID", user.Id, "错误", err)
		return "", apperr.NewBizError(500, "密码重置服务暂时不可用")
	}

	return resetToken, nil
}

// ResetPassword 重置密码第二步
func (s *authService) ResetPassword(ctx context.Context, req *request.ResetPasswordReq) error {
	userId, err := s.authCache.ConsumeResetToken(ctx, req.ResetToken)
	if err != nil {
		slog.ErrorContext(ctx, "读取密码重置凭证失败", "错误", err)
		return apperr.NewBizError(500, "密码重置服务暂时不可用")
	}
	if userId == 0 {
		return apperr.NewBizError(400, "密码重置凭证无效或已过期")
	}

	user, err := s.userRepo.GetById(ctx, userId)
	if err != nil {
		slog.ErrorContext(ctx, "重置密码时查询用户失败", "用户ID", userId, "错误", err)
		return apperr.NewBizError(500, "密码重置服务暂时不可用")
	}
	if user == nil {
		return apperr.NewBizError(404, "用户不存在")
	}

	// 密码哈希
	hashedPassword, err := s.hasher.HashPassword(req.NewPassword)
	if err != nil {
		slog.ErrorContext(ctx, "新密码加密失败", "用户ID", userId, "错误", err)
		return apperr.NewBizError(500, "密码重置失败")
	}

	// 更新密码
	if err := s.userRepo.UpdatePassword(ctx, userId, hashedPassword); err != nil {
		slog.ErrorContext(ctx, "更新用户密码失败", "用户ID", userId, "错误", err)
		return apperr.NewBizError(500, "密码重置失败")
	}
	return nil
}

// GetProfile 获取当前登录用户的基本信息
func (s *authService) GetProfile(ctx context.Context, userId uint64) (response.UserVO, error) {
	if userId == 0 {
		return response.UserVO{}, apperr.NewBizError(401, "用户身份无效")
	}
	user, err := s.userRepo.GetById(ctx, userId)
	if err != nil {
		slog.ErrorContext(ctx, "获取用户基本信息失败", "用户ID", userId, "错误", err)
		return response.UserVO{}, apperr.NewBizError(500, "获取用户信息失败")
	}
	if user == nil {
		return response.UserVO{}, apperr.NewBizError(404, "用户不存在")
	}

	profile := response.ToUserVO(user)
	return *profile, nil
}
