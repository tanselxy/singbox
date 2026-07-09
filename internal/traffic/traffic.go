// Package traffic polls sing-box's v2ray_api gRPC stats service for per-user
// upload/download counters and accumulates them into the store. Querying with
// reset=true makes each poll return the delta since the previous poll, so the
// store holds a running cumulative total per client.
package traffic

import (
	"context"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/tanselxy/singbox/internal/config"
	"github.com/tanselxy/singbox/internal/model"
	"github.com/tanselxy/singbox/internal/store"
	"github.com/tanselxy/singbox/internal/v2rayapi"
)

// QuotaEnforcer is called when a client's cumulative traffic reaches its quota,
// so the caller can disable the client and regenerate the config.
type QuotaEnforcer func(ctx context.Context, clientID int64) error

// Poller periodically collects per-user traffic.
type Poller struct {
	db       *store.Store
	interval time.Duration
	enforce  QuotaEnforcer
}

// NewPoller creates a traffic poller.
func NewPoller(db *store.Store, interval time.Duration, enforce QuotaEnforcer) *Poller {
	return &Poller{db: db, interval: interval, enforce: enforce}
}

// Run polls until ctx is cancelled. It dials lazily each tick so it tolerates
// sing-box restarts (config reloads) without needing to reconnect explicitly.
func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.pollOnce(ctx); err != nil {
				// Transient (sing-box restarting, API not up yet): skip this tick.
				continue
			}
		}
	}
}

func (p *Poller) pollOnce(ctx context.Context) error {
	clients, err := p.db.ListClients()
	if err != nil {
		return err
	}

	// Update per-user traffic from v2ray_api (best-effort: if sing-box is
	// restarting or the API is briefly unavailable, we still run enforcement
	// below so expiry is applied even without traffic).
	if deltas, err := QueryDeltas(ctx); err == nil {
		byName := make(map[string]int64, len(clients))
		for _, c := range clients {
			byName[c.Name] = c.ID
		}
		now := time.Now().Unix()
		for name, d := range deltas {
			if id, ok := byName[name]; ok && (d.up != 0 || d.down != 0) {
				_ = p.db.AddTraffic(id, d.up, d.down, now)
			}
		}
	}

	p.enforce_(ctx, clients)
	return nil
}

// enforce_ disables any enabled client that has reached its quota or passed its
// expiry, triggering a config regeneration via the enforce callback.
func (p *Poller) enforce_(ctx context.Context, clients []model.Client) {
	now := time.Now().Unix()
	for _, c := range clients {
		if !c.Enabled || p.enforce == nil {
			continue
		}
		overQuota := c.QuotaBytes > 0 && p.usage(c.ID) >= c.QuotaBytes
		expired := c.ExpiresAt > 0 && now >= c.ExpiresAt
		if overQuota || expired {
			_ = p.enforce(ctx, c.ID)
		}
	}
}

// usage returns a client's cumulative traffic (up+down).
func (p *Poller) usage(clientID int64) int64 {
	tr, err := p.db.GetTraffic(clientID)
	if err != nil {
		return 0
	}
	return tr.Up + tr.Down
}

// Delta is a per-user up/down traffic delta since the last query.
type Delta struct{ up, down int64 }

// Up returns the upload delta in bytes.
func (d Delta) Up() int64 { return d.up }

// Down returns the download delta in bytes.
func (d Delta) Down() int64 { return d.down }

// QueryDeltas fetches and resets all per-user counters from sing-box's v2ray_api,
// returning per-user deltas since the previous call.
func QueryDeltas(ctx context.Context) (map[string]Delta, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, config.V2RayAPIAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := v2rayapi.NewStatsServiceClient(conn)
	callCtx, cancel2 := context.WithTimeout(ctx, 3*time.Second)
	defer cancel2()

	resp, err := client.QueryStats(callCtx, &v2rayapi.QueryStatsRequest{
		Patterns: []string{"user>>>"},
		Reset_:   true,
	})
	if err != nil {
		return nil, err
	}

	out := map[string]Delta{}
	for _, s := range resp.GetStat() {
		name, dir, ok := parseUserStat(s.GetName())
		if !ok {
			continue
		}
		d := out[name]
		switch dir {
		case "uplink":
			d.up += s.GetValue()
		case "downlink":
			d.down += s.GetValue()
		}
		out[name] = d
	}
	return out, nil
}

// parseUserStat parses "user>>>NAME>>>traffic>>>uplink|downlink".
func parseUserStat(name string) (user, direction string, ok bool) {
	parts := strings.Split(name, ">>>")
	if len(parts) != 4 || parts[0] != "user" || parts[2] != "traffic" {
		return "", "", false
	}
	return parts[1], parts[3], true
}
