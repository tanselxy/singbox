// Package singbox installs the sing-box binary and manages its systemd service.
package singbox

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/tanselxy/singbox/internal/system"
)

// BinaryPath is where our custom sing-box build is installed. We ship our own
// build (compiled with the with_v2ray_api tag) because the official binaries do
// not include it, and per-user traffic stats require it. It is installed to a
// dedicated path — never relying on any pre-existing official sing-box on PATH.
const BinaryPath = "/usr/local/bin/sing-box"

// ReleaseBase is where our binaries are published. Assets are raw linux
// binaries named sing-box_linux_<arch> and singbox-panel_linux_<arch>.
const ReleaseBase = "https://github.com/tanselxy/singbox/releases/latest/download"

// Path returns the sing-box executable path (always our custom build).
func Path() string { return BinaryPath }

// EnsureInstalled downloads our custom sing-box build if it is not present.
func EnsureInstalled(ctx context.Context, _ system.OSInfo) error {
	if fileExists(BinaryPath) {
		return nil
	}
	return installCustomBuild(ctx)
}

// installCustomBuild downloads the sing-box binary (with v2ray_api) for the host
// architecture and installs it.
func installCustomBuild(ctx context.Context) error {
	arch, err := ReleaseArch()
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/sing-box_linux_%s", ReleaseBase, arch)

	bin, err := Download(ctx, url)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(BinaryPath), 0o755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}
	if err := os.WriteFile(BinaryPath, bin, 0o755); err != nil {
		return fmt.Errorf("write sing-box binary: %w", err)
	}
	return nil
}

// ReleaseArch maps the Go architecture to a release asset arch.
func ReleaseArch() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "amd64", nil
	case "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}
}

// Download fetches a URL and returns its body.
func Download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
