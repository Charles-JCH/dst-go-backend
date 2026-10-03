package jwt

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"strconv"
	"time"
)

type Config struct {
	AccessSecret  string
	AccessExpire  time.Duration
	RefreshSecret string
	RefreshExpire time.Duration
	AgentSecret   string
	Issuer        string
}

type CustomClaims struct {
	UserID       uint64 `json:"userID"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	TokenVersion uint64 `json:"tokenVersion"`
	jwt.RegisteredClaims
}

type TokenManager interface {
	GenerateAccessToken(userID uint64, username string, role string, tokenVersion uint64) (string, error)
	GenerateRefreshToken(userID uint64, username string, role string, tokenVersion uint64) (string, error)
	ParseToken(tokenString string, isRefresh bool) (*CustomClaims, error)

	GenerateAgentToken(serverID uint64) (string, error)
	ParseAgentToken(tokenString string) (uint64, error)
}

type tokenManager struct {
	cfg Config
}

func NewTokenManager(cfg Config) TokenManager {
	return &tokenManager{cfg: cfg}
}

func (j *tokenManager) generateToken(userID uint64, username, role string, tokenVersion uint64, secret string, expire time.Duration) (string, error) {
	now := time.Now()
	claims := CustomClaims{
		UserID:       userID,
		Username:     username,
		Role:         role,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Issuer:    j.cfg.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expire)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// GenerateAccessToken 生成 AccessToken
func (j *tokenManager) GenerateAccessToken(userID uint64, username string, role string, tokenVersion uint64) (string, error) {
	return j.generateToken(userID, username, role, tokenVersion, j.cfg.AccessSecret, j.cfg.AccessExpire)
}

// GenerateRefreshToken 生成 RefreshToken
func (j *tokenManager) GenerateRefreshToken(userID uint64, username string, role string, tokenVersion uint64) (string, error) {
	return j.generateToken(userID, username, role, tokenVersion, j.cfg.RefreshSecret, j.cfg.RefreshExpire)
}

// ParseToken 检验并解析 Token
func (j *tokenManager) ParseToken(tokenString string, isRefresh bool) (*CustomClaims, error) {
	secret := j.cfg.AccessSecret
	if isRefresh {
		secret = j.cfg.RefreshSecret
	}

	token, err := jwt.ParseWithClaims(
		tokenString,
		&CustomClaims{},
		func(token *jwt.Token) (any, error) {
			return []byte(secret), nil
		},
		jwt.WithValidMethods([]string{
			jwt.SigningMethodHS256.Alg(),
		}),
		jwt.WithIssuer(j.cfg.Issuer),
		jwt.WithExpirationRequired(),
	)

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("token 无效或已过期")
}

// GenerateAgentToken 生成 AgentToken
func (j *tokenManager) GenerateAgentToken(serverID uint64) (string, error) {
	if serverID == 0 {
		return "", errors.New("serverID 无效")
	}

	claims := jwt.RegisteredClaims{
		Subject: strconv.FormatUint(serverID, 10),
		Issuer:  j.cfg.Issuer,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.cfg.AgentSecret))
}

// ParseAgentToken 校验并解析 AgentToken
func (j *tokenManager) ParseAgentToken(tokenString string) (uint64, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			return []byte(j.cfg.AgentSecret), nil
		},
		jwt.WithValidMethods([]string{
			jwt.SigningMethodHS256.Alg(),
		}),
		jwt.WithIssuer(j.cfg.Issuer),
	)

	if err != nil {
		return 0, err
	}

	if !token.Valid {
		return 0, errors.New("agent token 无效")
	}

	serverID, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil || serverID == 0 {
		return 0, errors.New("agent token 的 serverID 无效")
	}

	return serverID, nil
}
