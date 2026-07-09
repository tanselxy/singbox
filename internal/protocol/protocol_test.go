package protocol

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tanselxy/singbox/internal/model"
)

func sampleServer() model.Server {
	return model.Server{
		ServerIP:        "203.0.113.7",
		SNI:             "www.apple.com",
		CDNDomain:       "cdn.example.com",
		CertFile:        "/etc/sing-box/cert/cert.pem",
		KeyFile:         "/etc/sing-box/cert/private.key",
		SS2022ServerKey: "c2VydmVyUFNL",
		Reality:         model.Reality{PrivateKey: "priv", PublicKey: "pub-key", ShortID: "0123456789abcdef"},
		// Deliberately non-pool ports to emulate NAT-mode random assignment.
		Ports: model.Ports{Reality: 41001, Hysteria2: 41002, ShadowTLS: 41003, TUIC: 41005, TrojanWS: 41006, VLESSCDN: 4433},
	}
}

func clients() []model.Client {
	return []model.Client{
		{ID: 1, Name: "alice", UUID: "11111111-1111-1111-1111-111111111111", Password: "aPass", SS2022Key: "YWxpY2VLZXk=", ShadowTLSPassword: "aStls"},
		{ID: 2, Name: "bob", UUID: "22222222-2222-2222-2222-222222222222", Password: "bPass", SS2022Key: "Ym9iS2V5", ShadowTLSPassword: "bStls"},
	}
}

// TestLinkPortsMatchInboundPorts guards against the old NAT port-drift bug:
// every client link must advertise the port of its serving inbound.
func TestLinkPortsMatchInboundPorts(t *testing.T) {
	srv := sampleServer()
	cs := clients()

	portByTag := map[string]int{}
	for _, in := range Inbounds(srv, cs) {
		portByTag[in.Tag] = in.ListenPort
	}

	linkToInbound := map[model.Kind]string{
		model.KindReality:   "vless-in",
		model.KindHysteria2: "hy2-in",
		model.KindTrojanWS:  "trojan-in",
		model.KindTUIC:      "tuic-in",
		model.KindShadowTLS: "st-in",
	}

	links := map[model.Kind]string{}
	for _, l := range ClientLinks(srv, cs[0], "") {
		links[l.Kind] = l.URL
	}

	for kind, tag := range linkToInbound {
		url, ok := links[kind]
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

func TestEveryInboundHasAllClientsAsUsers(t *testing.T) {
	srv := sampleServer()
	cs := clients()
	for _, in := range Inbounds(srv, cs) {
		if len(in.Users) != len(cs) {
			t.Errorf("inbound %s has %d users, want %d", in.Tag, len(in.Users), len(cs))
		}
	}
}

func TestEveryInboundCarriesDeviceLimit(t *testing.T) {
	srv := sampleServer()
	cs := clients()
	cs[0].DeviceLimit = 2
	for _, in := range Inbounds(srv, cs) {
		if len(in.Users) == 0 {
			t.Fatalf("inbound %s has no users", in.Tag)
		}
		if in.Users[0].DeviceLimit != 2 {
			t.Errorf("inbound %s device_limit = %d, want 2", in.Tag, in.Users[0].DeviceLimit)
		}
	}
}

func TestClientLinksCarryOwnCredentials(t *testing.T) {
	srv := sampleServer()
	cs := clients()

	aliceReality := ""
	for _, l := range ClientLinks(srv, cs[0], "") {
		if l.Kind == model.KindReality {
			aliceReality = l.URL
		}
	}
	if !strings.Contains(aliceReality, cs[0].UUID) {
		t.Errorf("alice's reality link must carry her uuid: %s", aliceReality)
	}
	if strings.Contains(aliceReality, cs[1].UUID) {
		t.Errorf("alice's link must not carry bob's uuid")
	}
}

func TestShadowTLSLinkUsesTwoLayerKey(t *testing.T) {
	srv := sampleServer()
	c := clients()[0]
	var stls string
	for _, l := range ClientLinks(srv, c, "") {
		if l.Kind == model.KindShadowTLS {
			stls = l.URL
		}
	}
	// user-info must decode to method:serverPSK:userPSK
	want := ssTLSMethod + ":" + srv.SS2022ServerKey + ":" + c.SS2022Key
	if !strings.Contains(stls, b64(want)) {
		t.Errorf("shadowtls link must embed two-layer key %q (b64) in %s", want, stls)
	}
}

func TestIPv6OnlyExposesOnlyCDN(t *testing.T) {
	srv := sampleServer()
	srv.IPv6Only = true
	cs := clients()

	ins := Inbounds(srv, cs)
	if len(ins) != 1 || ins[0].Tag != "vless-cdn" {
		t.Fatalf("ipv6-only should expose exactly the vless-cdn inbound, got %d", len(ins))
	}
	links := ClientLinks(srv, cs[0], "")
	if len(links) != 1 || links[0].Kind != model.KindVLESSCDN {
		t.Fatalf("ipv6-only should yield exactly the CDN link, got %d", len(links))
	}
}

func TestShadowTLSInboundHasDetourPair(t *testing.T) {
	srv := sampleServer()
	cs := clients()
	var haveST, haveSS bool
	for _, in := range Inbounds(srv, cs) {
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
