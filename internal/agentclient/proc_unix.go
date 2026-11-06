//go:build unix

package agentclient

import (
	"os/exec"
	"syscall"
)

// killWithChildren 让脚本及其启动的所有子进程属于同一个进程组，取消（超时）时整组结束
func killWithChildren(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
