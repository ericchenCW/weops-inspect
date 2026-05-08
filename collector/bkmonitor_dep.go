package collector

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"weops-inspect/config"
	"weops-inspect/model"
)

// CollectBKMonitorV3Deps probes bkmonitorv3 dependency endpoints from the
// inspector host, mirroring bk-install/health_check/deploy_check.py:bkmonitorv3().
// Returns nil when bkmonitorv3 is not deployed (即 BK_MONITORV3_IP_COMMA 与 4 个角色
// IP 列表全部为空)。
func CollectBKMonitorV3Deps(cfg *config.Config) []model.DependencyResult {
	if len(cfg.MonitorV3IPs) == 0 &&
		len(cfg.MonitorV3MonitorIPs) == 0 &&
		len(cfg.MonitorV3InfluxDBProxyIPs) == 0 &&
		len(cfg.MonitorV3TransferIPs) == 0 &&
		len(cfg.MonitorV3UnifyQueryIPs) == 0 {
		return nil
	}

	dep := cfg.BKMonitorV3
	results := []model.DependencyResult{
		probeRedisLogin("redis", dep.RedisHost, dep.RedisPort, dep.RedisPassword),
		probeMySQLLogin("mysql", dep.MySQLHost, dep.MySQLPort, dep.MySQLUser, dep.MySQLPassword),
		probeRabbitMQ("rabbitmq", dep.RabbitMQHost, dep.RabbitMQPort, dep.RabbitMQUser, dep.RabbitMQPassword, dep.RabbitMQVHost),
		probeZookeeper("zookeeper", dep.ZKHost, dep.ZKPort),
		probeES7("es7", dep.ES7Host, dep.ES7RestPort, dep.ES7User, dep.ES7Password),
		probeInfluxDB("influxdb", dep.InfluxDBHost, dep.InfluxDBPort),
	}
	return results
}

func skipIfMissing(item, endpoint string, missing bool) (model.DependencyResult, bool) {
	if missing {
		return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "skip", Detail: "missing config"}, true
	}
	return model.DependencyResult{}, false
}

func probeRedisLogin(item, host, port, password string) model.DependencyResult {
	endpoint := fmt.Sprintf("%s:%s", host, port)
	if r, ok := skipIfMissing(item, endpoint, host == "" || port == ""); ok {
		return r
	}
	if _, err := exec.LookPath("redis-cli"); err != nil {
		return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "unreachable", Detail: "redis-cli not available"}
	}
	args := []string{"-h", host, "-p", port}
	if password != "" {
		args = append(args, "-a", password, "--no-auth-warning")
	}
	args = append(args, "PING")
	out, err := runWithTimeout(5*time.Second, "redis-cli", args...)
	out = strings.TrimSpace(out)
	if err != nil {
		return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "unreachable", Detail: err.Error()}
	}
	if out == "PONG" {
		return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "ok"}
	}
	return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "fail", Detail: out}
}

func probeMySQLLogin(item, host, port, user, password string) model.DependencyResult {
	endpoint := fmt.Sprintf("%s:%s", host, port)
	if r, ok := skipIfMissing(item, endpoint, host == "" || port == "" || user == ""); ok {
		return r
	}
	if _, err := exec.LookPath("mysql"); err != nil {
		return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "unreachable", Detail: "mysql CLI not available"}
	}
	args := []string{
		fmt.Sprintf("-u%s", user),
		fmt.Sprintf("-p%s", password),
		fmt.Sprintf("-h%s", host),
		fmt.Sprintf("-P%s", port),
		"-N", "-s",
		"-e", "SELECT 1",
	}
	out, err := runWithTimeout(8*time.Second, "mysql", args...)
	if err != nil {
		return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "fail", Detail: firstLine(out, err)}
	}
	if strings.TrimSpace(out) == "1" {
		return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "ok"}
	}
	return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "fail", Detail: strings.TrimSpace(out)}
}

func probeRabbitMQ(item, host, port, user, password, vhost string) model.DependencyResult {
	endpoint := fmt.Sprintf("%s:%s", host, port)
	if r, ok := skipIfMissing(item, endpoint, host == "" || port == "" || user == ""); ok {
		return r
	}
	// Management API on port+10000 is the conventional layout (5672 → 15672) but
	// not guaranteed; use AMQP TCP-level auth via rabbitmqctl-less curl is
	// unreliable. We instead rely on a TCP connect + AMQP protocol header probe.
	addr := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "unreachable", Detail: err.Error()}
	}
	defer conn.Close()
	// AMQP 0-9-1 protocol header: "AMQP" + 0,0,9,1
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte{'A', 'M', 'Q', 'P', 0, 0, 9, 1}); err != nil {
		return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "fail", Detail: err.Error()}
	}
	buf := make([]byte, 8)
	n, err := conn.Read(buf)
	if err != nil || n == 0 {
		return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "fail", Detail: "no AMQP response"}
	}
	// Broker either closes on bad header or proceeds with Connection.Start frame.
	// Either way TCP+protocol handshake is enough to confirm the broker is alive
	// at the AMQP layer; full credential check would need an AMQP client lib.
	detail := fmt.Sprintf("vhost=%s (auth not validated)", vhost)
	return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "ok", Detail: detail}
}

func probeZookeeper(item, host, port string) model.DependencyResult {
	endpoint := fmt.Sprintf("%s:%s", host, port)
	if r, ok := skipIfMissing(item, endpoint, host == "" || port == ""); ok {
		return r
	}
	// 蓝鲸 ZK 默认 4lw whitelist 仅放 `srvr`,`ruok` 被禁。这里只做 TCP 连通性
	// 校验,能建连即视为可达;Detail 中记下取舍便于运维复核。
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 5*time.Second)
	if err != nil {
		return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "unreachable", Detail: err.Error()}
	}
	_ = conn.Close()
	return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "ok", Detail: "TCP-only probe; 4lw whitelist may exclude ruok"}
}

func probeES7(item, host, port, user, password string) model.DependencyResult {
	endpoint := fmt.Sprintf("%s:%s", host, port)
	if r, ok := skipIfMissing(item, endpoint, host == "" || port == ""); ok {
		return r
	}
	if _, err := exec.LookPath("curl"); err != nil {
		return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "unreachable", Detail: "curl not available"}
	}
	url := fmt.Sprintf("http://%s:%s/_cluster/health", host, port)
	args := []string{"-s", "--connect-timeout", "5", "-o", "/dev/null", "-w", "%{http_code}"}
	if user != "" {
		args = append(args, "-u", user+":"+password)
	}
	args = append(args, url)
	out, err := runWithTimeout(8*time.Second, "curl", args...)
	if err != nil {
		return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "unreachable", Detail: err.Error()}
	}
	code := strings.TrimSpace(out)
	if code == "200" {
		return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "ok"}
	}
	if code == "401" || code == "403" {
		return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "fail", Detail: "auth failed (HTTP " + code + ")"}
	}
	if code == "" || code == "000" {
		return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "unreachable", Detail: "no HTTP response"}
	}
	return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "fail", Detail: "HTTP " + code}
}

func probeInfluxDB(item, host, port string) model.DependencyResult {
	endpoint := fmt.Sprintf("%s:%s", host, port)
	if r, ok := skipIfMissing(item, endpoint, host == "" || port == ""); ok {
		return r
	}
	if _, err := exec.LookPath("curl"); err != nil {
		return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "unreachable", Detail: "curl not available"}
	}
	url := fmt.Sprintf("http://%s:%s/ping", host, port)
	args := []string{"-s", "--connect-timeout", "5", "-o", "/dev/null", "-w", "%{http_code}", url}
	out, err := runWithTimeout(8*time.Second, "curl", args...)
	if err != nil {
		return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "unreachable", Detail: err.Error()}
	}
	code := strings.TrimSpace(out)
	// InfluxDB 1.x returns 204 for /ping, 2.x returns 204 as well but /health returns 200.
	if code == "204" || code == "200" {
		return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "ok"}
	}
	if code == "" || code == "000" {
		return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "unreachable", Detail: "no HTTP response"}
	}
	return model.DependencyResult{Item: item, Endpoint: endpoint, Status: "fail", Detail: "HTTP " + code}
}

func runWithTimeout(timeout time.Duration, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	type result struct {
		out []byte
		err error
	}
	ch := make(chan result, 1)
	go func() {
		out, err := cmd.CombinedOutput()
		ch <- result{out, err}
	}()
	select {
	case r := <-ch:
		return string(r.out), r.err
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return "", fmt.Errorf("timeout after %s", timeout)
	}
}

func firstLine(out string, err error) string {
	s := strings.TrimSpace(out)
	if s == "" {
		if err != nil {
			return err.Error()
		}
		return ""
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
