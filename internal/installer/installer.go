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

	// Re-running install on a box that is already deployed is an in-place
	// upgrade: reuse the existing server settings (so existing client links keep
	// working) instead of regenerating keys/ports.
	existing := !opts.DryRun && state.Exists()

	var srv model.Server
	var err error
	if existing {
		if srv, err = state.LoadServer(); err != nil {
			return nil, fmt.Errorf("读取已有部署: %w", err)
		}
	} else if srv, err = buildServer(ctx, opts); err != nil {
		return nil, err
	}

	res := &Result{Server: srv}

	if opts.DryRun {
		client, err := newDefaultClient()
		if err != nil {
			return nil, err
		}
		res.Client, res.Links = client, protocol.ClientLinks(srv, client)
		res.ConfigPath, err = writeDryRun(opts.OutputDir, srv, []model.Client{client})
		return res, err
	}

	client, err := doInstall(ctx, srv, res)
	if err != nil {
		return nil, err
	}
	res.Client = client
	res.Links = protocol.ClientLinks(srv, client)
	res.ConfigPath = singbox.ConfigPath
	return res, nil
}

// newDefaultClient builds the initial client seeded on a fresh install.
func newDefaultClient() (model.Client, error) {
	c, err := secret.NewClient()
	if err != nil {
		return model.Client{}, err
	}
	c.Name = defaultClientName
	c.Enabled = true
	c.CreatedAt = time.Now().Unix()
	return c, nil
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

// doInstall applies the deployment to the live system and returns the client to
// display (the seeded default on a fresh install, or an existing one). It is
// idempotent: re-running preserves existing certs, clients and server settings.
func doInstall(ctx context.Context, srv model.Server, res *Result) (model.Client, error) {
	osInfo := detectOS(ctx)
	if err := singbox.EnsureInstalled(ctx, osInfo); err != nil {
		return model.Client{}, fmt.Errorf("安装 sing-box: %w", err)
	}

	// Generate the self-signed cert only if it does not already exist.
	if !fileExists(srv.CertFile) || !fileExists(srv.KeyFile) {
		ss, err := cert.GenerateSelfSigned(cert.DefaultCN, certValidity)
		if err != nil {
			return model.Client{}, err
		}
		if err := ss.WriteFiles(srv.CertFile, srv.KeyFile); err != nil {
			return model.Client{}, err
		}
	}

	if err := state.SaveServer(srv); err != nil {
		return model.Client{}, err
	}
	db, err := store.Open(state.DBPath)
	if err != nil {
		return model.Client{}, err
	}
	defer db.Close()

	// Seed the default client only when the store has none yet.
	existingClients, err := db.ListClients()
	if err != nil {
		return model.Client{}, err
	}
	var initial model.Client
	if len(existingClients) == 0 {
		c, err := newDefaultClient()
		if err != nil {
			return model.Client{}, err
		}
		if initial, err = db.CreateClient(c); err != nil {
			return model.Client{}, fmt.Errorf("创建初始客户: %w", err)
		}
	} else {
		initial = existingClients[0]
	}

	clients, err := db.ActiveClients(time.Now().Unix())
	if err != nil {
		return model.Client{}, err
	}

	if err := singbox.WriteUnit(ctx); err != nil {
		return model.Client{}, err
	}
	if err := deploy.Apply(ctx, srv, clients); err != nil {
		return model.Client{}, err
	}

	if err := setupPanel(ctx, srv, res); err != nil {
		return model.Client{}, err
	}
	return initial, nil
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

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
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
