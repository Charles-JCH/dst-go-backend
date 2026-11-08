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
	"game-panel/internal/repository/cache"
	serverrepo "game-panel/internal/repository/server"
	taskrepo "game-panel/internal/repository/task"
	"game-panel/internal/service/agentinstall"
	"log/slog"
	"net/netip"
	"strings"
	"time"
)

// metricsFreshness 最近一次资源使用情况超过这个时间视为过期，不返回（心跳间隔 30 秒）
const metricsFreshness = 2 * time.Minute

type Service interface {
	Create(ctx context.Context, userID uint64, req *request.CreateServerReq) (*response.ServerResp, error)
	GetByID(ctx context.Context, userID uint64, serverID uint64) (*response.ServerResp, error)
	List(ctx context.Context, userID uint64, req *request.ListServerReq) (*response.ServerListResp, error)
	Update(ctx context.Context, userID uint64, serverID uint64, req *request.UpdateServerReq) error
	Delete(ctx context.Context, userID uint64, serverID uint64) error
	InstallAgent(ctx context.Context, userID uint64, serverID uint64) (*response.TaskResp, error)
	// ListMetrics 最近 24 小时的资源使用情况，按时间升序
	ListMetrics(ctx context.Context, userID uint64, serverID uint64) (*response.MetricsListResp, error)
}

// AgentConn 查询、卸载和断开 Agent（agentmanager.Service 未导出接口）
type AgentConn interface {
	IsOnline(serverID uint64) bool
	HasPendingCommand(serverID uint64) bool
	Uninstall(serverID uint64) error
	Disconnect(serverID uint64)
}

type service struct {
	serverRepo   serverrepo.Repository
	taskRepo     taskrepo.Repository
	metricsCache cache.MetricsCache
	txManager    repository.Transaction
	encryptor    crypto.Encryptor
	installer    agentinstall.Service
	agent        AgentConn
}

func NewService(
	serverRepo serverrepo.Repository,
	taskRepo taskrepo.Repository,
	metricsCache cache.MetricsCache,
	txManager repository.Transaction,
	encryptor crypto.Encryptor,
	installer agentinstall.Service,
	agent AgentConn,
) Service {
	return &service{
		serverRepo:   serverRepo,
		taskRepo:     taskRepo,
		metricsCache: metricsCache,
		txManager:    txManager,
		encryptor:    encryptor,
		installer:    installer,
		agent:        agent,
	}
}

// Create 保存服务器并创建 Agent 安装任务；安装失败不影响服务器保存，可在页面重试
func (s *service) Create(ctx context.Context, userID uint64, req *request.CreateServerReq) (*response.ServerResp, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, apperr.NewBizError(400, "服务器名称为空")
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
		Username: req.Username,
		Password: password,
		GameEnv:  []entity.GameEnvRef{},
		Mods:     []string{},
		UserID:   userID,
	}

	// 服务器和安装任务同一事务写入，提交后再开始执行任务
	var installTask *entity.Task
	err = s.txManager.ExecTx(ctx, func(txCtx context.Context) error {
		if err := s.serverRepo.Create(txCtx, server); err != nil {
			return err
		}
		installTask = newInstallTask(userID, server.ID)
		return s.taskRepo.Create(txCtx, installTask)
	})
	if errors.Is(err, repository.ErrDuplicate) {
		return nil, apperr.NewBizError(409, "该IP和SSH端口的服务器已存在")
	}
	if err != nil {
		slog.ErrorContext(ctx, "服务器创建失败", "userID", userID, "error", err)
		return nil, apperr.NewBizError(500, "服务器创建失败")
	}

	s.installer.Enqueue(installTask.ID)
	return &s.toResps(ctx, []entity.Server{*server})[0], nil
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
	return &s.toResps(ctx, []entity.Server{*server})[0], nil
}

func (s *service) List(ctx context.Context, userID uint64, req *request.ListServerReq) (*response.ServerListResp, error) {
	servers, total, err := s.serverRepo.List(ctx, userID, req.Page, req.PageSize)
	if err != nil {
		slog.ErrorContext(ctx, "服务器列表查询失败", "userID", userID, "error", err)
		return nil, apperr.NewBizError(500, "服务器列表查询失败")
	}
	return &response.ServerListResp{
		List:     s.toResps(ctx, servers),
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func (s *service) Update(ctx context.Context, userID uint64, serverID uint64, req *request.UpdateServerReq) error {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return apperr.NewBizError(400, "服务器名称为空")
	}

	addr, err := netip.ParseAddr(req.IP)
	if err != nil {
		return apperr.NewBizError(400, "服务器IP无效")
	}
	ip := addr.Unmap().String()

	current, err := s.serverRepo.GetByID(ctx, userID, serverID)
	if err != nil {
		slog.ErrorContext(ctx, "服务器查询失败", "userID", userID, "serverID", serverID, "error", err)
		return apperr.NewBizError(500, "服务器更新失败")
	}
	if current == nil {
		return apperr.NewBizError(404, "服务器不存在")
	}

	fields := serverrepo.UpdateFields{Name: name, IP: ip, Port: req.Port, Username: req.Username}
	// 密码为 nil 表示不修改
	if req.Password != nil {
		password, err := s.encryptor.Encrypt(*req.Password)
		if err != nil {
			slog.ErrorContext(ctx, "SSH密码加密失败", "userID", userID, "serverID", serverID, "error", err)
			return apperr.NewBizError(500, "服务器更新失败")
		}
		fields.Password = &password
	}
	// 换了地址，或重新填写了密码（如服务器重装系统），下次连接重新信任主机指纹
	if ip != current.IP || req.Port != current.Port || req.Password != nil {
		empty := ""
		fields.SSHHostKey = &empty
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

// Delete 删除服务器；Agent 在线时通知它卸载自己，离线时需用户手动清理
func (s *service) Delete(ctx context.Context, userID uint64, serverID uint64) error {
	// 安装中删除会留下刚装好的 Agent，等安装结束再删
	isInstalling, err := s.hasActiveInstall(ctx, userID, serverID)
	if err != nil {
		slog.ErrorContext(ctx, "任务查询失败", "userID", userID, "serverID", serverID, "error", err)
		return apperr.NewBizError(500, "服务器删除失败")
	}
	if isInstalling {
		return apperr.NewBizError(409, "Agent 正在安装，请等待安装结束后再删除")
	}

	err = s.serverRepo.Delete(ctx, userID, serverID)
	if errors.Is(err, serverrepo.ErrServerHasDst) {
		return apperr.NewBizError(409, "服务器下仍有存档，请先删除存档")
	}
	if errors.Is(err, repository.ErrNotFound) {
		return apperr.NewBizError(404, "服务器不存在")
	}
	if err != nil {
		slog.ErrorContext(ctx, "服务器删除失败", "userID", userID, "serverID", serverID, "error", err)
		return apperr.NewBizError(500, "服务器删除失败")
	}

	// 服务器已删除，卸载通知失败不影响结果：Agent 的令牌随服务器一起失效，无法再连上平台
	if s.agent.IsOnline(serverID) {
		if err := s.agent.Uninstall(serverID); err != nil {
			slog.WarnContext(ctx, "Agent 卸载通知发送失败", "serverID", serverID, "error", err)
		}
	}
	s.agent.Disconnect(serverID)
	if err := s.metricsCache.Delete(ctx, serverID); err != nil {
		slog.WarnContext(ctx, "资源使用情况清理失败", "serverID", serverID, "error", err)
	}
	return nil
}

// InstallAgent 重新安装 Agent：离线时用于重新连接，在线时用于更新到平台当前版本；已有进行中的安装任务时返回 409
func (s *service) InstallAgent(ctx context.Context, userID uint64, serverID uint64) (*response.TaskResp, error) {
	server, err := s.serverRepo.GetByID(ctx, userID, serverID)
	if err != nil {
		slog.ErrorContext(ctx, "服务器查询失败", "userID", userID, "serverID", serverID, "error", err)
		return nil, apperr.NewBizError(500, "Agent 安装任务创建失败")
	}
	if server == nil {
		return nil, apperr.NewBizError(404, "服务器不存在")
	}
	// 安装会重启 Agent，正在执行的命令（如部署游戏）会丢失结果
	if s.agent.HasPendingCommand(serverID) {
		return nil, apperr.NewBizError(409, "Agent 正在执行命令，请等命令结束后再更新")
	}

	installTask := newInstallTask(userID, serverID)
	if s.agent.IsOnline(serverID) {
		installTask.Name = "更新 Agent"
	}
	err = s.taskRepo.Create(ctx, installTask)
	if errors.Is(err, repository.ErrDuplicate) {
		return nil, apperr.NewBizError(409, "Agent 安装任务正在进行中")
	}
	if err != nil {
		slog.ErrorContext(ctx, "Agent 安装任务创建失败", "userID", userID, "serverID", serverID, "error", err)
		return nil, apperr.NewBizError(500, "Agent 安装任务创建失败")
	}

	s.installer.Enqueue(installTask.ID)
	return response.ToTaskResp(installTask), nil
}

func (s *service) ListMetrics(ctx context.Context, userID uint64, serverID uint64) (*response.MetricsListResp, error) {
	server, err := s.serverRepo.GetByID(ctx, userID, serverID)
	if err != nil {
		slog.ErrorContext(ctx, "服务器查询失败", "userID", userID, "serverID", serverID, "error", err)
		return nil, apperr.NewBizError(500, "资源使用情况查询失败")
	}
	if server == nil {
		return nil, apperr.NewBizError(404, "服务器不存在")
	}

	metrics, err := s.metricsCache.List(ctx, serverID, time.Now().Add(-cache.MetricsRetention))
	if err != nil {
		slog.ErrorContext(ctx, "资源使用情况查询失败", "serverID", serverID, "error", err)
		return nil, apperr.NewBizError(500, "资源使用情况查询失败")
	}
	list := make([]response.MetricsResp, 0, len(metrics))
	for i := range metrics {
		list = append(list, *response.ToMetricsResp(&metrics[i]))
	}
	return &response.MetricsListResp{List: list}, nil
}

// newInstallTask 创建待执行的 Agent 安装任务；ActiveKey 保证同一服务器同时只有一个安装任务
func newInstallTask(userID uint64, serverID uint64) *entity.Task {
	return &entity.Task{
		Name:         "安装 Agent",
		Type:         entity.TaskTypeAgentInstall,
		Status:       entity.TaskStatusPending,
		Stage:        entity.TaskStageQueued,
		ResourceType: entity.TaskResourceServer,
		ResourceID:   serverID,
		UserID:       userID,
		ActiveKey:    entity.TaskActiveKey(entity.TaskTypeAgentInstall, entity.TaskResourceServer, serverID),
	}
}

// toResps 转换为响应，并附上在线服务器最近一次的资源使用情况；读取失败时不附带，不影响列表
func (s *service) toResps(ctx context.Context, servers []entity.Server) []response.ServerResp {
	onlineIDs := make([]uint64, 0, len(servers))
	for i := range servers {
		if s.agent.IsOnline(servers[i].ID) {
			onlineIDs = append(onlineIDs, servers[i].ID)
		}
	}
	latest, err := s.metricsCache.Latest(ctx, onlineIDs)
	if err != nil {
		slog.WarnContext(ctx, "资源使用情况查询失败", "error", err)
	}

	list := make([]response.ServerResp, 0, len(servers))
	for i := range servers {
		var metrics *entity.ServerMetrics
		if m, ok := latest[servers[i].ID]; ok && time.Since(m.Time) <= metricsFreshness {
			metrics = &m
		}
		list = append(list, *response.ToServerResp(&servers[i], s.agent.IsOnline(servers[i].ID), metrics))
	}
	return list
}

func (s *service) hasActiveInstall(ctx context.Context, userID uint64, serverID uint64) (bool, error) {
	tasks, err := s.taskRepo.ListActive(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, t := range tasks {
		if t.Type == entity.TaskTypeAgentInstall && t.ResourceType == entity.TaskResourceServer && t.ResourceID == serverID {
			return true, nil
		}
	}
	return false, nil
}
