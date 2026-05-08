package collector

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"weops-inspect/config"
	"weops-inspect/model"
	sshclient "weops-inspect/ssh"
)

// CollectAllServices collects service status for all modules on their respective hosts.
func CollectAllServices(client *sshclient.Client, cfg *config.Config) map[string][]model.ServiceStatus {
	results := make(map[string][]model.ServiceStatus)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, mh := range cfg.GetModuleHosts() {
		if len(mh.IPs) == 0 {
			continue
		}
		subModules, ok := ModuleRegistry[mh.Module]
		if !ok {
			continue
		}

		for _, ip := range mh.IPs {
			wg.Add(1)
			go func(module, host string, subs []SubModule) {
				defer wg.Done()
				fmt.Fprintf(logWriter, "  检查 %s @ %s ...\n", module, host)
				status := collectServiceOnHost(client, module, host, subs)
				mu.Lock()
				results[module] = append(results[module], status)
				mu.Unlock()
			}(mh.Module, ip, subModules)
		}
	}
	wg.Wait()
	return results
}

func collectServiceOnHost(client *sshclient.Client, module, host string, subs []SubModule) model.ServiceStatus {
	status := model.ServiceStatus{
		HostIP: host,
		Module: module,
	}

	// Build batch command for all sub-modules.
	//
	// Helper bash functions are defined once at the top:
	//   hz_http URL TIMEOUT  – probes URL, prints HTTP code only on real success
	//                          (silently fails on connect error / "000" code)
	//   hz_body URL TIMEOUT  – probes URL, prints body only if non-empty
	// Both end output with a newline so parseSections can find section headers.
	var cmdParts []string
	cmdParts = append(cmdParts,
		`hz_http(){ c=$(curl -s --connect-timeout "$2" -o /dev/null -w "%{http_code}" "$1" 2>/dev/null); [ -n "$c" ] && [ "$c" != "000" ] && echo "$c"; }`,
		`hz_body(){ b=$(curl -s --connect-timeout "$2" "$1" 2>/dev/null); [ -n "$b" ] && echo "$b"; }`,
	)
	for _, sub := range subs {
		// systemctl status
		cmdParts = append(cmdParts, fmt.Sprintf(
			`echo "===SVC_%s==="; systemctl show %s.service --property=ActiveState,MainPID,ExecMainStartTimestamp 2>/dev/null || echo "ActiveState=not-found"`,
			sub.Name, sub.ServiceUnit,
		))
		// healthz check — try 127.0.0.1 first (most uwsgi/web deployments),
		// fall back to the host's bound IP (services that bind to BK_*_IP only,
		// e.g. cmdb_apiserver --addrport=<host>:<port>).
		switch sub.HealthzType {
		case "http_status", "http_alive":
			cmdParts = append(cmdParts, fmt.Sprintf(
				`echo "===HZ_%s==="; hz_http http://127.0.0.1:%d%s 2 || hz_http http://%s:%d%s 2 || echo unreachable`,
				sub.Name, sub.Port, sub.HealthzPath, host, sub.Port, sub.HealthzPath,
			))
		case "json_ok":
			cmdParts = append(cmdParts, fmt.Sprintf(
				`echo "===HZ_%s==="; hz_body http://127.0.0.1:%d%s 2 || hz_body http://%s:%d%s 2 || echo unreachable`,
				sub.Name, sub.Port, sub.HealthzPath, host, sub.Port, sub.HealthzPath,
			))
		case "json_up":
			cmdParts = append(cmdParts, fmt.Sprintf(
				`echo "===HZ_%s==="; hz_body http://127.0.0.1:%d%s 5 || hz_body http://%s:%d%s 5 || echo unreachable`,
				sub.Name, sub.Port, sub.HealthzPath, host, sub.Port, sub.HealthzPath,
			))
		default:
			cmdParts = append(cmdParts, fmt.Sprintf(`echo "===HZ_%s==="; echo "N/A"`, sub.Name))
		}
		// worker count
		cmdParts = append(cmdParts, fmt.Sprintf(
			`echo "===WK_%s==="; ps -ef | grep '%s' | grep -v grep | wc -l`,
			sub.Name, sub.ProcessName,
		))
	}

	// Docker container status for appo/appt
	if module == "appo" || module == "appt" {
		cmdParts = append(cmdParts, `echo "===DOCKER==="; docker ps -a --format '{{.Status}}' 2>/dev/null || echo "docker_unavailable"`)
	}

	cmd := strings.Join(cmdParts, "; ")
	out, err := client.Run(host, cmd)
	if err != nil && out == "" {
		status.Error = fmt.Sprintf("SSH error: %v", err)
		return status
	}

	sections := parseSections(out)

	for _, sub := range subs {
		sm := model.ServiceModule{
			Module: sub.Name,
		}

		// Parse systemctl output
		if svc, ok := sections["SVC_"+sub.Name]; ok {
			props := parseProperties(svc)
			sm.Status = props["ActiveState"]
			if sm.Status == "" {
				sm.Status = "unknown"
			}
			sm.MainPID, _ = strconv.Atoi(props["MainPID"])
			sm.StartTime = props["ExecMainStartTimestamp"]
		}

		// Parse healthz
		if hz, ok := sections["HZ_"+sub.Name]; ok {
			hz = strings.TrimSpace(hz)
			switch sub.HealthzType {
			case "http_status":
				if hz == "200" {
					sm.HealthzAPI = "ok"
				} else {
					sm.HealthzAPI = hz
				}
			case "http_alive":
				// Service has no formal healthz path — any HTTP response code
				// proves the port is bound and serving. Only "unreachable"
				// (curl couldn't connect) is a real failure.
				if hz == "unreachable" || hz == "" {
					sm.HealthzAPI = "unreachable"
				} else {
					sm.HealthzAPI = "ok"
				}
			case "json_ok":
				if strings.Contains(hz, `"ok"`) && (strings.Contains(hz, "true") || strings.Contains(hz, "True")) {
					sm.HealthzAPI = "ok"
				} else if hz == "unreachable" {
					sm.HealthzAPI = "unreachable"
				} else {
					sm.HealthzAPI = "fail"
				}
			case "json_up":
				if strings.Contains(strings.ToUpper(hz), `"UP"`) {
					sm.HealthzAPI = "ok"
				} else if hz == "unreachable" {
					sm.HealthzAPI = "unreachable"
				} else {
					sm.HealthzAPI = "fail"
				}
			default:
				sm.HealthzAPI = "N/A"
			}
		}

		// Parse workers
		if wk, ok := sections["WK_"+sub.Name]; ok {
			sm.Workers, _ = strconv.Atoi(strings.TrimSpace(wk))
		}

		// DNS resolution check is skipped (simplified per design)
		sm.Resolved = "N/A"

		status.Services = append(status.Services, sm)
	}

	// Docker containers
	if docker, ok := sections["DOCKER"]; ok {
		for _, line := range strings.Split(docker, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Up") {
				status.ContainersUp++
			} else if strings.HasPrefix(line, "Exited") {
				status.ContainersExited++
			}
		}
	}

	return status
}

// parseProperties parses "Key=Value" lines into a map.
func parseProperties(output string) map[string]string {
	props := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if idx := strings.Index(line, "="); idx > 0 {
			key := line[:idx]
			val := line[idx+1:]
			props[key] = val
		}
	}
	return props
}
