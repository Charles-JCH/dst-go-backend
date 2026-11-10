package dst

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"game-panel/internal/apperr"
	"game-panel/internal/model/entity"
	"game-panel/internal/model/request"
	"game-panel/internal/model/response"
	"game-panel/internal/platform/crypto"
	"game-panel/internal/protocol"
	"game-panel/internal/repository"
	dstrepo "game-panel/internal/repository/dst"
	serverrepo "game-panel/internal/repository/server"
	taskrepo "game-panel/internal/repository/task"
	"game-panel/internal/service/agentcommand"
	"log/slog"
	"slices"
	"strconv"
	"strings"
)

const (
	// dstSlotCount 每台服务器的存档槽位数，与 dst.sh 的 DST_SLOT_COUNT 一致
	dstSlotCount = 5

	// 初始化任务参数：槽位和科雷令牌密文，第二步据此创建存档
	paramSlot         = "slot"
	paramClusterToken = "clusterToken"
	// paramDst 创建任务参数：待保存的存档（pendingDst 的 JSON），配置写入成功后保存
	paramDst = "dst"
)

// errServerGone 事务内复查时服务器已被删除，用于回滚
var errServerGone = errors.New("服务器已删除")

// Service 存档管理。新增存档分两步：Init 在服务器上初始化默认存档；Create 写入配置文件，
// 写入成功后才保存存档，所以数据库中的存档一定已在服务器上初始化并写好配置
type Service interface {
	// Init 下发初始化任务，返回任务；进度和日志通过任务查看
	Init(ctx context.Context, userID uint64, req *request.InitDstReq) (*response.TaskResp, error)
	// Create 按已成功的初始化任务下发配置写入任务，返回任务；任务成功后存档才会出现在列表中
	Create(ctx context.Context, userID uint64, req *request.CreateDstReq) (*response.TaskResp, error)
	GetByID(ctx context.Context, userID uint64, dstID uint64) (*response.DstResp, error)
	List(ctx context.Context, userID uint64, req *request.ListDstReq) (*response.DstListResp, error)
	// Update 下发配置写入任务，返回任务；写入成功后才更新存档，存档运行中时重启后生效
	Update(ctx context.Context, userID uint64, dstID uint64, req *request.UpdateDstReq) (*response.TaskResp, error)
	// Start 下发启动任务，返回任务；成功后存档标记为运行中。重复启动时脚本直接成功
	Start(ctx context.Context, userID uint64, dstID uint64) (*response.TaskResp, error)
	// Stop 下发停止任务，返回任务；成功后存档标记为已停止。存档未运行时脚本直接成功
	Stop(ctx context.Context, userID uint64, dstID uint64) (*response.TaskResp, error)
	Delete(ctx context.Context, userID uint64, dstID uint64) error
}

// pendingDst 待保存的存档，作为创建任务的参数；ClusterToken 为密文
type pendingDst struct {
	ServerID     uint64   `json:"serverID"`
	Slot         int      `json:"slot"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Password     string   `json:"password"`
	MaxPlayers   int      `json:"maxPlayers"`
	ClusterToken string   `json:"clusterToken"`
	Mods         []string `json:"mods"`
}

type service struct {
	dstRepo    dstrepo.Repository
	serverRepo serverrepo.Repository
	taskRepo   taskrepo.Repository
	txManager  repository.Transaction
	encryptor  crypto.Encryptor
	commands   agentcommand.Service
}

// NewService 同时向 commands 注册创建、更新任务成功后的处理
func NewService(dstRepo dstrepo.Repository, serverRepo serverrepo.Repository, taskRepo taskrepo.Repository, txManager repository.Transaction, encryptor crypto.Encryptor, commands agentcommand.Service) Service {
	s := &service{
		dstRepo:    dstRepo,
		serverRepo: serverRepo,
		taskRepo:   taskRepo,
		txManager:  txManager,
		encryptor:  encryptor,
		commands:   commands,
	}
	commands.OnSucceeded(entity.TaskTypeDstCreate, s.onCreated)
	commands.OnSucceeded(entity.TaskTypeDstUpdate, s.onUpdated)
	commands.OnSucceeded(entity.TaskTypeDstStart, s.statusUpdater(entity.DstStatusRunning))
	commands.OnSucceeded(entity.TaskTypeDstStop, s.statusUpdater(entity.DstStatusStopped))
	return s
}

func (s *service) Init(ctx context.Context, userID uint64, req *request.InitDstReq) (*response.TaskResp, error) {
	token := strings.TrimSpace(req.ClusterToken)
	if token == "" {
		return nil, apperr.NewBizError(400, "科雷令牌为空")
	}
	// 初始化会清空槽位目录，已有存档的槽位不能初始化
	if _, err := s.checkSlot(ctx, userID, req.ServerID, req.Slot); err != nil {
		return nil, err
	}

	encrypted, err := s.encryptor.Encrypt(token)
	if err != nil {
		slog.ErrorContext(ctx, "科雷令牌加密失败", "userID", userID, "error", err)
		return nil, apperr.NewBizError(500, "存档初始化失败")
	}
	task, err := s.commands.Dispatch(ctx, agentcommand.Command{
		UserID:       userID,
		ServerID:     req.ServerID,
		TaskType:     entity.TaskTypeDstInit,
		TaskName:     fmt.Sprintf("初始化存档（槽位 %d）", req.Slot),
		ResourceType: entity.TaskResourceServer,
		ResourceID:   req.ServerID,
		Params:       map[string]string{paramSlot: strconv.Itoa(req.Slot), paramClusterToken: encrypted},
		Payload:      protocol.CmdPayload{Action: protocol.ActionInit, Slot: req.Slot, ClusterToken: token},
	})
	if err != nil {
		return nil, err // 已是 BizError
	}
	slog.InfoContext(ctx, "存档初始化任务已创建", "userID", userID, "serverID", req.ServerID, "slot", req.Slot, "taskID", task.ID)
	return response.ToTaskResp(task), nil
}

func (s *service) Create(ctx context.Context, userID uint64, req *request.CreateDstReq) (*response.TaskResp, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, apperr.NewBizError(400, "存档名称为空")
	}

	initTask, err := s.taskRepo.GetByID(ctx, userID, req.InitTaskID)
	if err != nil {
		slog.ErrorContext(ctx, "任务查询失败", "userID", userID, "taskID", req.InitTaskID, "error", err)
		return nil, apperr.NewBizError(500, "存档创建失败")
	}
	if initTask == nil || initTask.Type != entity.TaskTypeDstInit {
		return nil, apperr.NewBizError(404, "初始化任务不存在")
	}
	if initTask.Status != entity.TaskStatusSucceeded {
		return nil, apperr.NewBizError(409, "存档尚未初始化成功")
	}
	slot, err := strconv.Atoi(initTask.Params[paramSlot])
	if err != nil {
		slog.ErrorContext(ctx, "初始化任务参数无效", "taskID", initTask.ID, "error", err)
		return nil, apperr.NewBizError(500, "存档创建失败")
	}
	serverID := initTask.ResourceID

	// 槽位已有存档说明这次初始化已经用过，或槽位已被占用，需要重新初始化
	existing, err := s.checkSlot(ctx, userID, serverID, slot)
	if err != nil {
		return nil, err
	}
	encryptedToken := initTask.Params[paramClusterToken]
	token, err := s.encryptor.Decrypt(encryptedToken)
	if err != nil {
		slog.ErrorContext(ctx, "科雷令牌解密失败", "taskID", initTask.ID, "error", err)
		return nil, apperr.NewBizError(500, "存档创建失败")
	}

	mods := req.Mods
	if mods == nil {
		mods = []string{}
	}
	pending := pendingDst{
		ServerID:     serverID,
		Slot:         slot,
		Name:         name,
		Description:  req.Description,
		Password:     req.Password,
		MaxPlayers:   req.MaxPlayers,
		ClusterToken: encryptedToken,
		Mods:         mods,
	}
	task, err := s.dispatchConfig(ctx, userID, agentcommand.Command{
		TaskType:     entity.TaskTypeDstCreate,
		TaskName:     fmt.Sprintf("创建存档（槽位 %d）", slot),
		ResourceType: entity.TaskResourceServer,
		ResourceID:   serverID,
	}, pending, token, existing)
	if err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "存档创建任务已创建", "userID", userID, "serverID", serverID, "slot", slot, "taskID", task.ID)
	return response.ToTaskResp(task), nil
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
	// 按服务器筛选时先校验服务器归属，不存在返回 404 而不是空列表
	if req.ServerID != 0 {
		server, err := s.serverRepo.GetByID(ctx, userID, req.ServerID)
		if err != nil {
			slog.ErrorContext(ctx, "服务器查询失败", "userID", userID, "serverID", req.ServerID, "error", err)
			return nil, apperr.NewBizError(500, "服务器查询失败")
		}
		if server == nil {
			return nil, apperr.NewBizError(404, "服务器不存在")
		}
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

func (s *service) Update(ctx context.Context, userID uint64, dstID uint64, req *request.UpdateDstReq) (*response.TaskResp, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, apperr.NewBizError(400, "存档名称为空")
	}

	dst, err := s.dstRepo.GetByID(ctx, userID, dstID)
	if err != nil {
		slog.ErrorContext(ctx, "存档查询失败", "userID", userID, "dstID", dstID, "error", err)
		return nil, apperr.NewBizError(500, "存档更新失败")
	}
	if dst == nil {
		return nil, apperr.NewBizError(404, "存档不存在")
	}

	// 令牌为 nil 表示不修改：配置文件仍要写入令牌，解密原令牌
	encryptedToken, token := dst.ClusterToken, ""
	if req.ClusterToken != nil {
		token = strings.TrimSpace(*req.ClusterToken)
		if token == "" {
			return nil, apperr.NewBizError(400, "科雷令牌为空")
		}
		encryptedToken, err = s.encryptor.Encrypt(token)
	} else {
		token, err = s.encryptor.Decrypt(dst.ClusterToken)
	}
	if err != nil {
		slog.ErrorContext(ctx, "科雷令牌加解密失败", "userID", userID, "dstID", dstID, "error", err)
		return nil, apperr.NewBizError(500, "存档更新失败")
	}

	// 指针字段为 nil 表示不修改，沿用原值；写入配置需要完整的存档信息
	pending := pendingDst{
		ServerID:     dst.ServerID,
		Slot:         dst.Slot,
		Name:         name,
		Description:  valueOr(req.Description, dst.Description),
		Password:     valueOr(req.Password, dst.Password),
		MaxPlayers:   req.MaxPlayers,
		ClusterToken: encryptedToken,
		Mods:         valueOr(req.Mods, dst.Mods),
	}
	if pending.Mods == nil {
		pending.Mods = []string{}
	}
	dsts, _, err := s.dstRepo.List(ctx, userID, dst.ServerID, 1, dstSlotCount)
	if err != nil {
		slog.ErrorContext(ctx, "存档列表查询失败", "userID", userID, "serverID", dst.ServerID, "error", err)
		return nil, apperr.NewBizError(500, "存档更新失败")
	}

	task, err := s.dispatchConfig(ctx, userID, agentcommand.Command{
		TaskType:     entity.TaskTypeDstUpdate,
		TaskName:     fmt.Sprintf("更新存档配置（槽位 %d）", dst.Slot),
		ResourceType: entity.TaskResourceDst,
		ResourceID:   dstID,
	}, pending, token, dsts)
	if err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "存档更新任务已创建", "userID", userID, "dstID", dstID, "taskID", task.ID)
	return response.ToTaskResp(task), nil
}

func (s *service) Start(ctx context.Context, userID uint64, dstID uint64) (*response.TaskResp, error) {
	return s.dispatchControl(ctx, userID, dstID, protocol.ActionStart, entity.TaskTypeDstStart, "启动存档")
}

func (s *service) Stop(ctx context.Context, userID uint64, dstID uint64) (*response.TaskResp, error) {
	return s.dispatchControl(ctx, userID, dstID, protocol.ActionStop, entity.TaskTypeDstStop, "停止存档")
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

	// 通知 Agent 清理服务器上的存档文件，结果在任务中查看；Agent 离线或正忙时只删除记录，文件需手动清理
	_, err = s.commands.Dispatch(ctx, agentcommand.Command{
		UserID:       userID,
		ServerID:     dst.ServerID,
		TaskType:     entity.TaskTypeDstDelete,
		TaskName:     fmt.Sprintf("清理存档文件（槽位 %d）", dst.Slot),
		ResourceType: entity.TaskResourceDst,
		ResourceID:   dstID,
		Payload:      protocol.CmdPayload{Action: protocol.ActionDelete, Slot: dst.Slot},
	})
	if err != nil {
		slog.WarnContext(ctx, "存档清理命令下发失败", "serverID", dst.ServerID, "slot", dst.Slot, "error", err)
	}
	return nil
}

// dispatchControl 下发启动、停止这类只需槽位的命令，任务名形如"启动存档「名称」"
func (s *service) dispatchControl(ctx context.Context, userID uint64, dstID uint64, action string, taskType string, verb string) (*response.TaskResp, error) {
	dst, err := s.dstRepo.GetByID(ctx, userID, dstID)
	if err != nil {
		slog.ErrorContext(ctx, "存档查询失败", "userID", userID, "dstID", dstID, "error", err)
		return nil, apperr.NewBizError(500, verb+"失败")
	}
	if dst == nil {
		return nil, apperr.NewBizError(404, "存档不存在")
	}

	task, err := s.commands.Dispatch(ctx, agentcommand.Command{
		UserID:       userID,
		ServerID:     dst.ServerID,
		TaskType:     taskType,
		TaskName:     fmt.Sprintf("%s「%s」", verb, dst.Name),
		ResourceType: entity.TaskResourceDst,
		ResourceID:   dstID,
		Payload:      protocol.CmdPayload{Action: action, Slot: dst.Slot},
	})
	if err != nil {
		return nil, err // 已是 BizError
	}
	slog.InfoContext(ctx, "存档命令已下发", "userID", userID, "dstID", dstID, "action", action, "taskID", task.ID)
	return response.ToTaskResp(task), nil
}

// checkSlot 校验服务器归属、已部署饥荒且槽位空闲，返回该服务器已有的存档
func (s *service) checkSlot(ctx context.Context, userID uint64, serverID uint64, slot int) ([]entity.Dst, error) {
	server, err := s.serverRepo.GetByID(ctx, userID, serverID)
	if err != nil {
		slog.ErrorContext(ctx, "服务器查询失败", "userID", userID, "serverID", serverID, "error", err)
		return nil, apperr.NewBizError(500, "服务器查询失败")
	}
	if server == nil {
		return nil, apperr.NewBizError(404, "服务器不存在")
	}
	if !slices.ContainsFunc(server.GameEnv, func(e entity.GameEnvRef) bool { return e.Code == entity.GameCodeDst }) {
		return nil, apperr.NewBizError(409, "该服务器未部署饥荒联机版，请先部署")
	}

	dsts, _, err := s.dstRepo.List(ctx, userID, serverID, 1, dstSlotCount)
	if err != nil {
		slog.ErrorContext(ctx, "存档列表查询失败", "userID", userID, "serverID", serverID, "error", err)
		return nil, apperr.NewBizError(500, "存档查询失败")
	}
	if slices.ContainsFunc(dsts, func(d entity.Dst) bool { return d.Slot == slot }) {
		return nil, apperr.NewBizError(409, "该服务器的存档槽位已被占用")
	}
	return dsts, nil
}

// dispatchConfig 生成存档配置并下发写入任务，cmd 只需填任务信息；任务成功后由注册的处理按 pending 保存。
// serverDsts 为该服务器已有的存档，与本存档槽位相同的（编辑时即它自己）不计入模组汇总
func (s *service) dispatchConfig(ctx context.Context, userID uint64, cmd agentcommand.Command, pending pendingDst, token string, serverDsts []entity.Dst) (*entity.Task, error) {
	serverMods := slices.Clone(pending.Mods)
	for _, d := range serverDsts {
		if d.Slot != pending.Slot {
			serverMods = append(serverMods, d.Mods...)
		}
	}
	files, err := renderConfig(pending.toEntity(userID), token, serverMods)
	if err != nil {
		slog.ErrorContext(ctx, "存档配置生成失败", "serverID", pending.ServerID, "slot", pending.Slot, "error", err)
		return nil, apperr.NewBizError(500, "存档配置生成失败")
	}
	params, err := json.Marshal(pending)
	if err != nil {
		slog.ErrorContext(ctx, "存档参数序列化失败", "serverID", pending.ServerID, "slot", pending.Slot, "error", err)
		return nil, apperr.NewBizError(500, "存档配置生成失败")
	}

	cmd.UserID = userID
	cmd.ServerID = pending.ServerID
	cmd.Params = map[string]string{paramDst: string(params)}
	cmd.Payload = protocol.CmdPayload{Action: protocol.ActionConfig, Slot: pending.Slot, Files: files}
	return s.commands.Dispatch(ctx, cmd) // 错误已是 BizError
}

// onCreated 配置写入成功后保存存档。先插入存档再复查服务器：与删除服务器的 NOT EXISTS 条件互斥，
// 避免并发删除服务器后留下孤儿存档。同一服务器的命令串行执行且下发前已校验槽位，正常不会重复插入
func (s *service) onCreated(ctx context.Context, task *entity.Task) error {
	var pending pendingDst
	if err := json.Unmarshal([]byte(task.Params[paramDst]), &pending); err != nil {
		return fmt.Errorf("创建任务参数解析失败: %w", err)
	}
	dst := pending.toEntity(task.UserID)
	err := s.txManager.ExecTx(ctx, func(txCtx context.Context) error {
		if err := s.dstRepo.Create(txCtx, dst); err != nil {
			return err
		}
		server, err := s.serverRepo.GetByID(txCtx, task.UserID, dst.ServerID)
		if err != nil {
			return err
		}
		if server == nil {
			return errServerGone
		}
		return nil
	})
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "存档创建完成", "userID", task.UserID, "serverID", dst.ServerID, "slot", dst.Slot, "dstID", dst.ID, "taskID", task.ID)
	return nil
}

// onUpdated 配置写入成功后更新存档；所有字段都按写入服务器的值保存
func (s *service) onUpdated(ctx context.Context, task *entity.Task) error {
	var pending pendingDst
	if err := json.Unmarshal([]byte(task.Params[paramDst]), &pending); err != nil {
		return fmt.Errorf("更新任务参数解析失败: %w", err)
	}
	err := s.dstRepo.Update(ctx, task.UserID, task.ResourceID, dstrepo.UpdateFields{
		Name:         pending.Name,
		Description:  &pending.Description,
		MaxPlayers:   pending.MaxPlayers,
		Mods:         &pending.Mods,
		Password:     &pending.Password,
		ClusterToken: &pending.ClusterToken,
	})
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "存档更新完成", "userID", task.UserID, "dstID", task.ResourceID, "taskID", task.ID)
	return nil
}

// statusUpdater 返回启动、停止成功后的处理：把存档改为指定状态；重复执行结果一致
func (s *service) statusUpdater(status int) agentcommand.SucceededHandler {
	return func(ctx context.Context, task *entity.Task) error {
		if err := s.dstRepo.UpdateStatus(ctx, task.UserID, task.ResourceID, status); err != nil {
			return err
		}
		slog.InfoContext(ctx, "存档状态已更新", "dstID", task.ResourceID, "status", status, "taskID", task.ID)
		return nil
	}
}

func (p pendingDst) toEntity(userID uint64) *entity.Dst {
	return &entity.Dst{
		ServerID:     p.ServerID,
		UserID:       userID,
		Slot:         p.Slot,
		Name:         p.Name,
		Description:  p.Description,
		Password:     p.Password,
		MaxPlayers:   p.MaxPlayers,
		ClusterToken: p.ClusterToken,
		Mods:         p.Mods,
	}
}

// valueOr 指针为 nil 时返回默认值
func valueOr[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}
