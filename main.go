package main

import (
	"fmt"
	"os"
)

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[启动] 游戏目录: %d 个, MOD目录: %d 个, 服务器: %s\n",
		len(cfg.GamePath), len(cfg.ModPath), cfg.ServerURL)

	if err := StartServer(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "启动服务器失败: %v\n", err)
		os.Exit(1)
	}
}
