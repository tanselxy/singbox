// Package panel implements the resident HTTPS control panel: bootstrap config,
// authentication with lockout, and the request handlers.
package panel

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/tanselxy/singbox/internal/cert"
	"github.com/tanselxy/singbox/internal/secret"
	"github.com/tanselxy/singbox/internal/singbox"
)

var (
	// ConfigPath stores panel bootstrap settings.
	ConfigPath = filepath.Join(singbox.ConfigDir, "panel.json")

	panelCertFile = filepath.Join(singbox.ConfigDir, "panel", "cert.pem")
	panelKeyFile  = filepath.Join(singbox.ConfigDir, "panel", "key.pem")
)

// Config is the panel's persisted bootstrap configuration.
type Config struct {
	Port         int    `json:"port"`
	PathPrefix   string `json:"path_prefix"` // random URL prefix, no slashes
	PasswordHash string `json:"password_hash"`
	SessionKey   string `json:"session_key"` // hex, HMAC key for signed cookies
	CertFile     string `json:"cert_file"`
	KeyFile      string `json:"key_file"`
}

// EnsureConfig loads the panel config, generating and persisting a fresh one
// (random port, random path prefix, random password, self-signed TLS) on first
// run. When newly generated, the plaintext password is returned so the caller
// can print it exactly once; on subsequent runs it is empty.
func EnsureConfig() (Config, string, error) {
	if b, err := os.ReadFile(ConfigPath); err == nil {
		var c Config
		if err := json.Unmarshal(b, &c); err != nil {
			return Config{}, "", fmt.Errorf("parse panel config: %w", err)
		}
		return c, "", nil
	}

	c, plain, err := generateConfig()
	if err != nil {
		return Config{}, "", err
	}
	if err := c.save(); err != nil {
		return Config{}, "", err
	}
	return c, plain, nil
}

func generateConfig() (Config, string, error) {
	port, err := randPort(20000, 60000)
	if err != nil {
		return Config{}, "", err
	}
	prefix, err := randHex(8)
	if err != nil {
		return Config{}, "", err
	}
	password, err := secret.Password(20)
	if err != nil {
		return Config{}, "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Config{}, "", fmt.Errorf("hash password: %w", err)
	}
	sessionKey, err := randHex(32)
	if err != nil {
		return Config{}, "", err
	}

	ss, err := cert.GenerateSelfSigned("localhost", 100*365*24*time.Hour)
	if err != nil {
		return Config{}, "", err
	}
	if err := ss.WriteFiles(panelCertFile, panelKeyFile); err != nil {
		return Config{}, "", err
	}

	return Config{
		Port:         port,
		PathPrefix:   prefix,
		PasswordHash: string(hash),
		SessionKey:   sessionKey,
		CertFile:     panelCertFile,
		KeyFile:      panelKeyFile,
	}, password, nil
}

func (c Config) save() error {
	if err := os.MkdirAll(filepath.Dir(ConfigPath), 0o700); err != nil {
		return fmt.Errorf("create panel config dir: %w", err)
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(ConfigPath, b, 0o600); err != nil {
		return fmt.Errorf("write panel config: %w", err)
	}
	return nil
}

// VerifyPassword reports whether password matches the stored hash.
func (c Config) VerifyPassword(password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(c.PasswordHash), []byte(password)) == nil
}

// SetPassword updates the stored hash and persists the config.
func (c *Config) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	c.PasswordHash = string(hash)
	return c.save()
}

func randPort(lo, hi int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(hi-lo+1)))
	if err != nil {
		return 0, err
	}
	return lo + int(n.Int64()), nil
}

func randHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
