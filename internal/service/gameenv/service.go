package gameenv

import (
	"context"
	"game-panel/internal/apperr"
	"game-panel/internal/model/response"
	"game-panel/internal/repository/gameenv"
	"log/slog"
)

type Service interface {
	List(ctx context.Context) ([]response.GameEnvResp, error)
}

type service struct {
	gameEnvRepo gameenv.Repository
}

func NewService(gameEnvRepo gameenv.Repository) Service {
	return &service{gameEnvRepo: gameEnvRepo}
}

// List 查询可部署的游戏环境
func (s *service) List(ctx context.Context) ([]response.GameEnvResp, error) {
	envs, err := s.gameEnvRepo.List(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "游戏环境查询失败", "error", err)
		return nil, apperr.NewBizError(500, "游戏环境查询失败")
	}
	list := make([]response.GameEnvResp, 0, len(envs))
	for _, env := range envs {
		list = append(list, response.GameEnvResp{Code: env.Code, Name: env.Name})
	}
	return list, nil
}
