package collector

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"weops-inspect/config"
	"weops-inspect/model"
)

// CollectRedis collects Redis node metrics.
func CollectRedis(cfg *config.Config) []model.RedisCluster {
	if cfg.RedisIP == "" {
		return nil
	}
	if _, err := exec.LookPath("redis-cli"); err != nil {
		return []model.RedisCluster{{Error: "redis-cli not available"}}
	}

	instance := fmt.Sprintf("%s:%s", cfg.RedisIP, cfg.RedisPort)
	cluster := model.RedisCluster{Instance: instance}

	// Get INFO
	args := []string{"-h", cfg.RedisIP, "-p", cfg.RedisPort}
	if cfg.Creds.RedisPassword != "" {
		args = append(args, "-a", cfg.Creds.RedisPassword, "--no-auth-warning")
	}

	infoArgs := append(append([]string{}, args...), "INFO")
	out, err := exec.Command("redis-cli", infoArgs...).Output()
	if err != nil {
		cluster.Error = fmt.Sprintf("redis-cli error: %v", err)
		return []model.RedisCluster{cluster}
	}

	node := model.RedisNode{IP: cfg.RedisIP}
	info := parseRedisInfo(string(out))

	node.Version = info["redis_version"]
	node.Role = info["role"]
	node.ClusterEnabled = info["cluster_enabled"]
	node.UsedMemory = info["used_memory"]
	node.MaxMemory = info["maxmemory"]
	node.UptimeDays = info["uptime_in_days"]
	node.ConnectedClients = info["connected_clients"]
	node.BlockedClients = info["blocked_clients"]

	// Queue lengths
	celeryArgs := append(append([]string{}, args...), "-n", "11", "LLEN", "celery")
	if out, err := exec.Command("redis-cli", celeryArgs...).Output(); err == nil {
		node.CeleryQueue, _ = strconv.Atoi(strings.TrimSpace(string(out)))
	}

	monitorArgs := append(append([]string{}, args...), "-n", "11", "LLEN", "monitor")
	if out, err := exec.Command("redis-cli", monitorArgs...).Output(); err == nil {
		node.MonitorQueue, _ = strconv.Atoi(strings.TrimSpace(string(out)))
	}

	cluster.Nodes = append(cluster.Nodes, node)
	return []model.RedisCluster{cluster}
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
