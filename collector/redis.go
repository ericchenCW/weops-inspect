package collector

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"weops-inspect/config"
	"weops-inspect/model"
)

// CollectRedisStandalone probes each standalone Redis node in cfg.RedisIPs
// individually using redis-cli INFO.
func CollectRedisStandalone(cfg *config.Config) []model.RedisNode {
	if len(cfg.RedisIPs) == 0 {
		return nil
	}
	if _, err := exec.LookPath("redis-cli"); err != nil {
		return []model.RedisNode{{Error: "redis-cli not available"}}
	}

	var nodes []model.RedisNode
	for _, ip := range cfg.RedisIPs {
		nodes = append(nodes, collectRedisNode(ip, cfg.RedisPort, cfg.Creds.RedisPassword))
	}
	return nodes
}

func collectRedisNode(ip, port, password string) model.RedisNode {
	args := []string{"-h", ip, "-p", port}
	if password != "" {
		args = append(args, "-a", password, "--no-auth-warning")
	}

	infoArgs := append(append([]string{}, args...), "INFO")
	out, err := exec.Command("redis-cli", infoArgs...).Output()
	if err != nil {
		return model.RedisNode{IP: ip, Error: fmt.Sprintf("redis-cli error: %v", err)}
	}

	info := parseRedisInfo(string(out))
	node := model.RedisNode{
		IP:               ip,
		Version:          info["redis_version"],
		Role:             info["role"],
		ClusterEnabled:   info["cluster_enabled"],
		UsedMemory:       info["used_memory"],
		MaxMemory:        info["maxmemory"],
		UptimeDays:       info["uptime_in_days"],
		ConnectedClients: info["connected_clients"],
		BlockedClients:   info["blocked_clients"],
	}

	celeryArgs := append(append([]string{}, args...), "-n", "11", "LLEN", "celery")
	if out, err := exec.Command("redis-cli", celeryArgs...).Output(); err == nil {
		node.CeleryQueue, _ = strconv.Atoi(strings.TrimSpace(string(out)))
	}
	monitorArgs := append(append([]string{}, args...), "-n", "11", "LLEN", "monitor")
	if out, err := exec.Command("redis-cli", monitorArgs...).Output(); err == nil {
		node.MonitorQueue, _ = strconv.Atoi(strings.TrimSpace(string(out)))
	}

	return node
}

// CollectRedisSentinel probes each sentinel node in cfg.RedisSentinelIPs and
// derives cluster-level health (master discovery + master reachability).
//
// Status is computed as:
//   - critical: all sentinels unreachable, OR master cannot be discovered, OR
//     discovered master is unreachable
//   - warn: any single sentinel unreachable, OR no sentinels configured
//   - ok: all sentinels reachable AND master discovered AND master reachable
func CollectRedisSentinel(cfg *config.Config) *model.SentinelClusterStatus {
	if len(cfg.RedisSentinelIPs) == 0 {
		return nil
	}
	if _, err := exec.LookPath("redis-cli"); err != nil {
		return &model.SentinelClusterStatus{Error: "redis-cli not available", Status: "critical"}
	}

	masterName := envOrDefault("BK_APIGW_REDIS_SENTINEL_MASTER_NAME", "mymaster")
	sentinelPort := envOrDefault("INSPECT_REDIS_SENTINEL_PORT", "26379")

	st := &model.SentinelClusterStatus{
		MasterName: masterName,
	}

	reachableCount := 0
	for _, ip := range cfg.RedisSentinelIPs {
		s := pingSentinel(ip, sentinelPort)
		st.Sentinels = append(st.Sentinels, s)
		if s.Reachable {
			reachableCount++
		}
	}

	if reachableCount == 0 {
		st.Status = "critical"
		st.Error = "no sentinel reachable"
		return st
	}

	// Discover master via the first reachable sentinel.
	for _, s := range st.Sentinels {
		if !s.Reachable {
			continue
		}
		masterIP, masterPort, err := sentinelGetMaster(s.IP, s.Port, masterName)
		if err == nil && masterIP != "" {
			st.DiscoveredMaster = masterIP + ":" + masterPort
			st.MasterReachable = redisPing(masterIP, masterPort, cfg.Creds.RedisPassword)
			break
		}
	}

	switch {
	case st.DiscoveredMaster == "" || !st.MasterReachable:
		st.Status = "critical"
	case reachableCount < len(cfg.RedisSentinelIPs):
		st.Status = "warn"
	default:
		st.Status = "ok"
	}
	return st
}

func pingSentinel(ip, port string) model.SentinelNodeStatus {
	s := model.SentinelNodeStatus{IP: ip, Port: port}
	out, err := exec.Command("redis-cli", "-h", ip, "-p", port, "PING").CombinedOutput()
	if err != nil {
		s.Error = fmt.Sprintf("ping error: %v", err)
		return s
	}
	if strings.TrimSpace(string(out)) == "PONG" {
		s.Reachable = true
	} else {
		s.Error = "unexpected response: " + strings.TrimSpace(string(out))
	}
	return s
}

func sentinelGetMaster(ip, port, masterName string) (string, string, error) {
	out, err := exec.Command("redis-cli", "-h", ip, "-p", port,
		"SENTINEL", "get-master-addr-by-name", masterName).CombinedOutput()
	if err != nil {
		return "", "", err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return "", "", fmt.Errorf("unexpected sentinel reply: %q", string(out))
	}
	return strings.TrimSpace(lines[0]), strings.TrimSpace(lines[1]), nil
}

func redisPing(ip, port, password string) bool {
	args := []string{"-h", ip, "-p", port}
	if password != "" {
		args = append(args, "-a", password, "--no-auth-warning")
	}
	args = append(args, "PING")
	out, err := exec.Command("redis-cli", args...).CombinedOutput()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "PONG"
}

func parseRedisInfo(output string) map[string]string {
	info := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.Index(line, ":"); idx > 0 {
			info[line[:idx]] = line[idx+1:]
		}
	}
	return info
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
