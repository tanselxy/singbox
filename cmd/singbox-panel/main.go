// Command singbox-panel installs, serves and manages a sing-box deployment
// together with a resident web control panel.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "install", "deploy":
		err = runInstall(ctx, args)
	case "serve":
		err = runServe(ctx, args)
	case "uninstall":
		err = runUninstall(ctx, args)
	case "version", "-v", "--version":
		fmt.Printf("singbox-panel %s\n", version)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `singbox-panel — sing-box 部署与管理面板

用法:
  singbox-panel <command> [flags]

命令:
  install     全新安装部署 sing-box 并生成配置
  serve       启动常驻 Web 管理面板
  uninstall   卸载 sing-box 与面板并清理所有产物
  version     显示版本
  help        显示帮助

示例:
  singbox-panel install
  singbox-panel serve
`)
}
