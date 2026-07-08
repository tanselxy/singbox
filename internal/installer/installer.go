// Package installer orchestrates a full deployment: OS detection, sing-box
// install, credential and certificate generation, port allocation, config
// rendering, service start and link output.
package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tanselxy/singbox/internal/cert"
	"github.com/tanselxy/singbox/internal/config"
	"github.com/tanselxy/singbox/internal/model"
	"github.com/tanselxy/singbox/internal/network"
	"github.com/tanselxy/singbox/internal/protocol"
	"github.com/tanselxy/singbox/internal/secret"
	"github.com/tanselxy/singbox/internal/singbox"
	"github.com/tanselxy/singbox/internal/system"
)

const (
	certDir  = singbox.ConfigDir + "/cert"
	certFile = certDir + "/cert.pem"
	keyFile  = certDir + "/private.key"

	certValidity = 100 * 365 * 24 * time.Hour
)

// Options controls an install run.
type Options struct {
	NAT       bool   // NAT mode: allocate random ports in [PortStart,PortEnd]
	PortStart int    // NAT port range lower bound
	PortEnd   int    // NAT port range upper bound
	CDNDomain string // optional real domain for the VLESS-CDN inbound

	// DryRun renders the config into OutputDir without touching the system
	// (no install, no systemd). Used for local verification.
	DryRun    bool
	OutputDir string
}

// Result summarises a completed deployment for the caller to display.
type Result struct {
	Deployment model.Deployment
	Links      []model.Link
	ConfigPath string
}

// Run performs the deployment described by opts.
func Run(ctx context.Context, opts Options) (*Result, error) {
	if !opts.DryRun && !system.IsRoot() {
		return nil, fmt.Errorf("install 需要 root 权限")
	}

	dep, err := buildDeployment(ctx, opts)
	if err != nil {
		return nil, err
	}

	cfg, err := config.Marshal(dep)
	if err != nil {
		return nil, err
	}

	res := &Result{Deployment: dep, Links: protocol.Links(dep)}

	if opts.DryRun {
		res.ConfigPath, err = writeDryRun(opts.OutputDir, dep, cfg)
		return res, err
	}

	if err := deploy(ctx, opts, dep, cfg); err != nil {
		return nil, err
	}
	res.ConfigPath = singbox.ConfigPath
	return res, nil
}

// buildDeployment gathers everything needed to describe the deployment.
func buildDeployment(ctx context.Context, opts Options) (model.Deployment, error) {
	creds, err := secret.NewCredentials()
	if err != nil {
		return model.Deployment{}, err
	}

	ports := DefaultPorts()
	if opts.NAT {
		ports, err = NATPorts(opts.PortStart, opts.PortEnd)
		if err != nil {
			return model.Deployment{}, err
		}
	}

	addrs := network.Detect(ctx)
	serverIP, ipv6Only := chooseServerIP(addrs)
	if serverIP == "" && !opts.DryRun {
		return model.Deployment{}, fmt.Errorf("未能检测到公网 IP")
	}
	if serverIP == "" {
		serverIP = "203.0.113.1" // placeholder for dry-run without network
	}

	return model.Deployment{
		ServerIP:  serverIP,
		SNI:       network.SelectDomain(ctx),
		CDNDomain: opts.CDNDomain,
		CertFile:  certFile,
		KeyFile:   keyFile,
		Creds:     creds,
		Ports:     ports,
		IPv6Only:  ipv6Only,
	}, nil
}

// chooseServerIP prefers IPv4; an IPv6-only host is marked accordingly and its
// address is what links embed (bracketing is applied at link-build time).
func chooseServerIP(a network.Addresses) (ip string, ipv6Only bool) {
	if a.IPv4 != "" {
		return a.IPv4, false
	}
	if a.IPv6 != "" {
		return a.IPv6, true
	}
	return "", false
}

// deploy applies the deployment to the live system.
func deploy(ctx context.Context, opts Options, dep model.Deployment, cfg []byte) error {
	os := detectOS(ctx)
	if err := singbox.EnsureInstalled(ctx, os); err != nil {
		return fmt.Errorf("安装 sing-box: %w", err)
	}

	ss, err := cert.GenerateSelfSigned(cert.DefaultCN, certValidity)
	if err != nil {
		return err
	}
	if err := ss.WriteFiles(dep.CertFile, dep.KeyFile); err != nil {
		return err
	}

	if err := writeConfig(singbox.ConfigPath, cfg); err != nil {
		return err
	}
	if err := singbox.WriteUnit(ctx); err != nil {
		return err
	}
	if err := singbox.Check(ctx, singbox.ConfigPath); err != nil {
		return fmt.Errorf("配置校验失败: %w", err)
	}
	if err := singbox.EnableAndStart(ctx); err != nil {
		return err
	}
	if !singbox.IsActive(ctx) {
		return fmt.Errorf("sing-box 启动后未处于 active 状态，请查看 journalctl -u sing-box")
	}
	return nil
}

func detectOS(ctx context.Context) system.OSInfo {
	info, err := system.Detect()
	if err != nil {
		// Fall back to apt-style handling; EnsureInstalled tolerates this.
		return system.OSInfo{Manager: system.APT}
	}
	return info
}

func writeConfig(path string, cfg []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(path, cfg, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func writeDryRun(dir string, dep model.Deployment, cfg []byte) (string, error) {
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, cfg, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
