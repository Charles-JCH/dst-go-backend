package agentinstall

import (
	"context"
	"errors"
	"fmt"
	"game-panel/internal/model/entity"
	"game-panel/internal/model/response"
	"game-panel/internal/platform/crypto"
	"game-panel/internal/platform/jwt"
	"game-panel/internal/platform/ssh"
	serverrepo "game-panel/internal/repository/server"
	taskrepo "game-panel/internal/repository/task"
	"game-panel/internal/validator"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"
)

// 远程服务器上的安装位置
const (
	remoteDir    = "/opt/dst-agent"
	remoteBinary = remoteDir + "/dst-agent"
	remoteScript = remoteDir + "/dst.sh"
	remoteConfig = remoteDir + "/config.yaml"
	remoteToken  = remoteDir + "/token"
	remoteUnit   = remoteDir + "/dst-agent.service"
	systemdUnit  = "/etc/systemd/system/dst-agent.service"
	serviceName  = "dst-agent"
	// dst.sh 以 charles 用户运行游戏，游戏文件都在它的主目录下
	gameDir = "/home/charles"

	maxConcurrent = 5                // 同时执行的安装任务数
	taskTimeout   = 10 * time.Minute // 单个安装任务的最长时间
	sshTimeout    = 15 * time.Second
)

// archNames uname -m 的输出对应的 Agent 程序后缀
var archNames = map[string]string{
	"x86_64":  "amd64",
	"aarch64": "arm64",
}

type Config struct {
	BinaryDir     string        // 存放编译好的 dst-agent-linux-amd64、dst-agent-linux-arm64
	ScriptPath    string        // dst.sh 路径，每次安装时读取，更新脚本无需重启平台
	WSURL         string        // Agent 连接平台的地址
	OnlineTimeout time.Duration // 启动后等待 Agent 上线的时间
}

// Service 通过 SSH 把 Agent 和 dst 脚本安装到用户服务器并启动
type Service interface {
	// Enqueue 异步执行安装任务，任务需已是 pending 状态
	Enqueue(taskID uint64)
	// Recover 平台启动时调用：中断的任务标记失败，待执行的任务重新排队
	Recover(ctx context.Context) error
}

// AgentConn 查询和断开 Agent 连接（agentmanager.Service 未导出接口）
type AgentConn interface {
	IsOnline(serverID uint64) bool
	Disconnect(serverID uint64)
}

// Notifier 向浏览器推送实时事件（realtime.Hub 未导出接口）
type Notifier interface {
	Publish(userID uint64, eventType string, data any)
}

// installError msg 是展示给用户的失败原因，err 是只记录到日志的内部错误
type installError struct {
	msg string
	err error
}

type service struct {
	cfg        Config
	serverRepo serverrepo.Repository
	taskRepo   taskrepo.Repository
	encryptor  crypto.Encryptor
	tokens     jwt.TokenManager
	dialer     ssh.Dialer
	agent      AgentConn
	notifier   Notifier
	sem        chan struct{}
}

func NewService(
	cfg Config,
	serverRepo serverrepo.Repository,
	taskRepo taskrepo.Repository,
	encryptor crypto.Encryptor,
	tokens jwt.TokenManager,
	dialer ssh.Dialer,
	agent AgentConn,
	notifier Notifier,
) Service {
	return &service{
		cfg:        cfg,
		serverRepo: serverRepo,
		taskRepo:   taskRepo,
		encryptor:  encryptor,
		tokens:     tokens,
		dialer:     dialer,
		agent:      agent,
		notifier:   notifier,
		sem:        make(chan struct{}, maxConcurrent),
	}
}

func (s *service) Enqueue(taskID uint64) {
	go func() {
		s.sem <- struct{}{}
		defer func() { <-s.sem }()
		s.run(taskID)
	}()
}

func (s *service) Recover(ctx context.Context) error {
	count, err := s.taskRepo.FailRunning(ctx, "平台重启，任务中断，请重试")
	if err != nil {
		return fmt.Errorf("中断任务清理失败: %w", err)
	}
	if count > 0 {
		slog.WarnContext(ctx, "中断任务已标记失败", "count", count)
	}

	tasks, err := s.taskRepo.ListPending(ctx, entity.TaskTypeAgentInstall)
	if err != nil {
		return fmt.Errorf("待执行任务查询失败: %w", err)
	}
	for _, t := range tasks {
		s.Enqueue(t.ID)
	}
	return nil
}

func (e *installError) Error() string {
	if e.err == nil {
		return e.msg
	}
	return e.msg + ": " + e.err.Error()
}

// run 执行一个安装任务：pending → running → succeeded / failed
func (s *service) run(taskID uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), taskTimeout)
	defer cancel()

	task, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil {
		slog.ErrorContext(ctx, "任务查询失败", "taskID", taskID, "error", err)
		return
	}
	if task == nil {
		return
	}
	if err := s.taskRepo.Start(ctx, taskID); err != nil {
		// 状态冲突说明任务已被处理过（重复排队），直接跳过
		if !errors.Is(err, taskrepo.ErrStatusConflict) {
			slog.ErrorContext(ctx, "任务状态更新失败", "taskID", taskID, "error", err)
		}
		return
	}
	s.publishTask(ctx, taskID)

	status, message := entity.TaskStatusSucceeded, ""
	if err := s.safeInstall(ctx, task); err != nil {
		status, message = entity.TaskStatusFailed, "Agent 安装失败"
		var ie *installError
		if errors.As(err, &ie) {
			message = ie.msg
		}
		slog.ErrorContext(ctx, "Agent 安装失败", "taskID", taskID, "serverID", task.ResourceID, "error", err)
		s.log(ctx, task, "安装失败："+message)
	} else {
		slog.InfoContext(ctx, "Agent 安装完成", "taskID", taskID, "serverID", task.ResourceID)
		s.log(ctx, task, "Agent 已连接平台，安装完成")
	}

	// 任务可能因超时结束，收尾用新的 ctx，保证状态能写回
	finishCtx, finishCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer finishCancel()
	if err := s.taskRepo.Finish(finishCtx, taskID, status, message); err != nil && !errors.Is(err, taskrepo.ErrStatusConflict) {
		slog.ErrorContext(finishCtx, "任务状态更新失败", "taskID", taskID, "error", err)
	}
	s.publishTask(finishCtx, taskID)
}

func (s *service) publishTask(ctx context.Context, taskID uint64) {
	t, err := s.taskRepo.FindByID(ctx, taskID)
	if err != nil || t == nil {
		slog.WarnContext(ctx, "任务查询失败，跳过推送", "taskID", taskID, "error", err)
		return
	}
	s.notifier.Publish(t.UserID, response.EventTask, response.ToTaskResp(t))
}

// safeInstall 执行安装，panic 转成错误，避免后台协程崩溃导致平台退出
func (s *service) safeInstall(ctx context.Context, task *entity.Task) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("安装过程异常: %v\n%s", r, debug.Stack())
		}
	}()
	return s.install(ctx, task)
}

func (s *service) install(ctx context.Context, task *entity.Task) error {
	server, err := s.serverRepo.FindByID(ctx, task.ResourceID)
	if err != nil {
		return fail("服务器查询失败", err)
	}
	if server == nil {
		return fail("服务器已删除", nil)
	}
	// 用户名会写进 systemd 配置，再校验一次
	if !validator.IsLinuxUsername(server.Username) {
		return fail("SSH 用户名格式不正确", nil)
	}
	password, err := s.encryptor.Decrypt(server.Password)
	if err != nil {
		return fail("SSH 密码解密失败", err)
	}
	script, err := os.ReadFile(s.cfg.ScriptPath)
	if err != nil {
		return fail("dst 脚本读取失败", err)
	}

	// 1. 连接：首次连接记录主机公钥，之后必须一致
	s.stage(ctx, task, entity.TaskStageConnecting, fmt.Sprintf("正在连接 %s:%d", server.IP, server.Port))
	client, err := s.dialer.Dial(ctx, ssh.Config{
		Host:     server.IP,
		Port:     server.Port,
		User:     server.Username,
		Password: password,
		HostKey:  server.SSHHostKey,
		Timeout:  sshTimeout,
	})
	if errors.Is(err, ssh.ErrHostKeyMismatch) {
		return fail("服务器主机指纹与首次连接时不一致，已停止连接；如服务器重装过系统，请在编辑中重新填写 SSH 密码后重试", err)
	}
	if err != nil {
		return fail("SSH 连接失败，请检查 IP、端口、用户名和密码", err)
	}
	defer client.Close()

	if server.SSHHostKey == "" {
		if err := s.serverRepo.SetSSHHostKey(ctx, server.ID, client.HostKey()); err != nil {
			return fail("主机指纹保存失败", err)
		}
		s.log(ctx, task, "首次连接，已记录主机指纹 "+client.Fingerprint())
	}

	// root 直接执行，其他用户通过免密 sudo 执行
	sudo := func(args ...string) []string {
		if server.Username == "root" {
			return args
		}
		return append([]string{"sudo", "-n"}, args...)
	}
	if server.Username != "root" {
		if out, err := client.Run(ctx, "sudo", "-n", "true"); err != nil {
			return fail("该用户没有免密 sudo 权限，请使用 root 或已配置 sudo 免密的用户", commandError(err, out))
		}
	}

	out, err := client.Run(ctx, "uname", "-m")
	if err != nil {
		return fail("系统架构检测失败", commandError(err, out))
	}
	arch, ok := archNames[strings.TrimSpace(out)]
	if !ok {
		return fail("不支持的系统架构 "+strings.TrimSpace(out), nil)
	}
	binary, err := os.ReadFile(filepath.Join(s.cfg.BinaryDir, "dst-agent-linux-"+arch))
	if err != nil {
		return fail("平台缺少该架构的 Agent 程序", err)
	}

	// 每次安装都换新令牌，旧 Agent 的令牌随之失效
	version, err := s.serverRepo.IncrAgentTokenVersion(ctx, server.ID)
	if err != nil {
		return fail("Agent 令牌生成失败", err)
	}
	token, err := s.tokens.GenerateAgentToken(server.ID, version)
	if err != nil {
		return fail("Agent 令牌生成失败", err)
	}
	s.agent.Disconnect(server.ID)

	// 2. 上传：先停掉旧服务，避免覆盖正在运行的程序
	s.stage(ctx, task, entity.TaskStageUploading, "正在上传 Agent 和 dst 脚本")
	_, _ = client.Run(ctx, sudo("systemctl", "stop", serviceName)...) // 首次安装时服务不存在，忽略错误
	for _, args := range [][]string{
		sudo("mkdir", "-p", remoteDir),
		sudo("chown", server.Username+":", remoteDir),
	} {
		if out, err := client.Run(ctx, args...); err != nil {
			return fail("安装目录创建失败", commandError(err, out))
		}
	}
	files := []struct {
		path    string
		content []byte
		mode    uint32
	}{
		{remoteBinary, binary, 0o755},
		{remoteScript, script, 0o755},
		{remoteConfig, s.agentConfig(), 0o600},
		{remoteToken, []byte(token), 0o600},
		{remoteUnit, unitFile(server.Username), 0o644},
	}
	for _, f := range files {
		if err := client.Upload(ctx, f.path, f.content, f.mode); err != nil {
			return fail("文件上传失败", err)
		}
	}

	// 3. 注册 systemd 服务并启动
	s.stage(ctx, task, entity.TaskStageInstalling, "正在注册并启动 Agent 服务")
	for _, args := range [][]string{
		sudo("install", "-m", "0644", remoteUnit, systemdUnit),
		sudo("systemctl", "daemon-reload"),
		sudo("systemctl", "enable", serviceName),
		sudo("systemctl", "restart", serviceName),
	} {
		if out, err := client.Run(ctx, args...); err != nil {
			return fail("Agent 服务启动失败", commandError(err, out))
		}
	}

	// 4. 等待 Agent 连上平台
	s.stage(ctx, task, entity.TaskStageWaitingOnline, "等待 Agent 连接平台")
	if !s.waitOnline(ctx, server.ID) {
		return fail("Agent 未在规定时间内连接平台，请检查服务器能否访问平台地址", nil)
	}
	return nil
}

func fail(msg string, err error) error {
	return &installError{msg: msg, err: err}
}

func (s *service) stage(ctx context.Context, task *entity.Task, stage string, message string) {
	if err := s.taskRepo.UpdateStage(ctx, task.ID, stage); err != nil {
		slog.WarnContext(ctx, "任务步骤更新失败", "taskID", task.ID, "stage", stage, "error", err)
	}
	s.log(ctx, task, message)
	s.publishTask(ctx, task.ID)
}

// log 保存一行任务日志并推送给浏览器；日志失败不影响安装
func (s *service) log(ctx context.Context, task *entity.Task, content string) {
	l := &entity.TaskLog{TaskID: task.ID, Content: content}
	if err := s.taskRepo.CreateLog(ctx, l); err != nil {
		slog.WarnContext(ctx, "任务日志保存失败", "taskID", task.ID, "error", err)
		return
	}
	s.notifier.Publish(task.UserID, response.EventTaskLog, response.ToTaskLogResp(l))
}

// commandError 把远程命令的输出附在错误里，方便排查
func commandError(err error, output string) error {
	output = strings.TrimSpace(output)
	if output == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, output)
}

// agentConfig 生成 Agent 的配置文件
func (s *service) agentConfig() []byte {
	return []byte(fmt.Sprintf(`server:
  ws_url: %q
  heartbeat_interval: 30s
  connect_timeout: 10s
  reconnect_interval: 5s

executor:
  script_path: %q
  exec_timeout: 2h

identity:
  path: %q

service:
  name: %q
  unit_path: %q
  install_dir: %q

sysinfo:
  disk_path: %q
`, s.cfg.WSURL, remoteScript, remoteToken, serviceName, systemdUnit, remoteDir, gameDir))
}

// unitFile 生成 systemd 服务配置；username 已校验为合法的 Linux 用户名
func unitFile(username string) []byte {
	return []byte(fmt.Sprintf(`[Unit]
Description=DST Agent
After=network-online.target
Wants=network-online.target

[Service]
User=%s
ExecStart=%s -c %s
Restart=always
RestartSec=5
# 游戏进程由 dst.sh 通过 screen 启动，重启 Agent 时不能连带结束
KillMode=process

[Install]
WantedBy=multi-user.target
`, username, remoteBinary, remoteConfig))
}

func (s *service) waitOnline(ctx context.Context, serverID uint64) bool {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.OnlineTimeout)
	defer cancel()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if s.agent.IsOnline(serverID) {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
}
