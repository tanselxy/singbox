// Package config assembles a complete sing-box configuration document from a
// deployment: the static DNS/route/outbounds blocks plus the generated inbounds.
package config

import (
	"encoding/json"
	"fmt"

	"github.com/tanselxy/singbox/internal/model"
	"github.com/tanselxy/singbox/internal/protocol"
	"github.com/tanselxy/singbox/internal/sbschema"
)

// dnsBlock, outboundsBlock and routeBlock are the static portions of the
// configuration, mirroring legacy/server_template.json. They are validated as
// JSON in TestStaticBlocksAreValidJSON.
const dnsBlock = `{
  "servers": [
    { "type": "https", "tag": "local", "server": "1.1.1.1", "server_port": 443, "detour": "direct" },
    { "type": "udp", "tag": "local-temp", "server": "2001:4860:4860::8888", "detour": "direct" },
    { "type": "local", "tag": "system" }
  ],
  "rules": [
    { "rule_set": ["cn"], "server": "local" },
    { "rule_set": ["category-ads-all"], "action": "predefined", "rcode": "NXDOMAIN" }
  ],
  "final": "local",
  "strategy": "prefer_ipv4"
}`

const outboundsBlock = `[
  { "type": "direct", "tag": "direct", "domain_resolver": "local" }
]`

// Local API endpoints, bound to loopback so only the on-host traffic poller can
// reach them.
const (
	ClashAPIAddr   = "127.0.0.1:9090"
	ClashAPISecret = "singbox-panel-local"

	// V2RayAPIAddr is the gRPC stats endpoint used for per-user traffic. It
	// requires a sing-box built with the with_v2ray_api tag.
	V2RayAPIAddr = "127.0.0.1:8080"
)

const routeBlock = `{
  "default_domain_resolver": "local",
  "rules": [
    { "rule_set": ["private", "cn"], "outbound": "direct" },
    { "rule_set": ["category-ads-all"], "action": "reject" }
  ],
  "rule_set": [
    { "tag": "cn", "type": "remote", "format": "binary", "url": "https://cdn.jsdelivr.net/gh/SagerNet/sing-geoip@rule-set/geoip-cn.srs", "download_detour": "direct" },
    { "tag": "private", "type": "remote", "format": "binary", "url": "https://cdn.jsdelivr.net/gh/SagerNet/sing-geosite@rule-set/geosite-private.srs", "download_detour": "direct" },
    { "tag": "category-ads-all", "type": "remote", "format": "binary", "url": "https://cdn.jsdelivr.net/gh/SagerNet/sing-geosite@rule-set/geosite-category-ads.srs", "download_detour": "direct" }
  ]
}`

// Build returns the full sing-box configuration for a server and its clients.
func Build(srv model.Server, clients []model.Client) sbschema.Config {
	return sbschema.Config{
		Log:          sbschema.Log{Level: "info", Timestamp: true},
		DNS:          json.RawMessage(dnsBlock),
		Inbounds:     protocol.Inbounds(srv, clients),
		Outbounds:    json.RawMessage(outboundsBlock),
		Route:        json.RawMessage(routeBlock),
		Experimental: experimental(clients),
	}
}

// experimental builds the experimental block: the clash_api plus a v2ray_api
// stats service listing every client's user name so per-user traffic counters
// are tracked.
func experimental(clients []model.Client) json.RawMessage {
	users := make([]string, 0, len(clients))
	for _, c := range clients {
		users = append(users, c.Name)
	}
	block := map[string]any{
		"clash_api": map[string]any{
			"external_controller": ClashAPIAddr,
			"secret":              ClashAPISecret,
		},
		"v2ray_api": map[string]any{
			"listen": V2RayAPIAddr,
			"stats": map[string]any{
				"enabled": true,
				"users":   users,
			},
		},
	}
	b, _ := json.Marshal(block)
	return b
}

// Marshal renders a configuration to indented config.json bytes.
func Marshal(srv model.Server, clients []model.Client) ([]byte, error) {
	b, err := json.MarshalIndent(Build(srv, clients), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal sing-box config: %w", err)
	}
	return b, nil
}
