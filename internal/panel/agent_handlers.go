package panel

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"

	"github.com/tanselxy/singbox/internal/agent"
	"github.com/tanselxy/singbox/internal/deploy"
	"github.com/tanselxy/singbox/internal/metrics"
	"github.com/tanselxy/singbox/internal/state"
	"github.com/tanselxy/singbox/internal/traffic"
)

// agentAuth wraps a handler with bearer-token authentication for the agent API.
func (s *Server) agentAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		const prefix = "Bearer "
		auth := r.Header.Get("Authorization")
		if len(auth) <= len(prefix) || subtle.ConstantTimeCompare(
			[]byte(auth[len(prefix):]), []byte(s.cfg.AgentToken)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h(w, r)
	}
}

// handleAgentServer returns this node's server settings for link generation.
func (s *Server) handleAgentServer(w http.ResponseWriter, r *http.Request) {
	srv, err := state.LoadServer()
	if err != nil {
		http.Error(w, "no deployment", http.StatusNotFound)
		return
	}
	writeJSON(w, srv)
}

// handleAgentApply receives the desired client set from the master, marks this
// panel as managed, and regenerates the local sing-box config.
func (s *Server) handleAgentApply(w http.ResponseWriter, r *http.Request) {
	var req agent.ApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	srv, err := state.LoadServer()
	if err != nil {
		http.Error(w, "no deployment", http.StatusNotFound)
		return
	}

	s.applyMu.Lock()
	defer s.applyMu.Unlock()

	// Becoming managed: stop the local traffic poller so it does not compete
	// with the master for the v2ray_api counters.
	if !s.cfg.Managed {
		if err := s.cfg.SetManaged(true); err != nil {
			http.Error(w, "persist managed flag", http.StatusInternalServerError)
			return
		}
		if s.pollerCancel != nil {
			s.pollerCancel()
			s.pollerCancel = nil
		}
	}

	applyFn := deploy.Apply
	if req.Force {
		applyFn = deploy.ForceApply
	}
	if err := applyFn(r.Context(), srv, req.Clients); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// handleAgentTraffic returns per-user traffic deltas since the last call.
func (s *Server) handleAgentTraffic(w http.ResponseWriter, r *http.Request) {
	deltas, err := traffic.QueryDeltas(r.Context())
	if err != nil {
		writeJSON(w, map[string]agent.TrafficDelta{}) // node up but no traffic yet
		return
	}
	out := make(map[string]agent.TrafficDelta, len(deltas))
	for name, d := range deltas {
		out[name] = agent.TrafficDelta{Up: d.Up(), Down: d.Down()}
	}
	writeJSON(w, out)
}

// handleAgentMetrics returns this node's system metrics.
func (s *Server) handleAgentMetrics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, metrics.Collect())
}
