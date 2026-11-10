package agentclient

import (
	"context"
	"errors"
	"fmt"
	"game-panel/internal/protocol"
	"os"
	"os/exec"
	"path"
	"strings"
)

// maxSlot 存档槽位上限，与 dst.sh 的 DST_SLOT_COUNT 一致
const maxSlot = 5

// writeConfig 写入平台生成的配置文件。先校验全部文件名再写入，避免写到一半才发现非法文件；
// 存档目录不存在时写入失败（需先初始化）
func (e *Executor) writeConfig(ctx context.Context, payload protocol.CmdPayload, onLog LogFunc) error {
	if payload.Slot < 1 || payload.Slot > maxSlot {
		return fmt.Errorf("存档槽位不合法 slot=%d", payload.Slot)
	}
	if len(payload.Files) == 0 {
		return errors.New("没有要写入的配置文件")
	}

	targets := make([]string, len(payload.Files))
	for i, f := range payload.Files {
		target, err := e.configPath(f.Name, payload.Slot)
		if err != nil {
			return err
		}
		targets[i] = target
	}

	for i, f := range payload.Files {
		if err := e.installFile(ctx, targets[i], f.Content, fileMode(f.Name)); err != nil {
			return fmt.Errorf("配置文件 %s 写入失败: %w", f.Name, err)
		}
		if onLog != nil {
			onLog("已写入配置文件 " + targets[i])
		}
	}
	return nil
}

// configPath 把配置文件名换算成固定路径，只接受协议中定义的文件名
func (e *Executor) configPath(name string, slot int) (string, error) {
	clusterDir := path.Join(e.cfg.ClusterRoot, fmt.Sprintf("Cluster_%d", slot))
	switch name {
	case protocol.FileClusterINI, protocol.FileClusterToken, protocol.FileMasterModOverrides, protocol.FileCavesModOverrides:
		return path.Join(clusterDir, name), nil
	case protocol.FileModsSetup:
		return path.Join(e.cfg.ModsDir, name), nil
	default:
		return "", fmt.Errorf("不支持写入配置文件 name=%s", name)
	}
}

// installFile 先写临时文件，再用 install 放到目标位置并交给游戏用户；
// Agent 不一定以 root 运行，游戏目录属于游戏用户，需要通过 sudo 写入
func (e *Executor) installFile(ctx context.Context, target string, content string, mode string) error {
	tmp, err := os.CreateTemp("", "dst-config-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	args := []string{"install", "-o", e.cfg.GameUser, "-g", e.cfg.GameUser, "-m", mode, tmp.Name(), target}
	if os.Geteuid() != 0 {
		args = append([]string{"sudo", "-n"}, args...)
	}
	output, err := exec.CommandContext(ctx, args[0], args[1:]...).CombinedOutput()
	if err != nil {
		if msg := strings.TrimSpace(string(output)); msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

// fileMode 科雷令牌只允许游戏用户读取
func fileMode(name string) string {
	if name == protocol.FileClusterToken {
		return "0600"
	}
	return "0644"
}
