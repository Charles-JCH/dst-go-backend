package agent_client

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func LoadToken(path string) (string, error) {
	// 读取文件
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("agent 身份文件读取失败: %w", err)
	}

	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", errors.New("agent token 为空")
	}

	return token, nil
}

func SaveToken(path string, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("agent token 为空")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("agent 身份目录创建失败: %w", err)
	}
	if err := os.WriteFile(path, []byte(token), 0600); err != nil {
		return fmt.Errorf("agent 身份文件保存失败: %w", err)
	}

	return nil
}
