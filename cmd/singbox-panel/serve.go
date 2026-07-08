package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/tanselxy/singbox/internal/network"
	"github.com/tanselxy/singbox/internal/panel"
)

// runServe starts the resident web panel, bootstrapping its config on first run.
func runServe(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, freshPassword, err := panel.EnsureConfig()
	if err != nil {
		return err
	}

	if freshPassword != "" {
		printPanelInfo(ctx, cfg, freshPassword)
	} else {
		fmt.Printf("面板已启动，端口 %d，路径前缀 /%s\n", cfg.Port, cfg.PathPrefix)
	}

	srv, err := panel.New(cfg)
	if err != nil {
		return err
	}
	return srv.ListenAndServe(ctx)
}

// printPanelInfo shows the one-time access URL and initial password.
func printPanelInfo(ctx context.Context, cfg panel.Config, password string) {
	host := "<服务器IP>"
	a := network.Detect(ctx)
	if a.IPv4 != "" {
		host = a.IPv4
	} else if a.IPv6 != "" {
		host = "[" + a.IPv6 + "]"
	}
	fmt.Println("\n=============== 面板访问信息（仅显示一次）===============")
	fmt.Printf("地址:   https://%s:%d/%s/\n", host, cfg.Port, cfg.PathPrefix)
	fmt.Printf("密码:   %s\n", password)
	fmt.Println("提示: 面板使用自签证书，浏览器会提示不安全，属正常。")
	fmt.Println("=======================================================")
}
