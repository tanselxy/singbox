package secret

import (
	"encoding/base64"
	"regexp"
	"testing"
)

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestUUIDFormat(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		u, err := UUID()
		if err != nil {
			t.Fatal(err)
		}
		if !uuidRe.MatchString(u) {
			t.Fatalf("bad uuid: %s", u)
		}
		if seen[u] {
			t.Fatalf("duplicate uuid: %s", u)
		}
		seen[u] = true
	}
}

func TestNewRealityIsUniqueAndValid(t *testing.T) {
	a, err := NewReality()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewReality()
	if err != nil {
		t.Fatal(err)
	}
	if a.PrivateKey == b.PrivateKey || a.PublicKey == b.PublicKey {
		t.Fatal("reality key pairs must differ between installs")
	}
	// Keys are base64 raw-url of 32-byte X25519 values.
	for _, k := range []string{a.PrivateKey, a.PublicKey} {
		raw, err := base64.RawURLEncoding.DecodeString(k)
		if err != nil {
			t.Fatalf("reality key not base64 raw-url: %q", k)
		}
		if len(raw) != 32 {
			t.Fatalf("reality key len = %d, want 32", len(raw))
		}
	}
	if len(a.ShortID) != 16 {
		t.Fatalf("short id len = %d, want 16", len(a.ShortID))
	}
}

func TestNewClientPopulated(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	if c.UUID == "" || c.Password == "" || c.SS2022Key == "" ||
		c.ShadowTLSPassword == "" || c.SubToken == "" {
		t.Fatalf("client field left empty: %+v", c)
	}
	if _, err := base64.StdEncoding.DecodeString(c.SS2022Key); err != nil {
		t.Errorf("SS2022Key must be valid base64: %v", err)
	}
}

func TestNewServerSecretsUnique(t *testing.T) {
	r1, k1, err := NewServerSecrets()
	if err != nil {
		t.Fatal(err)
	}
	r2, k2, err := NewServerSecrets()
	if err != nil {
		t.Fatal(err)
	}
	if r1.PrivateKey == r2.PrivateKey || k1 == k2 {
		t.Error("server secrets must differ between installs")
	}
}

func TestTokensAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 100; i++ {
		tok, err := Token(24)
		if err != nil {
			t.Fatal(err)
		}
		if seen[tok] {
			t.Fatal("duplicate token")
		}
		seen[tok] = true
	}
}
