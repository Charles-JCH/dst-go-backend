package agent_manager

import (
	"fmt"
	"game-panel/internal/protocol"
	"github.com/gorilla/websocket"
	"sync"
	"time"
)

type agentConn struct {
	conn    *websocket.Conn
	mu      sync.Mutex
	pending map[string]struct{}
	closed  bool
}

type Hub struct {
	conns         map[uint64]*agentConn
	mu            sync.RWMutex
	OnInterrupted func(serverID uint64, commandID string)
}

func NewHub() *Hub {
	return &Hub{
		conns: make(map[uint64]*agentConn),
	}
}

func (h *Hub) Register(serverID uint64, conn *websocket.Conn) {
	h.mu.Lock()
	old := h.conns[serverID]
	h.conns[serverID] = &agentConn{
		conn:    conn,
		pending: make(map[string]struct{}),
	}
	h.mu.Unlock()

	if old != nil {
		h.closeAgent(serverID, old)
	}
}

func (h *Hub) Unregister(serverID uint64, conn *websocket.Conn) {
	h.mu.Lock()
	current := h.conns[serverID]
	if current == nil || current.conn != conn {
		h.mu.Unlock()
		return
	}
	delete(h.conns, serverID)
	h.mu.Unlock()

	h.closeAgent(serverID, current)
}

func (h *Hub) closeAgent(serverID uint64, agent *agentConn) {
	agent.mu.Lock()
	agent.closed = true
	_ = agent.conn.Close()

	ids := make([]string, 0, len(agent.pending))
	for id := range agent.pending {
		ids = append(ids, id)
	}
	clear(agent.pending)
	agent.mu.Unlock()

	if h.OnInterrupted != nil {
		for _, id := range ids {
			h.OnInterrupted(serverID, id)
		}
	}
}

func (h *Hub) Send(serverID uint64, env protocol.Envelope) error {
	h.mu.RLock()
	agent, ok := h.conns[serverID]
	h.mu.RUnlock()

	if !ok {
		return fmt.Errorf("agent 离线 serverID=%d", serverID)
	}

	agent.mu.Lock()
	defer agent.mu.Unlock()

	if agent.closed {
		return fmt.Errorf("agent 连接已关闭 serverID=%d", serverID)
	}

	if err := agent.conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		_ = agent.conn.Close()
		return fmt.Errorf("agent 写超时设置失败: %w", err)
	}

	if env.Type == protocol.TypeCommand {
		agent.pending[env.ID] = struct{}{}
	}

	if err := agent.conn.WriteJSON(env); err != nil {
		delete(agent.pending, env.ID)
		_ = agent.conn.Close()
		return fmt.Errorf("命令发送失败，结果未知: %w", err)
	}

	return nil
}

func (h *Hub) Finish(serverID uint64, id string) bool {
	h.mu.RLock()
	agent := h.conns[serverID]
	h.mu.RUnlock()

	if agent == nil {
		return false
	}

	agent.mu.Lock()
	defer agent.mu.Unlock()

	if _, ok := agent.pending[id]; !ok {
		return false
	}
	delete(agent.pending, id)
	return true
}

func (h *Hub) IsOnline(serverID uint64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	_, ok := h.conns[serverID]
	return ok
}

func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.conns)
}
