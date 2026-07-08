// Package state persists the deployment model so the web panel can reload it
// after install to render nodes, links and QR codes without re-deriving them
// from config.json.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tanselxy/singbox/internal/model"
	"github.com/tanselxy/singbox/internal/singbox"
)

// DeploymentPath is where the serialized deployment lives.
var DeploymentPath = filepath.Join(singbox.ConfigDir, "deployment.json")

// Save writes the deployment as 0600 JSON.
func Save(d model.Deployment) error {
	if err := os.MkdirAll(filepath.Dir(DeploymentPath), 0o700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal deployment: %w", err)
	}
	if err := os.WriteFile(DeploymentPath, b, 0o600); err != nil {
		return fmt.Errorf("write deployment: %w", err)
	}
	return nil
}

// Load reads the persisted deployment.
func Load() (model.Deployment, error) {
	b, err := os.ReadFile(DeploymentPath)
	if err != nil {
		return model.Deployment{}, fmt.Errorf("read deployment: %w", err)
	}
	var d model.Deployment
	if err := json.Unmarshal(b, &d); err != nil {
		return model.Deployment{}, fmt.Errorf("parse deployment: %w", err)
	}
	return d, nil
}

// Exists reports whether a persisted deployment is present.
func Exists() bool {
	_, err := os.Stat(DeploymentPath)
	return err == nil
}
