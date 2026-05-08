package collector

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"weops-inspect/model"
	sshclient "weops-inspect/ssh"
)

// hostBatchCmd is the one-shot command that collects everything except CPU (which needs two samples).
const hostBatchCmd = `echo "===LOADAVG==="; cat /proc/loadavg
echo "===FREE==="; free -b
echo "===DF==="; df -h
echo "===DFI==="; df -i
echo "===ULIMIT==="; ulimit -n
echo "===PROCS==="; ps -eLf 2>/dev/null | wc -l
echo "===NTPD==="; systemctl is-active ntpd 2>/dev/null || echo "N/A"
echo "===CHRONYD==="; systemctl is-active chronyd 2>/dev/null || echo "N/A"
echo "===SELINUX==="; getenforce 2>/dev/null || echo "N/A"
echo "===FIREWALLD==="; systemctl is-active firewalld 2>/dev/null || echo "N/A"
echo "===IPTABLES==="; systemctl is-active iptables 2>/dev/null || echo "N/A"
echo "===SS==="; ss -s 2>/dev/null
echo "===UPTIME==="; cat /proc/uptime
echo "===VERSION==="; cat /etc/redhat-release 2>/dev/null || cat /etc/os-release 2>/dev/null | head -1
echo "===KERNEL==="; uname -r
echo "===MEMINFO==="; grep -E "MemTotal|SwapTotal" /proc/meminfo
echo "===CPUCOUNT==="; grep -c processor /proc/cpuinfo
echo "===DMIDECODE==="; dmidecode -s system-manufacturer 2>/dev/null || echo "N/A"; echo "---"; dmidecode -s system-product-name 2>/dev/null || echo "N/A"; echo "---"; dmidecode -s system-serial-number 2>/dev/null || echo "N/A"
`

const cpuStatCmd = `cat /proc/stat | head -1`

// CollectAllHosts collects host metrics from all hosts concurrently using two-phase CPU sampling.
func CollectAllHosts(client *sshclient.Client, hosts []string, mountPaths string) []model.HostMetrics {
	n := len(hosts)
	results := make([]model.HostMetrics, n)

	// Phase 1: read /proc/stat on all hosts concurrently
	cpuStat1 := make([]string, n)
	var wg1 sync.WaitGroup
	for i, host := range hosts {
		wg1.Add(1)
		go func(idx int, ip string) {
			defer wg1.Done()
			out, err := client.Run(ip, cpuStatCmd)
			if err != nil {
				results[idx].IP = ip
				results[idx].Error = fmt.Sprintf("SSH error: %v", err)
				return
			}
			cpuStat1[idx] = strings.TrimSpace(out)
		}(i, host)
	}
	wg1.Wait()

	// Wait 5 seconds
	fmt.Fprintf(logWriter, "  等待 5 秒进行 CPU 采样...\n")
	time.Sleep(5 * time.Second)

	// Phase 2: read /proc/stat again + all other metrics
	var wg2 sync.WaitGroup
	for i, host := range hosts {
		if results[i].Error != "" {
			continue // skip unreachable hosts
		}
		wg2.Add(1)
		go func(idx int, ip string) {
			defer wg2.Done()
			cmd := cpuStatCmd + "; " + hostBatchCmd
			out, _ := client.Run(ip, cmd)
			results[idx] = parseHostOutput(ip, cpuStat1[idx], out, mountPaths)
		}(i, host)
	}
	wg2.Wait()

	return results
}

func parseHostOutput(ip, cpuStat1, rawOutput, mountPaths string) model.HostMetrics {
	m := model.HostMetrics{IP: ip}

	// Split raw output: first line is cpu stat2, rest is batch output
	lines := strings.SplitN(rawOutput, "\n", 2)
	if len(lines) < 2 {
		m.Error = "incomplete output"
		return m
	}
	cpuStat2 := strings.TrimSpace(lines[0])
	batchOutput := lines[1]

	// Parse CPU usage from two /proc/stat samples
	m.CPUUsage = parseCPUUsage(cpuStat1, cpuStat2)

	// Parse sections
	sections := parseSections(batchOutput)

	// Load average
	if s, ok := sections["LOADAVG"]; ok {
		parts := strings.Fields(s)
		if len(parts) >= 3 {
			m.LoadAvg1, _ = strconv.ParseFloat(parts[0], 64)
			m.LoadAvg5, _ = strconv.ParseFloat(parts[1], 64)
			m.LoadAvg15, _ = strconv.ParseFloat(parts[2], 64)
		}
	}

	// Memory (free -b)
	if s, ok := sections["FREE"]; ok {
		m.MemUsage, m.SwapUsage = parseFree(s)
	}

	// Disk usage
	if s, ok := sections["DF"]; ok {
		m.DiskUsage = parseDiskUsage(s, mountPaths)
	}

	// Inode usage
	if s, ok := sections["DFI"]; ok {
		m.InodeUsage = parseDiskUsage(s, mountPaths)
	}

	// ulimit
	if s, ok := sections["ULIMIT"]; ok {
		m.MaxOpenFiles, _ = strconv.Atoi(strings.TrimSpace(s))
	}

	// process count
	if s, ok := sections["PROCS"]; ok {
		m.ProcessTotal, _ = strconv.Atoi(strings.TrimSpace(s))
	}

	// services
	if s, ok := sections["NTPD"]; ok {
		m.Ntpd = strings.TrimSpace(s)
	}
	if s, ok := sections["CHRONYD"]; ok {
		m.Chronyd = strings.TrimSpace(s)
	}
	if s, ok := sections["SELINUX"]; ok {
		m.SELinux = strings.TrimSpace(s)
	}
	if s, ok := sections["FIREWALLD"]; ok {
		m.Firewalld = strings.TrimSpace(s)
	}
	if s, ok := sections["IPTABLES"]; ok {
		m.Iptables = strings.TrimSpace(s)
	}

	// Network stats (ss -s)
	if s, ok := sections["SS"]; ok {
		m.Network = parseSSOutput(s)
	}

	// Uptime (run days)
	if s, ok := sections["UPTIME"]; ok {
		parts := strings.Fields(s)
		if len(parts) >= 1 {
			seconds, _ := strconv.ParseFloat(parts[0], 64)
			m.RunDays = int(seconds / 86400)
		}
	}

	// OS version
	if s, ok := sections["VERSION"]; ok {
		m.Version = strings.TrimSpace(s)
	}

	// Kernel
	if s, ok := sections["KERNEL"]; ok {
		m.Kernel = strings.TrimSpace(s)
	}

	// Memory total and swap total
	if s, ok := sections["MEMINFO"]; ok {
		for _, line := range strings.Split(s, "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				val, _ := strconv.ParseFloat(fields[1], 64)
				switch {
				case strings.HasPrefix(line, "MemTotal:"):
					m.Memory = val / 1024 // KB to MB
				case strings.HasPrefix(line, "SwapTotal:"):
					m.Swap = val / 1024
				}
			}
		}
	}

	// CPU cores
	if s, ok := sections["CPUCOUNT"]; ok {
		m.Core, _ = strconv.Atoi(strings.TrimSpace(s))
	}

	// Hardware info (dmidecode)
	if s, ok := sections["DMIDECODE"]; ok {
		parts := strings.SplitN(s, "---", 3)
		if len(parts) >= 3 {
			m.Manufacturer = strings.TrimSpace(parts[0])
			m.Product = strings.TrimSpace(parts[1])
			m.Serial = strings.TrimSpace(parts[2])
		}
	}

	return m
}

// parseSections splits batch output by ===TAG=== markers.
func parseSections(output string) map[string]string {
	sections := make(map[string]string)
	lines := strings.Split(output, "\n")
	var currentTag string
	var sb strings.Builder

	for _, line := range lines {
		if strings.HasPrefix(line, "===") && strings.HasSuffix(line, "===") {
			if currentTag != "" {
				sections[currentTag] = strings.TrimSpace(sb.String())
				sb.Reset()
			}
			currentTag = strings.Trim(line, "=")
		} else if currentTag != "" {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}
	if currentTag != "" {
		sections[currentTag] = strings.TrimSpace(sb.String())
	}
	return sections
}

// parseCPUUsage calculates CPU usage from two /proc/stat readings.
func parseCPUUsage(stat1, stat2 string) float64 {
	parse := func(line string) (idle, total float64) {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 0, 0
		}
		// fields[0] = "cpu", fields[1..] = user nice system idle iowait irq softirq steal
		var vals []float64
		for _, f := range fields[1:] {
			v, _ := strconv.ParseFloat(f, 64)
			vals = append(vals, v)
			total += v
		}
		if len(vals) >= 4 {
			idle = vals[3] // idle is the 4th value
		}
		return idle, total
	}

	idle1, total1 := parse(stat1)
	idle2, total2 := parse(stat2)
	deltaTotal := total2 - total1
	deltaIdle := idle2 - idle1
	if deltaTotal == 0 {
		return 0
	}
	usage := (1 - deltaIdle/deltaTotal) * 100
	// Round to 2 decimal places
	return float64(int(usage*100)) / 100
}

// parseFree parses `free -b` output for memory and swap usage percentages.
func parseFree(output string) (memUsage, swapUsage float64) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if strings.HasPrefix(line, "Mem:") && len(fields) >= 3 {
			total, _ := strconv.ParseFloat(fields[1], 64)
			used, _ := strconv.ParseFloat(fields[2], 64)
			if total > 0 {
				memUsage = float64(int(used/total*10000)) / 100
			}
		}
		if strings.HasPrefix(line, "Swap:") && len(fields) >= 3 {
			total, _ := strconv.ParseFloat(fields[1], 64)
			used, _ := strconv.ParseFloat(fields[2], 64)
			if total > 0 {
				swapUsage = float64(int(used/total*10000)) / 100
			}
		}
	}
	return
}

// parseDiskUsage parses df output for specific mount points.
func parseDiskUsage(output, mountPaths string) []model.DiskUsage {
	paths := strings.Split(mountPaths, ":")
	pathSet := make(map[string]bool)
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p != "" {
			pathSet[p] = true
		}
	}

	var results []model.DiskUsage
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		mountPoint := fields[len(fields)-1]
		if pathSet[mountPoint] {
			usage := fields[len(fields)-2] // e.g. "34%"
			du := model.DiskUsage{
				MountPoint: mountPoint,
				Usage:      usage,
			}
			// Parse numeric for rule checking
			numStr := strings.TrimSuffix(usage, "%")
			du.UsageFloat, _ = strconv.ParseFloat(numStr, 64)
			results = append(results, du)
		}
	}
	return results
}

// parseSSOutput parses `ss -s` output for TCP connection counts.
func parseSSOutput(output string) model.NetworkStats {
	stats := model.NetworkStats{}
	// ss -s gives summary; for detailed counts we need netstat-style parsing
	// Try to parse common patterns from ss output
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		// ss -s typically outputs: "TCP:   123 (estab 45, closed 10, orphaned 0, timewait 20)"
		if strings.HasPrefix(line, "TCP:") {
			// Parse established
			if idx := strings.Index(line, "estab "); idx >= 0 {
				val := extractNumber(line[idx+6:])
				stats.Established = val
			}
			if idx := strings.Index(line, "timewait "); idx >= 0 {
				val := extractNumber(line[idx+9:])
				stats.TimeWait = val
			}
		}
	}
	return stats
}

func extractNumber(s string) int {
	var numStr strings.Builder
	for _, c := range s {
		if c >= '0' && c <= '9' {
			numStr.WriteRune(c)
		} else {
			break
		}
	}
	v, _ := strconv.Atoi(numStr.String())
	return v
}
