package agent_client

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"game-panel/internal/protocol"
	"io"
	"os/exec"
	"strconv"
	"sync"
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
	if !e.mu.TryLock() {
		return &protocol.Result{
			Action:   payload.Action,
			Slot:     payload.Slot,
			Success:  false,
			ExitCode: -1,
			Error:    "已有任务执行中",
		}
	}
	defer e.mu.Unlock()

	args, err := e.buildArgs(payload)
	if err != nil {
		return &protocol.Result{
			Action:   payload.Action,
			Slot:     payload.Slot,
			Success:  false,
			ExitCode: -1,
			Error:    err.Error(),
		}
	}

	ctx, cancel := context.WithTimeout(ctx, e.cfg.ExecTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, e.cfg.ScriptPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return &protocol.Result{
			Action:   payload.Action,
			Slot:     payload.Slot,
			Success:  false,
			ExitCode: -1,
			Error:    fmt.Sprintf("stdout 管道创建失败: %v", err),
		}
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return &protocol.Result{
			Action:   payload.Action,
			Slot:     payload.Slot,
			Success:  false,
			ExitCode: -1,
			Error:    fmt.Sprintf("stderr 管道创建失败: %v", err),
		}
	}

	if err := cmd.Start(); err != nil {
		return &protocol.Result{
			Action:   payload.Action,
			Slot:     payload.Slot,
			Success:  false,
			ExitCode: -1,
			Error:    fmt.Sprintf("dst 脚本启动失败: %v", err),
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		e.readOutput(stdout, onLog)
	}()

	go func() {
		defer wg.Done()
		e.readOutput(stderr, onLog)
	}()

	stopClosing := context.AfterFunc(ctx, func() {
		_ = stdout.Close()
		_ = stderr.Close()
	})
	defer stopClosing()

	wg.Wait()
	err = cmd.Wait()

	result := &protocol.Result{
		Action:   payload.Action,
		Slot:     payload.Slot,
		Success:  true,
		ExitCode: 0,
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else {
			return &protocol.Result{
				Action:   payload.Action,
				Slot:     payload.Slot,
				Success:  false,
				ExitCode: -1,
				Error:    fmt.Sprintf("脚本执行失败: %v", err),
			}
		}
	}

	if payload.Action == protocol.ActionStatus {
		result.Success = result.ExitCode == 0 || result.ExitCode == 2
	} else {
		result.Success = result.ExitCode == 0
	}

	if !result.Success {
		if ctx.Err() != nil {
			result.Error = fmt.Sprintf("脚本执行失败: %v", ctx.Err())
		} else {
			result.Error = fmt.Sprintf("脚本异常退出 exitCode=%d", result.ExitCode)
		}
	}

	return result
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

func (e *Executor) buildArgs(payload protocol.CmdPayload) ([]string, error) {
	switch payload.Action {
	case protocol.ActionDeploy, protocol.ActionUpdate:
		return []string{payload.Action}, nil

	case protocol.ActionInit,
		protocol.ActionStart,
		protocol.ActionStop,
		protocol.ActionDelete,
		protocol.ActionStatus:

		args := []string{payload.Action, strconv.Itoa(payload.Slot)}
		if payload.Action == protocol.ActionInit {
			args = append(args, payload.ClusterToken)
		}
		return args, nil

	default:
		return nil, fmt.Errorf("不支持操作 action=%s", payload.Action)
	}
}
