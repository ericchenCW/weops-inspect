package collector

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"weops-inspect/config"
	"weops-inspect/model"
)

// CollectMySQL collects MySQL configuration and replication status.
func CollectMySQL(cfg *config.Config) []model.MySQLCluster {
	if cfg.MySQLIP == "" {
		return nil
	}
	if _, err := exec.LookPath("mysql"); err != nil {
		return []model.MySQLCluster{{Error: "mysql CLI not available"}}
	}

	instance := fmt.Sprintf("%s:%s", cfg.MySQLIP, cfg.MySQLPort)
	cluster := model.MySQLCluster{Instance: instance}

	// Build common mysql args
	baseArgs := []string{
		fmt.Sprintf("-u%s", cfg.Creds.MySQLUser),
		fmt.Sprintf("-p%s", cfg.Creds.MySQLPassword),
		fmt.Sprintf("-h%s", cfg.MySQLIP),
		fmt.Sprintf("-P%s", cfg.MySQLPort),
		"-N", "-s",
	}

	node := model.MySQLNode{IP: cfg.MySQLIP}

	// Version
	node.Version = mysqlQuery(baseArgs, "SELECT @@VERSION")

	// Variables
	node.MaxConnections = mysqlQueryInt(baseArgs, "SELECT @@max_connections")
	node.ExpireLogsDays = mysqlQueryInt(baseArgs, "SELECT @@expire_logs_days")
	node.MaxAllowedPacket = int64(mysqlQueryInt(baseArgs, "SELECT @@max_allowed_packet"))
	node.SlowQueryLog = mysqlQuery(baseArgs, "SELECT @@slow_query_log")
	if node.SlowQueryLog == "1" {
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

	// Replication status
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

	// Binlog count
	binlogOut := mysqlQuery(baseArgs, "SHOW MASTER LOGS")
	if binlogOut != "" {
		node.BinlogCount = len(strings.Split(strings.TrimSpace(binlogOut), "\n"))
	}

	cluster.Nodes = append(cluster.Nodes, node)
	return []model.MySQLCluster{cluster}
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
