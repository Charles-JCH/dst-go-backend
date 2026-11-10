package agentcommand

import (
	"context"
	"errors"
	"fmt"
	"game-panel/internal/apperr"
	"game-panel/internal/model/entity"
	"game-panel/internal/model/response"
	"game-panel/internal/protocol"
	"game-panel/internal/repository"
	taskrepo "game-panel/internal/repository/task"
	"github.com/google/uuid"
	"log/slog"
	"sync"
	"time"
)

const queryTimeout = 5 * time.Second

// Service 把下发给 Agent 的命令作为任务管理：创建任务 → 下发 → 确认后执行中 → 保存日志 → 按结果结束任务。
// 同一台服务器同时只能有一个命令任务，与 Agent 一次只执行一条命令一致
type Service interface {
	// Dispatch 创建命令任务并下发；Agent 离线、正在安装或已有命令在执行时返回 409
	Dispatch(ctx context.Context, cmd Command) (*entity.Task, error)
	// OnSucceeded 注册某类任务执行成功后的业务处理（如部署成功后记录已部署的游戏）；
	// 处理失败时任务标记为失败。只在启动时调用
	OnSucceeded(taskType string, handler SucceededHandler)
	// Recover 平台启动时调用：待执行的命令任务已不知道是否发出，标记失败
	Recover(ctx context.Context) error

	// 以下处理 Agent 回传的事件，由 agentmanager 回调
	OnAck(serverID uint64, commandID string, ack protocol.Ack)
	OnLog(serverID uint64, commandID string, line protocol.Log)
	OnResult(serverID uint64, commandID string, result protocol.Result)
	OnInterrupted(serverID uint64, commandID string, reason string)
}

// Command 一条要下发的命令及其任务信息
type Command struct {
	UserID       uint64
	ServerID     uint64
	TaskType     string
	TaskName     string
	ResourceType string
	ResourceID   uint64
	Params       map[string]string
	Payload      protocol.CmdPayload
}

// SucceededHandler 命令执行成功后的业务处理，result 为 Agent 回传的结果（如查看状态的退出码）；
// 返回的错误会让任务标记为失败
type SucceededHandler func(ctx context.Context, task *entity.Task, result protocol.Result) error

// AgentSender 查询 Agent 在线状态并下发命令（agentmanager.Service 未导出接口）
type AgentSender interface {
	IsOnline(serverID uint64) bool
	Send(serverID uint64, commandID string, payload protocol.CmdPayload) error
}

// Notifier 向浏览器推送实时事件（realtime.Hub 未导出接口）
type Notifier interface {
	Publish(userID uint64, eventType string, data any)
}

type service struct {
	taskRepo taskrepo.Repository
	agent    AgentSender
	notifier Notifier
	handlers map[string]SucceededHandler
	// running 进行中的命令 commandID → 任务，Agent 每输出一行日志都要找任务，避免每次查库；任务结束时删除
	running sync.Map
}

// runningTask 进行中命令的任务信息
type runningTask struct {
	taskID   uint64
	userID   uint64
	taskType string
}

func NewService(taskRepo taskrepo.Repository, agent AgentSender, notifier Notifier) Service {
	return &service{
		taskRepo: taskRepo,
		agent:    agent,
		notifier: notifier,
		handlers: make(map[string]SucceededHandler),
	}
}

func (s *service) Dispatch(ctx context.Context, cmd Command) (*entity.Task, error) {
	if !s.agent.IsOnline(cmd.ServerID) {
		return nil, apperr.NewBizError(409, "Agent 离线，请先重新连接")
	}
	isInstalling, err := s.hasActiveInstall(ctx, cmd.UserID, cmd.ServerID)
	if err != nil {
		slog.ErrorContext(ctx, "任务查询失败", "serverID", cmd.ServerID, "error", err)
		return nil, apperr.NewBizError(500, "命令下发失败")
	}
	if isInstalling {
		return nil, apperr.NewBizError(409, "Agent 正在安装或更新，请稍后再试")
	}

	// 先生成命令 ID 并保存任务，再下发：Agent 的确认可能比写库更快，必须先能按 ID 找到任务
	task := &entity.Task{
		Name:         cmd.TaskName,
		Type:         cmd.TaskType,
		Status:       entity.TaskStatusPending,
		Stage:        entity.TaskStageQueued,
		ResourceType: cmd.ResourceType,
		ResourceID:   cmd.ResourceID,
		UserID:       cmd.UserID,
		CommandID:    uuid.NewString(),
		ActiveKey:    entity.AgentCommandActiveKey(cmd.ServerID),
		Params:       cmd.Params,
	}
	err = s.taskRepo.Create(ctx, task)
	if errors.Is(err, repository.ErrDuplicate) {
		return nil, apperr.NewBizError(409, "服务器正在执行其他操作，请稍后再试")
	}
	if err != nil {
		slog.ErrorContext(ctx, "命令任务创建失败", "serverID", cmd.ServerID, "taskType", cmd.TaskType, "error", err)
		return nil, apperr.NewBizError(500, "命令下发失败")
	}
	s.running.Store(task.CommandID, runningTask{taskID: task.ID, userID: task.UserID, taskType: task.Type})
	s.publishTask(ctx, task.ID)

	if err := s.agent.Send(cmd.ServerID, task.CommandID, cmd.Payload); err != nil {
		slog.WarnContext(ctx, "命令下发失败", "serverID", cmd.ServerID, "taskID", task.ID, "error", err)
		s.finish(ctx, task.CommandID, entity.TaskStatusFailed, "命令下发失败，Agent 可能已离线")
		return nil, apperr.NewBizError(409, "命令下发失败，Agent 可能已离线，请稍后再试")
	}
	slog.InfoContext(ctx, "命令已下发", "serverID", cmd.ServerID, "taskID", task.ID, "action", cmd.Payload.Action)
	return task, nil
}

func (s *service) OnSucceeded(taskType string, handler SucceededHandler) {
	s.handlers[taskType] = handler
}

func (s *service) Recover(ctx context.Context) error {
	count, err := s.taskRepo.FailPendingCommands(ctx, "平台重启，命令可能未执行，请重试")
	if err != nil {
		return fmt.Errorf("待执行命令任务清理失败: %w", err)
	}
	if count > 0 {
		slog.WarnContext(ctx, "待执行命令任务已标记失败", "count", count)
	}
	return nil
}

func (s *service) OnAck(serverID uint64, commandID string, ack protocol.Ack) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	if !ack.IsAccepted {
		slog.WarnContext(ctx, "Agent 拒绝执行命令", "serverID", serverID, "commandID", commandID, "reason", ack.Reason)
		s.finish(ctx, commandID, entity.TaskStatusFailed, "Agent 拒绝执行："+ack.Reason)
		return
	}

	t, ok := s.lookup(ctx, commandID)
	if !ok {
		return
	}
	if err := s.taskRepo.Start(ctx, t.taskID); err != nil {
		if !errors.Is(err, taskrepo.ErrStatusConflict) { // 重复确认时任务已开始，忽略
			slog.ErrorContext(ctx, "任务状态更新失败", "taskID", t.taskID, "error", err)
		}
		return
	}
	if err := s.taskRepo.UpdateStage(ctx, t.taskID, entity.TaskStageExecuting); err != nil {
		slog.WarnContext(ctx, "任务步骤更新失败", "taskID", t.taskID, "error", err)
	}
	s.publishTask(ctx, t.taskID)
}

// OnLog 每行输出保存为一条任务日志并推送；保存失败不影响命令执行
func (s *service) OnLog(_ uint64, commandID string, line protocol.Log) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	t, ok := s.lookup(ctx, commandID)
	if !ok {
		return
	}
	l := &entity.TaskLog{TaskID: t.taskID, Content: line.Content}
	if err := s.taskRepo.CreateLog(ctx, l); err != nil {
		slog.WarnContext(ctx, "任务日志保存失败", "taskID", t.taskID, "error", err)
		return
	}
	s.notifier.Publish(t.userID, response.EventTaskLog, response.ToTaskLogResp(l))
}

func (s *service) OnResult(serverID uint64, commandID string, result protocol.Result) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	slog.InfoContext(ctx, "命令执行结束",
		"serverID", serverID,
		"commandID", commandID,
		"action", result.Action,
		"success", result.IsSuccess,
		"exitCode", result.ExitCode,
		"error", result.Error,
	)
	if !result.IsSuccess {
		s.finish(ctx, commandID, entity.TaskStatusFailed, "执行失败："+result.Error)
		return
	}

	status, message := entity.TaskStatusSucceeded, ""
	if err := s.handleSucceeded(ctx, commandID, result); err != nil {
		slog.ErrorContext(ctx, "命令执行成功后的处理失败", "commandID", commandID, "error", err)
		status, message = entity.TaskStatusFailed, "执行成功，但结果保存失败，请重试"
	}
	s.finish(ctx, commandID, status, message)
}

func (s *service) OnInterrupted(serverID uint64, commandID string, reason string) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	slog.WarnContext(ctx, "命令结果未知", "serverID", serverID, "commandID", commandID, "reason", reason)
	s.finish(ctx, commandID, entity.TaskStatusFailed, "Agent 连接中断，执行结果未知")
}

// hasActiveInstall 安装或更新 Agent 会重启它，期间不下发命令
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

func (s *service) publishTask(ctx context.Context, taskID uint64) {
	t, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil || t == nil {
		slog.WarnContext(ctx, "任务查询失败，跳过推送", "taskID", taskID, "error", err)
		return
	}
	s.notifier.Publish(t.UserID, response.EventTask, response.ToTaskResp(t))
}

// finish 结束任务并推送；任务已结束（重复回传）时忽略
func (s *service) finish(ctx context.Context, commandID string, status string, message string) {
	t, ok := s.lookup(ctx, commandID)
	if !ok {
		return
	}
	s.running.Delete(commandID)
	if err := s.taskRepo.Finish(ctx, t.taskID, status, message); err != nil {
		if !errors.Is(err, taskrepo.ErrStatusConflict) {
			slog.ErrorContext(ctx, "任务状态更新失败", "taskID", t.taskID, "error", err)
		}
		return
	}
	s.publishTask(ctx, t.taskID)
}

// lookup 按命令 ID 找任务：先查内存，平台重启前下发的命令再查库
func (s *service) lookup(ctx context.Context, commandID string) (runningTask, bool) {
	if v, ok := s.running.Load(commandID); ok {
		return v.(runningTask), true
	}
	task, err := s.taskRepo.FindByCommandID(ctx, commandID)
	if err != nil {
		slog.WarnContext(ctx, "任务查询失败", "commandID", commandID, "error", err)
		return runningTask{}, false
	}
	if task == nil {
		return runningTask{}, false
	}
	return runningTask{taskID: task.ID, userID: task.UserID, taskType: task.Type}, true
}

// handleSucceeded 执行该类任务注册的成功处理，没有注册时直接成功
func (s *service) handleSucceeded(ctx context.Context, commandID string, result protocol.Result) error {
	t, ok := s.lookup(ctx, commandID)
	if !ok {
		return nil
	}
	handler := s.handlers[t.taskType]
	if handler == nil {
		return nil
	}
	task, err := s.taskRepo.FindByID(ctx, t.taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return nil
	}
	return handler(ctx, task, result)
}
