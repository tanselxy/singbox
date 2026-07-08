package panel

import (
	"context"
	"fmt"
	"os"

	"github.com/tanselxy/singbox/internal/system"
)

const (
	unitPath   = "/etc/systemd/system/singbox-panel.service"
	ServiceTag = "singbox-panel"
)

const unitTemplate = `[Unit]
Description=sing-box control panel
After=network.target

[Service]
Type=simple
ExecStart=%s serve
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
`

// InstallService writes the panel's systemd unit and enables+starts it, so the
// panel is resident across reboots. binPath is the installed binary location.
func InstallService(ctx context.Context, binPath string) error {
	unit := fmt.Sprintf(unitTemplate, binPath)
	if err := os.WriteFile(unitPath, []byte(unit), 0o644); err != nil {
		return fmt.Errorf("write panel unit: %w", err)
	}
	if err := system.Run(ctx, "systemctl", "daemon-reload"); err != nil {
		return err
	}
	if err := system.Run(ctx, "systemctl", "enable", ServiceTag); err != nil {
		return err
	}
	return system.Run(ctx, "systemctl", "restart", ServiceTag)
}
