//go:build !linux

package agentclient

// diskUsage 非 Linux 系统不统计磁盘容量（Agent 实际运行在 Linux 上）
func diskUsage(_ string) (total int64, used int64) { return 0, 0 }
