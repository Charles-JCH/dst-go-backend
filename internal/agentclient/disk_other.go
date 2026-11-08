//go:build !linux

package agentclient

// diskSize 非 Linux 系统不统计磁盘容量（Agent 实际运行在 Linux 上）
func diskSize(_ string) int64 { return 0 }
