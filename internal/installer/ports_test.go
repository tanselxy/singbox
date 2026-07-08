package installer

import "testing"

func TestNATPortsAreDistinctAndInRange(t *testing.T) {
	start, end := 30000, 30020
	p, err := NATPorts(start, end)
	if err != nil {
		t.Fatal(err)
	}

	got := []int{p.Reality, p.Hysteria2, p.ShadowTLS, p.SSDirect, p.TUIC, p.TrojanWS}
	seen := map[int]bool{}
	for _, v := range got {
		if v < start || v > end {
			t.Errorf("port %d out of range [%d,%d]", v, start, end)
		}
		if seen[v] {
			t.Errorf("duplicate port %d", v)
		}
		seen[v] = true
	}
	if p.VLESSCDN != cdnPort {
		t.Errorf("VLESS-CDN port = %d, want fixed %d", p.VLESSCDN, cdnPort)
	}
}

func TestNATPortsRejectsTooSmallRange(t *testing.T) {
	if _, err := NATPorts(30000, 30003); err == nil {
		t.Error("expected error when range cannot fit 6 distinct ports")
	}
}

func TestDefaultPortsMatchPool(t *testing.T) {
	p := DefaultPorts()
	if p.Reality != 20000 || p.Hysteria2 != 50000 || p.ShadowTLS != 31000 ||
		p.SSDirect != 59000 || p.TUIC != 61555 || p.TrojanWS != 63333 || p.VLESSCDN != cdnPort {
		t.Errorf("default pool drifted from legacy values: %+v", p)
	}
}
