package protocol

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tanselxy/singbox/internal/model"
)

func sampleDeployment() model.Deployment {
	return model.Deployment{
		ServerIP:  "203.0.113.7",
		SNI:       "www.apple.com",
		CDNDomain: "cdn.example.com",
		CertFile:  "/etc/sing-box/cert/cert.pem",
		KeyFile:   "/etc/sing-box/cert/private.key",
		Creds: model.Credentials{
			UUID:              "11111111-2222-3333-4444-555555555555",
			HysteriaPassword:  "hyPass123",
			SSPassword:        "c3NwYXNz",
			ShadowTLSPassword: "stlsPass==",
			Reality: model.Reality{
				PrivateKey: "priv-key",
				PublicKey:  "pub-key",
				ShortID:    "0123456789abcdef",
			},
		},
		// Deliberately non-pool ports to emulate NAT-mode random assignment.
		Ports: model.Ports{
			Reality:   41001,
			Hysteria2: 41002,
			ShadowTLS: 41003,
			SSDirect:  41004,
			TUIC:      41005,
			TrojanWS:  41006,
			VLESSCDN:  4433,
		},
	}
}

// TestLinkPortsMatchInboundPorts is the core regression guard: every share
// link must advertise the same port the corresponding inbound listens on.
// The old bash code hard-coded 63333/61555/59000 in links while NAT mode
// randomised the server ports, so links pointed at dead ports.
func TestLinkPortsMatchInboundPorts(t *testing.T) {
	d := sampleDeployment()

	// Read the real listening port straight off each inbound, keyed by tag
	// (type is not unique: vless-in and vless-cdn are both "vless").
	portByTag := map[string]int{}
	for _, in := range Inbounds(d) {
		portByTag[in.Tag] = in.ListenPort
	}

	// Each client link must advertise the port of its serving inbound.
	linkToInbound := map[model.Kind]string{
		model.KindReality:   "vless-in",
		model.KindHysteria2: "hy2-in",
		model.KindTrojanWS:  "trojan-in",
		model.KindTUIC:      "tuic-in",
		model.KindSSDirect:  "ss-ix",
		model.KindShadowTLS: "st-in",
	}

	linkByKind := map[model.Kind]string{}
	for _, l := range Links(d) {
		linkByKind[l.Kind] = l.URL
	}

	for kind, tag := range linkToInbound {
		url, ok := linkByKind[kind]
		if !ok {
			t.Errorf("%s: no link generated", kind)
			continue
		}
		want := fmt.Sprintf(":%d", portByTag[tag])
		if !strings.Contains(url, want) {
			t.Errorf("%s link must advertise inbound %s port %d: %s", kind, tag, portByTag[tag], url)
		}
	}
}

func TestPerInstallSecretsAppearInLinks(t *testing.T) {
	d := sampleDeployment()
	links := Links(d)
	var reality string
	for _, l := range links {
		if l.Kind == model.KindReality {
			reality = l.URL
		}
	}
	if reality == "" {
		t.Fatal("no reality link")
	}
	if !strings.Contains(reality, "pbk="+d.Creds.Reality.PublicKey) {
		t.Errorf("reality link must carry the per-install public key: %s", reality)
	}
	if !strings.Contains(reality, "sid="+d.Creds.Reality.ShortID) {
		t.Errorf("reality link must carry the per-install short id: %s", reality)
	}
}

func TestIPv6OnlyExposesOnlyCDN(t *testing.T) {
	d := sampleDeployment()
	d.IPv6Only = true

	ins := Inbounds(d)
	if len(ins) != 1 || ins[0].Tag != "vless-cdn" {
		t.Fatalf("ipv6-only should expose exactly the vless-cdn inbound, got %d", len(ins))
	}

	links := Links(d)
	if len(links) != 1 || links[0].Kind != model.KindVLESSCDN {
		t.Fatalf("ipv6-only should yield exactly the CDN link, got %d", len(links))
	}
}

func TestShadowTLSInboundHasDetourPair(t *testing.T) {
	d := sampleDeployment()
	var haveST, haveSS bool
	for _, in := range Inbounds(d) {
		switch in.Tag {
		case "st-in":
			haveST = true
			if in.Detour != "ss-in" {
				t.Errorf("st-in detour = %q, want ss-in", in.Detour)
			}
		case "ss-in":
			haveSS = true
			if in.Listen != "127.0.0.1" {
				t.Errorf("ss-in should listen on loopback, got %q", in.Listen)
			}
		}
	}
	if !haveST || !haveSS {
		t.Errorf("shadowtls needs both st-in and ss-in inbounds (st=%v ss=%v)", haveST, haveSS)
	}
}
