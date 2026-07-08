package system

import (
	"os"
	"path/filepath"
	"testing"
)

func writeOSRelease(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "os-release")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	old := osReleasePath
	osReleasePath = p
	t.Cleanup(func() { osReleasePath = old })
}

func TestDetect(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantID  string
		wantMgr PackageManager
		wantErr bool
	}{
		{"ubuntu", "ID=ubuntu\nID_LIKE=debian\nVERSION_ID=\"22.04\"\n", "ubuntu", APT, false},
		{"debian", "ID=debian\nVERSION_ID=\"12\"\n", "debian", APT, false},
		{"rocky", "ID=\"rocky\"\nID_LIKE=\"rhel centos fedora\"\n", "rocky", DNF, false},
		{"centos", "ID=\"centos\"\nID_LIKE=\"rhel fedora\"\n", "centos", DNF, false},
		{"alpine", "ID=alpine\n", "alpine", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			writeOSRelease(t, c.content)
			info, err := Detect()
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error for unsupported distro")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if info.ID != c.wantID {
				t.Errorf("ID = %q, want %q", info.ID, c.wantID)
			}
			if info.Manager != c.wantMgr {
				t.Errorf("Manager = %q, want %q", info.Manager, c.wantMgr)
			}
		})
	}
}
