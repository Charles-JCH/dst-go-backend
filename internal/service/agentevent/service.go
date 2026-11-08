package agentevent

import (
	"context"
	"errors"
	"game-panel/internal/model/entity"
	"game-panel/internal/model/response"
	"game-panel/internal/protocol"
	"game-panel/internal/repository"
	"game-panel/internal/repository/cache"
	serverrepo "game-panel/internal/repository/server"
	"log/slog"
	"regexp"
	"strings"
	"sync"
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
	OnMetrics(serverID uint64, metrics protocol.Metrics)
	OnInterrupted(serverID uint64, id string, reason string)
	OnOnline(serverID uint64)
	OnOffline(serverID uint64)
}

// Notifier 向浏览器推送实时事件（realtime.Hub 未导出接口）
type Notifier interface {
	Publish(userID uint64, eventType string, data any)
}

type service struct {
	serverRepo   serverrepo.Repository
	metricsCache cache.MetricsCache
	notifier     Notifier
	// owners 在线服务器的所属用户 serverID → userID，心跳每 30 秒一次，避免每次都查库；Agent 下线时删除
	owners sync.Map
}

func NewService(serverRepo serverrepo.Repository, metricsCache cache.MetricsCache, notifier Notifier) Service {
	return &service{serverRepo: serverRepo, metricsCache: metricsCache, notifier: notifier}
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

	userID, ok := s.ownerID(ctx, serverID)
	if !ok {
		return
	}
	s.notifier.Publish(userID, response.EventSysInfo, response.SysInfoResp{
		ServerID: serverID,
		CPU:      fields.CPU,
		RAM:      fields.RAM,
		Disk:     fields.Disk,
		OSName:   fields.OSName,
		Arch:     fields.Arch,
	})
}

// OnMetrics 保存 Agent 随心跳上报的资源使用情况并推送给服务器所属用户；Agent 不可信，保存前校验
func (s *service) OnMetrics(serverID uint64, metrics protocol.Metrics) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	userID, ok := s.ownerID(ctx, serverID)
	if !ok {
		return
	}
	m := sanitizeMetrics(metrics, time.Now())
	// 保存失败只影响历史曲线，仍然推送最新值
	if err := s.metricsCache.Add(ctx, serverID, m); err != nil {
		slog.WarnContext(ctx, "资源使用情况保存失败", "serverID", serverID, "error", err)
	}
	s.notifier.Publish(userID, response.EventMetrics, response.ServerMetricsResp{
		ServerID:    serverID,
		MetricsResp: *response.ToMetricsResp(&m),
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
	s.owners.Delete(serverID)
}

// publishStatus 把在线状态推送给服务器所属用户；服务器已删除时不推送
func (s *service) publishStatus(serverID uint64, isOnline bool) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	userID, ok := s.ownerID(ctx, serverID)
	if !ok {
		return
	}
	s.notifier.Publish(userID, response.EventAgentStatus, response.AgentStatusResp{ServerID: serverID, IsOnline: isOnline})
}

// ownerID 查询服务器所属用户，结果缓存到 Agent 下线；服务器已删除或查询失败时返回 false
func (s *service) ownerID(ctx context.Context, serverID uint64) (uint64, bool) {
	if userID, ok := s.owners.Load(serverID); ok {
		return userID.(uint64), true
	}
	server, err := s.serverRepo.FindByID(ctx, serverID)
	if err != nil {
		slog.WarnContext(ctx, "服务器查询失败，跳过推送", "serverID", serverID, "error", err)
		return 0, false
	}
	if server == nil {
		return 0, false
	}
	s.owners.Store(serverID, server.UserID)
	return server.UserID, true
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

// sanitizeMetrics 数值限制在合理范围内：使用率 0～100，其余不小于 0
func sanitizeMetrics(m protocol.Metrics, now time.Time) entity.ServerMetrics {
	return entity.ServerMetrics{
		Time:       now,
		CPUPercent: min(max(m.CPUPercent, 0), 100),
		RAMUsed:    max(m.RAMUsed, 0),
		DiskUsed:   max(m.DiskUsed, 0),
		Load1:      max(m.Load1, 0),
		NetRx:      max(m.NetRx, 0),
		NetTx:      max(m.NetTx, 0),
	}
}
