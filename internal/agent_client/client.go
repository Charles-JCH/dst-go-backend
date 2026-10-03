package agent_client

import (
	"context"
	"encoding/json"
	"fmt"
	"game-panel/internal/protocol"
	"github.com/gorilla/websocket"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type Client struct {
	cfg      ServerConfig
	identity IdentityConfig
	executor *Executor

	token   string
	writeMu sync.Mutex
}

func NewClient(serverCfg ServerConfig, identityCfg IdentityConfig, exec *Executor) *Client {
	return &Client{
		cfg:      serverCfg,
		identity: identityCfg,
		executor: exec,
	}
}

func (c *Client) Start(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)

	if token != "" {
		if err := SaveToken(c.identity.Path, token); err != nil {
			return err
		}
	} else {
		var err error
		token, err = LoadToken(c.identity.Path)
		if err != nil {
			return err
		}
	}

	c.token = token

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err := c.connectAndServe(ctx); err != nil && ctx.Err() == nil {
			_, _ = fmt.Fprintf(os.Stderr, "agent 连接异常: %v\n", err)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.cfg.ReconnectInterval):
		}
	}
}

func (c *Client) connectAndServe(ctx context.Context) error {
	header := http.Header{}
	header.Set("Authorization", "Bearer "+c.token)

	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = c.cfg.ConnectTimeout

	conn, resp, err := dialer.DialContext(ctx, c.cfg.WSUrl, header)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("websocket 连接失败 status=%d: %w", resp.StatusCode, err)
		}
		return fmt.Errorf("websocket 连接失败: %w", err)
	}

	defer conn.Close()

	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)

	go func() {
		errCh <- c.readLoop(ctx, conn)
	}()

	go func() {
		errCh <- c.heartbeatLoop(childCtx, conn)
	}()

	err = <-errCh

	cancel()

	_ = conn.Close()

	return err
}

func (c *Client) readLoop(ctx context.Context, conn *websocket.Conn) error {
	for {
		var env protocol.Envelope
		if err := conn.ReadJSON(&env); err != nil {
			return fmt.Errorf("websocket 消息读取失败: %w", err)
		}

		switch env.Type {
		case protocol.TypeCommand:
			var payload protocol.CmdPayload
			if err := json.Unmarshal(env.Payload, &payload); err != nil {
				_ = c.sendAck(conn, env.ID, protocol.Ack{Accepted: false, Reason: "命令参数解析失败"})
				continue
			}
			go c.handleCommand(ctx, conn, env.ID, payload)
		}
	}
}

func (c *Client) handleCommand(ctx context.Context, conn *websocket.Conn, id string, payload protocol.CmdPayload) {
	if err := c.sendAck(conn, id, protocol.Ack{Accepted: true}); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "命令确认发送失败 commandID=%s error=%v\n", id, err)
		_ = conn.Close()
	}

	var reportOnce sync.Once

	result := c.executor.Execute(ctx, payload, func(line string) {
		if err := c.sendLog(conn, id, line); err != nil {
			reportOnce.Do(func() {
				_, _ = fmt.Fprintf(os.Stderr, "执行日志发送失败 commandID=%s error=%v\n", id, err)
				_ = conn.Close()
			})
		}
	})

	if err := c.sendResult(conn, id, *result); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "执行结果发送失败 commandID=%s success=%t exitCode=%d error=%v\n", id, result.Success, result.ExitCode, err)
		_ = conn.Close()
	}
}

func (c *Client) heartbeatLoop(ctx context.Context, conn *websocket.Conn) error {
	ticker := time.NewTicker(c.cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := c.send(conn, "", protocol.TypePing, nil); err != nil {
				return fmt.Errorf("心跳发送失败: %w", err)
			}
		}
	}
}

func (c *Client) sendAck(conn *websocket.Conn, id string, a protocol.Ack) error {
	return c.send(conn, id, protocol.TypeAck, a)
}
func (c *Client) sendLog(conn *websocket.Conn, id string, line string) error {
	return c.send(conn, id, protocol.TypeLog, protocol.Log{Content: line})
}
func (c *Client) sendResult(conn *websocket.Conn, id string, r protocol.Result) error {
	return c.send(conn, id, protocol.TypeResult, r)
}

func (c *Client) send(conn *websocket.Conn, id string, msgType string, payload any) error {
	env := protocol.Envelope{
		ID:   id,
		Type: msgType,
	}

	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("消息序列化失败: %w", err)
		}
		env.Payload = data
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return fmt.Errorf("websocket 写超时设置失败: %w", err)
	}

	if err := conn.WriteJSON(env); err != nil {
		return fmt.Errorf("websocket 消息发送失败: %w", err)
	}

	return nil
}
