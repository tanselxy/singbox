// Package installer orchestrates a full deployment: OS detection, sing-box
// install, server + first-client credential generation, certificate, config
// rendering, service start, resident panel setup and link output.
package installer

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/tanselxy/singbox/internal/cert"
	"github.com/tanselxy/singbox/internal/config"
	"github.com/tanselxy/singbox/internal/deploy"
	"github.com/tanselxy/singbox/internal/model"
	"github.com/tanselxy/singbox/internal/network"
	"github.com/tanselxy/singbox/internal/panel"
	"github.com/tanselxy/singbox/internal/protocol"
	"github.com/tanselxy/singbox/internal/secret"
	"github.com/tanselxy/singbox/internal/singbox"
	"github.com/tanselxy/singbox/internal/state"
	"github.com/tanselxy/singbox/internal/store"
	"github.com/tanselxy/singbox/internal/system"
)

const (
	certDir  = singbox.ConfigDir + "/cert"
	certFile = certDir + "/cert.pem"
	keyFile  = certDir + "/private.key"

	certValidity = 100 * 365 * 24 * time.Hour

	// defaultClientName is the initial client created on a fresh install.
	defaultClientName = "default"
)

// Options controls an install run.
type Options struct {
	NAT       bool
	PortStart int
	PortEnd   int
	CDNDomain string

	// DryRun renders the config into OutputDir without touching the system.
	DryRun    bool
	OutputDir string
}

// Result summarises a completed deployment for the caller to display.
type Result struct {
	Server     model.Server
	Client     model.Client // the initial client
	Links      []model.Link // the initial client's links
	ConfigPath string

	PanelURL      string
	PanelPassword string
}

// Run performs the deployment described by opts.
func Run(ctx context.Context, opts Options) (*Result, error) {
	if !opts.DryRun && !system.IsRoot() {
		return nil, fmt.Errorf("install 需要 root 权限")
	}

	srv, err := buildServer(ctx, opts)
	if err != nil {
		return nil, err
	}

	client, err := secret.NewClient()
	if err != nil {
		return nil, err
	}
	client.Name = defaultClientName
	client.Enabled = true
	client.CreatedAt = time.Now().Unix()
	client.ID = 1 // presentational for dry-run; the store assigns the real id

	res := &Result{Server: srv, Client: client, Links: protocol.ClientLinks(srv, client)}

	if opts.DryRun {
		res.ConfigPath, err = writeDryRun(opts.OutputDir, srv, []model.Client{client})
		return res, err
	}

	if err := doInstall(ctx, srv, &client, res); err != nil {
		return nil, err
	}
	res.Client = client
	res.ConfigPath = singbox.ConfigPath
	return res, nil
}

// buildServer gathers the server-level settings.
func buildServer(ctx context.Context, opts Options) (model.Server, error) {
	reality, ss2022Key, err := secret.NewServerSecrets()
	if err != nil {
		return model.Server{}, err
	}

	ports := DefaultPorts()
	if opts.NAT {
		ports, err = NATPorts(opts.PortStart, opts.PortEnd)
		if err != nil {
			return model.Server{}, err
		}
	}

	addrs := network.Detect(ctx)
	serverIP, ipv6Only := chooseServerIP(addrs)
	if serverIP == "" && !opts.DryRun {
		return model.Server{}, fmt.Errorf("未能检测到公网 IP")
	}
	if serverIP == "" {
		serverIP = "203.0.113.1" // placeholder for dry-run without network
	}

	return model.Server{
		ServerIP:        serverIP,
		SNI:             network.SelectDomain(ctx),
		CDNDomain:       opts.CDNDomain,
		CertFile:        certFile,
		KeyFile:         keyFile,
		IPv6Only:        ipv6Only,
		Ports:           ports,
		Reality:         reality,
		SS2022ServerKey: ss2022Key,
	}, nil
}

func chooseServerIP(a network.Addresses) (ip string, ipv6Only bool) {
	if a.IPv4 != "" {
		return a.IPv4, false
	}
	if a.IPv6 != "" {
		return a.IPv6, true
	}
	return "", false
}

// doInstall applies the deployment to the live system.
func doInstall(ctx context.Context, srv model.Server, client *model.Client, res *Result) error {
	osInfo := detectOS(ctx)
	if err := singbox.EnsureInstalled(ctx, osInfo); err != nil {
		return fmt.Errorf("安装 sing-box: %w", err)
	}

	ss, err := cert.GenerateSelfSigned(cert.DefaultCN, certValidity)
	if err != nil {
		return err
	}
	if err := ss.WriteFiles(srv.CertFile, srv.KeyFile); err != nil {
		return err
	}

	// Persist server settings and the first client.
	if err := state.SaveServer(srv); err != nil {
		return err
	}
	db, err := store.Open(state.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	created, err := db.CreateClient(*client)
	if err != nil {
		return fmt.Errorf("创建初始客户: %w", err)
	}
	*client = created

	clients, err := db.ActiveClients(time.Now().Unix())
	if err != nil {
		return err
	}

	if err := singbox.WriteUnit(ctx); err != nil {
		return err
	}
	if err := deploy.Apply(ctx, srv, clients); err != nil {
		return err
	}

	return setupPanel(ctx, srv, res)
}

// setupPanel bootstraps the panel config, installs its systemd unit so it is
// resident, and records the access URL (and first-run password) in res.
func setupPanel(ctx context.Context, srv model.Server, res *Result) error {
	cfg, freshPassword, err := panel.EnsureConfig()
	if err != nil {
		return fmt.Errorf("配置面板: %w", err)
	}
	binPath, err := os.Executable()
	if err != nil {
		binPath = "/usr/local/bin/singbox-panel"
	}
	if err := panel.InstallService(ctx, binPath); err != nil {
		return fmt.Errorf("安装面板服务: %w", err)
	}

	host := srv.ServerIP
	if srv.IPv6Only {
		host = "[" + srv.ServerIP + "]"
	}
	res.PanelURL = fmt.Sprintf("https://%s:%d/%s/", host, cfg.Port, cfg.PathPrefix)
	res.PanelPassword = freshPassword
	return nil
}

func detectOS(ctx context.Context) system.OSInfo {
	info, err := system.Detect()
	if err != nil {
		return system.OSInfo{Manager: system.APT}
	}
	return info
}

func writeDryRun(dir string, srv model.Server, clients []model.Client) (string, error) {
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	b, err := config.Marshal(srv, clients)
	if err != nil {
		return "", err
	}
	path := dir + "/config.json"
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
