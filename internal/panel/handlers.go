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
	ID            int64
	Name          string
	SubToken      string
	Enabled       bool
	Used          string
	Quota         string
	QuotaValue    string
	QuotaUnit     string
	OverQuota     bool
	Devices       string
	DeviceLimit   int
	Expiry        string
	ExpiresAtDate string
	Expired       bool
}

type dashboardData struct {
	Prefix       string
	Username     string
	ServerIP     string
	Active       bool
	Clients      []clientRow
	System       systemStatus
	Version      string
	View         string
	Summary      dashboardSummary
	SelfAgentURL string
	AccessCode   string
	Managed      bool
}

type dashboardSummary struct {
	TotalClients   int
	EnabledClients int
	LimitedClients int
	ExpiredClients int
}

type systemStatus struct {
	BBR               bool
	Fail2ban          bool
	Fail2banInstalled bool
	SSHPort           int
}

type nodeView struct {
	Name   string
	URL    string
	QRPath string
}

type clientDetailData struct {
	Prefix      string
	Client      model.Client
	Nodes       []nodeView
	SubURL      string
	ClashSubURL string
	Used        string
	Quota       string
	Devices     string
	Expiry      string
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
		"Prefix":   s.prefix,
		"Error":    r.URL.Query().Get("error"),
		"Username": s.loginUsername(),
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
	username := strings.TrimSpace(r.PostFormValue("username"))
	if username == "" {
		username = "admin" // allows password-only logins from older integrations.
	}
	if s.verifyCredentials(username, r.PostFormValue("password")) {
		s.auth.recordSuccess(ip)
		s.auth.issueSession(w, true)
		http.Redirect(w, r, s.prefix+"/dashboard", http.StatusSeeOther)
		return
	}
	s.auth.recordFailure(ip)
	http.Redirect(w, r, s.prefix+"/login?error=invalid", http.StatusSeeOther)
}

func (s *Server) loginUsername() string {
	s.accountMu.RLock()
	defer s.accountMu.RUnlock()
	return s.cfg.LoginUsername()
}

func (s *Server) verifyCredentials(username, password string) bool {
	s.accountMu.RLock()
	defer s.accountMu.RUnlock()
	return s.cfg.VerifyCredentials(username, password)
}

func (s *Server) handleAccountUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "表单格式错误"})
		return
	}
	username := strings.TrimSpace(r.PostFormValue("username"))
	currentPassword := r.PostFormValue("current_password")
	newPassword := r.PostFormValue("new_password")
	confirmPassword := r.PostFormValue("confirm_password")
	if currentPassword == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "请输入当前密码"})
		return
	}
	if newPassword == "" && confirmPassword != "" {
		writeJSON(w, map[string]any{"ok": false, "error": "请输入新密码"})
		return
	}
	if newPassword != "" && newPassword != confirmPassword {
		writeJSON(w, map[string]any{"ok": false, "error": "两次输入的新密码不一致"})
		return
	}

	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	if !s.cfg.VerifyCredentials(s.cfg.LoginUsername(), currentPassword) {
		writeJSON(w, map[string]any{"ok": false, "error": "当前密码不正确"})
		return
	}
	if newPassword == "" {
		newPassword = currentPassword
	}
	if err := s.cfg.SetCredentials(username, newPassword); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if err := s.auth.rotateKey(s.cfg.SessionKey); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	s.auth.clearSession(w)
	writeJSON(w, map[string]any{"ok": true, "relogin": true})
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
	case "overview", "clients", "monitoring", "notifications", "system", "toolbox", "logs":
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
		quotaValue, quotaUnit := quotaFormFields(c.QuotaBytes)
		rows = append(rows, clientRow{
			ID:            c.ID,
			Name:          c.Name,
			SubToken:      c.SubToken,
			Enabled:       c.Enabled,
			Used:          humanBytes(used),
			Quota:         quotaLabel(c.QuotaBytes),
			QuotaValue:    quotaValue,
			QuotaUnit:     quotaUnit,
			OverQuota:     c.QuotaBytes > 0 && used >= c.QuotaBytes,
			Devices:       deviceLabel(c.DeviceLimit),
			DeviceLimit:   c.DeviceLimit,
			Expiry:        expiryLabel(c.ExpiresAt),
			ExpiresAtDate: expiryInputDate(c.ExpiresAt),
			Expired:       c.ExpiresAt > 0 && now >= c.ExpiresAt,
		})
	}
	selfURL := selfAgentURL(srv, s.cfg.Port)
	return dashboardData{
		Prefix:   s.prefix,
		Username: s.loginUsername(),
		ServerIP: srv.ServerIP,
		Active:   singbox.IsActive(ctx),
		Clients:  rows,
		System: systemStatus{
			BBR:               system.BBREnabled(ctx),
			Fail2ban:          system.Fail2banActive(ctx),
			Fail2banInstalled: system.Fail2banInstalled(),
			SSHPort:           system.CurrentSSHPort(),
		},
		Version:      Version,
		Summary:      summary,
		SelfAgentURL: selfURL,
		AccessCode:   encodeAccessCode(selfURL, s.cfg.AgentToken),
		Managed:      s.cfg.Managed,
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
	case "fail2ban-uninstall":
		err = system.RemoveFail2ban(ctx, detectManager())
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

func (s *Server) handleFail2banBans(w http.ResponseWriter, r *http.Request) {
	page := parseIntDefault(r.URL.Query().Get("page"), 1)
	bans, err := system.ListFail2banBans(r.Context(), page, 25)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{
		"ok":        true,
		"bans":      bans.Bans,
		"page":      bans.Page,
		"page_size": bans.PageSize,
		"total":     bans.Total,
	})
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
	subBase := fmt.Sprintf("http://%s%s/sub/%s", r.Host, s.prefix, c.SubToken)
	subName := url.QueryEscape(c.Name)
	s.render(w, "client.html", clientDetailData{
		Prefix: s.prefix,
		Client: c,
		Nodes:  nodes,
		// http, not https: proxy apps reject the panel's self-signed cert, and
		// the plain-HTTP side of the listener serves only this endpoint.
		SubURL:      subBase + "#" + subName,
		ClashSubURL: subBase + "?target=clash#" + subName,
		Used:        humanBytes(tr.Up + tr.Down),
		Quota:       quotaLabel(c.QuotaBytes),
		Devices:     deviceLabel(c.DeviceLimit),
		Expiry:      expiryLabel(c.ExpiresAt),
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
	case "update":
		if err := r.ParseForm(); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": "bad form"})
			return
		}
		c, getErr := s.db.GetClient(id)
		if getErr != nil {
			writeJSON(w, map[string]any{"ok": false, "error": getErr.Error()})
			return
		}
		name := strings.TrimSpace(r.PostFormValue("name"))
		if name == "" {
			writeJSON(w, map[string]any{"ok": false, "error": "客户名不能为空"})
			return
		}
		c.Name = name
		c.QuotaBytes = parseQuota(r.PostFormValue("quota"), r.PostFormValue("quota_unit"))
		c.DeviceLimit = parseIntDefault(r.PostFormValue("device_limit"), 0)
		c.ExpiresAt = parseExpiry(r.PostFormValue("expires_at"))
		err, regen = s.db.UpdateClient(c), true
	case "reset-subscription":
		fresh, genErr := secret.NewClient()
		if genErr != nil {
			writeJSON(w, map[string]any{"ok": false, "error": genErr.Error()})
			return
		}
		fresh.ID = id
		err, regen = s.db.UpdateClientCredentials(fresh), true
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
	s.writeSubscriptionHeaders(w, c)

	if wantsClashSubscription(r) {
		w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
		_, _ = w.Write([]byte(clashSubscription(srv, nodes, c)))
		return
	}
	if r.URL.Query().Get("target") == "raw" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(body))
		return
	}
	// Default: base64 (v2rayN/Shadowrocket style).
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(base64.StdEncoding.EncodeToString([]byte(body))))
}

func wantsClashSubscription(r *http.Request) bool {
	target := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("target")))
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if target == "raw" {
		return false
	}
	if target == "clash" || target == "clashverge" || target == "mihomo" || format == "clash" || format == "yaml" || format == "yml" {
		return true
	}
	ua := strings.ToLower(r.UserAgent())
	return strings.Contains(ua, "clash") || strings.Contains(ua, "mihomo")
}

func (s *Server) writeSubscriptionHeaders(w http.ResponseWriter, c model.Client) {
	tr, _ := s.db.GetTraffic(c.ID)
	used := tr.Up + tr.Down
	display := subscriptionUsageTitle(used, c.QuotaBytes)
	w.Header().Set("Subscription-Userinfo", display)
	w.Header().Set("Profile-Title", url.QueryEscape(c.Name))
	w.Header().Set("Profile-Update-Interval", "24")
	w.Header().Set("X-Subscription-Usage", display)
}

func subscriptionUsageTitle(usedBytes, quotaBytes int64) string {
	total := "Unlimited"
	if quotaBytes > 0 {
		total = compactTrafficUnit(quotaBytes)
	}
	return fmt.Sprintf("Used %s / Total %s", compactTrafficUnit(usedBytes), total)
}

func compactTrafficUnit(n int64) string {
	const (
		mb = int64(1024 * 1024)
		gb = int64(1024 * 1024 * 1024)
	)
	if n < mb {
		return "0 MB"
	}
	if n < gb {
		return fmt.Sprintf("%.2f MB", float64(n)/float64(mb))
	}
	return fmt.Sprintf("%.2f GB", float64(n)/float64(gb))
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
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size < 1 {
		size = 50
	}
	if size > 100 {
		size = 100
	}
	if page > 20 {
		page = 20
	}

	fetchCount := page*size + 1
	out, err := system.Output(r.Context(), "journalctl", "-u", singbox.ServiceTag, "-n", strconv.Itoa(fetchCount), "-o", "short-iso", "--no-pager")
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "无法读取日志: " + err.Error()})
		return
	}
	lines := compactLines(out)
	end := len(lines) - (page-1)*size
	if end < 0 {
		end = 0
	}
	start := end - size
	if start < 0 {
		start = 0
	}
	if start > end {
		start = end
	}
	entries := make([]logEntry, 0, end-start)
	for _, line := range lines[start:end] {
		entries = append(entries, parseLogEntry(line))
	}
	writeJSON(w, map[string]any{
		"ok":       true,
		"page":     page,
		"size":     size,
		"has_more": len(lines) > page*size,
		"entries":  entries,
	})
}

type logEntry struct {
	Time    string `json:"time"`
	Source  string `json:"source"`
	Message string `json:"message"`
}

func compactLines(out string) []string {
	raw := strings.Split(strings.TrimSpace(out), "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func parseLogEntry(line string) logEntry {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return logEntry{Message: line}
	}
	msgStart := strings.Index(line, fields[2])
	if msgStart < 0 {
		msgStart = len(fields[0]) + len(fields[1]) + 2
	}
	source, msg, ok := strings.Cut(strings.TrimSpace(line[msgStart:]), ": ")
	if !ok {
		return logEntry{Time: fields[0], Source: fields[1], Message: strings.TrimSpace(line[msgStart:])}
	}
	return logEntry{Time: fields[0], Source: strings.TrimSpace(source), Message: strings.TrimSpace(msg)}
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
	return s.reconcileConfig(ctx, false)
}

// reconcileConfig re-asserts the desired config: it applies locally (a no-op
// when already applied) and pushes to every node whose last successful push
// is out of date. The traffic poller calls it every tick, which is what
// retries a failed apply or node push until the fleet converges. force
// bypasses the already-applied shortcut when the running service is provably
// stale.
func (s *Server) reconcileConfig(ctx context.Context, force bool) error {
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
	localErr := error(nil)
	if force {
		localErr = deploy.ForceApply(ctx, srv, clients)
	} else {
		localErr = deploy.Apply(ctx, srv, clients)
	}
	// Fan the client set out to out-of-date nodes even if the local apply
	// failed — nodes are independent of the local service.
	s.pushToNodes(ctx, clients)
	return localErr
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

func quotaFormFields(quotaBytes int64) (string, string) {
	if quotaBytes <= 0 {
		return "", "GB"
	}
	const mb = int64(1024 * 1024)
	const gb = int64(1024 * 1024 * 1024)
	if quotaBytes%gb == 0 {
		return strconv.FormatInt(quotaBytes/gb, 10), "GB"
	}
	return strconv.FormatFloat(float64(quotaBytes)/float64(mb), 'f', -1, 64), "MB"
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
	return time.Unix(expiresAt, 0).UTC().Format("2006-01-02 15:04")
}

func expiryInputDate(expiresAt int64) string {
	if expiresAt <= 0 {
		return ""
	}
	return time.Unix(expiresAt, 0).UTC().Format("2006-01-02T15:04")
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

// parseExpiry accepts an exact datetime-local value in UTC. It also accepts the
// former yyyy-mm-dd form so existing clients retain their end-of-day behavior.
func parseExpiry(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if t, err := time.Parse("2006-01-02T15:04", s); err == nil {
		return t.Unix()
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return 0
	}
	// Expire at the end of the selected day.
	return t.Add(24 * time.Hour).Unix()
}
