package system

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const sysctlFile = "/etc/sysctl.d/99-singbox.conf"

const managedFail2banJail = `# Managed by singbox-panel
[DEFAULT]
bantime = -1
findtime = 86400
maxretry = 10
ignoreip = 127.0.0.1/8 ::1

[sshd]
enabled = true
`

const legacyFail2banJail = `[DEFAULT]
bantime = -1
findtime = 86400
maxretry = 10
ignoreip = 127.0.0.1/8 ::1

[sshd]
enabled = true
`

// sysctlTuning enables BBR congestion control and a few TCP tweaks that improve
// proxy throughput and latency. Written to a dedicated drop-in so re-applying is
// idempotent (the file is simply overwritten).
const sysctlTuning = `# Managed by singbox-panel
net.core.default_qdisc = fq
net.ipv4.tcp_congestion_control = bbr
net.ipv4.tcp_fastopen = 3
net.ipv4.tcp_slow_start_after_idle = 0
net.ipv4.tcp_notsent_lowat = 16384
`

// OptimizeNetwork loads the BBR module and applies the sysctl tuning.
func OptimizeNetwork(ctx context.Context) error {
	_ = Run(ctx, "modprobe", "tcp_bbr") // best-effort; may be built-in
	if err := os.WriteFile(sysctlFile, []byte(sysctlTuning), 0o644); err != nil {
		return fmt.Errorf("write sysctl: %w", err)
	}
	if err := Run(ctx, "sysctl", "--system"); err != nil {
		return fmt.Errorf("apply sysctl: %w", err)
	}
	return nil
}

// BBREnabled reports whether the running congestion control is bbr.
func BBREnabled(ctx context.Context) bool {
	out, err := Output(ctx, "sysctl", "-n", "net.ipv4.tcp_congestion_control")
	return err == nil && strings.TrimSpace(out) == "bbr"
}

// SetupFail2ban installs fail2ban (via the host package manager), writes a jail
// protecting sshd, and enables the service.
func SetupFail2ban(ctx context.Context, mgr PackageManager) error {
	if err := installFail2ban(ctx, mgr); err != nil {
		return err
	}
	if err := os.MkdirAll("/etc/fail2ban", 0o755); err != nil {
		return fmt.Errorf("create fail2ban dir: %w", err)
	}
	if err := os.WriteFile("/etc/fail2ban/jail.local", []byte(managedFail2banJail), 0o644); err != nil {
		return fmt.Errorf("write jail.local: %w", err)
	}
	if err := Run(ctx, "systemctl", "enable", "fail2ban"); err != nil {
		return err
	}
	return Run(ctx, "systemctl", "restart", "fail2ban")
}

// RemoveFail2ban stops and removes fail2ban. Only the jail configuration
// created by this panel is deleted; a user-modified jail.local is preserved.
func RemoveFail2ban(ctx context.Context, mgr PackageManager) error {
	_ = Run(ctx, "systemctl", "disable", "--now", "fail2ban")
	if err := removeManagedFail2banJail(); err != nil {
		return err
	}
	switch mgr {
	case APT:
		return Run(ctx, "apt-get", "remove", "-y", "fail2ban")
	default:
		return Run(ctx, string(mgr), "remove", "-y", "fail2ban")
	}
}

func removeManagedFail2banJail() error {
	const jailPath = "/etc/fail2ban/jail.local"
	data, err := os.ReadFile(jailPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read jail.local: %w", err)
	}
	content := strings.TrimSpace(string(data))
	if content != strings.TrimSpace(managedFail2banJail) && content != strings.TrimSpace(legacyFail2banJail) {
		return nil
	}
	if err := os.Remove(jailPath); err != nil {
		return fmt.Errorf("remove jail.local: %w", err)
	}
	return nil
}

func installFail2ban(ctx context.Context, mgr PackageManager) error {
	switch mgr {
	case APT:
		if err := Run(ctx, "apt-get", "update", "-y"); err != nil {
			return err
		}
		return Run(ctx, "apt-get", "install", "-y", "fail2ban")
	default:
		_ = Run(ctx, string(mgr), "install", "-y", "epel-release")
		return Run(ctx, string(mgr), "install", "-y", "fail2ban")
	}
}

// Fail2banActive reports whether the fail2ban service is running.
func Fail2banActive(ctx context.Context) bool {
	out, err := Output(ctx, "systemctl", "is-active", "fail2ban")
	return err == nil && out == "active"
}

// Fail2banInstalled reports whether the fail2ban command is available on the host.
func Fail2banInstalled() bool { return LookPath("fail2ban-client") }

type Fail2banBan struct {
	IP     string `json:"ip"`
	Jail   string `json:"jail"`
	Status string `json:"status"`
}

type Fail2banBans struct {
	Bans     []Fail2banBan `json:"bans"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
	Total    int           `json:"total"`
}

// ListFail2banBans returns the currently banned IPs from the sshd jail.
func ListFail2banBans(ctx context.Context, page, pageSize int) (Fail2banBans, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 25
	}
	out, err := Output(ctx, "fail2ban-client", "status", "sshd")
	if err != nil {
		return Fail2banBans{}, fmt.Errorf("读取 fail2ban 封禁列表: %w", err)
	}
	ips := parseFail2banBannedIPs(out)
	start := (page - 1) * pageSize
	if start > len(ips) {
		start = len(ips)
	}
	end := start + pageSize
	if end > len(ips) {
		end = len(ips)
	}
	bans := make([]Fail2banBan, 0, end-start)
	for _, ip := range ips[start:end] {
		bans = append(bans, Fail2banBan{IP: ip, Jail: "sshd", Status: "当前封禁"})
	}
	return Fail2banBans{Bans: bans, Page: page, PageSize: pageSize, Total: len(ips)}, nil
}

func parseFail2banBannedIPs(status string) []string {
	for _, line := range strings.Split(status, "\n") {
		label, value, found := strings.Cut(line, "Banned IP list:")
		if !found || label == "" {
			continue
		}
		return strings.Fields(value)
	}
	return nil
}

const sshdConfigPath = "/etc/ssh/sshd_config"

// CurrentSSHPort reads the effective sshd Port (defaults to 22).
func CurrentSSHPort() int {
	f, err := os.Open(sshdConfigPath)
	if err != nil {
		return 22
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		if fields := strings.Fields(line); len(fields) == 2 && fields[0] == "Port" {
			if p, err := strconv.Atoi(fields[1]); err == nil {
				return p
			}
		}
	}
	return 22
}

// ChangeSSHPort updates sshd to listen on the given port, opens it in the
// firewall (best-effort), and restarts sshd. The firewall rule is added BEFORE
// restart so an active session is not lost.
func ChangeSSHPort(ctx context.Context, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("端口无效: %d", port)
	}
	if PortInUse(port) {
		return fmt.Errorf("端口 %d 已被占用", port)
	}

	data, err := os.ReadFile(sshdConfigPath)
	if err != nil {
		return fmt.Errorf("读取 sshd_config: %w", err)
	}
	_ = os.WriteFile(sshdConfigPath+".singbox.bak", data, 0o600)

	// Replace/append the Port directive.
	lines := strings.Split(string(data), "\n")
	var out []string
	replaced := false
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "Port ") || t == "Port" || strings.HasPrefix(t, "#Port") {
			if !replaced {
				out = append(out, "Port "+strconv.Itoa(port))
				replaced = true
			}
			continue
		}
		out = append(out, l)
	}
	if !replaced {
		out = append(out, "Port "+strconv.Itoa(port))
	}
	if err := os.WriteFile(sshdConfigPath, []byte(strings.Join(out, "\n")), 0o644); err != nil {
		return fmt.Errorf("写入 sshd_config: %w", err)
	}

	openFirewallPort(ctx, port)

	// Debian uses ssh.service, RHEL uses sshd.service.
	if err := Run(ctx, "systemctl", "restart", "sshd"); err != nil {
		if err2 := Run(ctx, "systemctl", "restart", "ssh"); err2 != nil {
			return fmt.Errorf("重启 sshd 失败: %w", err)
		}
	}
	return nil
}

// openFirewallPort adds a TCP allow rule via whichever firewall is present.
func openFirewallPort(ctx context.Context, port int) {
	p := strconv.Itoa(port)
	if LookPath("ufw") {
		_ = Run(ctx, "ufw", "allow", p+"/tcp")
	}
	if LookPath("firewall-cmd") {
		_ = Run(ctx, "firewall-cmd", "--permanent", "--add-port="+p+"/tcp")
		_ = Run(ctx, "firewall-cmd", "--reload")
	}
}

// PortInUse reports whether a local TCP port is currently bound.
func PortInUse(port int) bool {
	out, err := Output(context.Background(), "ss", "-tlnH", "sport", "=", ":"+strconv.Itoa(port))
	return err == nil && strings.TrimSpace(out) != ""
}
