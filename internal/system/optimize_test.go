package system

import "testing"

func TestParseFail2banBannedIPs(t *testing.T) {
	status := `Status for the jail: sshd
|- Filter
|  |- Currently failed:\t0
` + "`- Actions\n" + `   |- Currently banned:\t2
   |- Total banned:\t6
   ` + "`- Banned IP list:\t198.51.100.10 2001:db8::10\n"
	got := parseFail2banBannedIPs(status)
	if len(got) != 2 || got[0] != "198.51.100.10" || got[1] != "2001:db8::10" {
		t.Fatalf("parseFail2banBannedIPs() = %#v", got)
	}
}
