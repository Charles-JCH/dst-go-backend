package main

import (
	"flag"
	"fmt"
	"game-panel/internal/bootstrap"
	"os"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "c", "config/config.yaml", "配置文件路径 (例: -c config/config.yaml)")
	flag.Parse()

	if err := bootstrap.Run(configPath); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "服务异常退出: %v\n", err)
		os.Exit(1)
	}
}
