package main

import (
	"flag"
	"fmt"
	"game-panel/internal/bootstrap"
	"os"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "c", "config/config.yaml", "配置文件路径")
	flag.Parse()

	if err := bootstrap.Run(configPath); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "server 运行失败: %v\n", err)
		os.Exit(1)
	}
}
