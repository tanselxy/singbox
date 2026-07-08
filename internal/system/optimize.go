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
	const jail = `[DEFAULT]
bantime = -1
findtime = 86400
maxretry = 10
ignoreip = 127.0.0.1/8 ::1

[sshd]
enabled = true
`
	if err := os.MkdirAll("/etc/fail2ban", 0o755); err != nil {
		return fmt.Errorf("create fail2ban dir: %w", err)
	}
	if err := os.WriteFile("/etc/fail2ban/jail.local", []byte(jail), 0o644); err != nil {
		return fmt.Errorf("write jail.local: %w", err)
	}
	if err := Run(ctx, "systemctl", "enable", "fail2ban"); err != nil {
		return err
	}
	return Run(ctx, "systemctl", "restart", "fail2ban")
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
