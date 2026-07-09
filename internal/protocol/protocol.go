// Package protocol turns a server config plus a set of clients into sing-box
// inbounds and the matching per-client share links. Inbounds and links are
// generated from the same data so their ports and secrets can never drift.
//
// Multi-user notes (verified against sing-box 1.13.3):
//   - VLESS/TUIC/Trojan/Hysteria2/ShadowTLS take a users array directly.
//   - Shadowsocks-2022 multi-user uses a server-level PSK plus a per-user PSK;
//     the client key is "serverPSK:userPSK".
//   - Plain Shadowsocks (aes-128-gcm) cannot be multi-user and is dropped.
package protocol

import (
	"encoding/base64"
	"fmt"
	"net"

	"github.com/tanselxy/singbox/internal/model"
	"github.com/tanselxy/singbox/internal/sbschema"
)

const (
	// selfSignedSNI is the CN of the generated self-signed certificate.
	selfSignedSNI = "bing.com"

	// ssTLSMethod is the Shadowsocks-2022 method behind ShadowTLS. Multi-user
	// (EIH) support requires an AES method — chacha20-poly1305 does NOT support
	// multiple users. aes-256-gcm takes a 32-byte key, matching secret.Base64Key(32).
	ssTLSMethod = "2022-blake3-aes-256-gcm"
	ssTLSDetour = "ss-in"
	ssTLSSSPort = 10808 // loopback port the ShadowTLS detour forwards to

	wsPathVLESS  = "/vless"
	wsPathTrojan = "/trojan"

	cdnPort = 4433 // VLESS-CDN listens on a fixed port
)

// Inbounds builds the sing-box inbounds for a server and its clients. Each
// client becomes a user entry in every inbound. IPv6-only deployments expose
// only the VLESS-CDN listener.
//
// With no clients there are no inbounds at all: sing-box rejects a config
// whose inbounds have empty users ("missing users"), and an empty inbound
// list keeps the service validly running with every port closed.
func Inbounds(srv model.Server, clients []model.Client) []sbschema.Inbound {
	if len(clients) == 0 {
		return []sbschema.Inbound{}
	}
	if srv.IPv6Only {
		return []sbschema.Inbound{vlessCDNInbound(srv, clients)}
	}
	return []sbschema.Inbound{
		shadowTLSInbound(srv, clients),
		ssBehindShadowTLSInbound(srv, clients),
		tuicInbound(srv, clients),
		realityInbound(srv, clients),
		vlessCDNInbound(srv, clients),
		trojanInbound(srv, clients),
		hysteria2Inbound(srv, clients),
	}
}

// ClientLinks builds every share link for a single client. label, when set,
// replaces the client name in the link fragment so links from different nodes
// are distinguishable in client apps (e.g. "荷兰-Reality" vs "本机-Reality");
// only the display name changes, never the credentials.
func ClientLinks(srv model.Server, c model.Client, label string) []model.Link {
	if label != "" {
		c.Name = label
	}
	if srv.IPv6Only {
		if srv.CDNDomain == "" {
			return nil
		}
		return []model.Link{cdnLink(srv, c)}
	}
	links := []model.Link{
		realityLink(srv, c),
		hysteria2Link(srv, c),
		trojanLink(srv, c),
		tuicLink(srv, c),
		shadowTLSLink(srv, c),
	}
	if srv.CDNDomain != "" {
		links = append(links, cdnLink(srv, c))
	}
	return links
}

// ---------------------------------------------------------------------------
// Inbounds
// ---------------------------------------------------------------------------

func shadowTLSInbound(srv model.Server, clients []model.Client) sbschema.Inbound {
	users := make([]sbschema.User, 0, len(clients))
	for _, c := range clients {
		users = append(users, sbschema.User{Name: c.Name, Password: c.ShadowTLSPassword})
	}
	return sbschema.Inbound{
		Type:       "shadowtls",
		Tag:        "st-in",
		Listen:     "::",
		ListenPort: srv.Ports.ShadowTLS,
		Version:    3,
		Users:      users,
		Handshake:  &sbschema.Handshake{Server: srv.SNI, ServerPort: 443},
		StrictMode: true,
		Detour:     ssTLSDetour,
	}
}

func ssBehindShadowTLSInbound(srv model.Server, clients []model.Client) sbschema.Inbound {
	users := make([]sbschema.User, 0, len(clients))
	for _, c := range clients {
		users = append(users, sbschema.User{Name: c.Name, Password: c.SS2022Key})
	}
	return sbschema.Inbound{
		Type:       "shadowsocks",
		Tag:        ssTLSDetour,
		Listen:     "127.0.0.1",
		ListenPort: ssTLSSSPort,
		Network:    "tcp",
		Method:     ssTLSMethod,
		Password:   srv.SS2022ServerKey,
		Users:      users,
	}
}

func tuicInbound(srv model.Server, clients []model.Client) sbschema.Inbound {
	users := make([]sbschema.User, 0, len(clients))
	for _, c := range clients {
		users = append(users, sbschema.User{Name: c.Name, UUID: c.UUID})
	}
	return sbschema.Inbound{
		Type:              "tuic",
		Tag:               "tuic-in",
		Listen:            "::",
		ListenPort:        srv.Ports.TUIC,
		Users:             users,
		CongestionControl: "bbr",
		TLS: &sbschema.TLS{
			Enabled:         true,
			ServerName:      selfSignedSNI,
			ALPN:            []string{"h3"},
			CertificatePath: srv.CertFile,
			KeyPath:         srv.KeyFile,
		},
	}
}

func realityInbound(srv model.Server, clients []model.Client) sbschema.Inbound {
	users := make([]sbschema.User, 0, len(clients))
	for _, c := range clients {
		users = append(users, sbschema.User{Name: c.Name, UUID: c.UUID, Flow: "xtls-rprx-vision"})
	}
	return sbschema.Inbound{
		Type:       "vless",
		Tag:        "vless-in",
		Listen:     "::",
		ListenPort: srv.Ports.Reality,
		Users:      users,
		TLS: &sbschema.TLS{
			Enabled:    true,
			ServerName: srv.SNI,
			Reality: &sbschema.Reality{
				Enabled:    true,
				Handshake:  &sbschema.Handshake{Server: srv.SNI, ServerPort: 443},
				PrivateKey: srv.Reality.PrivateKey,
				ShortID:    []string{srv.Reality.ShortID},
			},
		},
	}
}

func vlessCDNInbound(srv model.Server, clients []model.Client) sbschema.Inbound {
	users := make([]sbschema.User, 0, len(clients))
	for _, c := range clients {
		users = append(users, sbschema.User{Name: c.Name, UUID: c.UUID})
	}
	return sbschema.Inbound{
		Type:       "vless",
		Tag:        "vless-cdn",
		Listen:     "::",
		ListenPort: cdnPort,
		Users:      users,
		Transport:  &sbschema.Transport{Type: "ws", Path: wsPathVLESS},
		TLS: &sbschema.TLS{
			Enabled:         true,
			ServerName:      srv.CDNDomain,
			CertificatePath: srv.CertFile,
			KeyPath:         srv.KeyFile,
		},
	}
}

func trojanInbound(srv model.Server, clients []model.Client) sbschema.Inbound {
	users := make([]sbschema.User, 0, len(clients))
	for _, c := range clients {
		users = append(users, sbschema.User{Name: c.Name, Password: c.Password})
	}
	return sbschema.Inbound{
		Type:       "trojan",
		Tag:        "trojan-in",
		Listen:     "::",
		ListenPort: srv.Ports.TrojanWS,
		Users:      users,
		TLS: &sbschema.TLS{
			Enabled:         true,
			ServerName:      selfSignedSNI,
			CertificatePath: srv.CertFile,
			KeyPath:         srv.KeyFile,
		},
		Transport: &sbschema.Transport{Type: "ws", Path: wsPathTrojan},
	}
}

func hysteria2Inbound(srv model.Server, clients []model.Client) sbschema.Inbound {
	users := make([]sbschema.User, 0, len(clients))
	for _, c := range clients {
		users = append(users, sbschema.User{Name: c.Name, Password: c.Password})
	}
	return sbschema.Inbound{
		Type:       "hysteria2",
		Tag:        "hy2-in",
		Listen:     "::",
		ListenPort: srv.Ports.Hysteria2,
		Users:      users,
		TLS: &sbschema.TLS{
			Enabled:         true,
			ALPN:            []string{"h3"},
			CertificatePath: srv.CertFile,
			KeyPath:         srv.KeyFile,
		},
	}
}

// ---------------------------------------------------------------------------
// Links (per client)
// ---------------------------------------------------------------------------

func realityLink(srv model.Server, c model.Client) model.Link {
	url := fmt.Sprintf(
		"vless://%s@%s:%d?security=reality&flow=xtls-rprx-vision&type=tcp&sni=%s&fp=chrome&pbk=%s&sid=%s&encryption=none#%s-Reality",
		c.UUID, hostForLink(srv.ServerIP), srv.Ports.Reality, srv.SNI,
		srv.Reality.PublicKey, srv.Reality.ShortID, c.Name,
	)
	return model.Link{Kind: model.KindReality, Name: "Reality", URL: url}
}

func hysteria2Link(srv model.Server, c model.Client) model.Link {
	url := fmt.Sprintf(
		"hysteria2://%s@%s:%d?insecure=1&alpn=h3&sni=%s#%s-Hysteria2",
		c.Password, hostForLink(srv.ServerIP), srv.Ports.Hysteria2, selfSignedSNI, c.Name,
	)
	return model.Link{Kind: model.KindHysteria2, Name: "Hysteria2", URL: url}
}

func trojanLink(srv model.Server, c model.Client) model.Link {
	url := fmt.Sprintf(
		"trojan://%s@%s:%d?sni=%s&type=ws&path=%%2Ftrojan&host=%s&allowInsecure=1&udp=true&alpn=http%%2F1.1#%s-Trojan",
		c.Password, hostForLink(srv.ServerIP), srv.Ports.TrojanWS, selfSignedSNI, selfSignedSNI, c.Name,
	)
	return model.Link{Kind: model.KindTrojanWS, Name: "Trojan WS", URL: url}
}

func tuicLink(srv model.Server, c model.Client) model.Link {
	url := fmt.Sprintf(
		"tuic://%s:@%s:%d?alpn=h3&allow_insecure=1&congestion_control=bbr#%s-TUIC",
		c.UUID, hostForLink(srv.ServerIP), srv.Ports.TUIC, c.Name,
	)
	return model.Link{Kind: model.KindTUIC, Name: "TUIC", URL: url}
}

func shadowTLSLink(srv model.Server, c model.Client) model.Link {
	// Shadowsocks-2022 multi-user client key is "serverPSK:userPSK".
	userInfo := b64(ssTLSMethod + ":" + srv.SS2022ServerKey + ":" + c.SS2022Key)
	shadowJSON := fmt.Sprintf(
		`{"address":"%s","password":"%s","version":"3","host":"%s","port":"%d"}`,
		srv.ServerIP, c.ShadowTLSPassword, srv.SNI, srv.Ports.ShadowTLS,
	)
	url := fmt.Sprintf(
		"ss://%s@%s:%d?shadow-tls=%s#%s-ShadowTLS-v3",
		userInfo, bracket(srv.ServerIP), srv.Ports.ShadowTLS, b64(shadowJSON), c.Name,
	)
	return model.Link{Kind: model.KindShadowTLS, Name: "ShadowTLS v3", URL: url}
}

func cdnLink(srv model.Server, c model.Client) model.Link {
	url := fmt.Sprintf(
		"vless://%s@%s:443?encryption=none&security=tls&type=ws&host=%s&sni=%s&path=%%2Fvless#%s-CDN",
		c.UUID, srv.CDNDomain, srv.CDNDomain, srv.CDNDomain, c.Name,
	)
	return model.Link{Kind: model.KindVLESSCDN, Name: "VLESS CDN", URL: url}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// hostForLink brackets bare IPv6 addresses for use in a host:port authority.
func hostForLink(host string) string {
	if ip := net.ParseIP(host); ip != nil && ip.To4() == nil {
		return "[" + host + "]"
	}
	return host
}

// bracket always brackets an IPv6 literal (IPv4 returned unchanged).
func bracket(host string) string { return hostForLink(host) }
