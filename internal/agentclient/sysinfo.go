package agentclient

import (
	"bufio"
	"game-panel/internal/protocol"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// CollectSysInfo 采集系统配置；某一项读取失败时留空，不影响连接平台
func CollectSysInfo(diskPath string) protocol.SysInfo {
	return protocol.SysInfo{
		CPU:    runtime.NumCPU(),
		RAM:    memTotal(),
		Disk:   diskSize(diskPath),
		OSName: osName(),
		// 安装时按 uname -m 选择对应架构的程序，程序架构即系统架构
		Arch: runtime.GOARCH,
	}
}

// memTotal 读取 /proc/meminfo 的 MemTotal，单位转换为字节
func memTotal() int64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// 格式：MemTotal:       16314880 kB
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "MemTotal:" {
			kb, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return 0
			}
			return kb * 1024
		}
	}
	return 0
}

// osName 读取 /etc/os-release 的 PRETTY_NAME，如 Ubuntu 22.04.4 LTS
func osName() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		value, ok := strings.CutPrefix(strings.TrimSpace(line), "PRETTY_NAME=")
		if !ok {
			continue
		}
		if unquoted, err := strconv.Unquote(value); err == nil {
			return unquoted
		}
		return strings.Trim(value, `"'`)
	}
	return ""
}
