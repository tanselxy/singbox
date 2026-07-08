// Package singbox installs the sing-box binary and manages its systemd service.
package singbox

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/tanselxy/singbox/internal/system"
)

// BinaryPath is where the manually-installed binary is placed.
const BinaryPath = "/usr/local/bin/sing-box"

// fallbackVersion is used when the GitHub API is unreachable.
const fallbackVersion = "1.11.4"

// Path returns the sing-box executable path, preferring one already on PATH.
func Path() string {
	if system.LookPath("sing-box") {
		return "sing-box"
	}
	return BinaryPath
}

// EnsureInstalled installs sing-box if it is not already present.
func EnsureInstalled(ctx context.Context, os system.OSInfo) error {
	if system.LookPath("sing-box") || fileExists(BinaryPath) {
		return nil
	}
	switch os.Manager {
	case system.APT:
		return installViaScript(ctx)
	default:
		return installFromRelease(ctx)
	}
}

// installViaScript uses the official Debian/Ubuntu installer.
func installViaScript(ctx context.Context) error {
	return system.Run(ctx, "bash", "-c",
		"curl -fsSL https://sing-box.app/deb-install.sh | bash")
}

// installFromRelease downloads the release tarball for the host architecture
// and installs the binary. Extraction is done in-process so no external tar is
// required.
func installFromRelease(ctx context.Context) error {
	arch, err := releaseArch()
	if err != nil {
		return err
	}
	version := latestVersion(ctx)
	name := fmt.Sprintf("sing-box-%s-linux-%s", version, arch)
	url := fmt.Sprintf("https://github.com/SagerNet/sing-box/releases/download/v%s/%s.tar.gz", version, name)

	bin, err := downloadBinaryFromTarGz(ctx, url, name+"/sing-box")
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

// releaseArch maps the Go architecture to a sing-box release asset arch.
func releaseArch() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "amd64", nil
	case "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}
}

// latestVersion queries the GitHub API for the newest release tag, falling back
// to a pinned version on any error.
func latestVersion(ctx context.Context) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/repos/SagerNet/sing-box/releases/latest", nil)
	if err != nil {
		return fallbackVersion
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fallbackVersion
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fallbackVersion
	}

	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return fallbackVersion
	}
	if v := strings.TrimPrefix(body.TagName, "v"); v != "" {
		return v
	}
	return fallbackVersion
}

// downloadBinaryFromTarGz fetches a .tar.gz and returns the named entry's bytes.
func downloadBinaryFromTarGz(ctx context.Context, url, entry string) ([]byte, error) {
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

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gunzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar: %w", err)
		}
		if filepath.Clean(hdr.Name) == filepath.Clean(entry) {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("entry %q not found in archive", entry)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
