package panel

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tanselxy/singbox/internal/model"
	"github.com/tanselxy/singbox/internal/state"
)

func testServer(t *testing.T) (*httptest.Server, Config) {
	t.Helper()
	// Redirect persisted paths into the test's temp dir.
	dir := t.TempDir()
	ConfigPath = filepath.Join(dir, "panel.json")
	state.DeploymentPath = filepath.Join(dir, "deployment.json")

	cfg := Config{
		Port:       0,
		PathPrefix: "abc",
		SessionKey: hex.EncodeToString([]byte("0123456789abcdef0123456789abcdef")),
	}
	if err := cfg.SetPassword("s3cret-pass"); err != nil {
		t.Fatal(err)
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewServer(srv.Handler()), cfg
}

// noRedirectClient returns a client that surfaces 3xx instead of following.
func noRedirectClient() *http.Client {
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

func TestLoginPageRenders(t *testing.T) {
	ts, _ := testServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/abc/login")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestProtectedRedirectsWhenUnauthed(t *testing.T) {
	ts, _ := testServer(t)
	defer ts.Close()

	resp, err := noRedirectClient().Get(ts.URL + "/abc/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.Contains(loc, "/abc/login") {
		t.Fatalf("redirect location = %q", loc)
	}
}

func login(t *testing.T, ts *httptest.Server, password string) *http.Cookie {
	t.Helper()
	resp, err := noRedirectClient().PostForm(ts.URL+"/abc/login",
		url.Values{"password": {password}})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login status = %d", resp.StatusCode)
	}
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			return c
		}
	}
	return nil
}

func TestLoginWrongPasswordNoSession(t *testing.T) {
	ts, _ := testServer(t)
	defer ts.Close()

	if c := login(t, ts, "wrong"); c != nil {
		t.Fatal("wrong password must not issue a session")
	}
}

func TestLoginSuccessGrantsAccess(t *testing.T) {
	ts, _ := testServer(t)
	defer ts.Close()

	// Save a deployment so the dashboard has something to render.
	if err := state.Save(sampleDeployment()); err != nil {
		t.Fatal(err)
	}

	cookie := login(t, ts, "s3cret-pass")
	if cookie == nil {
		t.Fatal("correct password should issue a session")
	}

	req, _ := http.NewRequest("GET", ts.URL+"/abc/dashboard", nil)
	req.AddCookie(cookie)
	resp, err := noRedirectClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("authed dashboard status = %d", resp.StatusCode)
	}
}

func TestQRRequiresAuthAndReturnsPNG(t *testing.T) {
	ts, _ := testServer(t)
	defer ts.Close()

	// Unauthed → redirect.
	resp, _ := noRedirectClient().Get(ts.URL + "/abc/qr?data=hello")
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("qr should require auth, got %d", resp.StatusCode)
	}

	cookie := login(t, ts, "s3cret-pass")
	req, _ := http.NewRequest("GET", ts.URL+"/abc/qr?data=hello", nil)
	req.AddCookie(cookie)
	resp2, err := noRedirectClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if ct := resp2.Header.Get("Content-Type"); ct != "image/png" {
		t.Fatalf("qr content-type = %q", ct)
	}
}

func TestLockoutAfterRepeatedFailures(t *testing.T) {
	ts, _ := testServer(t)
	defer ts.Close()

	for i := 0; i < lockoutThreshold; i++ {
		login(t, ts, "wrong")
	}
	// Next attempt (even with correct password) should be locked out.
	resp, err := noRedirectClient().PostForm(ts.URL+"/abc/login",
		url.Values{"password": {"s3cret-pass"}})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if loc := resp.Header.Get("Location"); !strings.Contains(loc, "locked") {
		t.Fatalf("expected lockout redirect, got %q", loc)
	}
}

func sampleDeployment() model.Deployment {
	return model.Deployment{
		ServerIP: "203.0.113.7",
		SNI:      "www.apple.com",
		CertFile: "/etc/sing-box/cert/cert.pem",
		KeyFile:  "/etc/sing-box/cert/private.key",
		Creds: model.Credentials{
			UUID:              "11111111-2222-3333-4444-555555555555",
			HysteriaPassword:  "hyPass123",
			SSPassword:        "c3NwYXNz",
			ShadowTLSPassword: "stlsPass==",
			Reality:           model.Reality{PrivateKey: "priv", PublicKey: "pub", ShortID: "0123456789abcdef"},
		},
		Ports: model.Ports{Reality: 20000, Hysteria2: 50000, ShadowTLS: 31000, SSDirect: 59000, TUIC: 61555, TrojanWS: 63333, VLESSCDN: 4433},
	}
}
