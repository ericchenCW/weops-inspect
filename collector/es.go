package collector

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"weops-inspect/config"
	"weops-inspect/model"
)

// CollectES collects Elasticsearch cluster health and node metrics.
func CollectES(cfg *config.Config) []model.ESCluster {
	if len(cfg.ES7IPs) == 0 {
		return nil
	}
	if _, err := exec.LookPath("curl"); err != nil {
		return []model.ESCluster{{Error: "curl CLI not available"}}
	}

	var clusters []model.ESCluster
	host := cfg.ES7IPs[0]
	port := "9200"
	instance := fmt.Sprintf("%s:%s", host, port)

	auth := ""
	if cfg.Creds.ES7Password != "" {
		auth = fmt.Sprintf("elastic:%s@", cfg.Creds.ES7Password)
	}

	cluster := model.ESCluster{Instance: instance}

	// Cluster health
	healthURL := fmt.Sprintf("http://%s%s:%s/_cluster/health", auth, host, port)
	out, err := exec.Command("curl", "-s", "--connect-timeout", "5", healthURL).Output()
	if err != nil {
		cluster.Error = fmt.Sprintf("curl error: %v", err)
		return []model.ESCluster{cluster}
	}

	var healthResp map[string]interface{}
	if err := json.Unmarshal(out, &healthResp); err == nil {
		cluster.ClusterName, _ = healthResp["cluster_name"].(string)
		cluster.Status, _ = healthResp["status"].(string)
		cluster.NumberOfNodes = jsonInt(healthResp["number_of_nodes"])
		cluster.NumberOfDataNodes = jsonInt(healthResp["number_of_data_nodes"])
		cluster.ActivePrimaryShards = jsonInt(healthResp["active_primary_shards"])
		cluster.ActiveShards = jsonInt(healthResp["active_shards"])
		cluster.UnassignedShards = jsonInt(healthResp["unassigned_shards"])
		cluster.PendingTasks = jsonInt(healthResp["number_of_pending_tasks"])
		if v, ok := healthResp["active_shards_percent_as_number"].(float64); ok {
			cluster.ActiveShardsPercent = v
		}
	}

	// Nodes info
	nodesURL := fmt.Sprintf("http://%s%s:%s/_cat/nodes?format=json&h=ip,heap.percent,ram.percent,cpu,load_1m,load_5m,load_15m,node.role", auth, host, port)
	out, err = exec.Command("curl", "-s", "--connect-timeout", "5", nodesURL).Output()
	if err == nil {
		var nodesResp []map[string]interface{}
		if json.Unmarshal(out, &nodesResp) == nil {
			for _, n := range nodesResp {
				node := model.ESNode{
					IP:          jsonStr(n["ip"]),
					HeapPercent: jsonIntFromStr(jsonStr(n["heap.percent"])),
					RAMPercent:  jsonIntFromStr(jsonStr(n["ram.percent"])),
					CPU:         jsonIntFromStr(jsonStr(n["cpu"])),
				}
				node.Load1m, _ = strconv.ParseFloat(jsonStr(n["load_1m"]), 64)
				node.Load5m, _ = strconv.ParseFloat(jsonStr(n["load_5m"]), 64)
				node.Load15m, _ = strconv.ParseFloat(jsonStr(n["load_15m"]), 64)
				role := jsonStr(n["node.role"])
				if strings.Contains(role, "m") {
					node.Role = "master"
				} else {
					node.Role = "data"
				}
				cluster.Nodes = append(cluster.Nodes, node)
			}
		}
	}

	clusters = append(clusters, cluster)
	return clusters
}

func jsonInt(v interface{}) int {
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}

func jsonStr(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func jsonIntFromStr(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}
