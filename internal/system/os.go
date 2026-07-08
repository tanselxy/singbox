// Package system detects the host OS and runs privileged system operations
// (package installs, sysctl, systemctl). Parsing is separated from execution so
// the detection logic can be unit-tested without a Linux host.
package system

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// PackageManager identifies the host's package manager.
type PackageManager string

const (
	APT PackageManager = "apt"
	DNF PackageManager = "dnf"
	YUM PackageManager = "yum"
)

// OSInfo describes the running distribution.
type OSInfo struct {
	ID      string // e.g. "ubuntu", "debian", "centos", "rocky"
	IDLike  string // e.g. "debian", "rhel fedora"
	Version string // VERSION_ID
	Manager PackageManager
}

// osReleasePath is a var so tests can point it at a fixture.
var osReleasePath = "/etc/os-release"

// Detect reads /etc/os-release and derives the package manager.
func Detect() (OSInfo, error) {
	f, err := os.Open(osReleasePath)
	if err != nil {
		return OSInfo{}, fmt.Errorf("read os-release: %w", err)
	}
	defer f.Close()

	fields := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		fields[key] = strings.Trim(val, `"'`)
	}
	if err := sc.Err(); err != nil {
		return OSInfo{}, fmt.Errorf("scan os-release: %w", err)
	}

	info := OSInfo{
		ID:      fields["ID"],
		IDLike:  fields["ID_LIKE"],
		Version: fields["VERSION_ID"],
	}
	mgr, err := managerFor(info.ID, info.IDLike)
	if err != nil {
		return OSInfo{}, err
	}
	info.Manager = mgr
	return info, nil
}

// managerFor maps a distro id/id_like to a package manager. dnf is preferred
// over yum when the distro is in the RHEL family; the caller verifies dnf
// actually exists and falls back to yum otherwise.
func managerFor(id, idLike string) (PackageManager, error) {
	haystack := strings.ToLower(id + " " + idLike)
	switch {
	case containsAny(haystack, "ubuntu", "debian"):
		return APT, nil
	case containsAny(haystack, "centos", "rhel", "rocky", "almalinux", "fedora"):
		return DNF, nil
	default:
		return "", fmt.Errorf("unsupported distribution: id=%q id_like=%q", id, idLike)
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
