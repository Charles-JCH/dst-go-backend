package agentevent

import (
	"game-panel/internal/protocol"
	"log/slog"
)

// Service 处理 Agent 回传的事件：更新任务状态、保存日志、转发给浏览器
type Service interface {
	OnAck(serverID uint64, id string, ack protocol.Ack)
	OnLog(serverID uint64, id string, line protocol.Log)
	OnResult(serverID uint64, id string, res protocol.Result)
	OnInterrupted(serverID uint64, id string, reason string)
}

type service struct{}

func NewService() Service {
	return &service{}
}

func (s *service) OnAck(serverID uint64, id string, ack protocol.Ack) {
	slog.Info("agent 命令确认",
		"serverID", serverID,
		"commandID", id,
		"accepted", ack.Accepted,
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

func (s *service) OnResult(serverID uint64, id string, res protocol.Result) {
	slog.Info("agent 执行结果",
		"serverID", serverID,
		"commandID", id,
		"success", res.Success,
		"exitCode", res.ExitCode,
		"error", res.Error,
	)
}

func (s *service) OnInterrupted(serverID uint64, id string, reason string) {
	slog.Warn("agent 命令结果未知",
		"serverID", serverID,
		"commandID", id,
		"reason", reason,
	)
}
