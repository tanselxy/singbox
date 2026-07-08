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
}

type dashboardData struct {
	Prefix   string
	ServerIP string
	Active   bool
	Clients  []clientRow
}

type nodeView struct {
	Name   string
	URL    string
	QRPath string
}

type clientDetailData struct {
	Prefix string
	Client model.Client
	Nodes  []nodeView
	SubURL string
	Used   string
	Quota  string
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
	srv, _ := state.LoadServer()
	clients, err := s.db.ListClients()
	if err != nil {
		http.Error(w, "读取客户失败", http.StatusInternalServerError)
		return
	}
	rows := make([]clientRow, 0, len(clients))
	for _, c := range clients {
		tr, _ := s.db.GetTraffic(c.ID)
		used := tr.Up + tr.Down
		rows = append(rows, clientRow{
			ID:        c.ID,
			Name:      c.Name,
			Enabled:   c.Enabled,
			Used:      humanBytes(used),
			Quota:     quotaLabel(c.QuotaBytes),
			OverQuota: c.QuotaBytes > 0 && used >= c.QuotaBytes,
		})
	}
	s.render(w, "dashboard.html", dashboardData{
		Prefix:   s.prefix,
		ServerIP: srv.ServerIP,
		Active:   singbox.IsActive(r.Context()),
		Clients:  rows,
	})
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

	nodes := make([]nodeView, 0)
	for _, l := range protocol.ClientLinks(srv, c) {
		nodes = append(nodes, nodeView{
			Name:   l.Name,
			URL:    l.URL,
			QRPath: s.prefix + "/qr?data=" + url.QueryEscape(l.URL),
		})
	}
	tr, _ := s.db.GetTraffic(c.ID)
	s.render(w, "client.html", clientDetailData{
		Prefix: s.prefix,
		Client: c,
		Nodes:  nodes,
		SubURL: fmt.Sprintf("https://%s%s/sub/%s", r.Host, s.prefix, c.SubToken),
		Used:   humanBytes(tr.Up + tr.Down),
		Quota:  quotaLabel(c.QuotaBytes),
	})
}

// ---- client CRUD ----

func (s *Server) handleClientCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "bad form"})
		return
	}
	name := strings.TrimSpace(r.PostFormValue("name"))
	if name == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "客户名不能为空"})
		return
	}
	quota := parseQuotaGB(r.PostFormValue("quota_gb"))

	c, err := secret.NewClient()
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	c.Name = name
	c.Enabled = true
	c.QuotaBytes = quota
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
	if err != nil || !c.Enabled {
		http.NotFound(w, r)
		return
	}
	srv, _ := state.LoadServer()

	var lines []string
	for _, l := range protocol.ClientLinks(srv, c) {
		lines = append(lines, l.URL)
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
	clients, err := s.db.EnabledClients()
	if err != nil {
		return err
	}
	return deploy.Apply(ctx, srv, clients)
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

func parseQuotaGB(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	gb, err := strconv.ParseFloat(s, 64)
	if err != nil || gb <= 0 {
		return 0
	}
	return int64(gb * 1024 * 1024 * 1024)
}
