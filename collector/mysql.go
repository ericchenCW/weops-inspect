package collector

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"weops-inspect/config"
	"weops-inspect/model"
)

// CollectMySQL iterates over each MySQL IP in cfg.MySQLIPs and collects
// per-node configuration / replication info, returning a single MySQLCluster
// containing all nodes.
func CollectMySQL(cfg *config.Config) []model.MySQLCluster {
	if len(cfg.MySQLIPs) == 0 {
		return nil
	}
	if _, err := exec.LookPath("mysql"); err != nil {
		return []model.MySQLCluster{{Error: "mysql CLI not available"}}
	}

	cluster := model.MySQLCluster{
		Instance: fmt.Sprintf("mysql:%s", cfg.MySQLPort),
	}

	for _, ip := range cfg.MySQLIPs {
		cluster.Nodes = append(cluster.Nodes, collectMySQLNode(ip, cfg.MySQLPort, cfg.Creds))
	}
	return []model.MySQLCluster{cluster}
}

func collectMySQLNode(ip, port string, creds config.Credentials) model.MySQLNode {
	baseArgs := []string{
		fmt.Sprintf("-u%s", creds.MySQLUser),
		fmt.Sprintf("-p%s", creds.MySQLPassword),
		fmt.Sprintf("-h%s", ip),
		fmt.Sprintf("-P%s", port),
		"-N", "-s",
	}

	node := model.MySQLNode{IP: ip}

	// Probe with version; if this fails, treat the node as unreachable.
	node.Version = mysqlQuery(baseArgs, "SELECT @@VERSION")
	if node.Version == "" {
		node.Error = "connect/query failed"
		return node
	}

	node.MaxConnections = mysqlQueryInt(baseArgs, "SELECT @@max_connections")
	node.ExpireLogsDays = mysqlQueryInt(baseArgs, "SELECT @@expire_logs_days")
	node.MaxAllowedPacket = int64(mysqlQueryInt(baseArgs, "SELECT @@max_allowed_packet"))
	if mysqlQuery(baseArgs, "SELECT @@slow_query_log") == "1" {
		node.SlowQueryLog = "ON"
	} else {
		node.SlowQueryLog = "OFF"
	}
	node.CharacterSet = mysqlQuery(baseArgs, "SELECT @@character_set_server")
	node.BufferPoolSize = int64(mysqlQueryInt(baseArgs, "SELECT @@innodb_buffer_pool_size"))
	node.BufferPoolInstances = mysqlQueryInt(baseArgs, "SELECT @@innodb_buffer_pool_instances")
	node.InnodbIOCapacity = mysqlQueryInt(baseArgs, "SELECT @@innodb_io_capacity")
	node.InnodbReadIOThreads = mysqlQueryInt(baseArgs, "SELECT @@innodb_read_io_threads")
	node.InnodbWriteIOThreads = mysqlQueryInt(baseArgs, "SELECT @@innodb_write_io_threads")
	node.InteractiveTimeout = mysqlQueryInt(baseArgs, "SELECT @@interactive_timeout")
	node.TableOpenCache = mysqlQueryInt(baseArgs, "SELECT @@table_open_cache")
	node.WaitTimeout = mysqlQueryInt(baseArgs, "SELECT @@wait_timeout")

	// Replication status — coarse role detection via SHOW SLAVE STATUS\G.
	slaveOut := mysqlQuery(baseArgs, "SHOW SLAVE STATUS\\G")
	if strings.Contains(slaveOut, "Slave_IO_Running") {
		node.Role = "slave"
		for _, line := range strings.Split(slaveOut, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Slave_IO_Running:") {
				node.SlaveIOState = strings.TrimSpace(strings.TrimPrefix(line, "Slave_IO_Running:"))
			}
			if strings.HasPrefix(line, "Slave_SQL_Running:") {
				node.SlaveSQLState = strings.TrimSpace(strings.TrimPrefix(line, "Slave_SQL_Running:"))
			}
		}
	} else {
		node.Role = "master"
	}

	if binlogOut := mysqlQuery(baseArgs, "SHOW MASTER LOGS"); binlogOut != "" {
		node.BinlogCount = len(strings.Split(strings.TrimSpace(binlogOut), "\n"))
	}

	return node
}

func mysqlQuery(baseArgs []string, query string) string {
	args := append(append([]string{}, baseArgs...), "-e", query)
	out, err := exec.Command("mysql", args...).CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func mysqlQueryInt(baseArgs []string, query string) int {
	v, _ := strconv.Atoi(mysqlQuery(baseArgs, query))
	return v
}
