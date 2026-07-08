// Package model holds the pure data types describing a sing-box deployment.
//
// A single Deployment value is the authoritative source for both the server
// configuration (sing-box inbounds) and the client-facing share links. The old
// bash implementation stored ports in one place and hard-coded them again in
// the link strings and Clash template, which drifted apart in NAT mode
// (Trojan/TUIC/SS links pointed at 63333/61555/59000 while the server listened
// on random ports). Deriving everything from this one struct removes that
// entire class of bug.
package model

// Kind identifies a proxy protocol offered by the deployment.
type Kind string

const (
	KindReality   Kind = "reality"   // VLESS + Reality (xtls-rprx-vision)
	KindHysteria2 Kind = "hysteria2" // Hysteria2
	KindShadowTLS Kind = "shadowtls" // ShadowTLS v3 fronting Shadowsocks-2022
	KindSSDirect  Kind = "ss-direct" // plain Shadowsocks (aes-128-gcm)
	KindTUIC      Kind = "tuic"      // TUIC v5
	KindTrojanWS  Kind = "trojan-ws" // Trojan over WebSocket
	KindVLESSCDN  Kind = "vless-cdn" // VLESS over WebSocket behind a CDN
)

// Ports holds the listening port for every protocol. In normal mode these come
// from the fixed pool; in NAT mode they are randomly assigned once and stored
// here, then read back by both the config generator and the link generator.
type Ports struct {
	Reality   int
	Hysteria2 int
	ShadowTLS int // public ShadowTLS port (SS_PORT in the old code)
	SSDirect  int
	TUIC      int
	TrojanWS  int
	VLESSCDN  int
}

// Reality is a per-install X25519 key pair plus short id. Unlike the old code,
// which shipped one hard-coded key pair for every user, these are generated
// fresh on each install via `sing-box generate reality-keypair`.
type Reality struct {
	PrivateKey string
	PublicKey  string
	ShortID    string
}

// Credentials bundles every secret used across the inbounds.
type Credentials struct {
	UUID string // VLESS / TUIC

	// HysteriaPassword is shared by Hysteria2, Trojan and the plain SS-direct
	// inbound, mirroring the old behaviour.
	HysteriaPassword string

	// SSPassword is the Shadowsocks-2022 key behind ShadowTLS.
	SSPassword string

	// ShadowTLSPassword is the ShadowTLS handshake password. Generated per
	// install (was hard-coded before).
	ShadowTLSPassword string

	Reality Reality
}

// Deployment is the authoritative description of one installed instance.
type Deployment struct {
	// ServerIP is the address embedded in client links: a public IPv4, a
	// bracketed IPv6, or an optimised domain.
	ServerIP string

	// SNI is the masquerade / handshake domain (the old SERVER variable), used
	// by Reality and ShadowTLS handshakes.
	SNI string

	// CDNDomain is the real domain used for the VLESS-CDN inbound (old
	// DOMAIN_NAME). Empty when no CDN/IPv6 domain is configured.
	CDNDomain string

	CertFile string
	KeyFile  string

	Creds Credentials
	Ports Ports

	// IPv6Only marks a deployment reachable only over IPv6 (WARP scenario),
	// where only the VLESS-CDN node is exposed.
	IPv6Only bool
}

// Link is a single client-facing share link.
type Link struct {
	Kind Kind
	Name string // human label, e.g. "Reality"
	URL  string // full share URL (vless://, hysteria2://, ...)
}
