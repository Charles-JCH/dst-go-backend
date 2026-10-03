package agent_manager

import (
	"context"
	"encoding/json"
	"errors"
	"game-panel/internal/protocol"
	"game-panel/internal/result"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	readTimeout = 90 * time.Second
	readLimit   = 1024 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024 * 64,
	WriteBufferSize: 1024 * 64,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Handler struct {
	hub     *Hub
	auth    *Auth
	service *Service
}

func NewHandler(hub *Hub, auth *Auth, service *Service) *Handler {
	return &Handler{
		hub:     hub,
		auth:    auth,
		service: service,
	}
}

func (h *Handler) ServeWS(c *gin.Context) {
	ctx := c.Request.Context()

	token, err := parseBearerToken(c.GetHeader("Authorization"))
	if err != nil {
		result.FailWithMsg(c, 401, "未授权")
		return
	}

	serverID, err := h.auth.Authenticate(token)
	if err != nil {
		result.FailWithMsg(c, 401, "agent 认证失败")
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.WarnContext(ctx, "websocket 升级失败", "serverID", serverID, "error", err)
		return
	}

	conn.SetReadLimit(readLimit)

	if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		_ = conn.Close()
		return
	}

	h.hub.Register(serverID, conn)
	defer h.hub.Unregister(serverID, conn)

	h.readLoop(ctx, serverID, conn)
}

func (h *Handler) readLoop(ctx context.Context, serverID uint64, conn *websocket.Conn) {
	for {
		var env protocol.Envelope
		if err := conn.ReadJSON(&env); err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				slog.InfoContext(ctx, "agent 连接关闭", "serverID", serverID)
			} else {
				slog.WarnContext(ctx, "agent 消息读取失败", "serverID", serverID, "error", err)
			}
			return
		}

		if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
			slog.ErrorContext(ctx, "agent 读取超时刷新失败", "serverID", serverID, "error", err)
			return
		}

		switch env.Type {
		case protocol.TypePing:
			continue
		case protocol.TypeAck:
			var ack protocol.Ack
			if err := json.Unmarshal(env.Payload, &ack); err != nil {
				slog.WarnContext(ctx, "命令确认解析失败", "serverID", serverID, "commandID", env.ID, "error", err)
				return
			}
			h.service.HandleAck(serverID, env.ID, ack)
		case protocol.TypeLog:
			var l protocol.Log
			if err := json.Unmarshal(env.Payload, &l); err != nil {
				slog.WarnContext(ctx, "执行日志解析失败", "serverID", serverID, "commandID", env.ID, "error", err)
				return
			}
			h.service.HandleLog(serverID, env.ID, l)
		case protocol.TypeResult:
			var res protocol.Result
			if err := json.Unmarshal(env.Payload, &res); err != nil {
				slog.WarnContext(ctx, "执行结果解析失败", "serverID", serverID, "commandID", env.ID, "error", err)
				return
			}
			h.service.HandleResult(serverID, env.ID, res)
		}
	}
}

func parseBearerToken(authorization string) (string, error) {
	authorization = strings.TrimSpace(authorization)
	if authorization == "" {
		return "", errors.New("authorization 为空")
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(authorization, prefix) {
		return "", errors.New("authorization 格式错误")
	}

	token := strings.TrimSpace(strings.TrimPrefix(authorization, prefix))
	if token == "" {
		return "", errors.New("agent token 为空")
	}

	return token, nil
}
