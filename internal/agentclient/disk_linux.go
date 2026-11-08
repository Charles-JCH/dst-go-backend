//go:build linux

package agentclient

import (
	"path/filepath"
	"syscall"
)

// diskSize 返回 path 所在分区的总容量（字节）；path 还不存在时（如游戏尚未部署）逐级取上级目录
func diskSize(path string) int64 {
	for p := filepath.Clean(path); ; p = filepath.Dir(p) {
		var st syscall.Statfs_t
		if err := syscall.Statfs(p, &st); err == nil {
			return int64(st.Blocks) * int64(st.Bsize)
		}
		if p == filepath.Dir(p) {
			return 0
		}
	}
}
