package agentclient

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const uninstallStepTimeout = 30 * time.Second

// Uninstall 停用 systemd 服务并删除安装文件，最后停止服务让 Agent 自己退出。
// 已启动的游戏进程不受影响（服务配置了 KillMode=process）。
func Uninstall(ctx context.Context, cfg ServiceConfig) {
	if err := checkServiceConfig(cfg); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "agent 卸载失败: %v\n", err)
		return
	}

	steps := [][]string{
		{"systemctl", "disable", cfg.Name},
		{"rm", "-f", cfg.UnitPath},
		{"rm", "-rf", cfg.InstallDir},
		{"systemctl", "daemon-reload"},
		// --no-block：只提交停止请求，命令返回后 systemd 再结束本进程
		{"systemctl", "stop", "--no-block", cfg.Name},
	}
	for _, args := range steps {
		if err := runPrivileged(ctx, args...); err != nil {
			// 继续执行后面的步骤，尽量清理干净
			_, _ = fmt.Fprintf(os.Stderr, "agent 卸载步骤失败 command=%v error=%v\n", args, err)
		}
	}
}

// checkServiceConfig 防止配置缺失时删除错误的路径
func checkServiceConfig(cfg ServiceConfig) error {
	if cfg.Name == "" || cfg.UnitPath == "" || cfg.InstallDir == "" {
		return fmt.Errorf("未配置安装位置")
	}
	for _, p := range []string{cfg.UnitPath, cfg.InstallDir} {
		clean := filepath.Clean(p)
		if !filepath.IsAbs(clean) || clean == "/" || filepath.Dir(clean) == "/" {
			return fmt.Errorf("安装路径不安全 path=%s", p)
		}
	}
	return nil
}

// runPrivileged root 直接执行，其他用户通过免密 sudo 执行
func runPrivileged(ctx context.Context, args ...string) error {
	if os.Geteuid() != 0 {
		args = append([]string{"sudo", "-n"}, args...)
	}
	ctx, cancel := context.WithTimeout(ctx, uninstallStepTimeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}
