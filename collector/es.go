package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"

	"weops-inspect/config"
	"weops-inspect/model"
)

// CollectES collects Elasticsearch cluster health and node metrics.
// HTTP layer still uses the local `curl` binary by design; the Probe wrapper
// brings ctx-based timeouts and error classification.
func CollectES(ctx context.Context, cfg *config.Config) []model.ESCluster {
	if len(cfg.ES7IPs) == 0 {
		return nil
	}
	if _, err := exec.LookPath("curl"); err != nil {
		return []model.ESCluster{{Error: "curl CLI not available", ErrorClass: string(ErrUnknown)}}
	}

	host := cfg.ES7IPs[0]
	port := "9200"
	instance := net.JoinHostPort(host, port)
	cluster := model.ESCluster{Instance: instance}

	auth := ""
	if cfg.Creds.ES7Password != "" {
		auth = fmt.Sprintf("elastic:%s@", cfg.Creds.ES7Password)
	}

	p := &esProbe{host: host, port: port, auth: auth, target: instance, cluster: &cluster}
	RunProbe(ctx, p)

	return []model.ESCluster{cluster}
}

type esProbe struct {
	host, port, auth string
	target           string
	cluster          *model.ESCluster
}

func (p *esProbe) Name() string { return "elasticsearch" }

func (p *esProbe) Run(ctx context.Context) ProbeResult {
	healthURL := fmt.Sprintf("http://%s%s:%s/_cluster/health", p.auth, p.host, p.port)
	out, err := curlGet(ctx, healthURL)
	if err != nil {
		p.cluster.Error = fmt.Sprintf("curl error: %v", err)
		p.cluster.ErrorClass = string(curlErrClass(err))
		return ProbeResult{Target: p.target, Err: err, ErrClass: curlErrClass(err)}
	}

	var healthResp map[string]interface{}
	if err := json.Unmarshal(out, &healthResp); err == nil {
		p.cluster.ClusterName, _ = healthResp["cluster_name"].(string)
		p.cluster.Status, _ = healthResp["status"].(string)
		p.cluster.NumberOfNodes = jsonInt(healthResp["number_of_nodes"])
		p.cluster.NumberOfDataNodes = jsonInt(healthResp["number_of_data_nodes"])
		p.cluster.ActivePrimaryShards = jsonInt(healthResp["active_primary_shards"])
		p.cluster.ActiveShards = jsonInt(healthResp["active_shards"])
		p.cluster.UnassignedShards = jsonInt(healthResp["unassigned_shards"])
		p.cluster.PendingTasks = jsonInt(healthResp["number_of_pending_tasks"])
		if v, ok := healthResp["active_shards_percent_as_number"].(float64); ok {
			p.cluster.ActiveShardsPercent = v
		}
	}

	nodesURL := fmt.Sprintf("http://%s%s:%s/_cat/nodes?format=json&h=ip,heap.percent,ram.percent,cpu,load_1m,load_5m,load_15m,node.role", p.auth, p.host, p.port)
	if out, err := curlGet(ctx, nodesURL); err == nil {
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
				p.cluster.Nodes = append(p.cluster.Nodes, node)
			}
		}
	}

	return ProbeResult{Target: p.target}
}

func curlGet(ctx context.Context, url string) ([]byte, error) {
	return exec.CommandContext(ctx, "curl", "-s", "-S", "--max-time", "5", url).Output()
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
