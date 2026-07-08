package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mdp/qrterminal/v3"
	"github.com/tanselxy/singbox/internal/installer"
	"github.com/tanselxy/singbox/internal/model"
)

// runInstall performs a fresh sing-box deployment.
func runInstall(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	nat := fs.Bool("nat", false, "NAT 机器模式：在端口范围内随机分配端口")
	portRange := fs.String("port-range", "", "NAT 模式端口范围，格式 start-end，例如 20000-30000")
	domain := fs.String("domain", "", "CDN/IPv6 场景使用的真实域名（VLESS-CDN 入站）")
	dryRun := fs.Bool("dry-run", false, "仅生成配置到 --out 目录，不改动系统")
	out := fs.String("out", ".", "--dry-run 时配置输出目录")
	qr := fs.Bool("qr", true, "在终端打印节点二维码")
	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := installer.Options{
		NAT:       *nat,
		CDNDomain: *domain,
		DryRun:    *dryRun,
		OutputDir: *out,
	}
	if *nat {
		start, end, err := parsePortRange(*portRange)
		if err != nil {
			return err
		}
		opts.PortStart, opts.PortEnd = start, end
	}

	res, err := installer.Run(ctx, opts)
	if err != nil {
		return err
	}

	printResult(res, *qr)
	printPanel(res)
	return nil
}

func printPanel(res *installer.Result) {
	if res.PanelURL == "" {
		return
	}
	fmt.Println("\n=============== Web 面板 ===============")
	fmt.Printf("地址: %s\n", res.PanelURL)
	if res.PanelPassword != "" {
		fmt.Printf("密码: %s  （仅显示一次，请妥善保存）\n", res.PanelPassword)
	} else {
		fmt.Println("密码: 使用已有面板密码（未变更）")
	}
	fmt.Println("提示: 面板为自签证书，浏览器会提示不安全，属正常。")
	fmt.Println("=======================================")
}

func parsePortRange(s string) (int, int, error) {
	lo, hi, ok := strings.Cut(s, "-")
	if !ok {
		return 0, 0, fmt.Errorf("端口范围格式应为 start-end，例如 20000-30000")
	}
	start, err := strconv.Atoi(strings.TrimSpace(lo))
	if err != nil {
		return 0, 0, fmt.Errorf("端口范围起始无效: %w", err)
	}
	end, err := strconv.Atoi(strings.TrimSpace(hi))
	if err != nil {
		return 0, 0, fmt.Errorf("端口范围结束无效: %w", err)
	}
	return start, end, nil
}

func printResult(res *installer.Result, qr bool) {
	fmt.Printf("\n配置已写入: %s\n", res.ConfigPath)
	fmt.Printf("服务器地址: %s\n\n", res.Deployment.ServerIP)
	fmt.Println("=============== 节点链接 ===============")
	for _, l := range res.Links {
		fmt.Printf("\n【%s】\n%s\n", l.Name, l.URL)
		if qr {
			printQR(l)
		}
	}
	fmt.Println("\n=======================================")
}

func printQR(l model.Link) {
	qrterminal.GenerateWithConfig(l.URL, qrterminal.Config{
		Level:      qrterminal.L,
		Writer:     os.Stdout,
		HalfBlocks: true,
		BlackChar:  qrterminal.BLACK_BLACK,
		WhiteChar:  qrterminal.WHITE_WHITE,
		QuietZone:  1,
	})
}
