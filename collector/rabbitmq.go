package collector

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"weops-inspect/config"
	"weops-inspect/model"
)

// CollectRabbitMQ collects RabbitMQ cluster status via Management API.
func CollectRabbitMQ(cfg *config.Config) *model.RabbitMQStatus {
	if len(cfg.RabbitMQIPs) == 0 {
		return nil
	}
	if _, err := exec.LookPath("curl"); err != nil {
		return &model.RabbitMQStatus{Error: "curl CLI not available"}
	}

	host := cfg.RabbitMQIPs[0]
	port := "15672"
	user := cfg.Creds.RabbitMQUser
	pass := cfg.Creds.RabbitMQPassword

	status := &model.RabbitMQStatus{}

	// Fetch nodes
	nodesJSON := rmqAPI(host, port, user, pass, "nodes")
	if nodesJSON != nil {
		var nodes []map[string]interface{}
		if json.Unmarshal(nodesJSON, &nodes) == nil {
			for _, n := range nodes {
				alarm := model.RabbitMQAlarm{
					Node: jsonStr(n["name"]),
				}
				if v, ok := n["mem_alarm"].(bool); ok {
					alarm.MemAlarm = v
				}
				if v, ok := n["disk_free_alarm"].(bool); ok {
					alarm.DiskFreeAlarm = v
				}
				if alarm.MemAlarm || alarm.DiskFreeAlarm {
					status.NodeAlarms = append(status.NodeAlarms, alarm)
				}

				// Cluster partition check
				if parts, ok := n["partitions"].([]interface{}); ok && len(parts) > 0 {
					status.ClusterPartition = true
				}

				// Uptime from first node
				if status.Uptime == "" {
					if uptimeMs, ok := n["uptime"].(float64); ok {
						secs := int(uptimeMs / 1000)
						days := secs / 86400
						hours := (secs % 86400) / 3600
						mins := (secs % 3600) / 60
						status.Uptime = fmt.Sprintf("%dd %dh %dm", days, hours, mins)
					}
				}
			}
		}
	}

	// Fetch connections
	connsJSON := rmqAPI(host, port, user, pass, "connections")
	if connsJSON != nil {
		var conns []map[string]interface{}
		if json.Unmarshal(connsJSON, &conns) == nil {
			status.TotalConnections = len(conns)
			for _, c := range conns {
				state := jsonStr(c["state"])
				if state != "running" && state != "" {
					status.AbnormalConnections++
				}
			}
		}
	}

	// Fetch channels
	chansJSON := rmqAPI(host, port, user, pass, "channels")
	if chansJSON != nil {
		var chans []interface{}
		if json.Unmarshal(chansJSON, &chans) == nil {
			status.TotalChannels = len(chans)
		}
	}

	// Fetch queues
	queuesJSON := rmqAPI(host, port, user, pass, "queues")
	if queuesJSON != nil {
		var queues []map[string]interface{}
		if json.Unmarshal(queuesJSON, &queues) == nil {
			for _, q := range queues {
				msgs := jsonInt(q["messages"])
				consumers := jsonInt(q["consumers"])
				queueInfo := model.RabbitMQQueue{
					VHost:        jsonStr(q["vhost"]),
					Queue:        jsonStr(q["name"]),
					MessageCount: msgs,
					Consumers:    consumers,
				}
				if v, ok := q["durable"].(bool); ok {
					queueInfo.Durable = v
				}

				if msgs > 1000 {
					status.ExceedingQueues = append(status.ExceedingQueues, queueInfo)
				}
				if consumers == 0 && msgs > 0 {
					status.NoConsumerQueues = append(status.NoConsumerQueues, queueInfo)
				}
			}
		}
	}

	return status
}

func rmqAPI(host, port, user, pass, endpoint string) []byte {
	url := fmt.Sprintf("http://%s:%s/api/%s", host, port, endpoint)
	authHeader := fmt.Sprintf("%s:%s", user, pass)

	args := []string{"-s", "-u", authHeader, "-H", "Accept: application/json", url}
	out, err := exec.Command("curl", args...).Output()
	if err != nil {
		return nil
	}
	// Validate it's JSON
	trimmed := strings.TrimSpace(string(out))
	if len(trimmed) == 0 || (trimmed[0] != '[' && trimmed[0] != '{') {
		return nil
	}
	return out
}
