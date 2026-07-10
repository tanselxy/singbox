package panel

import (
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tanselxy/singbox/internal/model"
	"github.com/tanselxy/singbox/internal/state"
	"github.com/tanselxy/singbox/internal/store"
)

func testServer(t *testing.T) (*httptest.Server, Config) {
	t.Helper()
	dir := t.TempDir()
	ConfigPath = filepath.Join(dir, "panel.json")
	state.ServerPath = filepath.Join(dir, "server.json")
	state.DBPath = filepath.Join(dir, "panel.db")

	// Seed a server config and one client.
	if err := state.SaveServer(sampleServer()); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(state.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateClient(sampleClient("alice")); err != nil {
		t.Fatal(err)
	}
	db.Close()

	cfg := Config{
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
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(func() { ts.Close(); srv.db.Close() })
	return ts, cfg
}

func noRedirectClient() *http.Client {
	return &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
}

func login(t *testing.T, ts *httptest.Server, password string) *http.Cookie {
	t.Helper()
	resp, err := noRedirectClient().PostForm(ts.URL+"/abc/login", url.Values{"password": {password}})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			return c
		}
	}
	return nil
}

func TestLoginPageRenders(t *testing.T) {
	ts, _ := testServer(t)
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
	resp, err := noRedirectClient().Get(ts.URL + "/abc/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("expected redirect, got %d", resp.StatusCode)
	}
}

func TestLoginWrongPasswordNoSession(t *testing.T) {
	ts, _ := testServer(t)
	if c := login(t, ts, "wrong"); c != nil {
		t.Fatal("wrong password must not issue a session")
	}
}

func TestDashboardListsClients(t *testing.T) {
	ts, _ := testServer(t)
	cookie := login(t, ts, "s3cret-pass")
	if cookie == nil {
		t.Fatal("login failed")
	}
	req, _ := http.NewRequest("GET", ts.URL+"/abc/dashboard", nil)
	req.AddCookie(cookie)
	resp, err := noRedirectClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("dashboard status = %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "alice") {
		t.Error("dashboard should list client alice")
	}
}

func TestClientDetailShowsNodes(t *testing.T) {
	ts, _ := testServer(t)
	cookie := login(t, ts, "s3cret-pass")
	req, _ := http.NewRequest("GET", ts.URL+"/abc/client/1", nil)
	req.AddCookie(cookie)
	resp, err := noRedirectClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("client detail status = %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "Reality") || !strings.Contains(body, "/sub/") {
		t.Error("client detail should show nodes and subscription URL")
	}
}

func TestSubscriptionByToken(t *testing.T) {
	ts, _ := testServer(t)
	// alice's token is "tok-alice" (from sampleClient).
	resp, err := http.Get(ts.URL + "/abc/sub/tok-alice")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("subscription status = %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(body))
	if err != nil {
		t.Fatalf("subscription should be base64: %v", err)
	}
	if !strings.Contains(string(decoded), "vless://") {
		t.Error("decoded subscription should contain node links")
	}
	if got := resp.Header.Get("Subscription-Userinfo"); strings.Contains(got, "upload=") || strings.Contains(got, "download=") || strings.Contains(got, "total=") {
		t.Fatalf("subscription traffic header should be human-readable, got %q", got)
	} else if !strings.Contains(got, "当前已使用") || !strings.Contains(got, "流量总额度") {
		t.Fatalf("subscription should include human traffic header, got %q", got)
	}
	if got := resp.Header.Get("Profile-Title"); got == "" {
		t.Fatal("subscription should include profile title")
	}
	if got := resp.Header.Get("X-Subscription-Usage"); !strings.Contains(got, "当前已使用") || !strings.Contains(got, "流量总额度") {
		t.Fatalf("subscription should include human usage title, got %q", got)
	}

	// Unknown token → 404.
	resp2, _ := http.Get(ts.URL + "/abc/sub/nope")
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("unknown token should 404, got %d", resp2.StatusCode)
	}
}

func TestSubscriptionUsageTitleUsesMBBelowGB(t *testing.T) {
	got := subscriptionUsageTitle(512*1024*1024, 900*1024*1024)
	if got != "当前已使用512.00 MB流量/流量总额度900.00 MB" {
		t.Fatalf("title = %q", got)
	}
}

func TestQRRequiresAuthAndReturnsPNG(t *testing.T) {
	ts, _ := testServer(t)
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
	for i := 0; i < lockoutThreshold; i++ {
		login(t, ts, "wrong")
	}
	resp, err := noRedirectClient().PostForm(ts.URL+"/abc/login", url.Values{"password": {"s3cret-pass"}})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if loc := resp.Header.Get("Location"); !strings.Contains(loc, "locked") {
		t.Fatalf("expected lockout redirect, got %q", loc)
	}
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func sampleServer() model.Server {
	return model.Server{
		ServerIP:        "203.0.113.7",
		SNI:             "www.apple.com",
		CertFile:        "/etc/sing-box/cert/cert.pem",
		KeyFile:         "/etc/sing-box/cert/private.key",
		SS2022ServerKey: "c2VydmVyUFNL",
		Reality:         model.Reality{PrivateKey: "priv", PublicKey: "pub", ShortID: "0123456789abcdef"},
		Ports:           model.Ports{Reality: 20000, Hysteria2: 50000, ShadowTLS: 31000, TUIC: 61555, TrojanWS: 63333, VLESSCDN: 4433},
	}
}

func sampleClient(name string) model.Client {
	return model.Client{
		Name:              name,
		UUID:              "11111111-2222-3333-4444-555555555555",
		Password:          "pw-" + name,
		SS2022Key:         "YWxpY2U=",
		ShadowTLSPassword: "st-" + name,
		SubToken:          "tok-" + name,
		Enabled:           true,
		CreatedAt:         1000,
	}
}
