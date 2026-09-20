package jwt

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"time"
)

type Config struct {
	AccessSecret  string
	AccessExpire  time.Duration
	RefreshSecret string
	RefreshExpire time.Duration
	Issuer        string
}

type CustomClaims struct {
	UserId       uint64 `json:"userId"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	TokenVersion uint64 `json:"tokenVersion"`
	jwt.RegisteredClaims
}

type TokenManager interface {
	GenerateAccessToken(userId uint64, username string, role string, tokenVersion uint64) (string, error)
	GenerateRefreshToken(userId uint64, username string, role string, tokenVersion uint64) (string, error)
	ParseToken(tokenString string, isRefresh bool) (*CustomClaims, error)
}

type tokenManager struct {
	cfg Config
}

func NewTokenManager(cfg Config) TokenManager {
	return &tokenManager{cfg: cfg}
}

func (j *tokenManager) generateToken(userId uint64, username, role string, tokenVersion uint64, secret string, expire time.Duration) (string, error) {
	now := time.Now()
	claims := CustomClaims{
		UserId:       userId,
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
func (j *tokenManager) GenerateAccessToken(userId uint64, username string, role string, tokenVersion uint64) (string, error) {
	return j.generateToken(userId, username, role, tokenVersion, j.cfg.AccessSecret, j.cfg.AccessExpire)
}

// GenerateRefreshToken 生成 RefreshToken
func (j *tokenManager) GenerateRefreshToken(userId uint64, username string, role string, tokenVersion uint64) (string, error) {
	return j.generateToken(userId, username, role, tokenVersion, j.cfg.RefreshSecret, j.cfg.RefreshExpire)
}

// ParseToken 检验并解析 Token
func (j *tokenManager) ParseToken(tokenString string, isRefresh bool) (*CustomClaims, error) {
	secret := j.cfg.AccessSecret
	if isRefresh {
		secret = j.cfg.RefreshSecret
	}

	token, err := jwt.ParseWithClaims(
		tokenString, &CustomClaims{},
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
