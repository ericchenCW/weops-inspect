package collector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"sort"
	"strings"

	"weops-inspect/config"
	"weops-inspect/model"
)

// CollectRabbitMQ collects RabbitMQ cluster status via the Management HTTP API.
// Implementation still calls the local `curl` binary by design; only the
// network layer is wrapped in the Probe framework for ctx/error-class parity.
func CollectRabbitMQ(ctx context.Context, cfg *config.Config) *model.RabbitMQStatus {
	if len(cfg.RabbitMQIPs) == 0 {
		return nil
	}
	if _, err := exec.LookPath("curl"); err != nil {
		return &model.RabbitMQStatus{Error: "curl CLI not available", ErrorClass: string(ErrUnknown)}
	}

	host := cfg.RabbitMQIPs[0]
	port := "15672"
	user := cfg.Creds.RabbitMQUser
	pass := cfg.Creds.RabbitMQPassword

	status := &model.RabbitMQStatus{}
	target := net.JoinHostPort(host, port)

	probe := &rmqProbe{
		host: host, port: port, user: user, pass: pass,
		target: target, status: status,
		backlogThreshold: cfg.Thresholds.RabbitMQQueueBacklog,
	}
	RunProbe(ctx, probe)
	return status
}

type rmqProbe struct {
	host, port, user, pass string
	target                 string
	status                 *model.RabbitMQStatus
	backlogThreshold       int
}

func (p *rmqProbe) Name() string { return "rabbitmq" }

func (p *rmqProbe) Run(ctx context.Context) ProbeResult {
	if nodesJSON, err := rmqAPI(ctx, p.host, p.port, p.user, p.pass, "nodes"); err != nil {
		p.status.Error = err.Error()
		p.status.ErrorClass = string(curlErrClass(err))
		return ProbeResult{Target: p.target, Err: err, ErrClass: curlErrClass(err)}
	} else if nodesJSON != nil {
		var nodes []map[string]interface{}
		if json.Unmarshal(nodesJSON, &nodes) == nil {
			for _, n := range nodes {
				alarm := model.RabbitMQAlarm{Node: jsonStr(n["name"])}
				if v, ok := n["mem_alarm"].(bool); ok {
					alarm.MemAlarm = v
				}
				if v, ok := n["disk_free_alarm"].(bool); ok {
					alarm.DiskFreeAlarm = v
				}
				if alarm.MemAlarm || alarm.DiskFreeAlarm {
					p.status.NodeAlarms = append(p.status.NodeAlarms, alarm)
				}
				if parts, ok := n["partitions"].([]interface{}); ok && len(parts) > 0 {
					p.status.ClusterPartition = true
				}
				if p.status.Uptime == "" {
					if uptimeMs, ok := n["uptime"].(float64); ok {
						secs := int(uptimeMs / 1000)
						days := secs / 86400
						hours := (secs % 86400) / 3600
						mins := (secs % 3600) / 60
						p.status.Uptime = fmt.Sprintf("%dd %dh %dm", days, hours, mins)
					}
				}
			}
		}
	}

	if connsJSON, err := rmqAPI(ctx, p.host, p.port, p.user, p.pass, "connections"); err == nil && connsJSON != nil {
		var conns []map[string]interface{}
		if json.Unmarshal(connsJSON, &conns) == nil {
			p.status.TotalConnections = len(conns)
			for _, c := range conns {
				state := jsonStr(c["state"])
				if state != "running" && state != "" {
					p.status.AbnormalConnections++
				}
			}
		}
	}

	if chansJSON, err := rmqAPI(ctx, p.host, p.port, p.user, p.pass, "channels"); err == nil && chansJSON != nil {
		var chans []interface{}
		if json.Unmarshal(chansJSON, &chans) == nil {
			p.status.TotalChannels = len(chans)
		}
	}

	if queuesJSON, err := rmqAPI(ctx, p.host, p.port, p.user, p.pass, "queues"); err == nil && queuesJSON != nil {
		var queues []map[string]interface{}
		if json.Unmarshal(queuesJSON, &queues) == nil {
			summaryByVHost := map[string]*model.RabbitMQVHostSummary{}
			for _, q := range queues {
				vhost := jsonStr(q["vhost"])
				name := jsonStr(q["name"])
				if vhost == "bk_usermgr" || strings.HasPrefix(name, "celeryev") {
					continue
				}
				msgs := jsonInt(q["messages"])
				consumers := jsonInt(q["consumers"])

				agg, ok := summaryByVHost[vhost]
				if !ok {
					agg = &model.RabbitMQVHostSummary{VHost: vhost}
					summaryByVHost[vhost] = agg
				}
				agg.Queues++
				agg.Messages += msgs
				agg.Consumers += consumers

				queueInfo := model.RabbitMQQueue{
					VHost:        vhost,
					Queue:        name,
					MessageCount: msgs,
					Consumers:    consumers,
				}
				if v, ok := q["durable"].(bool); ok {
					queueInfo.Durable = v
				}
				if msgs >= p.backlogThreshold {
					p.status.ExceedingQueues = append(p.status.ExceedingQueues, queueInfo)
				}
				if consumers == 0 && msgs > 0 {
					p.status.NoConsumerQueues = append(p.status.NoConsumerQueues, queueInfo)
				}
			}
			vhosts := make([]string, 0, len(summaryByVHost))
			for v := range summaryByVHost {
				vhosts = append(vhosts, v)
			}
			sort.Strings(vhosts)
			for _, v := range vhosts {
				p.status.VHostSummary = append(p.status.VHostSummary, *summaryByVHost[v])
			}
		}
	}

	return ProbeResult{Target: p.target}
}

func rmqAPI(ctx context.Context, host, port, user, pass, endpoint string) ([]byte, error) {
	url := fmt.Sprintf("http://%s:%s/api/%s", host, port, endpoint)
	authHeader := fmt.Sprintf("%s:%s", user, pass)

	args := []string{"-s", "-S", "--max-time", "5", "-u", authHeader, "-H", "Accept: application/json", url}
	out, err := exec.CommandContext(ctx, "curl", args...).Output()
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(string(out))
	if len(trimmed) == 0 || (trimmed[0] != '[' && trimmed[0] != '{') {
		return nil, fmt.Errorf("rabbitmq api %s: non-JSON response", endpoint)
	}
	return out, nil
}

// curlErrClass 从 curl 的 exit code 反推 ErrorClass。
func curlErrClass(err error) ErrorClass {
	if err == nil {
		return ErrNone
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		return Classify(err)
	}
	switch ee.ExitCode() {
	case 6, 7: // couldn't resolve / couldn't connect
		return ErrNetwork
	case 22: // HTTP non-2xx
		return ErrProtocol
	case 28: // operation timed out
		return ErrTimeout
	case 67: // login failed
		return ErrAuth
	}
	return ErrUnknown
}
