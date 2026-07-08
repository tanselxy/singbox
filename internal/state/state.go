// Package state persists server-level deployment settings so the web panel and
// config regenerator can reload them. Per-client data lives in the SQLite store
// (internal/store); this file holds only the shared Server settings.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tanselxy/singbox/internal/model"
	"github.com/tanselxy/singbox/internal/singbox"
)

var (
	// ServerPath is where server-level settings are stored.
	ServerPath = filepath.Join(singbox.ConfigDir, "server.json")
	// DBPath is the SQLite database of clients and traffic.
	DBPath = filepath.Join(singbox.ConfigDir, "panel.db")
)

// SaveServer writes server settings as 0600 JSON.
func SaveServer(s model.Server) error {
	if err := os.MkdirAll(filepath.Dir(ServerPath), 0o700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal server: %w", err)
	}
	if err := os.WriteFile(ServerPath, b, 0o600); err != nil {
		return fmt.Errorf("write server: %w", err)
	}
	return nil
}

// LoadServer reads the persisted server settings.
func LoadServer() (model.Server, error) {
	b, err := os.ReadFile(ServerPath)
	if err != nil {
		return model.Server{}, fmt.Errorf("read server: %w", err)
	}
	var s model.Server
	if err := json.Unmarshal(b, &s); err != nil {
		return model.Server{}, fmt.Errorf("parse server: %w", err)
	}
	return s, nil
}

// Exists reports whether server settings are present.
func Exists() bool {
	_, err := os.Stat(ServerPath)
	return err == nil
}
