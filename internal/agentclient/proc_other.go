//go:build !unix

package agentclient

import "os/exec"

// killWithChildren 非 unix 系统只结束脚本进程本身（Agent 实际运行在 Linux 上）
func killWithChildren(_ *exec.Cmd) {}
