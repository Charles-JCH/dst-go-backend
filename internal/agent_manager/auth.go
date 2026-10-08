package agent_manager

import (
	"context"
	"game-panel/internal/apperr"
	"log/slog"
)

type TokenParser interface {
	ParseAgentToken(token string) (uint64, error)
}

type ServerStore interface {
	ExistsByID(ctx context.Context, serverID uint64) (bool, error)
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

func (a *Auth) Authenticate(ctx context.Context, token string) (uint64, error) {
	if token == "" {
		return 0, apperr.NewBizError(401, "agent token 为空")
	}
	serverID, err := a.tokenParser.ParseAgentToken(token)
	if err != nil {
		return 0, apperr.NewBizError(401, "agent token 无效")
	}
	if err := a.CheckServer(ctx, serverID); err != nil {
		return 0, err
	}
	return serverID, nil
}

func (a *Auth) CheckServer(ctx context.Context, serverID uint64) error {
	exists, err := a.serverStore.ExistsByID(ctx, serverID)
	if err != nil {
		slog.ErrorContext(ctx, "agent 服务器查询失败", "serverID", serverID, "error", err)
		return apperr.NewBizError(500, "agent 认证服务不可用")
	}
	if !exists {
		return apperr.NewBizError(401, "agent 对应的服务器不存在")
	}
	return nil
}
