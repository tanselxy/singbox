// Package protocol turns a model.Deployment into sing-box inbounds and the
// matching client share links. Inbounds and links are generated side by side
// from the same Deployment so their ports and secrets can never drift apart.
package protocol

import (
	"encoding/base64"
	"fmt"
	"net"

	"github.com/tanselxy/singbox/internal/model"
	"github.com/tanselxy/singbox/internal/sbschema"
)

const (
	// selfSignedSNI is the CN of the generated self-signed certificate. TUIC,
	// Trojan and Hysteria2 present it; clients connect with insecure=1 so the
	// exact value only needs to be internally consistent.
	selfSignedSNI = "bing.com"

	ssTLSMethod  = "2022-blake3-chacha20-poly1305" // Shadowsocks-2022 behind ShadowTLS
	ssTLSDetour  = "ss-in"
	ssTLSSSPort  = 10808 // loopback port the ShadowTLS detour forwards to
	ssDirectAlgo = "aes-128-gcm"

	wsPathVLESS  = "/vless"
	wsPathTrojan = "/trojan"

	cdnPort = 4433 // VLESS-CDN listens on a fixed port
)

// Inbounds builds the sing-box inbounds for a deployment. IPv6-only
// deployments expose only the VLESS-CDN listener.
func Inbounds(d model.Deployment) []sbschema.Inbound {
	if d.IPv6Only {
		return []sbschema.Inbound{vlessCDNInbound(d)}
	}
	return []sbschema.Inbound{
		shadowTLSInbound(d),
		ssBehindShadowTLSInbound(d),
		ssDirectInbound(d),
		tuicInbound(d),
		realityInbound(d),
		vlessCDNInbound(d),
		trojanInbound(d),
		hysteria2Inbound(d),
	}
}

// Links builds every client-facing share link for a deployment.
func Links(d model.Deployment) []model.Link {
	if d.IPv6Only {
		if d.CDNDomain == "" {
			return nil
		}
		return []model.Link{cdnLink(d)}
	}
	links := []model.Link{
		realityLink(d),
		hysteria2Link(d),
		trojanLink(d),
		tuicLink(d),
		shadowTLSLink(d),
		ssDirectLink(d),
	}
	if d.CDNDomain != "" {
		links = append(links, cdnLink(d))
	}
	return links
}

// ---------------------------------------------------------------------------
// Inbounds
// ---------------------------------------------------------------------------

func shadowTLSInbound(d model.Deployment) sbschema.Inbound {
	return sbschema.Inbound{
		Type:       "shadowtls",
		Tag:        "st-in",
		Listen:     "::",
		ListenPort: d.Ports.ShadowTLS,
		Version:    3,
		Users: []sbschema.User{
			{Name: "username", Password: d.Creds.ShadowTLSPassword},
		},
		Handshake:  &sbschema.Handshake{Server: d.SNI, ServerPort: 443},
		StrictMode: true,
		Detour:     ssTLSDetour,
	}
}

func ssBehindShadowTLSInbound(d model.Deployment) sbschema.Inbound {
	return sbschema.Inbound{
		Type:       "shadowsocks",
		Tag:        ssTLSDetour,
		Listen:     "127.0.0.1",
		ListenPort: ssTLSSSPort,
		Network:    "tcp",
		Method:     ssTLSMethod,
		Password:   d.Creds.SSPassword,
	}
}

func ssDirectInbound(d model.Deployment) sbschema.Inbound {
	return sbschema.Inbound{
		Type:       "shadowsocks",
		Tag:        "ss-ix",
		Listen:     "::",
		ListenPort: d.Ports.SSDirect,
		Method:     ssDirectAlgo,
		Password:   d.Creds.HysteriaPassword,
	}
}

func tuicInbound(d model.Deployment) sbschema.Inbound {
	return sbschema.Inbound{
		Type:              "tuic",
		Tag:               "tuic-in",
		Listen:            "::",
		ListenPort:        d.Ports.TUIC,
		Users:             []sbschema.User{{UUID: d.Creds.UUID}},
		CongestionControl: "bbr",
		TLS: &sbschema.TLS{
			Enabled:         true,
			ServerName:      selfSignedSNI,
			ALPN:            []string{"h3"},
			CertificatePath: d.CertFile,
			KeyPath:         d.KeyFile,
		},
	}
}

func realityInbound(d model.Deployment) sbschema.Inbound {
	return sbschema.Inbound{
		Type:       "vless",
		Tag:        "vless-in",
		Listen:     "::",
		ListenPort: d.Ports.Reality,
		Users:      []sbschema.User{{UUID: d.Creds.UUID, Flow: "xtls-rprx-vision"}},
		TLS: &sbschema.TLS{
			Enabled:    true,
			ServerName: d.SNI,
			Reality: &sbschema.Reality{
				Enabled:    true,
				Handshake:  &sbschema.Handshake{Server: d.SNI, ServerPort: 443},
				PrivateKey: d.Creds.Reality.PrivateKey,
				ShortID:    []string{d.Creds.Reality.ShortID},
			},
		},
	}
}

func vlessCDNInbound(d model.Deployment) sbschema.Inbound {
	return sbschema.Inbound{
		Type:       "vless",
		Tag:        "vless-cdn",
		Listen:     "::",
		ListenPort: cdnPort,
		Users:      []sbschema.User{{UUID: d.Creds.UUID}},
		Transport:  &sbschema.Transport{Type: "ws", Path: wsPathVLESS},
		TLS: &sbschema.TLS{
			Enabled:         true,
			ServerName:      d.CDNDomain,
			CertificatePath: d.CertFile,
			KeyPath:         d.KeyFile,
		},
	}
}

func trojanInbound(d model.Deployment) sbschema.Inbound {
	return sbschema.Inbound{
		Type:       "trojan",
		Tag:        "trojan-in",
		Listen:     "::",
		ListenPort: d.Ports.TrojanWS,
		Users:      []sbschema.User{{Password: d.Creds.HysteriaPassword}},
		TLS: &sbschema.TLS{
			Enabled:         true,
			ServerName:      selfSignedSNI,
			CertificatePath: d.CertFile,
			KeyPath:         d.KeyFile,
		},
		Transport: &sbschema.Transport{Type: "ws", Path: wsPathTrojan},
	}
}

func hysteria2Inbound(d model.Deployment) sbschema.Inbound {
	return sbschema.Inbound{
		Type:       "hysteria2",
		Tag:        "hy2-in",
		Listen:     "::",
		ListenPort: d.Ports.Hysteria2,
		Users:      []sbschema.User{{Password: d.Creds.HysteriaPassword}},
		TLS: &sbschema.TLS{
			Enabled:         true,
			ALPN:            []string{"h3"},
			CertificatePath: d.CertFile,
			KeyPath:         d.KeyFile,
		},
	}
}

// ---------------------------------------------------------------------------
// Links
// ---------------------------------------------------------------------------

func realityLink(d model.Deployment) model.Link {
	url := fmt.Sprintf(
		"vless://%s@%s:%d?security=reality&flow=xtls-rprx-vision&type=tcp&sni=%s&fp=chrome&pbk=%s&sid=%s&encryption=none#Reality",
		d.Creds.UUID, hostForLink(d.ServerIP), d.Ports.Reality, d.SNI,
		d.Creds.Reality.PublicKey, d.Creds.Reality.ShortID,
	)
	return model.Link{Kind: model.KindReality, Name: "Reality", URL: url}
}

func hysteria2Link(d model.Deployment) model.Link {
	url := fmt.Sprintf(
		"hysteria2://%s@%s:%d?insecure=1&alpn=h3&sni=%s#Hysteria2",
		d.Creds.HysteriaPassword, hostForLink(d.ServerIP), d.Ports.Hysteria2, selfSignedSNI,
	)
	return model.Link{Kind: model.KindHysteria2, Name: "Hysteria2", URL: url}
}

func trojanLink(d model.Deployment) model.Link {
	url := fmt.Sprintf(
		"trojan://%s@%s:%d?sni=%s&type=ws&path=%%2Ftrojan&host=%s&allowInsecure=1&udp=true&alpn=http%%2F1.1#Trojan",
		d.Creds.HysteriaPassword, hostForLink(d.ServerIP), d.Ports.TrojanWS, selfSignedSNI, selfSignedSNI,
	)
	return model.Link{Kind: model.KindTrojanWS, Name: "Trojan WS", URL: url}
}

func tuicLink(d model.Deployment) model.Link {
	url := fmt.Sprintf(
		"tuic://%s:@%s:%d?alpn=h3&allow_insecure=1&congestion_control=bbr#TUIC",
		d.Creds.UUID, hostForLink(d.ServerIP), d.Ports.TUIC,
	)
	return model.Link{Kind: model.KindTUIC, Name: "TUIC", URL: url}
}

func ssDirectLink(d model.Deployment) model.Link {
	userInfo := b64(ssDirectAlgo + ":" + d.Creds.HysteriaPassword)
	url := fmt.Sprintf("ss://%s@%s:%d#SS%%E4%%B8%%93%%E7%%BA%%BF", userInfo, hostForLink(d.ServerIP), d.Ports.SSDirect)
	return model.Link{Kind: model.KindSSDirect, Name: "SS 专线", URL: url}
}

func shadowTLSLink(d model.Deployment) model.Link {
	userInfo := b64(ssTLSMethod + ":" + d.Creds.SSPassword)
	shadowJSON := fmt.Sprintf(
		`{"address":"%s","password":"%s","version":"3","host":"%s","port":"%d"}`,
		d.ServerIP, d.Creds.ShadowTLSPassword, d.SNI, d.Ports.ShadowTLS,
	)
	url := fmt.Sprintf(
		"ss://%s@%s:%d?shadow-tls=%s#ShadowTLS-v3",
		userInfo, bracket(d.ServerIP), d.Ports.ShadowTLS, b64(shadowJSON),
	)
	return model.Link{Kind: model.KindShadowTLS, Name: "ShadowTLS v3", URL: url}
}

func cdnLink(d model.Deployment) model.Link {
	url := fmt.Sprintf(
		"vless://%s@%s:443?encryption=none&security=tls&type=ws&host=%s&sni=%s&path=%%2Fvless#CDN",
		d.Creds.UUID, d.CDNDomain, d.CDNDomain, d.CDNDomain,
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

// bracket always brackets an IPv6 literal; the ShadowTLS link format kept the
// brackets even for IPv4 in the old code, but bracketing only v6 is correct.
func bracket(host string) string { return hostForLink(host) }
