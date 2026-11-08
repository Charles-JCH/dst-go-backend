package ssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	gossh "golang.org/x/crypto/ssh"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrHostKeyMismatch 服务器主机公钥与首次记录的不一致，可能是中间人攻击或服务器已重装
var ErrHostKeyMismatch = errors.New("ssh 主机公钥不匹配")

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	// HostKey 首次连接时为空，表示信任并记录本次的主机公钥；之后必须一致
	HostKey string
	Timeout time.Duration
}

type Dialer interface {
	Dial(ctx context.Context, cfg Config) (Client, error)
}

type Client interface {
	// HostKey 本次连接的主机公钥（authorized_keys 格式），首次连接时由调用方保存
	HostKey() string
	// Fingerprint 本次连接的主机公钥指纹（SHA256），用于展示
	Fingerprint() string
	// Run 执行命令，参数逐个转义后拼成命令行，不经过调用方拼接；返回合并后的输出
	Run(ctx context.Context, args ...string) (string, error)
	// Upload 把内容写入远程文件并设置权限，目录需已存在且有写权限
	Upload(ctx context.Context, path string, content []byte, mode uint32) error
	Close() error
}

type dialer struct{}

func NewDialer() Dialer {
	return &dialer{}
}

type client struct {
	client  *gossh.Client
	hostKey gossh.PublicKey
}

// lockedBuffer 并发安全的输出缓冲区
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (d *dialer) Dial(ctx context.Context, cfg Config) (Client, error) {
	var hostKey gossh.PublicKey
	sshCfg := &gossh.ClientConfig{
		User:    cfg.User,
		Auth:    []gossh.AuthMethod{gossh.Password(cfg.Password)},
		Timeout: cfg.Timeout,
		HostKeyCallback: func(_ string, _ net.Addr, key gossh.PublicKey) error {
			if cfg.HostKey != "" {
				expected, _, _, _, err := gossh.ParseAuthorizedKey([]byte(cfg.HostKey))
				if err != nil {
					return fmt.Errorf("已记录的主机公钥解析失败: %w", err)
				}
				if !bytes.Equal(expected.Marshal(), key.Marshal()) {
					return ErrHostKeyMismatch
				}
			}
			hostKey = key
			return nil
		},
	}

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	netDialer := net.Dialer{Timeout: cfg.Timeout}
	conn, err := netDialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("ssh 连接失败 addr=%s: %w", addr, err)
	}

	// 握手阶段也受 ctx 控制
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	sshConn, chans, reqs, err := gossh.NewClientConn(conn, addr, sshCfg)
	stop()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ssh 握手失败 addr=%s: %w", addr, err)
	}

	return &client{
		client:  gossh.NewClient(sshConn, chans, reqs),
		hostKey: hostKey,
	}, nil
}

func (c *client) HostKey() string {
	return strings.TrimSpace(string(gossh.MarshalAuthorizedKey(c.hostKey)))
}

func (c *client) Fingerprint() string {
	return gossh.FingerprintSHA256(c.hostKey)
}

func (c *client) Run(ctx context.Context, args ...string) (string, error) {
	if len(args) == 0 {
		return "", errors.New("ssh 命令为空")
	}
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = quote(arg)
	}
	return c.exec(ctx, strings.Join(quoted, " "), nil)
}

func (c *client) Upload(ctx context.Context, path string, content []byte, mode uint32) error {
	// 内容通过 stdin 传输，不出现在命令行里
	cmd := fmt.Sprintf("cat > %s && chmod %o %s", quote(path), mode, quote(path))
	if _, err := c.exec(ctx, cmd, content); err != nil {
		return fmt.Errorf("文件上传失败 path=%s: %w", path, err)
	}
	return nil
}

func (c *client) Close() error {
	return c.client.Close()
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// quote 用单引号包裹参数，参数中的单引号转义为 '\”，保证参数原样传给远程命令
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (c *client) exec(ctx context.Context, cmd string, stdin []byte) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh 会话创建失败: %w", err)
	}
	defer session.Close()

	// stdout 和 stderr 由两个协程分别写入，共用缓冲区需要加锁
	var output lockedBuffer
	session.Stdout = &output
	session.Stderr = &output
	if stdin != nil {
		session.Stdin = bytes.NewReader(stdin)
	}

	// ctx 取消时关闭会话，避免命令卡住
	stop := context.AfterFunc(ctx, func() { _ = session.Close() })
	defer stop()

	if err := session.Run(cmd); err != nil {
		if ctx.Err() != nil {
			return output.String(), ctx.Err()
		}
		return output.String(), err
	}
	return output.String(), nil
}
