package dst

import (
	"context"
	"encoding/json"
	"errors"
	"game-panel/internal/apperr"
	"game-panel/internal/model/entity"
	"game-panel/internal/model/request"
	"game-panel/internal/model/response"
	"game-panel/internal/repository/dst"
	"game-panel/internal/repository/server"
	"gorm.io/gorm"
	"log/slog"
	"strings"
)

type Service interface {
	Create(ctx context.Context, userID uint64, req *request.CreateDstReq) (*response.DstResp, error)
	GetByID(ctx context.Context, userID uint64, dstID uint64) (*response.DstResp, error)
	List(ctx context.Context, userID uint64, req *request.ListDstReq) (*response.DstListResp, error)
	Update(ctx context.Context, userID uint64, dstID uint64, req *request.UpdateDstReq) error
	Delete(ctx context.Context, userID uint64, dstID uint64) error
}

type service struct {
	dstRepo    dst.Repository
	serverRepo server.Repository
}

func NewService(dstRepo dst.Repository, serverRepo server.Repository) Service {
	return &service{
		dstRepo:    dstRepo,
		serverRepo: serverRepo,
	}
}

func (s *service) Create(ctx context.Context, userID uint64, req *request.CreateDstReq) (*response.DstResp, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, apperr.NewBizError(400, "存档名称为空")
	}
	if req.ServerID == 0 || req.Slot < 1 || req.Slot > 5 || req.MaxPlayers < 1 {
		return nil, apperr.NewBizError(400, "存档参数无效")
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
	dst := &entity.Dst{
		ServerID:     req.ServerID,
		UserID:       userID,
		Slot:         req.Slot,
		Name:         name,
		Description:  req.Description,
		Password:     req.Password,
		MaxPlayers:   req.MaxPlayers,
		ClusterToken: strings.TrimSpace(req.ClusterToken),
		Status:       0,
		Mods:         mods,
	}

	// 创建存档
	err = s.dstRepo.Create(ctx, dst)
	if errors.Is(err, gorm.ErrDuplicatedKey) {
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
	if req.ServerID == 0 ||
		req.Page < 1 || req.Page > 1000000 ||
		req.PageSize < 1 || req.PageSize > 100 {
		return nil, apperr.NewBizError(400, "查询参数无效")
	}

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
	if req.MaxPlayers < 1 {
		return apperr.NewBizError(400, "最大人数无效")
	}

	mods := req.Mods
	if mods == nil {
		mods = []string{}
	}
	modsJSON, err := json.Marshal(mods)
	if err != nil {
		slog.ErrorContext(ctx, "模组配置序列化失败", "userID", userID, "dstID", dstID, "error", err)
		return apperr.NewBizError(500, "存档更新失败")
	}

	updates := map[string]any{
		"name":        name,
		"description": req.Description,
		"max_players": req.MaxPlayers,
		"mods":        string(modsJSON),
	}
	if req.Password != nil {
		updates["password"] = *req.Password
	}
	if token := strings.TrimSpace(req.ClusterToken); token != "" {
		updates["cluster_token"] = token
	}

	err = s.dstRepo.Update(ctx, userID, dstID, updates)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperr.NewBizError(404, "存档不存在")
	}
	if err != nil {
		slog.ErrorContext(ctx, "存档更新失败", "userID", userID, "dstID", dstID, "error", err)
		return apperr.NewBizError(500, "存档更新失败")
	}
	return nil
}

func (s *service) Delete(ctx context.Context, userID uint64, dstID uint64) error {
	err := s.dstRepo.Delete(ctx, userID, dstID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperr.NewBizError(404, "存档不存在")
	}
	if err != nil {
		slog.ErrorContext(ctx, "存档删除失败", "userID", userID, "dstID", dstID, "error", err)
		return apperr.NewBizError(500, "存档删除失败")
	}
	return nil
}
