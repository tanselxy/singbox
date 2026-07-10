package panel

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tanselxy/singbox/internal/metrics"
	"github.com/tanselxy/singbox/internal/model"
	"github.com/tanselxy/singbox/internal/state"
	"github.com/tanselxy/singbox/internal/store"
)

func TestNodeRowsUseCachedMetrics(t *testing.T) {
	dir := t.TempDir()
	ConfigPath = filepath.Join(dir, "panel.json")
	state.ServerPath = filepath.Join(dir, "server.json")
	state.DBPath = filepath.Join(dir, "panel.db")
	if err := state.SaveServer(sampleServer()); err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(state.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	node, err := db.CreateNode(model.Node{Name: "cached-node", Address: "https://192.0.2.1:54622", Token: "node-token", CreatedAt: 1})
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	cfg := Config{PathPrefix: "abc", SessionKey: hex.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))}
	srv, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.db.Close() })
	srv.metricsMu.Lock()
	srv.nodeMetrics[node.ID] = nodeMetricSnapshot{
		metrics: metrics.System{CPUPercent: 12.5, CPUCores: 2, MemUsed: 256 << 20, MemTotal: 1 << 30},
		online:  true, sampledAt: 1234,
	}
	srv.metricsMu.Unlock()

	rows, err := srv.nodeRows(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Pending || !rows[0].Online {
		t.Fatalf("cached node row = %#v", rows)
	}
	if rows[0].CPU != "12.5%" || rows[0].SampledAt != 1234 {
		t.Fatalf("cached metrics were not applied: %#v", rows[0])
	}
}

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
		AgentToken: "test-agent-token",
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
	return loginAs(t, ts, "admin", password)
}

func loginAs(t *testing.T, ts *httptest.Server, username, password string) *http.Cookie {
	t.Helper()
	resp, err := noRedirectClient().PostForm(ts.URL+"/abc/login", url.Values{"username": {username}, "password": {password}})
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

func TestParseExpirySupportsMinutePrecision(t *testing.T) {
	got := parseExpiry("2030-02-03T04:05")
	want := time.Date(2030, time.February, 3, 4, 5, 0, 0, time.UTC).Unix()
	if got != want {
		t.Fatalf("parseExpiry() = %d, want %d", got, want)
	}
	if displayed := expiryInputDate(got); displayed != "2030-02-03T04:05" {
		t.Fatalf("expiry input = %q", displayed)
	}
	if label := expiryLabel(got); label != "2030-02-03 04:05" {
		t.Fatalf("expiry label = %q", label)
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

func TestPublicMonitoringDoesNotRequireAuthAndHidesAddress(t *testing.T) {
	ts, cfg := testServer(t)
	db, err := store.Open(state.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateNode(model.Node{
		Name: "public-node", Address: ts.URL, Token: cfg.AgentToken,
		ServerJSON: "{}", CreatedAt: 1000,
	}); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	resp, err := http.Get(ts.URL + "/abc/public/monitoring")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("public monitoring status = %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "public-monitoring-root") {
		t.Fatal("public monitoring page should render public root")
	}

	apiResp, err := http.Get(ts.URL + "/abc/api/public/node-metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer apiResp.Body.Close()
	if apiResp.StatusCode != http.StatusOK {
		t.Fatalf("public metrics status = %d", apiResp.StatusCode)
	}
	apiBody := readBody(t, apiResp)
	if strings.Contains(apiBody, "https://") || strings.Contains(apiBody, "127.0.0.1") {
		t.Fatalf("public metrics should hide node addresses: %s", apiBody)
	}
}

func TestLoginWrongPasswordNoSession(t *testing.T) {
	ts, _ := testServer(t)
	if c := login(t, ts, "wrong"); c != nil {
		t.Fatal("wrong password must not issue a session")
	}
}

func TestAccountUpdateChangesCredentialsAndInvalidatesSessions(t *testing.T) {
	ts, _ := testServer(t)
	cookie := login(t, ts, "s3cret-pass")
	if cookie == nil {
		t.Fatal("initial login failed")
	}
	form := url.Values{
		"username":         {"operator"},
		"current_password": {"s3cret-pass"},
		"new_password":     {"new-secure-password"},
		"confirm_password": {"new-secure-password"},
	}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/abc/api/account", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	resp, err := noRedirectClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("account update status = %d", resp.StatusCode)
	}
	if !strings.Contains(readBody(t, resp), `"ok":true`) {
		t.Fatal("account update should succeed")
	}

	oldSessionReq, _ := http.NewRequest(http.MethodGet, ts.URL+"/abc/dashboard", nil)
	oldSessionReq.AddCookie(cookie)
	oldSessionResp, err := noRedirectClient().Do(oldSessionReq)
	if err != nil {
		t.Fatal(err)
	}
	defer oldSessionResp.Body.Close()
	if oldSessionResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("old session status = %d, want redirect", oldSessionResp.StatusCode)
	}
	if loginAs(t, ts, "admin", "s3cret-pass") != nil {
		t.Fatal("old credentials should no longer work")
	}
	if loginAs(t, ts, "operator", "new-secure-password") == nil {
		t.Fatal("new credentials should work")
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
	} else if !strings.Contains(got, "Used") || !strings.Contains(got, "Total") {
		t.Fatalf("subscription should include human traffic header, got %q", got)
	}
	if got := resp.Header.Get("Profile-Title"); got == "" {
		t.Fatal("subscription should include profile title")
	}
	if got := resp.Header.Get("X-Subscription-Usage"); !strings.Contains(got, "Used") || !strings.Contains(got, "Total") {
		t.Fatalf("subscription should include human usage title, got %q", got)
	}

	// Unknown token → 404.
	resp2, _ := http.Get(ts.URL + "/abc/sub/nope")
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("unknown token should 404, got %d", resp2.StatusCode)
	}
}

func TestSubscriptionSupportsClashYAML(t *testing.T) {
	ts, _ := testServer(t)
	resp, err := http.Get(ts.URL + "/abc/sub/tok-alice?target=clash")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("clash subscription status = %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/yaml") {
		t.Fatalf("clash content-type = %q, want text/yaml", ct)
	}
	for _, want := range []string{"proxies:", "proxy-groups:", "rules:", `type: "vless"`, `name: "alice-Reality"`, "reality-opts:"} {
		if !strings.Contains(body, want) {
			t.Fatalf("clash subscription missing %q:\n%s", want, body)
		}
	}
}

func TestSubscriptionDetectsClashUserAgent(t *testing.T) {
	ts, _ := testServer(t)
	req, _ := http.NewRequest("GET", ts.URL+"/abc/sub/tok-alice", nil)
	req.Header.Set("User-Agent", "clash-verge/v2")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body := readBody(t, resp)
	if !strings.Contains(body, "proxy-groups:") {
		t.Fatalf("clash user-agent should receive YAML, got:\n%s", body)
	}
}

func TestSubscriptionUsageTitleUsesMBBelowGB(t *testing.T) {
	got := subscriptionUsageTitle(512*1024*1024, 900*1024*1024)
	if got != "Used 512.00 MB / Total 900.00 MB" {
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
