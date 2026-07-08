package cert

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerateSelfSignedIsParseable(t *testing.T) {
	ss, err := GenerateSelfSigned("", 100*365*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	block, _ := pem.Decode(ss.CertPEM)
	if block == nil {
		t.Fatal("cert PEM did not decode")
	}
	crt, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	if crt.Subject.CommonName != DefaultCN {
		t.Errorf("CN = %q, want %q", crt.Subject.CommonName, DefaultCN)
	}
	if crt.NotAfter.Before(time.Now().Add(50 * 365 * 24 * time.Hour)) {
		t.Error("certificate should be long-lived")
	}
}

func TestWriteFilesPermissions(t *testing.T) {
	ss, err := GenerateSelfSigned("example.test", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certPath := filepath.Join(dir, "sub", "cert.pem")
	keyPath := filepath.Join(dir, "sub", "private.key")
	if err := ss.WriteFiles(certPath, keyPath); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{certPath, keyPath} {
		fi, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("%s perm = %o, want 600", p, fi.Mode().Perm())
		}
	}
}
