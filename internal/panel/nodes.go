package panel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tanselxy/singbox/internal/agent"
	"github.com/tanselxy/singbox/internal/metrics"
	"github.com/tanselxy/singbox/internal/model"
	"github.com/tanselxy/singbox/internal/state"
)

// nodePollInterval is how often the master polls remote nodes for traffic.
const nodePollInterval = 30 * time.Second

// pushToNodes applies the current active client set to every registered node.
// Best-effort: a node that is down is skipped and retried next change/poll.
func (s *Server) pushToNodes(ctx context.Context) {
	nodes, err := s.db.ListNodes()
	if err != nil || len(nodes) == 0 {
		return
	}
	clients, err := s.db.ActiveClients(time.Now().Unix())
	if err != nil {
		return
	}
	var wg sync.WaitGroup
	for _, n := range nodes {
		wg.Add(1)
		go func(n model.Node) {
			defer wg.Done()
			c := agent.New(n.Address, n.Token)
			cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			_ = c.Apply(cctx, clients)
		}(n)
	}
	wg.Wait()
}

// runNodePoller periodically collects per-user traffic from each node and
// accumulates it into the master store (keyed by client name). Quota/expiry
// enforcement is handled by the local poller, which sees the combined totals.
func (s *Server) runNodePoller(ctx context.Context, _ func(context.Context, int64) error) {
	ticker := time.NewTicker(nodePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pollNodesOnce(ctx)
		}
	}
}

func (s *Server) pollNodesOnce(ctx context.Context) {
	nodes, err := s.db.ListNodes()
	if err != nil || len(nodes) == 0 {
		return
	}
	clients, err := s.db.ListClients()
	if err != nil {
		return
	}
	idByName := make(map[string]int64, len(clients))
	for _, c := range clients {
		idByName[c.Name] = c.ID
	}

	now := time.Now().Unix()
	for _, n := range nodes {
		c := agent.New(n.Address, n.Token)
		cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		deltas, err := c.Traffic(cctx)
		cancel()
		if err != nil {
			continue
		}
		for name, d := range deltas {
			if id, ok := idByName[name]; ok && (d.Up != 0 || d.Down != 0) {
				_ = s.db.AddTraffic(id, d.Up, d.Down, now)
			}
		}
	}
}

// ---- node management handlers (master side) ----

type nodeRow struct {
	ID      int64
	Name    string
	Address string
	Online  bool
	CPU     string
	Mem     string
	Disk    string
}

func (s *Server) handleNodesPage(w http.ResponseWriter, r *http.Request) {
	nodes, err := s.db.ListNodes()
	if err != nil {
		http.Error(w, "读取节点失败", http.StatusInternalServerError)
		return
	}
	rows := make([]nodeRow, len(nodes))
	var wg sync.WaitGroup
	for i, n := range nodes {
		rows[i] = nodeRow{ID: n.ID, Name: n.Name, Address: n.Address}
		wg.Add(1)
		go func(i int, n model.Node) {
			defer wg.Done()
			c := agent.New(n.Address, n.Token)
			cctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			m, err := c.Metrics(cctx)
			if err != nil {
				return
			}
			rows[i].Online = true
			rows[i].CPU = strconv.FormatFloat(m.CPUPercent, 'f', 1, 64) + "%"
			rows[i].Mem = metrics.Format(m.MemUsed) + " / " + metrics.Format(m.MemTotal)
			rows[i].Disk = metrics.Format(m.DiskUsed) + " / " + metrics.Format(m.DiskTotal)
		}(i, n)
	}
	wg.Wait()

	srv, _ := state.LoadServer()
	host := srv.ServerIP
	if srv.IPv6Only {
		host = "[" + srv.ServerIP + "]"
	}
	s.render(w, "nodes.html", map[string]any{
		"Prefix":       s.prefix,
		"Nodes":        rows,
		"SelfAgentURL": fmt.Sprintf("https://%s:%d", host, s.cfg.Port),
		"SelfToken":    s.cfg.AgentToken,
		"Managed":      s.cfg.Managed,
	})
}

func (s *Server) handleNodeCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "bad form"})
		return
	}
	name := strings.TrimSpace(r.PostFormValue("name"))
	address := strings.TrimRight(strings.TrimSpace(r.PostFormValue("address")), "/")
	token := strings.TrimSpace(r.PostFormValue("token"))
	if name == "" || address == "" || token == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "名称/地址/令牌都不能为空"})
		return
	}

	// Validate connectivity + token by fetching the node's server settings.
	c := agent.New(address, token)
	cctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	srv, err := c.FetchServer(cctx)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "连接节点失败: " + err.Error()})
		return
	}
	serverJSON, _ := json.Marshal(srv)

	node, err := s.db.CreateNode(model.Node{
		Name: name, Address: address, Token: token,
		ServerJSON: string(serverJSON), CreatedAt: time.Now().Unix(),
	})
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "保存失败（名称可能重复）"})
		return
	}

	// Push current clients to the new node immediately.
	clients, _ := s.db.ActiveClients(time.Now().Unix())
	pctx, pcancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer pcancel()
	if err := agent.New(node.Address, node.Token).Apply(pctx, clients); err != nil {
		writeJSON(w, map[string]any{"ok": true, "warn": "已添加，但首次下发失败: " + err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleNodeDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "bad id"})
		return
	}
	if err := s.db.DeleteNode(id); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// nodeServers returns the cached model.Server of every registered node, for
// aggregating subscription links.
func (s *Server) nodeServers() []model.Server {
	nodes, err := s.db.ListNodes()
	if err != nil {
		return nil
	}
	out := make([]model.Server, 0, len(nodes))
	for _, n := range nodes {
		var srv model.Server
		if json.Unmarshal([]byte(n.ServerJSON), &srv) == nil && srv.ServerIP != "" {
			out = append(out, srv)
		}
	}
	return out
}
