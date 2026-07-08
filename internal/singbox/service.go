package singbox

import (
	"context"
	"fmt"
	"os"

	"github.com/tanselxy/singbox/internal/system"
)

const (
	unitPath   = "/etc/systemd/system/sing-box.service"
	ConfigDir  = "/etc/sing-box"
	ConfigPath = "/etc/sing-box/config.json"
	ServiceTag = "sing-box"
)

// unitTemplate is a self-contained service definition so behaviour is identical
// across distros, regardless of how the binary was installed.
const unitTemplate = `[Unit]
Description=sing-box service
Documentation=https://sing-box.sagernet.org
After=network.target nss-lookup.target

[Service]
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_SYS_PTRACE CAP_DAC_READ_SEARCH
AmbientCapabilities=CAP_NET_ADMIN CAP_NET_BIND_SERVICE CAP_SYS_PTRACE CAP_DAC_READ_SEARCH
ExecStart=%s run -c %s
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5s
LimitNOFILE=infinity

[Install]
WantedBy=multi-user.target
`

// WriteUnit installs the systemd unit and reloads the daemon.
func WriteUnit(ctx context.Context) error {
	unit := fmt.Sprintf(unitTemplate, Path(), ConfigPath)
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("write unit: %w", err)
	}
	return system.Run(ctx, "systemctl", "daemon-reload")
}

// Check validates a configuration file with `sing-box check`.
func Check(ctx context.Context, configPath string) error {
	return system.Run(ctx, Path(), "check", "-c", configPath)
}

// EnableAndStart enables the service on boot and (re)starts it.
func EnableAndStart(ctx context.Context) error {
	if err := system.Run(ctx, "systemctl", "enable", ServiceTag); err != nil {
		return err
	}
	return system.Run(ctx, "systemctl", "restart", ServiceTag)
}

// IsActive reports whether the service is currently running.
func IsActive(ctx context.Context) bool {
	out, err := system.Output(ctx, "systemctl", "is-active", ServiceTag)
	return err == nil && out == "active"
}

// Stop stops the service (ignoring "not loaded" style errors is left to caller).
func Stop(ctx context.Context) error {
	return system.Run(ctx, "systemctl", "stop", ServiceTag)
}
