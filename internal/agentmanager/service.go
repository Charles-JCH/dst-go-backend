package agentmanager

import (
	"encoding/json"
	"fmt"
	"game-panel/internal/protocol"
	"github.com/google/uuid"
)

type Service struct {
	hub           *Hub
	OnAck         func(serverID uint64, id string, ack protocol.Ack)
	OnLog         func(serverID uint64, id string, line protocol.Log)
	OnResult      func(serverID uint64, id string, result protocol.Result)
	OnSysInfo     func(serverID uint64, info protocol.SysInfo)
	OnInterrupted func(serverID uint64, id string, reason string)
	OnOnline      func(serverID uint64)
	OnOffline     func(serverID uint64)
}

func NewService(hub *Hub) *Service {
	s := &Service{hub: hub}

	hub.OnInterrupted = func(serverID uint64, id string) {
		if s.OnInterrupted != nil {
			s.OnInterrupted(
				serverID,
				id,
				"agent 连接中断，结果未知",
			)
		}
	}
	hub.OnOnline = func(serverID uint64) {
		if s.OnOnline != nil {
			s.OnOnline(serverID)
		}
	}
	hub.OnOffline = func(serverID uint64) {
		if s.OnOffline != nil {
			s.OnOffline(serverID)
		}
	}
	return s
}

func (s *Service) Deploy(serverID uint64) (string, error) {
	return s.send(serverID, protocol.CmdPayload{Action: protocol.ActionDeploy})
}

func (s *Service) Init(serverID uint64, slot int, clusterToken string) (string, error) {
	return s.send(serverID, protocol.CmdPayload{
		Action: protocol.ActionInit, Slot: slot, ClusterToken: clusterToken,
	})
}

func (s *Service) Start(serverID uint64, slot int) (string, error) {
	return s.send(serverID, protocol.CmdPayload{Action: protocol.ActionStart, Slot: slot})
}

func (s *Service) Stop(serverID uint64, slot int) (string, error) {
	return s.send(serverID, protocol.CmdPayload{Action: protocol.ActionStop, Slot: slot})
}

func (s *Service) Update(serverID uint64) (string, error) {
	return s.send(serverID, protocol.CmdPayload{Action: protocol.ActionUpdate})
}

func (s *Service) Delete(serverID uint64, slot int) (string, error) {
	return s.send(serverID, protocol.CmdPayload{Action: protocol.ActionDelete, Slot: slot})
}

func (s *Service) Status(serverID uint64, slot int) (string, error) {
	return s.send(serverID, protocol.CmdPayload{Action: protocol.ActionStatus, Slot: slot})
}

func (s *Service) IsOnline(serverID uint64) bool {
	return s.hub.IsOnline(serverID)
}

// HasPendingCommand Agent 是否正在执行平台下发的命令；更新 Agent 会中断命令，需先等它结束
func (s *Service) HasPendingCommand(serverID uint64) bool {
	return s.hub.HasPending(serverID)
}

func (s *Service) OnlineCount() int {
	return s.hub.Count()
}

// Disconnect 断开服务器的 Agent 连接，用于删除服务器或重装 Agent
func (s *Service) Disconnect(serverID uint64) {
	s.hub.Disconnect(serverID)
}

// Uninstall 通知 Agent 停止服务并删除安装文件，Agent 不回传结果
func (s *Service) Uninstall(serverID uint64) error {
	return s.hub.Send(serverID, protocol.Envelope{Type: protocol.TypeUninstall})
}

func (s *Service) HandleAck(serverID uint64, id string, ack protocol.Ack) {
	if !ack.IsAccepted {
		if !s.hub.Finish(serverID, id) {
			return
		}
	}
	if s.OnAck != nil {
		s.OnAck(serverID, id, ack)
	}
}

func (s *Service) HandleLog(serverID uint64, id string, line protocol.Log) {
	if s.OnLog != nil {
		s.OnLog(serverID, id, line)
	}
}

func (s *Service) HandleResult(serverID uint64, id string, result protocol.Result) {
	if !s.hub.Finish(serverID, id) {
		return
	}
	if s.OnResult != nil {
		s.OnResult(serverID, id, result)
	}
}

func (s *Service) HandleSysInfo(serverID uint64, info protocol.SysInfo) {
	if s.OnSysInfo != nil {
		s.OnSysInfo(serverID, info)
	}
}

func (s *Service) send(serverID uint64, payload protocol.CmdPayload) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("命令序列化失败: %w", err)
	}
	env := protocol.Envelope{
		ID:      uuid.NewString(),
		Type:    protocol.TypeCommand,
		Payload: data,
	}
	if err := s.hub.Send(serverID, env); err != nil {
		return "", err
	}
	return env.ID, nil
}
