// Package deploy renders the sing-box config for the current server + clients
// and applies it to the running service. It is shared by the installer (first
// install) and the panel (whenever clients change), so it lives in its own
// package to avoid an import cycle between them.
package deploy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/tanselxy/singbox/internal/config"
	"github.com/tanselxy/singbox/internal/model"
	"github.com/tanselxy/singbox/internal/singbox"
)

// appliedHash remembers the config this process last applied successfully
// (written + validated + service restarted), so re-applying an unchanged
// desired state is a no-op instead of bouncing every connection. It is only
// set after a fully successful apply: a failed apply leaves it stale, which
// makes the next Apply retry for real.
var (
	appliedMu   sync.Mutex
	appliedHash [sha256.Size]byte
	appliedSet  bool
)

// WriteConfig renders and writes /etc/sing-box/config.json with 0600 perms.
func WriteConfig(srv model.Server, clients []model.Client) error {
	b, err := config.Marshal(srv, clients)
	if err != nil {
		return err
	}
	return writeBytes(b)
}

func writeBytes(b []byte) error {
	if err := os.MkdirAll(filepath.Dir(singbox.ConfigPath), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(singbox.ConfigPath, b, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// Apply brings the running sing-box in sync with srv + clients: it writes the
// config, validates it, and restarts the service. A validation failure leaves
// the previous running service untouched (the new config is on disk but
// sing-box is only restarted after check passes).
//
// Apply is idempotent: if this process already applied an identical config it
// returns immediately without touching the service, so callers may re-apply
// the desired state on a timer to retry earlier failures.
func Apply(ctx context.Context, srv model.Server, clients []model.Client) error {
	return apply(ctx, srv, clients, false)
}

// ForceApply applies even when the config appears already applied. Use it when
// there is proof the running service is stale — e.g. sing-box reports traffic
// for a client that is not in the desired config.
func ForceApply(ctx context.Context, srv model.Server, clients []model.Client) error {
	return apply(ctx, srv, clients, true)
}

func apply(ctx context.Context, srv model.Server, clients []model.Client, force bool) error {
	b, err := config.Marshal(srv, clients)
	if err != nil {
		return err
	}
	h := sha256.Sum256(b)

	appliedMu.Lock()
	defer appliedMu.Unlock()

	if !force {
		if appliedSet && h == appliedHash {
			return nil
		}
		// First Apply after process start: if the desired config already sits
		// on disk, adopt it without restarting so a panel restart does not
		// bounce sing-box. A running process that diverges from its disk config
		// is caught by traffic-based detection, which uses ForceApply.
		if !appliedSet {
			if cur, err := os.ReadFile(singbox.ConfigPath); err == nil && bytes.Equal(cur, b) {
				appliedHash, appliedSet = h, true
				return nil
			}
		}
	}

	if err := writeBytes(b); err != nil {
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
	appliedHash, appliedSet = h, true
	return nil
}
