package panel

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
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

// encodeAccessCode packs a node's agent address and token into one copy-paste
// string, so registering a node needs a single copy instead of two.
func encodeAccessCode(address, token string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(address + "\n" + token))
}

func selfAgentURL(srv model.Server, port int) string {
	host := srv.ServerIP
	if srv.IPv6Only {
		host = "[" + srv.ServerIP + "]"
	}
	return fmt.Sprintf("https://%s:%d", host, port)
}

// decodeAccessCode unpacks an access code into address and token.
func decodeAccessCode(code string) (address, token string, err error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(code))
	if err != nil {
		return "", "", fmt.Errorf("接入码格式错误")
	}
	address, token, ok := strings.Cut(string(raw), "\n")
	if !ok || address == "" || token == "" {
		return "", "", fmt.Errorf("接入码内容不完整")
	}
	return address, token, nil
}

// nodePollInterval is how often the master polls remote nodes for traffic.
const nodePollInterval = 30 * time.Second

// clientSetHash fingerprints a client set for the per-node push ledger.
func clientSetHash(clients []model.Client) [32]byte {
	b, _ := json.Marshal(clients)
	return sha256.Sum256(b)
}

// pushToNodes applies the given active client set to every registered node
// whose last successful push differs from it. A failed push leaves the node's
// ledger entry stale, so the poller's reconcile pass retries it every tick
// until the node converges.
func (s *Server) pushToNodes(ctx context.Context, clients []model.Client) {
	nodes, err := s.db.ListNodes()
	if err != nil || len(nodes) == 0 {
		return
	}
	want := clientSetHash(clients)
	var wg sync.WaitGroup
	for _, n := range nodes {
		s.pushedMu.Lock()
		current := s.pushed[n.ID] == want
		s.pushedMu.Unlock()
		if current {
			continue
		}
		wg.Add(1)
		go func(n model.Node) {
			defer wg.Done()
			s.pushNode(ctx, n, clients, want, false)
		}(n)
	}
	wg.Wait()
}

// pushNode pushes one client set to one node, recording success in the ledger.
func (s *Server) pushNode(ctx context.Context, n model.Node, clients []model.Client, want [32]byte, force bool) {
	c := agent.New(n.Address, n.Token)
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := c.Apply(cctx, clients, force); err != nil {
		log.Printf("panel: 下发配置到节点 %s 失败（将自动重试）: %v", n.Name, err)
		return
	}
	s.pushedMu.Lock()
	s.pushed[n.ID] = want
	s.pushedMu.Unlock()
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
	byName := make(map[string]model.Client, len(clients))
	for _, c := range clients {
		byName[c.Name] = c
	}

	now := time.Now().Unix()
	var active []model.Client
	var want [32]byte
	for _, n := range nodes {
		c := agent.New(n.Address, n.Token)
		cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		deltas, err := c.Traffic(cctx)
		cancel()
		if err != nil {
			continue
		}
		// A node only tracks stats for the clients in its pushed config, so
		// traffic for a disabled or deleted client proves the node is still
		// running a stale config: force a fresh push to it.
		stale := false
		for name, d := range deltas {
			if d.Up == 0 && d.Down == 0 {
				continue
			}
			cl, ok := byName[name]
			if !ok || !cl.Enabled {
				stale = true
			}
			if ok {
				_ = s.db.AddTraffic(cl.ID, d.Up, d.Down, now)
			}
		}
		if stale {
			if active == nil {
				if active, err = s.db.ActiveClients(now); err != nil {
					continue
				}
				want = clientSetHash(active)
			}
			log.Printf("panel: 节点 %s 仍在服务已停用客户，强制重新下发配置", n.Name)
			s.pushNode(ctx, n, active, want, true)
		}
	}
}

// ---- node management handlers (master side) ----

type nodeRow struct {
	ID          int64
	Name        string
	Address     string
	Online      bool
	CPU         string
	CPUCores    int
	Load1       float64
	Load5       float64
	Load15      float64
	Mem         string
	Disk        string
	CPUPercent  float64
	MemUsed     uint64
	MemTotal    uint64
	MemPercent  float64
	DiskUsed    uint64
	DiskTotal   uint64
	DiskPercent float64
	NetRxBytes  uint64
	NetTxBytes  uint64
}

func (s *Server) handleNodesPage(w http.ResponseWriter, r *http.Request) {
	rows, err := s.nodeRows(r.Context())
	if err != nil {
		http.Error(w, "读取节点失败", http.StatusInternalServerError)
		return
	}

	srv, _ := state.LoadServer()
	selfURL := selfAgentURL(srv, s.cfg.Port)
	s.render(w, "nodes.html", map[string]any{
		"Prefix":       s.prefix,
		"Nodes":        rows,
		"SelfAgentURL": selfURL,
		"SelfToken":    s.cfg.AgentToken,
		"AccessCode":   encodeAccessCode(selfURL, s.cfg.AgentToken),
		"Managed":      s.cfg.Managed,
	})
}

func (s *Server) handleNodeMetricsAPI(w http.ResponseWriter, r *http.Request) {
	rows, err := s.nodeRows(r.Context())
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "读取节点失败"})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "nodes": rows})
}

func (s *Server) nodeRows(ctx context.Context) ([]nodeRow, error) {
	nodes, err := s.db.ListNodes()
	if err != nil {
		return nil, err
	}
	rows := make([]nodeRow, len(nodes))
	var wg sync.WaitGroup
	for i, n := range nodes {
		rows[i] = nodeRow{ID: n.ID, Name: n.Name, Address: n.Address}
		wg.Add(1)
		go func(i int, n model.Node) {
			defer wg.Done()
			c := agent.New(n.Address, n.Token)
			cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			m, err := c.Metrics(cctx)
			if err != nil {
				return
			}
			rows[i].Online = true
			rows[i].CPU = strconv.FormatFloat(m.CPUPercent, 'f', 1, 64) + "%"
			rows[i].CPUCores = m.CPUCores
			rows[i].Load1 = m.Load1
			rows[i].Load5 = m.Load5
			rows[i].Load15 = m.Load15
			rows[i].Mem = metrics.Format(m.MemUsed) + " / " + metrics.Format(m.MemTotal)
			rows[i].Disk = metrics.Format(m.DiskUsed) + " / " + metrics.Format(m.DiskTotal)
			rows[i].CPUPercent = m.CPUPercent
			rows[i].MemUsed = m.MemUsed
			rows[i].MemTotal = m.MemTotal
			rows[i].MemPercent = m.MemPercent
			rows[i].DiskUsed = m.DiskUsed
			rows[i].DiskTotal = m.DiskTotal
			rows[i].DiskPercent = m.DiskPercent
			rows[i].NetRxBytes = m.NetRxBytes
			rows[i].NetTxBytes = m.NetTxBytes
		}(i, n)
	}
	wg.Wait()
	return rows, nil
}

func (s *Server) handleNodeCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "bad form"})
		return
	}
	name := strings.TrimSpace(r.PostFormValue("name"))

	// Prefer a single access code; fall back to separate address/token fields.
	var address, token string
	if code := strings.TrimSpace(r.PostFormValue("code")); code != "" {
		var err error
		if address, token, err = decodeAccessCode(code); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
	} else {
		address = strings.TrimRight(strings.TrimSpace(r.PostFormValue("address")), "/")
		token = strings.TrimSpace(r.PostFormValue("token"))
	}
	if address == "" || token == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "请填写接入码"})
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
	// Auto-name from the node's IP when no name is given.
	if name == "" {
		name = srv.ServerIP
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

	// Push current clients to the new node immediately. On failure the ledger
	// stays empty for this node, so the reconcile pass retries automatically.
	clients, _ := s.db.ActiveClients(time.Now().Unix())
	pctx, pcancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer pcancel()
	if err := agent.New(node.Address, node.Token).Apply(pctx, clients, false); err != nil {
		writeJSON(w, map[string]any{"ok": true, "warn": "已添加，但首次下发失败（将自动重试）: " + err.Error()})
		return
	}
	s.pushedMu.Lock()
	s.pushed[node.ID] = clientSetHash(clients)
	s.pushedMu.Unlock()
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
	s.pushedMu.Lock()
	delete(s.pushed, id)
	s.pushedMu.Unlock()
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleNodeUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "bad id"})
		return
	}
	if err := r.ParseForm(); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "bad form"})
		return
	}
	name := strings.TrimSpace(r.PostFormValue("name"))
	if name == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "节点名不能为空"})
		return
	}
	if err := s.db.UpdateNodeName(id, name); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// namedServer is a node's cached server settings plus its display name.
type namedServer struct {
	Name   string
	Server model.Server
}

// nodeServers returns the cached server of every registered node, for
// aggregating subscription links, each labelled with the node's name.
func (s *Server) nodeServers() []namedServer {
	nodes, err := s.db.ListNodes()
	if err != nil {
		return nil
	}
	out := make([]namedServer, 0, len(nodes))
	for _, n := range nodes {
		var srv model.Server
		if json.Unmarshal([]byte(n.ServerJSON), &srv) == nil && srv.ServerIP != "" {
			out = append(out, namedServer{Name: n.Name, Server: srv})
		}
	}
	return out
}
