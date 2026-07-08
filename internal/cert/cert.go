// Package cert issues the self-signed certificate used by the TLS-based
// inbounds (TUIC/Trojan/Hysteria2). It uses crypto/x509 directly, dropping the
// old code's dependency on the openssl CLI. ACME (Let's Encrypt) for the
// CDN/IPv6 scenario lands in P4.
package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// DefaultCN is the common name of the self-signed certificate. It matches the
// server_name the self-signed inbounds present.
const DefaultCN = "bing.com"

// SelfSigned holds a PEM-encoded certificate/key pair.
type SelfSigned struct {
	CertPEM []byte
	KeyPEM  []byte
}

// GenerateSelfSigned creates a long-lived P-256 self-signed certificate for cn.
func GenerateSelfSigned(cn string, validity time.Duration) (*SelfSigned, error) {
	if cn == "" {
		cn = DefaultCN
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate serial: %w", err)
	}

	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: cn},
		DNSNames:              []string{cn},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(validity),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal key: %w", err)
	}

	return &SelfSigned{
		CertPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		KeyPEM:  pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
	}, nil
}

// WriteFiles writes the certificate and key to certPath/keyPath with 0600
// permissions, creating parent directories as needed.
func (s *SelfSigned) WriteFiles(certPath, keyPath string) error {
	for _, p := range []string{certPath, keyPath} {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			return fmt.Errorf("create cert dir: %w", err)
		}
	}
	if err := os.WriteFile(certPath, s.CertPEM, 0o600); err != nil {
		return fmt.Errorf("write cert: %w", err)
	}
	if err := os.WriteFile(keyPath, s.KeyPEM, 0o600); err != nil {
		return fmt.Errorf("write key: %w", err)
	}
	return nil
}
