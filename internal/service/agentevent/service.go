package agentevent

import (
	"context"
	"errors"
	"game-panel/internal/model/response"
	"game-panel/internal/protocol"
	"game-panel/internal/repository"
	serverrepo "game-panel/internal/repository/server"
	"log/slog"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	queryTimeout = 5 * time.Second
	maxCPU       = 4096 // 上报的 CPU 核数超出范围视为无效
	maxOSName    = 128  // 与 server.os_name 列宽一致
)

// archPattern Go 的架构名，如 amd64、arm64
var archPattern = regexp.MustCompile(`^[a-z0-9]{1,16}$`)

// Service 处理 Agent 回传的事件：更新任务状态、保存日志、转发给浏览器
type Service interface {
	OnAck(serverID uint64, id string, ack protocol.Ack)
	OnLog(serverID uint64, id string, line protocol.Log)
	OnResult(serverID uint64, id string, result protocol.Result)
	OnSysInfo(serverID uint64, info protocol.SysInfo)
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

// OnSysInfo 保存 Agent 上报的系统配置并推送给服务器所属用户；Agent 不可信，入库前校验
func (s *service) OnSysInfo(serverID uint64, info protocol.SysInfo) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	fields := sanitizeSysInfo(info)
	err := s.serverRepo.UpdateSysInfo(ctx, serverID, fields)
	if errors.Is(err, repository.ErrNotFound) {
		return // 服务器已删除
	}
	if err != nil {
		slog.ErrorContext(ctx, "服务器配置保存失败", "serverID", serverID, "error", err)
		return
	}

	server, err := s.serverRepo.FindByID(ctx, serverID)
	if err != nil || server == nil {
		slog.WarnContext(ctx, "服务器查询失败，跳过配置推送", "serverID", serverID, "error", err)
		return
	}
	s.notifier.Publish(server.UserID, response.EventSysInfo, response.SysInfoResp{
		ServerID: serverID,
		CPU:      fields.CPU,
		RAM:      fields.RAM,
		Disk:     fields.Disk,
		OSName:   fields.OSName,
		Arch:     fields.Arch,
	})
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

// sanitizeSysInfo 超出范围的数值置 0，系统名称截断到列宽，架构名不合法时置空
func sanitizeSysInfo(info protocol.SysInfo) serverrepo.SysInfoFields {
	fields := serverrepo.SysInfoFields{
		CPU:    info.CPU,
		RAM:    max(info.RAM, 0),
		Disk:   max(info.Disk, 0),
		OSName: strings.TrimSpace(strings.ToValidUTF8(info.OSName, "")),
		Arch:   info.Arch,
	}
	if fields.CPU < 0 || fields.CPU > maxCPU {
		fields.CPU = 0
	}
	if utf8.RuneCountInString(fields.OSName) > maxOSName {
		fields.OSName = string([]rune(fields.OSName)[:maxOSName])
	}
	if !archPattern.MatchString(fields.Arch) {
		fields.Arch = ""
	}
	return fields
}
