package panel

import (
	"context"
	"crypto/tls"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"time"

	"github.com/tanselxy/singbox/web"
)

// Server is the resident HTTPS control panel.
type Server struct {
	cfg    Config
	auth   *auth
	tmpl   *template.Template
	prefix string // "/<path-prefix>"
}

// New builds a panel server from its bootstrap config.
func New(cfg Config) (*Server, error) {
	a, err := newAuth(cfg.SessionKey)
	if err != nil {
		return nil, err
	}
	tmpl, err := template.ParseFS(web.FS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	return &Server{cfg: cfg, auth: a, tmpl: tmpl, prefix: "/" + cfg.PathPrefix}, nil
}

// Handler returns the routed, security-wrapped HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	p := s.prefix

	mux.HandleFunc("GET "+p+"/", s.handleRoot)
	mux.HandleFunc("GET "+p+"/login", s.handleLoginPage)
	mux.HandleFunc("POST "+p+"/login", s.handleLogin)
	mux.HandleFunc("POST "+p+"/logout", s.handleLogout)

	mux.HandleFunc("GET "+p+"/dashboard", s.protected(s.handleDashboard))
	mux.HandleFunc("GET "+p+"/tools", s.protected(s.handleTools))
	mux.HandleFunc("GET "+p+"/qr", s.protected(s.handleQR))
	mux.HandleFunc("GET "+p+"/api/status", s.protected(s.handleStatus))
	mux.HandleFunc("GET "+p+"/api/logs", s.protected(s.handleLogs))
	mux.HandleFunc("POST "+p+"/api/service/{action}", s.protected(s.handleServiceAction))

	// Embedded static assets, served under <prefix>/static/.
	mux.Handle("GET "+p+"/static/", http.StripPrefix(p+"/", http.FileServer(http.FS(web.FS))))

	return securityHeaders(mux)
}

// ListenAndServe starts the HTTPS panel and blocks until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", s.cfg.Port),
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS12},
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServeTLS(s.cfg.CertFile, s.cfg.KeyFile)
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

// protected wraps a handler, redirecting unauthenticated requests to /login.
func (s *Server) protected(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.auth.validSession(r) {
			http.Redirect(w, r, s.prefix+"/login", http.StatusSeeOther)
			return
		}
		h(w, r)
	}
}

// securityHeaders adds conservative headers and hides the server identity.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'")
		h.Set("Server", "")
		next.ServeHTTP(w, r)
	})
}

// clientIP extracts the remote host for lockout accounting.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
