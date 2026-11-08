package gamedeploy

import (
	"context"
	"fmt"
	"game-panel/internal/apperr"
	"game-panel/internal/model/entity"
	"game-panel/internal/model/request"
	"game-panel/internal/model/response"
	"game-panel/internal/protocol"
	gameenvrepo "game-panel/internal/repository/gameenv"
	serverrepo "game-panel/internal/repository/server"
	"game-panel/internal/service/agentcommand"
	"log/slog"
	"slices"
)

// paramGameCode 部署任务参数：要部署的游戏编码，部署成功后据此记录已部署的游戏
const paramGameCode = "gameCode"

// deployActions 游戏编码对应的 Agent 部署命令；新增游戏时在这里加一行
var deployActions = map[string]string{
	"dst": protocol.ActionDeploy,
}

// Service 通过 Agent 在服务器上部署游戏环境
type Service interface {
	// Deploy 创建部署任务并下发给 Agent，返回任务；进度和日志通过任务查看
	Deploy(ctx context.Context, userID uint64, serverID uint64, req *request.DeployGameReq) (*response.TaskResp, error)
}

type service struct {
	serverRepo  serverrepo.Repository
	gameEnvRepo gameenvrepo.Repository
	commands    agentcommand.Service
}

// NewService 同时向 commands 注册部署成功后的处理
func NewService(serverRepo serverrepo.Repository, gameEnvRepo gameenvrepo.Repository, commands agentcommand.Service) Service {
	s := &service{serverRepo: serverRepo, gameEnvRepo: gameEnvRepo, commands: commands}
	commands.OnSucceeded(entity.TaskTypeGameDeploy, s.onDeployed)
	return s
}

func (s *service) Deploy(ctx context.Context, userID uint64, serverID uint64, req *request.DeployGameReq) (*response.TaskResp, error) {
	server, err := s.serverRepo.GetByID(ctx, userID, serverID)
	if err != nil {
		slog.ErrorContext(ctx, "服务器查询失败", "userID", userID, "serverID", serverID, "error", err)
		return nil, apperr.NewBizError(500, "游戏部署失败")
	}
	if server == nil {
		return nil, apperr.NewBizError(404, "服务器不存在")
	}

	env, err := s.gameEnvRepo.FindByCode(ctx, req.GameCode)
	if err != nil {
		slog.ErrorContext(ctx, "游戏环境查询失败", "gameCode", req.GameCode, "error", err)
		return nil, apperr.NewBizError(500, "游戏部署失败")
	}
	action, isSupported := deployActions[req.GameCode]
	if env == nil || !isSupported {
		return nil, apperr.NewBizError(400, "不支持部署该游戏")
	}
	if slices.ContainsFunc(server.GameEnv, func(e entity.GameEnvRef) bool { return e.Code == env.Code }) {
		return nil, apperr.NewBizError(409, "该游戏已部署")
	}

	task, err := s.commands.Dispatch(ctx, agentcommand.Command{
		UserID:       userID,
		ServerID:     serverID,
		TaskType:     entity.TaskTypeGameDeploy,
		TaskName:     "部署" + env.Name,
		ResourceType: entity.TaskResourceServer,
		ResourceID:   serverID,
		Params:       map[string]string{paramGameCode: env.Code},
		Payload:      protocol.CmdPayload{Action: action},
	})
	if err != nil {
		return nil, err // 已是 BizError
	}
	slog.InfoContext(ctx, "游戏部署任务已创建", "userID", userID, "serverID", serverID, "gameCode", env.Code, "taskID", task.ID)
	return response.ToTaskResp(task), nil
}

// onDeployed 部署成功后把游戏加入服务器已部署的游戏环境；重复执行不会重复添加
func (s *service) onDeployed(ctx context.Context, task *entity.Task) error {
	code := task.Params[paramGameCode]
	env, err := s.gameEnvRepo.FindByCode(ctx, code)
	if err != nil {
		return err
	}
	if env == nil {
		return fmt.Errorf("游戏环境不存在 gameCode=%s", code)
	}
	if err := s.serverRepo.AddGameEnv(ctx, task.ResourceID, entity.GameEnvRef{Code: env.Code, Name: env.Name}); err != nil {
		return err
	}
	slog.InfoContext(ctx, "游戏部署完成", "serverID", task.ResourceID, "gameCode", env.Code, "taskID", task.ID)
	return nil
}
