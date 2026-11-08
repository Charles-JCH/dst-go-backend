package realtime

import (
	"github.com/gorilla/websocket"
	"sync"
)

// sendBuffer 每个连接待发送事件的缓冲数（部署时日志可能短时间内大量到达），写满说明浏览器处理不过来，直接断开让它重连
const sendBuffer = 256

// Event 推送给浏览器的消息
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// Hub 按用户管理浏览器连接，同一用户可以同时打开多个页面
type Hub struct {
	mu      sync.RWMutex
	clients map[uint64]map[*client]struct{}
}

type client struct {
	conn      *websocket.Conn
	send      chan Event
	done      chan struct{}
	closeOnce sync.Once
}

func NewHub() *Hub {
	return &Hub{clients: make(map[uint64]map[*client]struct{})}
}

// Publish 向用户的所有连接推送事件，不阻塞调用方
func (h *Hub) Publish(userID uint64, eventType string, data any) {
	event := Event{Type: eventType, Data: data}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients[userID] {
		select {
		case c.send <- event:
		default:
			c.close()
		}
	}
}

func (h *Hub) register(userID uint64, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[userID] == nil {
		h.clients[userID] = make(map[*client]struct{})
	}
	h.clients[userID][c] = struct{}{}
}

func (h *Hub) unregister(userID uint64, c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.clients[userID], c)
	if len(h.clients[userID]) == 0 {
		delete(h.clients, userID)
	}
}

func newClient(conn *websocket.Conn) *client {
	return &client{conn: conn, send: make(chan Event, sendBuffer), done: make(chan struct{})}
}

// close 关闭连接并通知写协程退出，可重复调用
func (c *client) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}
