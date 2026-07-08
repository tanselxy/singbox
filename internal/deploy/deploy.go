// Package deploy renders the sing-box config for the current server + clients
// and applies it to the running service. It is shared by the installer (first
// install) and the panel (whenever clients change), so it lives in its own
// package to avoid an import cycle between them.
package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tanselxy/singbox/internal/config"
	"github.com/tanselxy/singbox/internal/model"
	"github.com/tanselxy/singbox/internal/singbox"
)

// WriteConfig renders and writes /etc/sing-box/config.json with 0600 perms.
func WriteConfig(srv model.Server, clients []model.Client) error {
	b, err := config.Marshal(srv, clients)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(singbox.ConfigPath), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(singbox.ConfigPath, b, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// Apply writes the config, validates it, and restarts sing-box. A validation
// failure leaves the previous running service untouched (the new config is on
// disk but sing-box is only restarted after check passes).
func Apply(ctx context.Context, srv model.Server, clients []model.Client) error {
	if err := WriteConfig(srv, clients); err != nil {
		return err
	}
	if err := singbox.Check(ctx, singbox.ConfigPath); err != nil {
		return fmt.Errorf("配置校验失败: %w", err)
	}
	if err := singbox.EnableAndStart(ctx); err != nil {
		return err
	}
	if !singbox.IsActive(ctx) {
		return fmt.Errorf("sing-box 重启后未处于 active 状态")
	}
	return nil
}
