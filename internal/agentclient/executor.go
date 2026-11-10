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

// Acquire 占用执行器，已有命令在执行时返回 false；脚本一次只跑一条命令
func (e *Executor) Acquire() bool {
	return e.mu.TryLock()
}

// Release 释放执行器，与 Acquire 成对调用
func (e *Executor) Release() {
	e.mu.Unlock()
}

// Execute 执行一条命令，调用方需先 Acquire
func (e *Executor) Execute(ctx context.Context, payload protocol.CmdPayload, onLog LogFunc) *protocol.Result {
	failed := func(msg string) *protocol.Result {
		return &protocol.Result{Action: payload.Action, Slot: payload.Slot, ExitCode: -1, Error: msg}
	}

	ctx, cancel := context.WithTimeout(ctx, e.cfg.ExecTimeout)
	defer cancel()

	if payload.Action == protocol.ActionConfig {
		if err := e.writeConfig(ctx, payload, onLog); err != nil {
			return failed(err.Error())
		}
		return &protocol.Result{Action: payload.Action, Slot: payload.Slot, IsSuccess: true}
	}

	args, err := e.buildArgs(payload)
	if err != nil {
		return failed(err.Error())
	}

	// stdout 和 stderr 合并到同一个管道，日志行的先后顺序与脚本输出一致
	pr, pw := io.Pipe()
	cmd := exec.CommandContext(ctx, e.cfg.ScriptPath, args...)
	cmd.Stdout, cmd.Stderr = pw, pw
	// 脚本版本由平台统一推送，关闭脚本自更新；科雷令牌走环境变量，不出现在进程参数里
	cmd.Env = append(os.Environ(), "DST_NO_AUTO_UPDATE=1")
	if payload.Action == protocol.ActionInit {
		cmd.Env = append(cmd.Env, "DST_TOKEN="+payload.ClusterToken)
	}
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
		return []string{payload.Action, strconv.Itoa(payload.Slot)}, nil
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
