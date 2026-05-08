package collector

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"weops-inspect/config"
	"weops-inspect/model"
)

// CollectReplication walks through master/slave IPs declared in cfg and
// gathers replication health for both MySQL and Redis using native drivers.
// Returns nil when no master/slave info is configured.
func CollectReplication(ctx context.Context, cfg *config.Config) *model.ReplicationReport {
	rep := &model.ReplicationReport{}

	for _, ip := range cfg.MySQLMasterIPs {
		rep.MySQLMasters = append(rep.MySQLMasters, collectMySQLMaster(ctx, ip, cfg.MySQLPort, cfg.Creds))
	}
	for _, ip := range cfg.MySQLSlaveIPs {
		rep.MySQLSlaves = append(rep.MySQLSlaves, collectMySQLSlave(ctx, ip, cfg.MySQLPort, cfg.Creds, cfg.Thresholds.MySQLReplLagSec))
	}
	for _, ip := range cfg.RedisMasterIPs {
		rep.RedisNodes = append(rep.RedisNodes, collectRedisReplication(ctx, ip, cfg.RedisPort, cfg.Creds.RedisPassword, "master", cfg.Thresholds.RedisReplIOSec))
	}
	for _, ip := range cfg.RedisSlaveIPs {
		rep.RedisNodes = append(rep.RedisNodes, collectRedisReplication(ctx, ip, cfg.RedisPort, cfg.Creds.RedisPassword, "slave", cfg.Thresholds.RedisReplIOSec))
	}

	if len(rep.MySQLMasters) == 0 && len(rep.MySQLSlaves) == 0 && len(rep.RedisNodes) == 0 {
		return nil
	}
	return rep
}

// collectMySQLMaster checks read_only on a master via native driver.
func collectMySQLMaster(ctx context.Context, ip, port string, creds config.Credentials) model.MySQLMasterStatus {
	st := model.MySQLMasterStatus{IP: ip}

	db, err := openMySQL(ip, port, creds)
	if err != nil {
		st.Status = "warn"
		st.Error = "connect failed: " + RedactDSN(err.Error())
		st.ErrorClass = string(classifyMySQL(err))
		return st
	}
	defer db.Close()

	var v int
	if err := db.QueryRowContext(ctx, "SELECT @@global.read_only").Scan(&v); err != nil {
		st.Status = "warn"
		st.Error = "query failed: " + RedactDSN(err.Error())
		st.ErrorClass = string(classifyMySQL(err))
		return st
	}
	st.ReadOnly = v == 1
	if st.ReadOnly {
		st.Status = "warn"
	} else {
		st.Status = "ok"
	}
	return st
}

// collectMySQLSlave runs SHOW SLAVE STATUS via native driver and grades the slave.
func collectMySQLSlave(ctx context.Context, ip, port string, creds config.Credentials, lagThreshold int) model.MySQLSlaveStatus {
	res := model.MySQLSlaveStatus{IP: ip}

	db, err := openMySQL(ip, port, creds)
	if err != nil {
		res.Error = "connect failed: " + RedactDSN(err.Error())
		res.ErrorClass = string(classifyMySQL(err))
		return res
	}
	defer db.Close()

	cols, row, qerr := queryStatusVertical(ctx, db, "SHOW SLAVE STATUS")
	if qerr != nil || len(cols) == 0 {
		// Fallback for MySQL 8.4+.
		cols, row, qerr = queryStatusVertical(ctx, db, "SHOW REPLICA STATUS")
	}
	if qerr != nil {
		res.Error = "query failed: " + RedactDSN(qerr.Error())
		res.ErrorClass = string(classifyMySQL(qerr))
		return res
	}
	if len(cols) == 0 {
		// 节点未配置为 slave。
		res.Replication = &model.MySQLReplicationStatus{Status: "not-configured-as-slave"}
		return res
	}

	r := &model.MySQLReplicationStatus{}
	r.IORunning = pickFirst(cols, row, "Slave_IO_Running", "Replica_IO_Running")
	r.SQLRunning = pickFirst(cols, row, "Slave_SQL_Running", "Replica_SQL_Running")
	r.LastIOError = pickFirst(cols, row, "Last_IO_Error")
	r.LastSQLError = pickFirst(cols, row, "Last_SQL_Error")
	r.MasterHost = pickFirst(cols, row, "Master_Host", "Source_Host")
	if v := pickFirst(cols, row, "Seconds_Behind_Master", "Seconds_Behind_Source"); v != "" && v != "NULL" {
		r.SecondsBehindMaster, _ = strconv.Atoi(v)
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

// collectRedisReplication runs INFO replication via native driver and grades the node.
func collectRedisReplication(ctx context.Context, ip, port, password, declaredRole string, ioThreshold int) model.RedisReplicationStatus {
	r := model.RedisReplicationStatus{IP: ip, RoleConsistencyStatus: "ok"}

	addr := net.JoinHostPort(ip, port)
	rdb := newRedisClient(addr, password, 0)
	defer rdb.Close()

	out, err := rdb.Info(ctx, "replication").Result()
	if err != nil {
		r.Error = fmt.Sprintf("redis info error: %v", err)
		r.ErrorClass = string(classifyRedis(err))
		r.RoleConsistencyStatus = "N/A"
		return r
	}

	info := parseRedisInfo(out)
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
// cfg.RedisMasterIPs.
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
