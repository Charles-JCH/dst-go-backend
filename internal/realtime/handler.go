package realtime

import (
	"context"
	"game-panel/internal/service/auth"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log/slog"
	"net/http"
	"slices"
	"time"
)

const (
	authTimeout  = 10 * time.Second // 连接后必须在这段时间内发送 ticket
	pingInterval = 30 * time.Second
	readTimeout  = 70 * time.Second // 超过这段时间没收到 pong 视为断开
	writeTimeout = 10 * time.Second
	readLimit    = 4 * 1024
)

// authMessage 浏览器连接后发送的第一条消息；ticket 不放在 URL 里，避免被访问日志记录
type authMessage struct {
	Type   string `json:"type"`
	Ticket string `json:"ticket"`
}

type Handler struct {
	hub         *Hub
	authService auth.Service
	upgrader    websocket.Upgrader
}

func NewHandler(hub *Hub, authService auth.Service, allowedOrigins []string) *Handler {
	return &Handler{
		hub:         hub,
		authService: authService,
		upgrader: websocket.Upgrader{
			// 浏览器会自动带 Origin，只接受配置中的前端地址
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				return origin != "" && slices.Contains(allowedOrigins, origin)
			},
		},
	}
}

// ServeWS 浏览器实时事件连接
// [GET] /api/v1/ws
func (h *Handler) ServeWS(c *gin.Context) {
	ctx := c.Request.Context()

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade 失败时已向客户端返回错误响应
		slog.WarnContext(ctx, "浏览器 websocket 升级失败", "error", err)
		return
	}
	conn.SetReadLimit(readLimit)

	userID, ok := h.authenticate(ctx, conn)
	if !ok {
		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "unauthorized"),
			time.Now().Add(writeTimeout),
		)
		_ = conn.Close()
		return
	}

	cl := newClient(conn)
	h.hub.register(userID, cl)
	defer h.hub.unregister(userID, cl)
	defer cl.close()

	go writeLoop(cl)
	readLoop(cl)
}

// authenticate 读取第一条消息并消费一次性 ticket
func (h *Handler) authenticate(ctx context.Context, conn *websocket.Conn) (uint64, bool) {
	if err := conn.SetReadDeadline(time.Now().Add(authTimeout)); err != nil {
		return 0, false
	}
	var msg authMessage
	if err := conn.ReadJSON(&msg); err != nil || msg.Type != "auth" || msg.Ticket == "" {
		return 0, false
	}
	userID, err := h.authService.ConsumeWSTicket(ctx, msg.Ticket)
	if err != nil {
		return 0, false
	}
	return userID, true
}

// writeLoop 所有写操作都在这里，保证同一连接不会并发写
func writeLoop(cl *client) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	defer cl.close()

	for {
		select {
		case <-cl.done:
			return
		case event := <-cl.send:
			_ = cl.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := cl.conn.WriteJSON(event); err != nil {
				return
			}
		case <-ticker.C:
			if err := cl.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeTimeout)); err != nil {
				return
			}
		}
	}
}

// readLoop 浏览器不需要发送业务消息，这里只处理 pong 和检测断开
func readLoop(cl *client) {
	_ = cl.conn.SetReadDeadline(time.Now().Add(readTimeout))
	cl.conn.SetPongHandler(func(string) error {
		return cl.conn.SetReadDeadline(time.Now().Add(readTimeout))
	})
	for {
		if _, _, err := cl.conn.ReadMessage(); err != nil {
			return
		}
	}
}
