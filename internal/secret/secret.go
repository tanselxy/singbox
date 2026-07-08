// Package secret generates the credentials for a deployment: UUIDs, passwords
// and the Reality key pair. Everything is produced in-process with crypto/rand,
// replacing the old code's fragile shell fallbacks (uuidgen/openssl/python) and
// its single hard-coded Reality key shared by every install.
package secret

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/tanselxy/singbox/internal/model"
)

const (
	alphaNum = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	hexChars = "0123456789abcdef"
)

// NewCredentials builds a fresh, fully-populated credential set.
func NewCredentials() (model.Credentials, error) {
	uuid, err := UUID()
	if err != nil {
		return model.Credentials{}, err
	}
	hyPass, err := Password(15)
	if err != nil {
		return model.Credentials{}, err
	}
	// Shadowsocks-2022 (chacha20) needs a 32-byte base64 key.
	ssPass, err := Base64Key(32)
	if err != nil {
		return model.Credentials{}, err
	}
	stlsPass, err := Base64Key(32)
	if err != nil {
		return model.Credentials{}, err
	}
	reality, err := NewReality()
	if err != nil {
		return model.Credentials{}, err
	}
	return model.Credentials{
		UUID:              uuid,
		HysteriaPassword:  hyPass,
		SSPassword:        ssPass,
		ShadowTLSPassword: stlsPass,
		Reality:           reality,
	}, nil
}

// UUID returns a random RFC 4122 version-4 UUID.
func UUID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate uuid: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// Password returns a random alphanumeric password of length n.
func Password(n int) (string, error) { return randString(n, alphaNum) }

// Base64Key returns the standard-base64 encoding of n random bytes, suitable
// for Shadowsocks-2022 and ShadowTLS passwords.
func Base64Key(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// ShortID returns a random hex short id for Reality (default 8 bytes / 16 hex).
func ShortID() (string, error) { return randString(16, hexChars) }

// NewReality generates an X25519 key pair encoded the way sing-box expects
// (base64 raw-url of the 32-byte scalar / public key), matching the output of
// `sing-box generate reality-keypair`.
func NewReality() (model.Reality, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return model.Reality{}, fmt.Errorf("generate reality key: %w", err)
	}
	sid, err := ShortID()
	if err != nil {
		return model.Reality{}, err
	}
	enc := base64.RawURLEncoding
	return model.Reality{
		PrivateKey: enc.EncodeToString(priv.Bytes()),
		PublicKey:  enc.EncodeToString(priv.PublicKey().Bytes()),
		ShortID:    sid,
	}, nil
}

func randString(n int, charset string) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random string: %w", err)
	}
	out := make([]byte, n)
	for i, v := range b {
		out[i] = charset[int(v)%len(charset)]
	}
	return string(out), nil
}
