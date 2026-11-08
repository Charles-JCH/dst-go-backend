package agentclient

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"game-panel/internal/protocol"
	"io"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

type LogFunc func(line string)

type Executor struct {
	cfg ExecutorConfig
	mu  sync.Mutex
}

func NewExecutor(cfg ExecutorConfig) *Executor {
	return &Executor{cfg: cfg}
}

func (e *Executor) Execute(ctx context.Context, payload protocol.CmdPayload, onLog LogFunc) *protocol.Result {
	failed := func(msg string) *protocol.Result {
		return &protocol.Result{Action: payload.Action, Slot: payload.Slot, ExitCode: -1, Error: msg}
	}

	if !e.mu.TryLock() {
		return failed("已有任务执行中")
	}
	defer e.mu.Unlock()

	args, err := e.buildArgs(payload)
	if err != nil {
		return failed(err.Error())
	}

	ctx, cancel := context.WithTimeout(ctx, e.cfg.ExecTimeout)
	defer cancel()

	// stdout 和 stderr 合并到同一个管道，日志行的先后顺序与脚本输出一致
	pr, pw := io.Pipe()
	cmd := exec.CommandContext(ctx, e.cfg.ScriptPath, args...)
	cmd.Stdout, cmd.Stderr = pw, pw
	// 脚本版本由平台统一推送，关闭脚本自更新
	cmd.Env = append(os.Environ(), "DST_NO_AUTO_UPDATE=1")
	killWithChildren(cmd) // 超时时连同子进程一起结束
	cmd.WaitDelay = 5 * time.Second

	logsDone := make(chan struct{})
	go func() {
		defer close(logsDone)
		e.readOutput(pr, onLog)
	}()

	runErr := cmd.Run()
	_ = pw.Close()
	<-logsDone

	result := &protocol.Result{Action: payload.Action, Slot: payload.Slot}
	if runErr != nil {
		var exitErr *exec.ExitError
		if !errors.As(runErr, &exitErr) {
			return failed(fmt.Sprintf("dst 脚本执行失败: %v", runErr))
		}
		result.ExitCode = exitErr.ExitCode()
	}

	if payload.Action == protocol.ActionStatus {
		result.IsSuccess = result.ExitCode == 0 || result.ExitCode == 2
	} else {
		result.IsSuccess = result.ExitCode == 0
	}

	if !result.IsSuccess {
		if ctx.Err() != nil {
			result.Error = fmt.Sprintf("脚本执行失败: %v", ctx.Err())
		} else {
			result.Error = fmt.Sprintf("脚本异常退出 exitCode=%d", result.ExitCode)
		}
	}

	return result
}

func (e *Executor) buildArgs(payload protocol.CmdPayload) ([]string, error) {
	switch payload.Action {
	case protocol.ActionDeploy, protocol.ActionUpdate:
		return []string{payload.Action}, nil
	case protocol.ActionInit, protocol.ActionStart, protocol.ActionStop, protocol.ActionDelete, protocol.ActionStatus:
		args := []string{payload.Action, strconv.Itoa(payload.Slot)}
		if payload.Action == protocol.ActionInit {
			args = append(args, payload.ClusterToken)
		}
		return args, nil
	default:
		return nil, fmt.Errorf("不支持操作 action=%s", payload.Action)
	}
}

func (e *Executor) readOutput(r io.Reader, onLog LogFunc) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		if onLog != nil {
			onLog(scanner.Text())
		}
	}

	if err := scanner.Err(); err != nil && onLog != nil {
		onLog(fmt.Sprintf("日志读取失败: %v", err))
	}
}
