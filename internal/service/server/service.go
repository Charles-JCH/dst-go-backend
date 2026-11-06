package server

import (
	"context"
	"errors"
	"game-panel/internal/apperr"
	"game-panel/internal/model/entity"
	"game-panel/internal/model/request"
	"game-panel/internal/model/response"
	"game-panel/internal/platform/crypto"
	"game-panel/internal/repository"
	"game-panel/internal/repository/server"
	"game-panel/internal/repository/task"
	"log/slog"
	"net/netip"
	"strings"
)

type Service interface {
	Create(ctx context.Context, userID uint64, req *request.CreateServerReq) (*response.ServerResp, error)
	GetByID(ctx context.Context, userID uint64, serverID uint64) (*response.ServerResp, error)
	List(ctx context.Context, userID uint64, req *request.ListServerReq) (*response.ServerListResp, error)
	Update(ctx context.Context, userID uint64, serverID uint64, req *request.UpdateServerReq) error
	Delete(ctx context.Context, userID uint64, serverID uint64) error
}

type AgentHub interface {
	Disconnect(serverID uint64)
}

type service struct {
	serverRepo server.Repository
	taskRepo   task.Repository
	txManager  repository.Transaction
	encryptor  crypto.Encryptor
	agent      AgentHub
}

func NewService(serverRepo server.Repository, taskRepo task.Repository, txManager repository.Transaction, encryptor crypto.Encryptor, agent AgentHub) Service {
	return &service{
		serverRepo: serverRepo,
		taskRepo:   taskRepo,
		txManager:  txManager,
		encryptor:  encryptor,
		agent:      agent,
	}
}

func (s *service) Create(ctx context.Context, userID uint64, req *request.CreateServerReq) (*response.ServerResp, error) {
	name := strings.TrimSpace(req.Name)
	username := strings.TrimSpace(req.Username)
	if name == "" || username == "" {
		return nil, apperr.NewBizError(400, "服务器名称或SSH用户名为空")
	}

	addr, err := netip.ParseAddr(req.IP)
	if err != nil {
		return nil, apperr.NewBizError(400, "服务器IP无效")
	}
	ip := addr.Unmap().String()

	// 密码加密
	password, err := s.encryptor.Encrypt(req.Password)
	if err != nil {
		slog.ErrorContext(ctx, "SSH密码加密失败", "error", err)
		return nil, apperr.NewBizError(500, "服务器创建失败")
	}
	server := &entity.Server{
		Name:     name,
		IP:       ip,
		Port:     req.Port,
		Username: username,
		Password: password,
		GameEnv:  []entity.GameEnvRef{},
		Mods:     []string{},
		UserID:   userID,
	}

	// 创建服务器
	err = s.txManager.ExecTx(ctx, func(txCtx context.Context) error {
		if err := s.serverRepo.Create(txCtx, server); err != nil {
			return err
		}
		task := &entity.Task{
			Name:         "部署服务器 Agent",
			Type:         entity.TaskTypeAgentDeploy,
			Status:       entity.TaskStatusPending,
			Stage:        "queued",
			ResourceType: entity.TaskResourceServer,
			ResourceID:   server.ID,
			UserID:       userID,
		}

		return s.taskRepo.Create(txCtx, task)
	})
	if errors.Is(err, repository.ErrDuplicate) {
		return nil, apperr.NewBizError(409, "该IP和SSH端口的服务器已存在")
	}
	if err != nil {
		slog.ErrorContext(ctx, "服务器创建失败", "userID", userID, "error", err)
		return nil, apperr.NewBizError(500, "服务器创建失败")
	}
	return response.ToServerResp(server), nil
}

func (s *service) GetByID(ctx context.Context, userID uint64, serverID uint64) (*response.ServerResp, error) {
	server, err := s.serverRepo.GetByID(ctx, userID, serverID)
	if err != nil {
		slog.ErrorContext(ctx, "服务器查询失败", "userID", userID, "serverID", serverID, "error", err)
		return nil, apperr.NewBizError(500, "服务器查询失败")
	}
	if server == nil {
		return nil, apperr.NewBizError(404, "服务器不存在")
	}
	return response.ToServerResp(server), nil
}

func (s *service) List(ctx context.Context, userID uint64, req *request.ListServerReq) (*response.ServerListResp, error) {
	servers, total, err := s.serverRepo.List(ctx, userID, req.Page, req.PageSize)
	if err != nil {
		slog.ErrorContext(ctx, "服务器列表查询失败", "userID", userID, "error", err)
		return nil, apperr.NewBizError(500, "服务器列表查询失败")
	}
	list := make([]response.ServerResp, 0, len(servers))
	for i := range servers {
		list = append(list, *response.ToServerResp(&servers[i]))
	}
	return &response.ServerListResp{
		List:     list,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func (s *service) Update(ctx context.Context, userID uint64, serverID uint64, req *request.UpdateServerReq) error {
	name := strings.TrimSpace(req.Name)
	username := strings.TrimSpace(req.Username)
	if name == "" || username == "" {
		return apperr.NewBizError(400, "服务器名称或SSH用户名为空")
	}

	addr, err := netip.ParseAddr(req.IP)
	if err != nil {
		return apperr.NewBizError(400, "服务器IP无效")
	}
	ip := addr.Unmap().String()

	fields := server.UpdateFields{Name: name, IP: ip, Port: req.Port, Username: username}
	// 密码为 nil 表示不修改
	if req.Password != nil {
		password, err := s.encryptor.Encrypt(*req.Password)
		if err != nil {
			slog.ErrorContext(ctx, "SSH密码加密失败", "userID", userID, "serverID", serverID, "error", err)
			return apperr.NewBizError(500, "服务器更新失败")
		}
		fields.Password = &password
	}
	err = s.serverRepo.Update(ctx, userID, serverID, fields)
	if errors.Is(err, repository.ErrNotFound) {
		return apperr.NewBizError(404, "服务器不存在")
	}
	if errors.Is(err, repository.ErrDuplicate) {
		return apperr.NewBizError(409, "该IP和SSH端口的服务器已存在")
	}
	if err != nil {
		slog.ErrorContext(ctx, "服务器更新失败", "userID", userID, "serverID", serverID, "error", err)
		return apperr.NewBizError(500, "服务器更新失败")
	}
	return nil
}

func (s *service) Delete(ctx context.Context, userID uint64, serverID uint64) error {
	err := s.serverRepo.Delete(ctx, userID, serverID)
	if errors.Is(err, server.ErrServerHasDst) {
		return apperr.NewBizError(409, "服务器下仍有存档，请先删除存档")
	}
	if errors.Is(err, repository.ErrNotFound) {
		return apperr.NewBizError(404, "服务器不存在")
	}
	if err != nil {
		slog.ErrorContext(ctx, "服务器删除失败", "userID", userID, "serverID", serverID, "error", err)
		return apperr.NewBizError(500, "服务器删除失败")
	}

	// 关闭 agent 连接
	s.agent.Disconnect(serverID)
	return nil
}
