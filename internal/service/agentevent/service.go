package agentevent

import (
	"context"
	"game-panel/internal/model/response"
	"game-panel/internal/protocol"
	serverrepo "game-panel/internal/repository/server"
	"log/slog"
	"time"
)

const queryTimeout = 5 * time.Second

// Service 处理 Agent 回传的事件：更新任务状态、保存日志、转发给浏览器
type Service interface {
	OnAck(serverID uint64, id string, ack protocol.Ack)
	OnLog(serverID uint64, id string, line protocol.Log)
	OnResult(serverID uint64, id string, result protocol.Result)
	OnInterrupted(serverID uint64, id string, reason string)
	OnOnline(serverID uint64)
	OnOffline(serverID uint64)
}

// Notifier 向浏览器推送实时事件（realtime.Hub 未导出接口）
type Notifier interface {
	Publish(userID uint64, eventType string, data any)
}

type service struct {
	serverRepo serverrepo.Repository
	notifier   Notifier
}

func NewService(serverRepo serverrepo.Repository, notifier Notifier) Service {
	return &service{serverRepo: serverRepo, notifier: notifier}
}

func (s *service) OnAck(serverID uint64, id string, ack protocol.Ack) {
	slog.Info("agent 命令确认",
		"serverID", serverID,
		"commandID", id,
		"accepted", ack.IsAccepted,
		"reason", ack.Reason,
	)
}

func (s *service) OnLog(serverID uint64, id string, line protocol.Log) {
	slog.Info("agent 执行日志",
		"serverID", serverID,
		"commandID", id,
		"content", line.Content,
	)
}

func (s *service) OnResult(serverID uint64, id string, result protocol.Result) {
	slog.Info("agent 执行结果",
		"serverID", serverID,
		"commandID", id,
		"success", result.IsSuccess,
		"exitCode", result.ExitCode,
		"error", result.Error,
	)
}

func (s *service) OnInterrupted(serverID uint64, id string, reason string) {
	slog.Warn("agent 命令结果未知",
		"serverID", serverID,
		"commandID", id,
		"reason", reason,
	)
}

func (s *service) OnOnline(serverID uint64) {
	slog.Info("agent 上线", "serverID", serverID)
	s.publishStatus(serverID, true)
}

func (s *service) OnOffline(serverID uint64) {
	slog.Info("agent 下线", "serverID", serverID)
	s.publishStatus(serverID, false)
}

// publishStatus 把在线状态推送给服务器所属用户；服务器已删除时不推送
func (s *service) publishStatus(serverID uint64, isOnline bool) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	server, err := s.serverRepo.FindByID(ctx, serverID)
	if err != nil {
		slog.WarnContext(ctx, "服务器查询失败，跳过 agent 状态推送", "serverID", serverID, "error", err)
		return
	}
	if server == nil {
		return
	}
	s.notifier.Publish(server.UserID, response.EventAgentStatus, response.AgentStatusResp{ServerID: serverID, IsOnline: isOnline})
}
