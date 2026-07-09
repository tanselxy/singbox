package panel

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/tanselxy/singbox/internal/cert"
)

// TestSplitTLS verifies that one port serves both HTTPS and plain HTTP.
func TestSplitTLS(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tlsLn, plainLn := splitTLS(ln)

	ss, err := cert.GenerateSelfSigned("localhost", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	pair, err := tls.X509KeyPair(ss.CertPEM, ss.KeyPEM)
	if err != nil {
		t.Fatal(err)
	}

	tag := func(name string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, name)
		})
	}
	tlsSrv := &http.Server{Handler: tag("tls"), TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}}}
	plainSrv := &http.Server{Handler: tag("plain")}
	go tlsSrv.Serve(tls.NewListener(tlsLn, tlsSrv.TLSConfig))
	go plainSrv.Serve(plainLn)
	defer tlsSrv.Close()
	defer plainSrv.Close()

	addr := ln.Addr().String()
	get := func(url string) string {
		t.Helper()
		client := &http.Client{
			Timeout:   5 * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		}
		resp, err := client.Get(url)
		if err != nil {
			t.Fatalf("GET %s: %v", url, err)
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	if got := get("http://" + addr); got != "plain" {
		t.Errorf("plain HTTP: got %q, want %q", got, "plain")
	}
	if got := get("https://" + addr); got != "tls" {
		t.Errorf("HTTPS: got %q, want %q", got, "tls")
	}
}
