package agentmanager

import (
	"context"
	"game-panel/internal/apperr"
	"log/slog"
)

type TokenParser interface {
	ParseAgentToken(token string) (uint64, uint64, error)
}

type ServerStore interface {
	AgentTokenVersion(ctx context.Context, serverID uint64) (uint64, bool, error)
}

type Auth struct {
	tokenParser TokenParser
	serverStore ServerStore
}

func NewAuth(tokenParser TokenParser, serverStore ServerStore) *Auth {
	return &Auth{
		tokenParser: tokenParser,
		serverStore: serverStore,
	}
}

// Authenticate 校验令牌签名并核对令牌版本
func (a *Auth) Authenticate(ctx context.Context, token string) (uint64, error) {
	if token == "" {
		return 0, apperr.NewBizError(401, "agent token 为空")
	}
	serverID, version, err := a.tokenParser.ParseAgentToken(token)
	if err != nil {
		return 0, apperr.NewBizError(401, "agent token 无效")
	}
	return serverID, a.check(ctx, serverID, version)
}

// CheckServer 连接建立后再次确认服务器仍存在
func (a *Auth) CheckServer(ctx context.Context, serverID uint64) error {
	_, exists, err := a.serverStore.AgentTokenVersion(ctx, serverID)
	if err != nil {
		slog.ErrorContext(ctx, "agent 服务器查询失败", "serverID", serverID, "error", err)
		return apperr.NewBizError(500, "agent 认证服务不可用")
	}
	if !exists {
		return apperr.NewBizError(401, "agent 对应的服务器不存在")
	}
	return nil
}

func (a *Auth) check(ctx context.Context, serverID, version uint64) error {
	current, exists, err := a.serverStore.AgentTokenVersion(ctx, serverID)
	if err != nil {
		slog.ErrorContext(ctx, "agent 服务器查询失败", "serverID", serverID, "error", err)
		return apperr.NewBizError(500, "agent 认证服务不可用")
	}
	if !exists || current != version {
		return apperr.NewBizError(401, "agent token 已失效")
	}
	return nil
}
