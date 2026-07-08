package installer

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/tanselxy/singbox/internal/model"
)

// cdnPort is fixed for the VLESS-CDN inbound in every mode.
const cdnPort = 4433

// DefaultPorts returns the fixed port pool used in normal (non-NAT) mode,
// matching legacy defaults.conf.
func DefaultPorts() model.Ports {
	return model.Ports{
		Reality:   20000,
		Hysteria2: 50000,
		ShadowTLS: 31000,
		SSDirect:  59000,
		TUIC:      61555,
		TrojanWS:  63333,
		VLESSCDN:  cdnPort,
	}
}

// NATPorts assigns six distinct random ports within [start,end] for the
// randomised protocols, keeping VLESS-CDN on its fixed port. Because links and
// inbounds both read these back from model.Ports, they can never disagree.
func NATPorts(start, end int) (model.Ports, error) {
	got, err := distinctRandomPorts(start, end, 6)
	if err != nil {
		return model.Ports{}, err
	}
	return model.Ports{
		Reality:   got[0],
		Hysteria2: got[1],
		ShadowTLS: got[2],
		SSDirect:  got[3],
		TUIC:      got[4],
		TrojanWS:  got[5],
		VLESSCDN:  cdnPort,
	}, nil
}

func distinctRandomPorts(start, end, n int) ([]int, error) {
	if start < 1 || end > 65535 || start > end {
		return nil, fmt.Errorf("invalid port range %d-%d", start, end)
	}
	span := end - start + 1
	if span < n {
		return nil, fmt.Errorf("port range %d-%d too small for %d ports", start, end, n)
	}
	seen := map[int]bool{}
	out := make([]int, 0, n)
	for len(out) < n {
		v, err := rand.Int(rand.Reader, big.NewInt(int64(span)))
		if err != nil {
			return nil, err
		}
		p := start + int(v.Int64())
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out, nil
}
