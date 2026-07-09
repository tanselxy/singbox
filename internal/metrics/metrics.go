// Package metrics collects host CPU, memory and disk usage on Linux. Parsing is
// separated from the /proc and syscall reads so it can be unit-tested.
package metrics

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// System is a snapshot of host resource usage.
type System struct {
	CPUPercent  float64 `json:"cpu_percent"`
	CPUCores    int     `json:"cpu_cores"`
	Load1       float64 `json:"load1"`
	Load5       float64 `json:"load5"`
	Load15      float64 `json:"load15"`
	MemUsed     uint64  `json:"mem_used"`
	MemTotal    uint64  `json:"mem_total"`
	MemPercent  float64 `json:"mem_percent"`
	DiskUsed    uint64  `json:"disk_used"`
	DiskTotal   uint64  `json:"disk_total"`
	DiskPercent float64 `json:"disk_percent"`
	NetRxBytes  uint64  `json:"net_rx_bytes"`
	NetTxBytes  uint64  `json:"net_tx_bytes"`
}

// Collect gathers a system snapshot. CPU usage is measured over a short window.
func Collect() System {
	s := System{CPUCores: runtime.NumCPU()}
	if total, avail, ok := readMem(); ok {
		s.MemTotal = total
		s.MemUsed = total - avail
		s.MemPercent = percent(s.MemUsed, total)
	}
	if total, free, ok := readDisk("/"); ok {
		s.DiskTotal = total
		s.DiskUsed = total - free
		s.DiskPercent = percent(s.DiskUsed, total)
	}
	s.Load1, s.Load5, s.Load15 = readLoad()
	s.NetRxBytes, s.NetTxBytes = readNetDev()
	s.CPUPercent = readCPUPercent(200 * time.Millisecond)
	return s
}

func percent(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
}

// readMem returns total and available bytes from /proc/meminfo.
func readMem() (total, avail uint64, ok bool) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, false
	}
	return parseMeminfo(string(b))
}

func parseMeminfo(s string) (total, avail uint64, ok bool) {
	var haveTotal, haveAvail bool
	for _, line := range strings.Split(s, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		kb, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			total, haveTotal = kb*1024, true
		case "MemAvailable:":
			avail, haveAvail = kb*1024, true
		}
	}
	return total, avail, haveTotal && haveAvail
}

// readDisk returns total and free bytes of the filesystem at path.
func readDisk(path string) (total, free uint64, ok bool) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0, false
	}
	bsize := uint64(st.Bsize)
	return st.Blocks * bsize, st.Bavail * bsize, true
}

func readLoad() (float64, float64, float64) {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}
	fields := strings.Fields(string(b))
	if len(fields) < 3 {
		return 0, 0, 0
	}
	return parseFloat(fields[0]), parseFloat(fields[1]), parseFloat(fields[2])
}

func parseFloat(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

func readNetDev() (rx, tx uint64) {
	b, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return 0, 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		name, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		iface := strings.TrimSpace(name)
		if iface == "" || iface == "lo" {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) < 16 {
			continue
		}
		if v, err := strconv.ParseUint(fields[0], 10, 64); err == nil {
			rx += v
		}
		if v, err := strconv.ParseUint(fields[8], 10, 64); err == nil {
			tx += v
		}
	}
	return rx, tx
}

// readCPUPercent samples /proc/stat twice over window and returns busy percent.
func readCPUPercent(window time.Duration) float64 {
	b1, e1, ok1 := readCPUTimes()
	if !ok1 {
		return 0
	}
	time.Sleep(window)
	b2, e2, ok2 := readCPUTimes()
	if !ok2 {
		return 0
	}
	dBusy := float64(b2 - b1)
	dTotal := float64((b2 + e2) - (b1 + e1))
	if dTotal <= 0 {
		return 0
	}
	return dBusy / dTotal * 100
}

// readCPUTimes returns cumulative busy and idle jiffies from /proc/stat.
func readCPUTimes() (busy, idle uint64, ok bool) {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	return parseCPUStat(string(b))
}

// parseCPUStat parses the aggregate "cpu" line of /proc/stat.
// Fields: user nice system idle iowait irq softirq steal ...
func parseCPUStat(s string) (busy, idle uint64, ok bool) {
	for _, line := range strings.Split(s, "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)[1:]
		var vals []uint64
		for _, f := range fields {
			v, err := strconv.ParseUint(f, 10, 64)
			if err != nil {
				break
			}
			vals = append(vals, v)
		}
		if len(vals) < 5 {
			return 0, 0, false
		}
		idleTime := vals[3] + vals[4] // idle + iowait
		var total uint64
		for _, v := range vals {
			total += v
		}
		return total - idleTime, idleTime, true
	}
	return 0, 0, false
}

// Format renders a byte count as a human-readable string (e.g. "1.5 GiB").
func Format(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
