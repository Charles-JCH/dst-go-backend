package service

import (
	"context"
	"errors"
	"game-panel/internal/apperr"
	"game-panel/internal/model/entity"
	"game-panel/internal/model/request"
	"game-panel/internal/model/response"
	"game-panel/internal/platform/crypto"
	"game-panel/internal/platform/jwt"
	"game-panel/internal/repository"
	"game-panel/internal/repository/cache"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"log/slog"
	"time"
)

type Identity struct {
	UserID   uint64
	Username string
	Role     string
}

type AuthService interface {
	Authenticate(ctx context.Context, accessToken string) (*Identity, error)
	GenerateCaptcha(ctx context.Context) (response.GetCaptchaResp, error)
	SendSMSCode(ctx context.Context, clientIP string, req *request.SendSMSCodeReq) error
	Register(ctx context.Context, req *request.RegisterReq) (response.LoginResp, string, error)
	Login(ctx context.Context, req *request.LoginReq) (response.LoginResp, string, error)
	LoginBySMS(ctx context.Context, req *request.LoginBySMSReq) (response.LoginResp, string, error)
	RefreshToken(ctx context.Context, refreshToken string) (string, string, error)
	Logout(ctx context.Context, accessToken string, refreshToken string) error
	VerifyResetCode(ctx context.Context, req *request.VerifyResetCodeReq) (string, error)
	ResetPassword(ctx context.Context, req *request.ResetPasswordReq) error
	GetProfile(ctx context.Context, userID uint64) (response.UserResp, error)
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

func (s *authService) Authenticate(ctx context.Context, accessToken string) (*Identity, error) {
	if accessToken == "" {
		return nil, apperr.NewBizError(401, "登录凭证缺失")
	}

	// 校验 accessToken
	claims, err := s.jwtManager.ParseToken(accessToken, false)
	if err != nil {
		return nil, apperr.NewBizError(401, "登录凭证无效或已过期")
	}

	// 是否注销
	isBlacklisted, err := s.authCache.IsTokenInBlacklist(ctx, accessToken)
	if err != nil {
		slog.ErrorContext(ctx, "登录凭证黑名单查询失败", "error", err)
		return nil, apperr.NewBizError(500, "身份认证服务不可用")
	}
	if isBlacklisted {
		return nil, apperr.NewBizError(401, "登录凭证已注销")
	}

	// 查询用户
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "用户查询失败", "userID", claims.UserID, "error", err)
		return nil, apperr.NewBizError(500, "身份认证服务不可用")
	}
	if user == nil || user.Status != 1 ||
		user.TokenVersion != claims.TokenVersion {
		return nil, apperr.NewBizError(401, "登录状态已失效")
	}

	return &Identity{
		UserID:   user.ID,
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
	// 用户名是否已注册
	existUser, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		slog.ErrorContext(ctx, "用户名查询失败", "error", err)
		return response.LoginResp{}, "", apperr.NewBizError(500, "注册失败")
	}
	if existUser != nil {
		return response.LoginResp{}, "", apperr.NewBizError(409, "用户名已注册")
	}

	// 手机号是否已注册
	existUser, err = s.userRepo.GetByPhone(ctx, req.Phone)
	if err != nil {
		slog.ErrorContext(ctx, "手机号查询失败", "error", err)
		return response.LoginResp{}, "", apperr.NewBizError(500, "注册失败")
	}
	if existUser != nil {
		return response.LoginResp{}, "", apperr.NewBizError(409, "手机号已注册")
	}

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
		slog.ErrorContext(ctx, "密码哈希失败", "error", err)
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

	if err := s.userRepo.Create(ctx, newUser); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return response.LoginResp{}, "", apperr.NewBizError(409, "用户名或手机号已注册")
		}
		slog.ErrorContext(ctx, "用户创建失败", "error", err)
		return response.LoginResp{}, "", apperr.NewBizError(500, "注册失败")
	}

	// 签发 accessToken 和 refreshToken
	accessToken, err := s.jwtManager.GenerateAccessToken(newUser.ID, newUser.Username, newUser.Role, newUser.TokenVersion)
	if err != nil {
		slog.ErrorContext(ctx, "accessToken 签发失败", "error", err)
		return response.LoginResp{}, "", apperr.NewBizError(500, "accessToken 签发失败")
	}
	refreshToken, err := s.jwtManager.GenerateRefreshToken(newUser.ID, newUser.Username, newUser.Role, newUser.TokenVersion)
	if err != nil {
		slog.ErrorContext(ctx, "refreshToken 签发失败", "error", err)
		return response.LoginResp{}, "", apperr.NewBizError(500, "refreshToken 签发失败")
	}
	return response.LoginResp{AccessToken: accessToken, UserID: newUser.ID, Username: newUser.Username, Role: newUser.Role}, refreshToken, nil
}

// Login 用户登录逻辑
func (s *authService) Login(ctx context.Context, req *request.LoginReq) (response.LoginResp, string, error) {
	// 校验图形验证码
	valid, err := s.captchaService.Verify(ctx, req.CaptchaID, req.CaptchaCode, true)
	if err != nil {
		slog.ErrorContext(ctx, "图形验证码读取失败", "error", err)
		return response.LoginResp{}, "", apperr.NewBizError(503, "图形验证码服务不可用")
	}
	if !valid {
		return response.LoginResp{}, "", apperr.NewBizError(400, "图形验证码错误或已过期")
	}

	// 查询用户
	user, err := s.userRepo.GetByPhone(ctx, req.Phone)
	if err != nil {
		slog.ErrorContext(ctx, "用户查询失败", "phone", req.Phone, "error", err)
		return response.LoginResp{}, "", apperr.NewBizError(500, "登录失败")
	}

	// 校验密码
	if user == nil || !s.hasher.ComparePassword(user.Password, req.Password) {
		return response.LoginResp{}, "", apperr.NewBizError(401, "手机号或密码错误")
	}

	// 校验账号状态
	if user.Status != 1 {
		return response.LoginResp{}, "", apperr.NewBizError(403, "账号已禁用")
	}

	// 签发 accessToken 和 refreshToken
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username, user.Role, user.TokenVersion)
	if err != nil {
		slog.ErrorContext(ctx, "accessToken 签发失败", "error", err)
		return response.LoginResp{}, "", apperr.NewBizError(500, "accessToken 签发失败")
	}
	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID, user.Username, user.Role, user.TokenVersion)
	if err != nil {
		slog.ErrorContext(ctx, "refreshToken 签发失败", "error", err)
		return response.LoginResp{}, "", apperr.NewBizError(500, "refreshToken 签发失败")
	}

	return response.LoginResp{AccessToken: accessToken, UserID: user.ID, Username: user.Username, Role: user.Role}, refreshToken, nil
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
		slog.ErrorContext(ctx, "用户查询失败", "phone", req.Phone, "error", err)
		return response.LoginResp{}, "", apperr.NewBizError(500, "登录失败")
	}
	if user == nil {
		return response.LoginResp{}, "", apperr.NewBizError(404, "手机号未注册")
	}

	// 校验账号状态
	if user.Status != 1 {
		return response.LoginResp{}, "", apperr.NewBizError(403, "账号已禁用")
	}

	// 签发 accessToken 和 refreshToken
	accessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username, user.Role, user.TokenVersion)
	if err != nil {
		slog.ErrorContext(ctx, "accessToken 签发失败", "error", err)
		return response.LoginResp{}, "", apperr.NewBizError(500, "accessToken 签发失败")
	}
	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID, user.Username, user.Role, user.TokenVersion)
	if err != nil {
		slog.ErrorContext(ctx, "refreshToken 签发失败", "error", err)
		return response.LoginResp{}, "", apperr.NewBizError(500, "refreshToken 签发失败")
	}
	return response.LoginResp{AccessToken: accessToken, UserID: user.ID, Username: user.Username, Role: user.Role}, refreshToken, nil
}

// RefreshToken 刷新 AccessToken、RefreshToken
func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (string, string, error) {
	if refreshToken == "" {
		return "", "", apperr.NewBizError(401, "刷新凭证缺失")
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
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "用户查询失败", "userID", claims.UserID, "error", err)
		return "", "", apperr.NewBizError(500, "身份认证服务不可用")
	}
	if user == nil {
		return "", "", apperr.NewBizError(401, "用户不存在")
	}
	if claims.TokenVersion != user.TokenVersion {
		return "", "", apperr.NewBizError(401, "登录状态已失效")
	}

	// 校验用户状态
	if user.Status != 1 {
		return "", "", apperr.NewBizError(403, "账号已禁用")
	}

	// refreshToken 剩余时间
	remainingTTL := time.Until(claims.ExpiresAt.Time)
	if remainingTTL <= 0 {
		return "", "", apperr.NewBizError(401, "刷新凭证已过期")
	}

	// 签发 accessToken 和 refreshToken
	newAccessToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username, user.Role, user.TokenVersion)
	if err != nil {
		slog.ErrorContext(ctx, "accessToken 签发失败", "error", err)
		return "", "", apperr.NewBizError(500, "accessToken 签发失败")
	}
	newRefreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID, user.Username, user.Role, user.TokenVersion)
	if err != nil {
		slog.ErrorContext(ctx, "refreshToken 签发失败", "error", err)
		return "", "", apperr.NewBizError(500, "refreshToken 签发失败")
	}

	// 消费旧 refreshToken
	consumed, err := s.authCache.ConsumeRefreshToken(ctx, refreshToken, remainingTTL)
	if err != nil {
		slog.ErrorContext(ctx, "刷新凭证消费失败", "userID", user.ID, "error", err)
		return "", "", apperr.NewBizError(500, "身份认证服务不可用")
	}
	if !consumed {
		return "", "", apperr.NewBizError(409, "刷新凭证已使用")
	}
	return newAccessToken, newRefreshToken, nil
}

// Logout 登出
func (s *authService) Logout(ctx context.Context, accessToken string, refreshToken string) error {
	if err := s.revokeToken(ctx, refreshToken, true); err != nil {
		slog.ErrorContext(ctx, "刷新凭证注销失败", "error", err)
		return apperr.NewBizError(503, "退出登录失败")
	}
	if err := s.revokeToken(ctx, accessToken, false); err != nil {
		slog.ErrorContext(ctx, "访问凭证注销失败", "error", err)
		return apperr.NewBizError(503, "退出登录失败")
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
		slog.ErrorContext(ctx, "用户查询失败", "phone", req.Phone, "error", err)
		return "", apperr.NewBizError(500, "密码重置服务不可用")
	}
	if user == nil {
		return "", apperr.NewBizError(404, "手机号未注册")
	}

	// 生成 resetToken
	resetToken := uuid.NewString()
	if err := s.authCache.SetResetToken(ctx, resetToken, user.ID, user.TokenVersion, 10*time.Minute); err != nil {
		slog.ErrorContext(ctx, "密码重置凭证保存失败", "userID", user.ID, "error", err)
		return "", apperr.NewBizError(500, "密码重置服务不可用")
	}

	return resetToken, nil
}

// ResetPassword 重置密码第二步
func (s *authService) ResetPassword(ctx context.Context, req *request.ResetPasswordReq) error {
	data, err := s.authCache.GetResetToken(ctx, req.ResetToken)
	if err != nil {
		slog.ErrorContext(ctx, "密码重置凭证读取失败", "error", err)
		return apperr.NewBizError(500, "密码重置服务不可用")
	}
	if data == nil {
		return apperr.NewBizError(400, "密码重置凭证无效或已过期")
	}

	user, err := s.userRepo.GetByID(ctx, data.UserID)
	if err != nil {
		slog.ErrorContext(ctx, "用户查询失败", "userID", data.UserID, "error", err)
		return apperr.NewBizError(500, "密码重置服务不可用")
	}
	if user == nil {
		return apperr.NewBizError(404, "用户不存在")
	}

	// 密码哈希
	hashedPassword, err := s.hasher.HashPassword(req.NewPassword)
	if err != nil {
		slog.ErrorContext(ctx, "密码哈希失败", "userID", data.UserID, "error", err)
		return apperr.NewBizError(500, "密码重置失败")
	}

	// 更新密码
	updated, err := s.userRepo.UpdatePassword(ctx, data.UserID, hashedPassword, data.TokenVersion)
	if err != nil {
		slog.ErrorContext(ctx, "用户密码更新失败", "userID", data.UserID, "error", err)
		return apperr.NewBizError(500, "密码重置失败")
	}
	if !updated {
		return apperr.NewBizError(400, "密码重置凭证已失效")
	}

	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
	defer cancel()

	if err := s.authCache.DelResetToken(cleanupCtx, req.ResetToken); err != nil {
		slog.ErrorContext(ctx, "密码重置凭证清理失败", "userID", data.UserID, "error", err)
	}
	return nil
}

// GetProfile 获取当前登录用户的基本信息
func (s *authService) GetProfile(ctx context.Context, userID uint64) (response.UserResp, error) {
	if userID == 0 {
		return response.UserResp{}, apperr.NewBizError(401, "用户标识无效")
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		slog.ErrorContext(ctx, "用户查询失败", "userID", userID, "error", err)
		return response.UserResp{}, apperr.NewBizError(500, "用户信息查询失败")
	}
	if user == nil {
		return response.UserResp{}, apperr.NewBizError(404, "用户不存在")
	}

	profile := response.ToUserResp(user)
	return *profile, nil
}
