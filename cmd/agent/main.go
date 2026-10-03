package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"game-panel/internal/agent_client"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var configPath, token string
	flag.StringVar(&configPath, "c", "config/agent-config.yaml", "配置文件路径")
	flag.StringVar(&token, "token", "", "Agent Token")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := agent_client.Load(configPath)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "agent 配置加载失败: %v\n", err)
		os.Exit(1)
	}

	exec := agent_client.NewExecutor(cfg.Executor)

	agentClient := agent_client.NewClient(cfg.Server, cfg.Identity, exec)

	if err := agentClient.Start(ctx, token); err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		_, _ = fmt.Fprintf(os.Stderr, "agent 运行失败: %v\n", err)
		os.Exit(1)
	}
}
