package panel

import (
	"encoding/json"
	"net/http"
	"net/url"

	"rsc.io/qr"

	"github.com/tanselxy/singbox/internal/protocol"
	"github.com/tanselxy/singbox/internal/singbox"
	"github.com/tanselxy/singbox/internal/state"
	"github.com/tanselxy/singbox/internal/system"
)

type nodeView struct {
	Name   string
	URL    string
	QRPath string
}

type dashboardData struct {
	Prefix   string
	ServerIP string
	Active   bool
	Nodes    []nodeView
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if s.auth.validSession(r) {
		http.Redirect(w, r, s.prefix+"/dashboard", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, s.prefix+"/login", http.StatusSeeOther)
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Prefix": s.prefix,
		"Error":  r.URL.Query().Get("error"),
	}
	s.render(w, "login.html", data)
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

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	dep, err := state.Load()
	if err != nil {
		http.Error(w, "尚未检测到部署，请先运行 install", http.StatusNotFound)
		return
	}
	nodes := make([]nodeView, 0)
	for _, l := range protocol.Links(dep) {
		nodes = append(nodes, nodeView{
			Name:   l.Name,
			URL:    l.URL,
			QRPath: s.prefix + "/qr?data=" + url.QueryEscape(l.URL),
		})
	}
	s.render(w, "dashboard.html", dashboardData{
		Prefix:   s.prefix,
		ServerIP: dep.ServerIP,
		Active:   singbox.IsActive(r.Context()),
		Nodes:    nodes,
	})
}

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
	writeJSON(w, map[string]any{
		"active": singbox.IsActive(r.Context()),
	})
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
