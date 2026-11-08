package dst

import (
	"context"
	"errors"
	"game-panel/internal/apperr"
	"game-panel/internal/model/entity"
	"game-panel/internal/model/request"
	"game-panel/internal/model/response"
	"game-panel/internal/platform/crypto"
	"game-panel/internal/repository"
	dstrepo "game-panel/internal/repository/dst"
	serverrepo "game-panel/internal/repository/server"
	"log/slog"
	"strings"
)

// errServerGone 事务内复查时服务器已被删除，用于回滚
var errServerGone = errors.New("服务器已删除")

type Service interface {
	Create(ctx context.Context, userID uint64, req *request.CreateDstReq) (*response.DstResp, error)
	GetByID(ctx context.Context, userID uint64, dstID uint64) (*response.DstResp, error)
	List(ctx context.Context, userID uint64, req *request.ListDstReq) (*response.DstListResp, error)
	Update(ctx context.Context, userID uint64, dstID uint64, req *request.UpdateDstReq) error
	Delete(ctx context.Context, userID uint64, dstID uint64) error
}

// AgentCommander 向服务器上的 Agent 下发命令
type AgentCommander interface {
	Delete(serverID uint64, slot int) (string, error)
}

type service struct {
	dstRepo    dstrepo.Repository
	serverRepo serverrepo.Repository
	txManager  repository.Transaction
	encryptor  crypto.Encryptor
	agent      AgentCommander
}

func NewService(dstRepo dstrepo.Repository, serverRepo serverrepo.Repository, txManager repository.Transaction, encryptor crypto.Encryptor, agent AgentCommander) Service {
	return &service{
		dstRepo:    dstRepo,
		serverRepo: serverRepo,
		txManager:  txManager,
		encryptor:  encryptor,
		agent:      agent,
	}
}

func (s *service) Create(ctx context.Context, userID uint64, req *request.CreateDstReq) (*response.DstResp, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, apperr.NewBizError(400, "存档名称为空")
	}

	server, err := s.serverRepo.GetByID(ctx, userID, req.ServerID)
	if err != nil {
		slog.ErrorContext(ctx, "服务器查询失败", "userID", userID, "serverID", req.ServerID, "error", err)
		return nil, apperr.NewBizError(500, "服务器查询失败")
	}
	if server == nil {
		return nil, apperr.NewBizError(404, "服务器不存在")
	}

	mods := req.Mods
	if mods == nil {
		mods = []string{}
	}
	token, err := s.encryptor.Encrypt(strings.TrimSpace(req.ClusterToken))
	if err != nil {
		slog.ErrorContext(ctx, "科雷令牌加密失败", "userID", userID, "error", err)
		return nil, apperr.NewBizError(500, "存档创建失败")
	}
	dst := &entity.Dst{
		ServerID:     req.ServerID,
		UserID:       userID,
		Slot:         req.Slot,
		Name:         name,
		Description:  req.Description,
		Password:     req.Password,
		MaxPlayers:   req.MaxPlayers,
		ClusterToken: token,
		Status:       0,
		Mods:         mods,
	}

	// 先插入存档再复查服务器：与删除服务器的 NOT EXISTS 条件互斥，避免并发删除后留下孤儿存档
	err = s.txManager.ExecTx(ctx, func(txCtx context.Context) error {
		if err := s.dstRepo.Create(txCtx, dst); err != nil {
			return err
		}
		server, err := s.serverRepo.GetByID(txCtx, userID, req.ServerID)
		if err != nil {
			return err
		}
		if server == nil {
			return errServerGone
		}
		return nil
	})
	if errors.Is(err, errServerGone) {
		return nil, apperr.NewBizError(404, "服务器不存在")
	}
	if errors.Is(err, repository.ErrDuplicate) {
		return nil, apperr.NewBizError(409, "该服务器的存档槽位已被占用")
	}
	if err != nil {
		slog.ErrorContext(ctx, "存档创建失败", "userID", userID, "serverID", req.ServerID, "slot", req.Slot, "error", err)
		return nil, apperr.NewBizError(500, "存档创建失败")
	}
	return response.ToDstResp(dst), nil
}

func (s *service) GetByID(ctx context.Context, userID uint64, dstID uint64) (*response.DstResp, error) {
	dst, err := s.dstRepo.GetByID(ctx, userID, dstID)
	if err != nil {
		slog.ErrorContext(ctx, "存档查询失败", "userID", userID, "dstID", dstID, "error", err)
		return nil, apperr.NewBizError(500, "存档查询失败")
	}
	if dst == nil {
		return nil, apperr.NewBizError(404, "存档不存在")
	}
	return response.ToDstResp(dst), nil
}

func (s *service) List(ctx context.Context, userID uint64, req *request.ListDstReq) (*response.DstListResp, error) {
	server, err := s.serverRepo.GetByID(ctx, userID, req.ServerID)
	if err != nil {
		slog.ErrorContext(ctx, "服务器查询失败", "userID", userID, "serverID", req.ServerID, "error", err)
		return nil, apperr.NewBizError(500, "服务器查询失败")
	}
	if server == nil {
		return nil, apperr.NewBizError(404, "服务器不存在")
	}

	dsts, total, err := s.dstRepo.List(ctx, userID, req.ServerID, req.Page, req.PageSize)
	if err != nil {
		slog.ErrorContext(ctx, "存档列表查询失败", "userID", userID, "serverID", req.ServerID, "error", err)
		return nil, apperr.NewBizError(500, "存档列表查询失败")
	}

	list := make([]response.DstResp, 0, len(dsts))
	for i := range dsts {
		list = append(list, *response.ToDstResp(&dsts[i]))
	}
	return &response.DstListResp{
		List:     list,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func (s *service) Update(ctx context.Context, userID uint64, dstID uint64, req *request.UpdateDstReq) error {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return apperr.NewBizError(400, "存档名称为空")
	}

	fields := dstrepo.UpdateFields{
		Name:        name,
		Description: req.Description,
		MaxPlayers:  req.MaxPlayers,
		Mods:        req.Mods,
		Password:    req.Password,
	}
	// 令牌为 nil 表示不修改
	if req.ClusterToken != nil {
		encrypted, err := s.encryptor.Encrypt(strings.TrimSpace(*req.ClusterToken))
		if err != nil {
			slog.ErrorContext(ctx, "科雷令牌加密失败", "userID", userID, "dstID", dstID, "error", err)
			return apperr.NewBizError(500, "存档更新失败")
		}
		fields.ClusterToken = &encrypted
	}

	err := s.dstRepo.Update(ctx, userID, dstID, fields)
	if errors.Is(err, repository.ErrNotFound) {
		return apperr.NewBizError(404, "存档不存在")
	}
	if err != nil {
		slog.ErrorContext(ctx, "存档更新失败", "userID", userID, "dstID", dstID, "error", err)
		return apperr.NewBizError(500, "存档更新失败")
	}
	return nil
}

func (s *service) Delete(ctx context.Context, userID uint64, dstID uint64) error {
	// 先取出存档，删除后需要用它的服务器和槽位通知 Agent
	dst, err := s.dstRepo.GetByID(ctx, userID, dstID)
	if err != nil {
		slog.ErrorContext(ctx, "存档查询失败", "userID", userID, "dstID", dstID, "error", err)
		return apperr.NewBizError(500, "存档删除失败")
	}
	if dst == nil {
		return apperr.NewBizError(404, "存档不存在")
	}

	err = s.dstRepo.Delete(ctx, userID, dstID)
	if errors.Is(err, repository.ErrNotFound) {
		return apperr.NewBizError(404, "存档不存在")
	}
	if err != nil {
		slog.ErrorContext(ctx, "存档删除失败", "userID", userID, "dstID", dstID, "error", err)
		return apperr.NewBizError(500, "存档删除失败")
	}

	// 通知 Agent 清理服务器上的存档文件
	if _, err := s.agent.Delete(dst.ServerID, dst.Slot); err != nil {
		slog.WarnContext(ctx, "Agent 存档清理命令下发失败", "serverID", dst.ServerID, "slot", dst.Slot, "error", err)
	}
	return nil
}
