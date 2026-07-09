package panel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/tanselxy/singbox/internal/singbox"
)

// Version is the running panel version, set from main at startup.
var Version = "dev"

// latestReleaseAPI is the GitHub API endpoint for the newest release.
const latestReleaseAPI = "https://api.github.com/repos/tanselxy/singbox/releases/latest"

// handleVersion reports the current and latest available versions.
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	latest := latestReleaseTag(r.Context())
	writeJSON(w, map[string]any{
		"current":    Version,
		"latest":     latest,
		"upgradable": latest != "" && latest != Version && Version != "dev",
	})
}

func latestReleaseTag(ctx context.Context) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestReleaseAPI, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if json.NewDecoder(resp.Body).Decode(&body) != nil {
		return ""
	}
	return body.TagName
}

// handleUpgrade downloads the latest panel and sing-box binaries, replaces the
// installed ones and restarts both services. The panel restart happens after
// the response is sent (it exec-replaces the running process).
func (s *Server) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	arch, err := singbox.ReleaseArch()
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	ctx := r.Context()
	panelBin, err := singbox.Download(ctx, fmt.Sprintf("%s/singbox-panel_linux_%s", singbox.ReleaseBase, arch))
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "下载面板失败: " + err.Error()})
		return
	}
	sbBin, err := singbox.Download(ctx, fmt.Sprintf("%s/sing-box_linux_%s", singbox.ReleaseBase, arch))
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "下载 sing-box 失败: " + err.Error()})
		return
	}
	if len(panelBin) < 1<<20 || len(sbBin) < 1<<20 {
		writeJSON(w, map[string]any{"ok": false, "error": "下载的文件不完整"})
		return
	}

	self, err := os.Executable()
	if err != nil || self == "" {
		self = "/usr/local/bin/singbox-panel"
	}
	if err := replaceBinary(self, panelBin); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "替换面板失败: " + err.Error()})
		return
	}
	if err := replaceBinary(singbox.BinaryPath, sbBin); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "替换 sing-box 失败: " + err.Error()})
		return
	}

	writeJSON(w, map[string]any{"ok": true})

	// Restart after the response is flushed. Uses a detached context so it
	// survives this request; restarting the panel replaces this process.
	go func() {
		time.Sleep(1500 * time.Millisecond)
		bg := context.Background()
		_ = exec.CommandContext(bg, "systemctl", "restart", "sing-box").Run()
		_ = exec.CommandContext(bg, "systemctl", "restart", "singbox-panel").Run()
	}()
}

// replaceBinary atomically replaces the file at path with data. Renaming over a
// running executable is safe on Linux (the running process keeps the old inode).
func replaceBinary(path string, data []byte) error {
	tmp := path + ".new"
	if err := os.WriteFile(tmp, data, 0o755); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
