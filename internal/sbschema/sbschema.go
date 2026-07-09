// Package sbschema contains the subset of the sing-box configuration schema
// this project emits. Fields use omitempty so a single Inbound type can render
// every protocol without leaking irrelevant keys.
package sbschema

import "encoding/json"

// Config is a full sing-box server configuration document.
type Config struct {
	Log          Log             `json:"log"`
	DNS          json.RawMessage `json:"dns"`
	Inbounds     []Inbound       `json:"inbounds"`
	Outbounds    json.RawMessage `json:"outbounds"`
	Route        json.RawMessage `json:"route"`
	Experimental json.RawMessage `json:"experimental,omitempty"`
}

// Log is the logging block.
type Log struct {
	Disabled  bool   `json:"disabled"`
	Level     string `json:"level"`
	Timestamp bool   `json:"timestamp"`
}

// Inbound is one listener. Only the fields relevant to a given protocol are set.
type Inbound struct {
	Type       string `json:"type"`
	Tag        string `json:"tag"`
	Listen     string `json:"listen,omitempty"`
	ListenPort int    `json:"listen_port,omitempty"`

	// ShadowTLS
	Version    int        `json:"version,omitempty"`
	Handshake  *Handshake `json:"handshake,omitempty"`
	StrictMode bool       `json:"strict_mode,omitempty"`
	Detour     string     `json:"detour,omitempty"`

	// Shadowsocks
	Network string `json:"network,omitempty"`
	Method  string `json:"method,omitempty"`

	// Users (VLESS/TUIC/Trojan/Hysteria2/ShadowTLS)
	Users []User `json:"users,omitempty"`

	// Shared single-password protocols use the inbound-level password.
	Password string `json:"password,omitempty"`

	// TUIC
	CongestionControl string `json:"congestion_control,omitempty"`

	TLS       *TLS       `json:"tls,omitempty"`
	Transport *Transport `json:"transport,omitempty"`
}

// User is an inbound user entry; unused fields are omitted.
type User struct {
	Name        string `json:"name,omitempty"`
	UUID        string `json:"uuid,omitempty"`
	Password    string `json:"password,omitempty"`
	Flow        string `json:"flow,omitempty"`
	DeviceLimit int    `json:"device_limit,omitempty"`
}

// Handshake is the upstream a Reality/ShadowTLS listener forwards probes to.
type Handshake struct {
	Server     string `json:"server"`
	ServerPort int    `json:"server_port"`
}

// TLS is a listener TLS block.
type TLS struct {
	Enabled         bool     `json:"enabled"`
	ServerName      string   `json:"server_name,omitempty"`
	ALPN            []string `json:"alpn,omitempty"`
	CertificatePath string   `json:"certificate_path,omitempty"`
	KeyPath         string   `json:"key_path,omitempty"`
	Reality         *Reality `json:"reality,omitempty"`
}

// Reality is the reality sub-block of a TLS config.
type Reality struct {
	Enabled    bool       `json:"enabled"`
	Handshake  *Handshake `json:"handshake"`
	PrivateKey string     `json:"private_key"`
	ShortID    []string   `json:"short_id"`
}

// Transport is a v2ray-style transport (ws/grpc/...).
type Transport struct {
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
}
