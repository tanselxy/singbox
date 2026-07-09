package panel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"rsc.io/qr"

	"github.com/tanselxy/singbox/internal/deploy"
	"github.com/tanselxy/singbox/internal/model"
	"github.com/tanselxy/singbox/internal/protocol"
	"github.com/tanselxy/singbox/internal/secret"
	"github.com/tanselxy/singbox/internal/singbox"
	"github.com/tanselxy/singbox/internal/state"
	"github.com/tanselxy/singbox/internal/system"
)

// ---- view models ----

type clientRow struct {
	ID        int64
	Name      string
	Enabled   bool
	Used      string
	Quota     string
	OverQuota bool
	Devices   string
	Expiry    string
	Expired   bool
}

type dashboardData struct {
	Prefix   string
	ServerIP string
	Active   bool
	Clients  []clientRow
	System   systemStatus
	Version  string
	View     string
	Summary  dashboardSummary
}

type dashboardSummary struct {
	TotalClients   int
	EnabledClients int
	LimitedClients int
	ExpiredClients int
}

type systemStatus struct {
	BBR      bool
	Fail2ban bool
	SSHPort  int
}

type nodeView struct {
	Name   string
	URL    string
	QRPath string
}

type clientDetailData struct {
	Prefix  string
	Client  model.Client
	Nodes   []nodeView
	SubURL  string
	Used    string
	Quota   string
	Devices string
	Expiry  string
}

// ---- auth pages ----

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if s.auth.validSession(r) {
		http.Redirect(w, r, s.prefix+"/dashboard", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, s.prefix+"/login", http.StatusSeeOther)
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, "login.html", map[string]any{
		"Prefix": s.prefix,
		"Error":  r.URL.Query().Get("error"),
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if s.auth.locked(ip) {
		http.Redirect(w, r, s.prefix+"/login?error=locked", http.StatusSeeOther)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if s.cfg.VerifyPassword(r.PostFormValue("password")) {
		s.auth.recordSuccess(ip)
		s.auth.issueSession(w, true)
		http.Redirect(w, r, s.prefix+"/dashboard", http.StatusSeeOther)
		return
	}
	s.auth.recordFailure(ip)
	http.Redirect(w, r, s.prefix+"/login?error=invalid", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.auth.clearSession(w)
	http.Redirect(w, r, s.prefix+"/login", http.StatusSeeOther)
}

// ---- dashboard: client list ----

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	view := r.PathValue("view")
	if view == "" {
		view = "overview"
	}
	switch view {
	case "overview", "clients", "monitoring", "system", "logs":
	default:
		http.NotFound(w, r)
		return
	}

	data, err := s.dashboardData(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data.View = view
	s.render(w, "dashboard.html", data)
}

func (s *Server) handleDashboardAPI(w http.ResponseWriter, r *http.Request) {
	data, err := s.dashboardData(r.Context())
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "dashboard": data})
}

func (s *Server) dashboardData(ctx context.Context) (dashboardData, error) {
	srv, _ := state.LoadServer()
	clients, err := s.db.ListClients()
	if err != nil {
		return dashboardData{}, fmt.Errorf("读取客户失败")
	}
	now := time.Now().Unix()
	rows := make([]clientRow, 0, len(clients))
	summary := dashboardSummary{TotalClients: len(clients)}
	for _, c := range clients {
		tr, _ := s.db.GetTraffic(c.ID)
		used := tr.Up + tr.Down
		if c.Enabled {
			summary.EnabledClients++
		}
		if c.QuotaBytes > 0 {
			summary.LimitedClients++
		}
		if c.ExpiresAt > 0 && now >= c.ExpiresAt {
			summary.ExpiredClients++
		}
		rows = append(rows, clientRow{
			ID:        c.ID,
			Name:      c.Name,
			Enabled:   c.Enabled,
			Used:      humanBytes(used),
			Quota:     quotaLabel(c.QuotaBytes),
			OverQuota: c.QuotaBytes > 0 && used >= c.QuotaBytes,
			Devices:   deviceLabel(c.DeviceLimit),
			Expiry:    expiryLabel(c.ExpiresAt),
			Expired:   c.ExpiresAt > 0 && now >= c.ExpiresAt,
		})
	}
	return dashboardData{
		Prefix:   s.prefix,
		ServerIP: srv.ServerIP,
		Active:   singbox.IsActive(ctx),
		Clients:  rows,
		System: systemStatus{
			BBR:      system.BBREnabled(ctx),
			Fail2ban: system.Fail2banActive(ctx),
			SSHPort:  system.CurrentSSHPort(),
		},
		Version: Version,
		Summary: summary,
	}, nil
}

// handleSystemAction applies a system optimization / security action.
func (s *Server) handleSystemAction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	switch r.PathValue("action") {
	case "bbr":
		err = system.OptimizeNetwork(ctx)
	case "fail2ban":
		err = system.SetupFail2ban(ctx, detectManager())
	case "ssh-port":
		port, perr := strconv.Atoi(strings.TrimSpace(r.PostFormValue("port")))
		if perr != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "端口格式错误"})
			return
		}
		err = system.ChangeSSHPort(ctx, port)
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// detectManager returns the host package manager, defaulting to apt.
func detectManager() system.PackageManager {
	if info, err := system.Detect(); err == nil {
		return info.Manager
	}
	return system.APT
}

// ---- client detail: links, QR, subscription ----

func (s *Server) handleClientDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad id", http.StatusBadRequest)
		return
	}
	c, err := s.db.GetClient(id)
	if err != nil {
		http.Error(w, "客户不存在", http.StatusNotFound)
		return
	}
	srv, _ := state.LoadServer()
	remote := s.nodeServers()

	localLabel := ""
	if len(remote) > 0 {
		localLabel = "本机"
	}

	nodes := make([]nodeView, 0)
	appendLinks := func(server model.Server, label string) {
		for _, l := range protocol.ClientLinks(server, c, label) {
			name := l.Name
			if label != "" {
				name = label + " · " + l.Name
			}
			nodes = append(nodes, nodeView{
				Name:   name,
				URL:    l.URL,
				QRPath: s.prefix + "/qr?data=" + url.QueryEscape(l.URL),
			})
		}
	}
	appendLinks(srv, localLabel)
	for _, ns := range remote {
		appendLinks(ns.Server, ns.Name)
	}
	tr, _ := s.db.GetTraffic(c.ID)
	s.render(w, "client.html", clientDetailData{
		Prefix:  s.prefix,
		Client:  c,
		Nodes:   nodes,
		SubURL:  fmt.Sprintf("https://%s%s/sub/%s", r.Host, s.prefix, c.SubToken),
		Used:    humanBytes(tr.Up + tr.Down),
		Quota:   quotaLabel(c.QuotaBytes),
		Devices: deviceLabel(c.DeviceLimit),
		Expiry:  expiryLabel(c.ExpiresAt),
	})
}

// ---- client CRUD ----

func (s *Server) handleClientCreate(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Managed {
		writeJSON(w, map[string]any{"ok": false, "error": "本机为受控节点，请在主控面板管理客户"})
		return
	}
	if err := r.ParseForm(); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "bad form"})
		return
	}
	name := strings.TrimSpace(r.PostFormValue("name"))
	if name == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "客户名不能为空"})
		return
	}

	c, err := secret.NewClient()
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	c.Name = name
	c.Enabled = true
	c.QuotaBytes = parseQuota(r.PostFormValue("quota"), r.PostFormValue("quota_unit"))
	c.DeviceLimit = parseIntDefault(r.PostFormValue("device_limit"), 0)
	c.ExpiresAt = parseExpiry(r.PostFormValue("expires_at"))
	c.CreatedAt = time.Now().Unix()

	if _, err := s.db.CreateClient(c); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "创建失败（客户名可能重复）"})
		return
	}
	if err := s.applyConfig(r.Context()); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "已创建但配置应用失败: " + err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleClientAction(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Managed {
		writeJSON(w, map[string]any{"ok": false, "error": "本机为受控节点，请在主控面板管理客户"})
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "bad id"})
		return
	}
	action := r.PathValue("action")

	var regen bool
	switch action {
	case "enable":
		err, regen = s.db.SetEnabled(id, true), true
	case "disable":
		err, regen = s.db.SetEnabled(id, false), true
	case "delete":
		err, regen = s.db.DeleteClient(id), true
	case "reset-traffic":
		err = s.db.ResetTraffic(id)
	default:
		writeJSON(w, map[string]any{"ok": false, "error": "unknown action"})
		return
	}
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if regen {
		if err := s.applyConfig(r.Context()); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "配置应用失败: " + err.Error()})
			return
		}
	}
	writeJSON(w, map[string]any{"ok": true})
}

// ---- subscription (token auth, no login) ----

func (s *Server) handleSubscription(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	c, err := s.db.GetClientByToken(token)
	if err != nil || !c.Active(time.Now().Unix()) {
		http.NotFound(w, r)
		return
	}
	srv, _ := state.LoadServer()
	nodes := s.nodeServers()

	// With remote nodes present, label each server so client apps show them as
	// distinct entries instead of deduping identical names.
	localLabel := ""
	if len(nodes) > 0 {
		localLabel = "本机"
	}

	var lines []string
	for _, l := range protocol.ClientLinks(srv, c, localLabel) {
		lines = append(lines, l.URL)
	}
	for _, ns := range nodes {
		for _, l := range protocol.ClientLinks(ns.Server, c, ns.Name) {
			lines = append(lines, l.URL)
		}
	}
	body := strings.Join(lines, "\n")

	if r.URL.Query().Get("target") == "raw" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(body))
		return
	}
	// Default: base64 (v2rayN/Shadowrocket style).
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(base64.StdEncoding.EncodeToString([]byte(body))))
}

// ---- misc ----

func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	s.render(w, "tools.html", map[string]any{"Prefix": s.prefix})
}

func (s *Server) handleQR(w http.ResponseWriter, r *http.Request) {
	data := r.URL.Query().Get("data")
	if data == "" {
		http.Error(w, "missing data", http.StatusBadRequest)
		return
	}
	code, err := qr.Encode(data, qr.M)
	if err != nil {
		http.Error(w, "qr encode failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(code.PNG())
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"active": singbox.IsActive(r.Context())})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	out, err := system.Output(r.Context(), "journalctl", "-u", singbox.ServiceTag, "-n", "100", "--no-pager")
	if err != nil {
		out = "无法读取日志: " + err.Error()
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(out))
}

func (s *Server) handleServiceAction(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")
	var err error
	switch action {
	case "restart":
		err = system.Run(r.Context(), "systemctl", "restart", singbox.ServiceTag)
	case "stop":
		err = system.Run(r.Context(), "systemctl", "stop", singbox.ServiceTag)
	case "start":
		err = system.Run(r.Context(), "systemctl", "start", singbox.ServiceTag)
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "active": singbox.IsActive(r.Context())})
}

// applyConfig regenerates the sing-box config from enabled clients and restarts
// the service, serialized against concurrent edits.
func (s *Server) applyConfig(ctx context.Context) error {
	s.applyMu.Lock()
	defer s.applyMu.Unlock()

	srv, err := state.LoadServer()
	if err != nil {
		return fmt.Errorf("读取服务器配置: %w", err)
	}
	clients, err := s.db.ActiveClients(time.Now().Unix())
	if err != nil {
		return err
	}
	if err := deploy.Apply(ctx, srv, clients); err != nil {
		return err
	}
	// Fan the same client set out to all registered nodes (best-effort).
	s.pushToNodes(ctx)
	return nil
}

// ---- helpers ----

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func quotaLabel(quotaBytes int64) string {
	if quotaBytes <= 0 {
		return "不限"
	}
	return humanBytes(quotaBytes)
}

func deviceLabel(n int) string {
	if n <= 0 {
		return "不限"
	}
	return strconv.Itoa(n)
}

func expiryLabel(expiresAt int64) string {
	if expiresAt <= 0 {
		return "永久"
	}
	return time.Unix(expiresAt, 0).UTC().Format("2006-01-02")
}

// parseQuota converts a value + unit ("MB" or "GB") into bytes. 0 = unlimited.
func parseQuota(value, unit string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	n, err := strconv.ParseFloat(value, 64)
	if err != nil || n <= 0 {
		return 0
	}
	mult := float64(1024 * 1024 * 1024) // default GB
	if strings.EqualFold(strings.TrimSpace(unit), "MB") {
		mult = 1024 * 1024
	}
	return int64(n * mult)
}

func parseIntDefault(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return def
	}
	return n
}

// parseExpiry parses a yyyy-mm-dd date (client's local calendar day) into a unix
// timestamp at end of that day (UTC). Empty means never expire (0).
func parseExpiry(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return 0
	}
	// Expire at the end of the selected day.
	return t.Add(24 * time.Hour).Unix()
}
