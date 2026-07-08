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
	srv := model.Server{
		ServerIP:        "203.0.113.7",
		SNI:             "www.apple.com",
		CertFile:        "/etc/sing-box/cert/cert.pem",
		KeyFile:         "/etc/sing-box/cert/private.key",
		SS2022ServerKey: "c2VydmVyUFNL",
		Reality:         model.Reality{PrivateKey: "priv", PublicKey: "pub", ShortID: "0123456789abcdef"},
		Ports:           model.Ports{Reality: 20000, Hysteria2: 50000, ShadowTLS: 31000, TUIC: 61555, TrojanWS: 63333, VLESSCDN: 4433},
	}
	clients := []model.Client{
		{ID: 1, Name: "alice", UUID: "11111111-2222-3333-4444-555555555555", Password: "aPass", SS2022Key: "YWxpY2U=", ShadowTLSPassword: "aStls"},
	}

	b, err := Marshal(srv, clients)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !json.Valid(b) {
		t.Fatal("marshalled config is not valid JSON")
	}

	var doc struct {
		Inbounds     []json.RawMessage `json:"inbounds"`
		Experimental struct {
			ClashAPI struct {
				ExternalController string `json:"external_controller"`
			} `json:"clash_api"`
			V2RayAPI struct {
				Stats struct {
					Enabled bool     `json:"enabled"`
					Users   []string `json:"users"`
				} `json:"stats"`
			} `json:"v2ray_api"`
		} `json:"experimental"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Inbounds) != 7 {
		t.Errorf("expected 7 inbounds, got %d", len(doc.Inbounds))
	}
	if doc.Experimental.ClashAPI.ExternalController != ClashAPIAddr {
		t.Errorf("clash_api controller = %q, want %q", doc.Experimental.ClashAPI.ExternalController, ClashAPIAddr)
	}
	if !doc.Experimental.V2RayAPI.Stats.Enabled {
		t.Error("v2ray_api stats should be enabled")
	}
	if len(doc.Experimental.V2RayAPI.Stats.Users) != 1 || doc.Experimental.V2RayAPI.Stats.Users[0] != "alice" {
		t.Errorf("v2ray_api stats.users = %v, want [alice]", doc.Experimental.V2RayAPI.Stats.Users)
	}
}
