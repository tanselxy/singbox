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

// Build returns the full sing-box configuration for a deployment.
func Build(d model.Deployment) sbschema.Config {
	return sbschema.Config{
		Log:       sbschema.Log{Level: "info", Timestamp: true},
		DNS:       json.RawMessage(dnsBlock),
		Inbounds:  protocol.Inbounds(d),
		Outbounds: json.RawMessage(outboundsBlock),
		Route:     json.RawMessage(routeBlock),
	}
}

// Marshal renders a deployment to indented config.json bytes.
func Marshal(d model.Deployment) ([]byte, error) {
	b, err := json.MarshalIndent(Build(d), "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal sing-box config: %w", err)
	}
	return b, nil
}
