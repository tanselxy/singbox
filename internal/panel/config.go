// Package panel implements the resident HTTPS control panel: bootstrap config,
// authentication with lockout, and the request handlers.
package panel

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
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
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
	SessionKey   string `json:"session_key"` // hex, HMAC key for signed cookies
	CertFile     string `json:"cert_file"`
	KeyFile      string `json:"key_file"`

	// AgentToken authenticates a master controlling this panel as a node.
	AgentToken string `json:"agent_token"`

	// Managed is set once a master pushes config to this panel via the agent
	// API. A managed panel defers all client state to its master: local client
	// management is disabled and its local traffic poller does not run.
	Managed bool `json:"managed"`
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
		changed := false
		// Backfill settings for configs created before these fields existed.
		if c.AgentToken == "" {
			if c.AgentToken, err = randHex(24); err != nil {
				return Config{}, "", err
			}
			changed = true
		}
		if c.Username == "" {
			c.Username = "admin"
			changed = true
		}
		if changed {
			if err := c.save(); err != nil {
				return Config{}, "", err
			}
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
	agentToken, err := randHex(24)
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
		Username:     "admin",
		PasswordHash: string(hash),
		SessionKey:   sessionKey,
		CertFile:     panelCertFile,
		KeyFile:      panelKeyFile,
		AgentToken:   agentToken,
	}, password, nil
}

// SetManaged persists the managed flag.
func (c *Config) SetManaged(managed bool) error {
	c.Managed = managed
	return c.save()
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

// LoginUsername keeps admin as the default for panel files created before
// username support was introduced.
func (c Config) LoginUsername() string {
	if username := strings.TrimSpace(c.Username); username != "" {
		return username
	}
	return "admin"
}

// VerifyCredentials checks both values without skipping the password hash
// comparison when the username is wrong.
func (c Config) VerifyCredentials(username, password string) bool {
	nameOK := subtle.ConstantTimeCompare([]byte(strings.TrimSpace(username)), []byte(c.LoginUsername())) == 1
	passwordOK := c.VerifyPassword(password)
	return nameOK && passwordOK
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

// SetCredentials persists a new login name and password, and rotates the
// session signing key so every existing browser session becomes invalid.
func (c *Config) SetCredentials(username, password string) error {
	username = strings.TrimSpace(username)
	if !validUsername(username) {
		return fmt.Errorf("用户名需为 3-32 位字母、数字、点、下划线或连字符")
	}
	if len(password) < 8 {
		return fmt.Errorf("密码至少需要 8 位")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	sessionKey, err := randHex(32)
	if err != nil {
		return err
	}
	c.Username = username
	c.PasswordHash = string(hash)
	c.SessionKey = sessionKey
	return c.save()
}

func validUsername(username string) bool {
	if len(username) < 3 || len(username) > 32 {
		return false
	}
	for _, char := range username {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '.' || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
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
