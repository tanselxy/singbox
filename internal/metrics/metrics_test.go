package metrics

import "testing"

func TestParseMeminfo(t *testing.T) {
	sample := `MemTotal:        2048000 kB
MemFree:          100000 kB
MemAvailable:    1024000 kB
Buffers:           50000 kB
`
	total, avail, ok := parseMeminfo(sample)
	if !ok {
		t.Fatal("expected ok")
	}
	if total != 2048000*1024 {
		t.Errorf("total = %d", total)
	}
	if avail != 1024000*1024 {
		t.Errorf("avail = %d", avail)
	}
}

func TestParseMeminfoMissingAvailable(t *testing.T) {
	if _, _, ok := parseMeminfo("MemTotal: 2048000 kB\n"); ok {
		t.Error("should not be ok without MemAvailable")
	}
}

func TestParseCPUStat(t *testing.T) {
	// user=100 nice=0 system=50 idle=800 iowait=50 -> busy=150 idle=850 total=1000
	busy, idle, ok := parseCPUStat("cpu  100 0 50 800 50 0 0 0 0 0\ncpu0 ...\n")
	if !ok {
		t.Fatal("expected ok")
	}
	if idle != 850 {
		t.Errorf("idle = %d, want 850", idle)
	}
	if busy != 150 {
		t.Errorf("busy = %d, want 150", busy)
	}
}

func TestFormat(t *testing.T) {
	cases := map[uint64]string{
		512:                 "512 B",
		1536:                "1.5 KiB",
		1024 * 1024 * 3 / 2: "1.5 MiB",
	}
	for in, want := range cases {
		if got := Format(in); got != want {
			t.Errorf("Format(%d) = %q, want %q", in, got, want)
		}
	}
}
