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

type TokenManager interface {
	GenerateAccessToken(userID uint64, username string, role string, tokenVersion uint64) (string, error)
	GenerateRefreshToken(userID uint64, username string, role string, tokenVersion uint64) (string, error)
	ParseToken(tokenString string, isRefresh bool) (*CustomClaims, error)

	GenerateAgentToken(serverID uint64, version uint64) (string, error)
	ParseAgentToken(tokenString string) (uint64, uint64, error)
}

type CustomClaims struct {
	UserID       uint64 `json:"userID"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	TokenVersion uint64 `json:"tokenVersion"`
	jwt.RegisteredClaims
}

type agentClaims struct {
	Version uint64 `json:"ver"`
	jwt.RegisteredClaims
}

type tokenManager struct {
	cfg Config
}

func NewTokenManager(cfg Config) TokenManager {
	return &tokenManager{cfg: cfg}
}

// GenerateAccessToken 生成 AccessToken
func (m *tokenManager) GenerateAccessToken(userID uint64, username string, role string, tokenVersion uint64) (string, error) {
	return m.generateToken(userID, username, role, tokenVersion, m.cfg.AccessSecret, m.cfg.AccessExpire)
}

// GenerateRefreshToken 生成 RefreshToken
func (m *tokenManager) GenerateRefreshToken(userID uint64, username string, role string, tokenVersion uint64) (string, error) {
	return m.generateToken(userID, username, role, tokenVersion, m.cfg.RefreshSecret, m.cfg.RefreshExpire)
}

// ParseToken 检验并解析 Token
func (m *tokenManager) ParseToken(tokenString string, isRefresh bool) (*CustomClaims, error) {
	secret := m.cfg.AccessSecret
	if isRefresh {
		secret = m.cfg.RefreshSecret
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
		jwt.WithIssuer(m.cfg.Issuer),
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
func (m *tokenManager) GenerateAgentToken(serverID uint64, version uint64) (string, error) {
	if serverID == 0 || version == 0 {
		return "", errors.New("serverID 或 version 无效")
	}

	claims := agentClaims{
		Version: version,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: strconv.FormatUint(serverID, 10),
			Issuer:  m.cfg.Issuer,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.cfg.AgentSecret))
}

// ParseAgentToken 校验并解析 AgentToken
func (m *tokenManager) ParseAgentToken(tokenString string) (uint64, uint64, error) {
	claims := &agentClaims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			return []byte(m.cfg.AgentSecret), nil
		},
		jwt.WithValidMethods([]string{
			jwt.SigningMethodHS256.Alg(),
		}),
		jwt.WithIssuer(m.cfg.Issuer),
	)

	if err != nil {
		return 0, 0, err
	}

	if !token.Valid {
		return 0, 0, errors.New("agent token 无效")
	}

	serverID, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil || serverID == 0 || claims.Version == 0 {
		return 0, 0, errors.New("agent token 的 serverID 或 version 无效")
	}

	return serverID, claims.Version, nil
}

func (m *tokenManager) generateToken(userID uint64, username, role string, tokenVersion uint64, secret string, expire time.Duration) (string, error) {
	now := time.Now()
	claims := CustomClaims{
		UserID:       userID,
		Username:     username,
		Role:         role,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        uuid.NewString(),
			Issuer:    m.cfg.Issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expire)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
