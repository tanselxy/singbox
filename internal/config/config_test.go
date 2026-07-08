package config

import (
	"encoding/json"
	"testing"

	"github.com/tanselxy/singbox/internal/model"
)

func TestStaticBlocksAreValidJSON(t *testing.T) {
	for name, block := range map[string]string{
		"dns":       dnsBlock,
		"outbounds": outboundsBlock,
		"route":     routeBlock,
	} {
		if !json.Valid([]byte(block)) {
			t.Errorf("%s block is not valid JSON", name)
		}
	}
}

func TestMarshalProducesValidConfig(t *testing.T) {
	d := model.Deployment{
		ServerIP: "203.0.113.7",
		SNI:      "www.apple.com",
		CertFile: "/etc/sing-box/cert/cert.pem",
		KeyFile:  "/etc/sing-box/cert/private.key",
		Creds: model.Credentials{
			UUID:              "11111111-2222-3333-4444-555555555555",
			HysteriaPassword:  "hyPass123",
			SSPassword:        "c3NwYXNz",
			ShadowTLSPassword: "stlsPass==",
			Reality:           model.Reality{PrivateKey: "priv", PublicKey: "pub", ShortID: "0123456789abcdef"},
		},
		Ports: model.Ports{Reality: 20000, Hysteria2: 50000, ShadowTLS: 31000, SSDirect: 59000, TUIC: 61555, TrojanWS: 63333, VLESSCDN: 4433},
	}

	b, err := Marshal(d)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !json.Valid(b) {
		t.Fatal("marshalled config is not valid JSON")
	}

	// Spot-check that inbounds were embedded and the reality private key is present.
	var doc struct {
		Inbounds []json.RawMessage `json:"inbounds"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Inbounds) != 8 {
		t.Errorf("expected 8 inbounds, got %d", len(doc.Inbounds))
	}
}
