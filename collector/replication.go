package collector

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"weops-inspect/config"
	"weops-inspect/model"
)

// 段一过渡:replication.go 仍走 CLI 路径,保留私有 helper 直到段二切换到原生驱动。
func mysqlQuery(baseArgs []string, query string) string {
	args := append(append([]string{}, baseArgs...), "-e", query)
	out, err := exec.Command("mysql", args...).CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// CollectReplication walks through master/slave IPs declared in cfg and
// gathers replication health for both MySQL and Redis. Returns nil when no
// master/slave info is configured, so the field can be omitted from reports.
func CollectReplication(cfg *config.Config) *model.ReplicationReport {
	rep := &model.ReplicationReport{}

	// MySQL — collected only when CLI is available.
	if _, err := exec.LookPath("mysql"); err == nil {
		for _, ip := range cfg.MySQLMasterIPs {
			rep.MySQLMasters = append(rep.MySQLMasters, collectMySQLMaster(ip, cfg.MySQLPort, cfg.Creds))
		}
		for _, ip := range cfg.MySQLSlaveIPs {
			rep.MySQLSlaves = append(rep.MySQLSlaves, collectMySQLSlave(ip, cfg.MySQLPort, cfg.Creds, cfg.Thresholds.MySQLReplLagSec))
		}
	}

	// Redis — collected only when CLI is available.
	if _, err := exec.LookPath("redis-cli"); err == nil {
		for _, ip := range cfg.RedisMasterIPs {
			rep.RedisNodes = append(rep.RedisNodes, collectRedisReplication(ip, cfg.RedisPort, cfg.Creds.RedisPassword, "master", cfg.Thresholds.RedisReplIOSec))
		}
		for _, ip := range cfg.RedisSlaveIPs {
			rep.RedisNodes = append(rep.RedisNodes, collectRedisReplication(ip, cfg.RedisPort, cfg.Creds.RedisPassword, "slave", cfg.Thresholds.RedisReplIOSec))
		}
	}

	// If nothing was collected (env declares no master/slave at all), omit.
	if len(rep.MySQLMasters) == 0 && len(rep.MySQLSlaves) == 0 && len(rep.RedisNodes) == 0 {
		return nil
	}
	return rep
}

// collectMySQLMaster checks read_only on a master.
func collectMySQLMaster(ip, port string, creds config.Credentials) model.MySQLMasterStatus {
	baseArgs := []string{
		"-u" + creds.MySQLUser,
		"-p" + creds.MySQLPassword,
		"-h" + ip,
		"-P" + port,
		"-N", "-s",
	}
	out := mysqlQuery(baseArgs, "SELECT @@read_only")
	if out == "" {
		return model.MySQLMasterStatus{IP: ip, Status: "warn", Error: "query failed"}
	}
	readOnly := out == "1"
	st := model.MySQLMasterStatus{IP: ip, ReadOnly: readOnly}
	if readOnly {
		st.Status = "warn"
	} else {
		st.Status = "ok"
	}
	return st
}

// collectMySQLSlave runs SHOW SLAVE STATUS and grades the slave.
func collectMySQLSlave(ip, port string, creds config.Credentials, lagThreshold int) model.MySQLSlaveStatus {
	baseArgs := []string{
		"-u" + creds.MySQLUser,
		"-p" + creds.MySQLPassword,
		"-h" + ip,
		"-P" + port,
		"-N", "-s",
	}
	res := model.MySQLSlaveStatus{IP: ip}
	out := mysqlQuery(baseArgs, "SHOW SLAVE STATUS\\G")
	if out == "" {
		res.Error = "query failed"
		return res
	}
	if !strings.Contains(out, "Slave_IO_Running") {
		// Empty result set = node is not configured as slave.
		res.Replication = &model.MySQLReplicationStatus{Status: "not-configured-as-slave"}
		return res
	}

	r := &model.MySQLReplicationStatus{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Slave_IO_Running:"):
			r.IORunning = strings.TrimSpace(strings.TrimPrefix(line, "Slave_IO_Running:"))
		case strings.HasPrefix(line, "Slave_SQL_Running:"):
			r.SQLRunning = strings.TrimSpace(strings.TrimPrefix(line, "Slave_SQL_Running:"))
		case strings.HasPrefix(line, "Seconds_Behind_Master:"):
			v := strings.TrimSpace(strings.TrimPrefix(line, "Seconds_Behind_Master:"))
			if v == "NULL" || v == "" {
				r.SecondsBehindMaster = 0
			} else {
				r.SecondsBehindMaster, _ = strconv.Atoi(v)
			}
		case strings.HasPrefix(line, "Last_IO_Error:"):
			r.LastIOError = strings.TrimSpace(strings.TrimPrefix(line, "Last_IO_Error:"))
		case strings.HasPrefix(line, "Last_SQL_Error:"):
			r.LastSQLError = strings.TrimSpace(strings.TrimPrefix(line, "Last_SQL_Error:"))
		case strings.HasPrefix(line, "Master_Host:"):
			r.MasterHost = strings.TrimSpace(strings.TrimPrefix(line, "Master_Host:"))
		}
	}

	switch {
	case r.IORunning != "Yes" || r.SQLRunning != "Yes":
		r.Status = "critical"
	case r.SecondsBehindMaster > lagThreshold:
		r.Status = "warn"
	default:
		r.Status = "ok"
	}

	res.Replication = r
	return res
}

// collectRedisReplication runs `INFO replication` and grades the node.
// declaredRole is "master" or "slave" per env; the result records whether the
// actual role matches.
func collectRedisReplication(ip, port, password, declaredRole string, ioThreshold int) model.RedisReplicationStatus {
	r := model.RedisReplicationStatus{IP: ip, RoleConsistencyStatus: "ok"}

	args := []string{"-h", ip, "-p", port}
	if password != "" {
		args = append(args, "-a", password, "--no-auth-warning")
	}
	args = append(args, "INFO", "replication")
	out, err := exec.Command("redis-cli", args...).Output()
	if err != nil {
		r.Error = fmt.Sprintf("redis-cli error: %v", err)
		r.RoleConsistencyStatus = "N/A"
		return r
	}

	info := parseRedisInfo(string(out))
	r.Role = info["role"]

	if r.Role != declaredRole {
		r.RoleConsistencyStatus = "warn"
	}

	switch r.Role {
	case "master":
		r.ConnectedSlaves, _ = strconv.Atoi(info["connected_slaves"])
		r.LinkStatus = "N/A"
	case "slave":
		r.MasterHost = info["master_host"]
		r.MasterPort, _ = strconv.Atoi(info["master_port"])
		r.MasterLinkStatus = info["master_link_status"]
		r.MasterLastIOSeconds, _ = strconv.Atoi(info["master_last_io_seconds_ago"])
		r.MasterSyncInProgress = info["master_sync_in_progress"] == "1"

		switch {
		case r.MasterLinkStatus != "up":
			r.LinkStatus = "critical"
		case r.MasterLastIOSeconds > ioThreshold:
			r.LinkStatus = "warn"
		default:
			r.LinkStatus = "ok"
		}
	default:
		r.LinkStatus = "N/A"
	}

	return r
}

// CrossCheckSentinelMaster annotates a SentinelClusterStatus with
// MasterEnvMatch by comparing the discovered master IP against
// cfg.RedisMasterIPs. Called from main.go after CollectRedisSentinel.
func CrossCheckSentinelMaster(s *model.SentinelClusterStatus, masterIPs []string) {
	if s == nil {
		return
	}
	if len(masterIPs) == 0 {
		s.MasterEnvMatch = "N/A"
		return
	}
	if s.DiscoveredMaster == "" {
		s.MasterEnvMatch = "warn"
		return
	}
	// DiscoveredMaster is "ip:port".
	ip := s.DiscoveredMaster
	if i := strings.LastIndex(ip, ":"); i > 0 {
		ip = ip[:i]
	}
	for _, m := range masterIPs {
		if m == ip {
			s.MasterEnvMatch = "ok"
			return
		}
	}
	s.MasterEnvMatch = "warn"
}
