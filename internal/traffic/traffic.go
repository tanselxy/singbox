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
	deltas, err := query(ctx)
	if err != nil {
		return err
	}
	if len(deltas) == 0 {
		return nil
	}

	clients, err := p.db.ListClients()
	if err != nil {
		return err
	}
	byName := make(map[string]int64, len(clients))
	for _, c := range clients {
		byName[c.Name] = c.ID
	}

	now := time.Now().Unix()
	for name, d := range deltas {
		id, ok := byName[name]
		if !ok {
			continue
		}
		if d.up != 0 || d.down != 0 {
			_ = p.db.AddTraffic(id, d.up, d.down, now)
		}
	}

	p.enforceQuotas(ctx, clients)
	return nil
}

// enforceQuotas disables any enabled client whose cumulative usage has reached
// its quota.
func (p *Poller) enforceQuotas(ctx context.Context, clients []model.Client) {
	for _, c := range clients {
		if !c.Enabled || c.QuotaBytes <= 0 {
			continue
		}
		tr, err := p.db.GetTraffic(c.ID)
		if err != nil {
			continue
		}
		if tr.Up+tr.Down >= c.QuotaBytes && p.enforce != nil {
			_ = p.enforce(ctx, c.ID)
		}
	}
}

type delta struct{ up, down int64 }

// query fetches and resets all user counters, returning per-user deltas.
func query(ctx context.Context) (map[string]delta, error) {
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

	out := map[string]delta{}
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
