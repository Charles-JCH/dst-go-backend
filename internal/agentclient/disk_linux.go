//go:build linux

package agentclient

import (
	"path/filepath"
	"syscall"
)

// diskUsage 返回 path 所在分区的总容量和已用容量（字节）；path 还不存在时（如游戏尚未部署）逐级取上级目录
func diskUsage(path string) (total int64, used int64) {
	for p := filepath.Clean(path); ; p = filepath.Dir(p) {
		var st syscall.Statfs_t
		if err := syscall.Statfs(p, &st); err == nil {
			bsize := int64(st.Bsize)
			return int64(st.Blocks) * bsize, int64(st.Blocks-st.Bfree) * bsize
		}
		if p == filepath.Dir(p) {
			return 0, 0
		}
	}
}
