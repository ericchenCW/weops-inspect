package collector

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"weops-inspect/config"
	"weops-inspect/model"
)

// CollectMySQL iterates over each MySQL IP in cfg.MySQLIPs and collects
// per-node configuration / replication info via the native MySQL driver.
func CollectMySQL(ctx context.Context, cfg *config.Config) []model.MySQLCluster {
	if len(cfg.MySQLIPs) == 0 {
		return nil
	}

	cluster := model.MySQLCluster{
		Instance: fmt.Sprintf("mysql:%s", cfg.MySQLPort),
	}

	for _, ip := range cfg.MySQLIPs {
		p := &mysqlNodeProbe{ip: ip, port: cfg.MySQLPort, creds: cfg.Creds}
		RunProbe(ctx, p)
		cluster.Nodes = append(cluster.Nodes, p.node)
	}
	return []model.MySQLCluster{cluster}
}

type mysqlNodeProbe struct {
	ip    string
	port  string
	creds config.Credentials
	node  model.MySQLNode
}

func (p *mysqlNodeProbe) Name() string { return "mysql" }

func (p *mysqlNodeProbe) Run(ctx context.Context) ProbeResult {
	target := fmt.Sprintf("%s:%s", p.ip, p.port)
	p.node = model.MySQLNode{IP: p.ip}

	db, err := openMySQL(p.ip, p.port, p.creds)
	if err != nil {
		p.node.Error = RedactDSN(err.Error())
		p.node.ErrorClass = string(classifyMySQL(err))
		return ProbeResult{Target: target, Err: WrapErr(err), ErrClass: classifyMySQL(err)}
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		p.node.Error = "connect/query failed: " + RedactDSN(err.Error())
		p.node.ErrorClass = string(classifyMySQL(err))
		return ProbeResult{Target: target, Err: WrapErr(err), ErrClass: classifyMySQL(err)}
	}

	// 单次往返一次性把所有 @@xxx 拉回来,减少握手次数。
	row := db.QueryRowContext(ctx, `SELECT
		@@VERSION,
		@@max_connections,
		@@expire_logs_days,
		@@max_allowed_packet,
		@@slow_query_log,
		@@character_set_server,
		@@innodb_buffer_pool_size,
		@@innodb_buffer_pool_instances,
		@@innodb_io_capacity,
		@@innodb_read_io_threads,
		@@innodb_write_io_threads,
		@@interactive_timeout,
		@@table_open_cache,
		@@wait_timeout`)

	var (
		version, charset                   string
		maxConn, expireLogs, slowLog       sql.NullInt64
		maxPacket, bufPool                 sql.NullInt64
		bufInst, ioCap, rdThr, wrThr       sql.NullInt64
		interTO, tabCache, waitTO          sql.NullInt64
	)
	if err := row.Scan(&version, &maxConn, &expireLogs, &maxPacket, &slowLog, &charset,
		&bufPool, &bufInst, &ioCap, &rdThr, &wrThr, &interTO, &tabCache, &waitTO); err != nil {
		p.node.Error = "variables query failed: " + RedactDSN(err.Error())
		p.node.ErrorClass = string(classifyMySQL(err))
		return ProbeResult{Target: target, Err: WrapErr(err), ErrClass: classifyMySQL(err)}
	}

	p.node.Version = version
	p.node.MaxConnections = int(maxConn.Int64)
	p.node.ExpireLogsDays = int(expireLogs.Int64)
	p.node.MaxAllowedPacket = maxPacket.Int64
	if slowLog.Int64 == 1 {
		p.node.SlowQueryLog = "ON"
	} else {
		p.node.SlowQueryLog = "OFF"
	}
	p.node.CharacterSet = charset
	p.node.BufferPoolSize = bufPool.Int64
	p.node.BufferPoolInstances = int(bufInst.Int64)
	p.node.InnodbIOCapacity = int(ioCap.Int64)
	p.node.InnodbReadIOThreads = int(rdThr.Int64)
	p.node.InnodbWriteIOThreads = int(wrThr.Int64)
	p.node.InteractiveTimeout = int(interTO.Int64)
	p.node.TableOpenCache = int(tabCache.Int64)
	p.node.WaitTimeout = int(waitTO.Int64)

	// Replication 状态:优先 SHOW SLAVE STATUS,失败回退 SHOW REPLICA STATUS(MySQL 8.4+)。
	slaveCols, slaveRow, slaveErr := queryStatusVertical(ctx, db, "SHOW SLAVE STATUS")
	if slaveErr != nil || len(slaveCols) == 0 {
		slaveCols, slaveRow, _ = queryStatusVertical(ctx, db, "SHOW REPLICA STATUS")
	}
	if len(slaveCols) > 0 {
		p.node.Role = "slave"
		p.node.SlaveIOState = pickFirst(slaveCols, slaveRow, "Slave_IO_Running", "Replica_IO_Running")
		p.node.SlaveSQLState = pickFirst(slaveCols, slaveRow, "Slave_SQL_Running", "Replica_SQL_Running")
	} else {
		p.node.Role = "master"
	}

	// Binlog 数量:8.4 已废弃 SHOW MASTER LOGS,优先 SHOW BINARY LOGS。
	if n, err := countRows(ctx, db, "SHOW BINARY LOGS"); err == nil {
		p.node.BinlogCount = n
	} else if n, err := countRows(ctx, db, "SHOW MASTER LOGS"); err == nil {
		p.node.BinlogCount = n
	}

	return ProbeResult{Target: target}
}

func openMySQL(ip, port string, creds config.Credentials) (*sql.DB, error) {
	cfg := mysql.NewConfig()
	cfg.User = creds.MySQLUser
	cfg.Passwd = creds.MySQLPassword
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%s", ip, port)
	cfg.Timeout = 3 * time.Second
	cfg.ReadTimeout = 5 * time.Second
	cfg.WriteTimeout = 5 * time.Second
	cfg.InterpolateParams = true
	cfg.AllowNativePasswords = true

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(0)
	db.SetConnMaxLifetime(10 * time.Second)
	return db, nil
}

// queryStatusVertical 执行 status 类查询并按列名返回首行结果(命名仿 \G 输出语义)。
// SHOW SLAVE STATUS 返回 0 或 1 行多列,这里取首行。
func queryStatusVertical(ctx context.Context, db *sql.DB, q string) ([]string, []sql.RawBytes, error) {
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}
	if !rows.Next() {
		return nil, nil, nil
	}

	values := make([]sql.RawBytes, len(cols))
	scanArgs := make([]interface{}, len(cols))
	for i := range values {
		scanArgs[i] = &values[i]
	}
	if err := rows.Scan(scanArgs...); err != nil {
		return nil, nil, err
	}
	// 复制一份,避免 RawBytes 在 rows 关闭后失效。
	out := make([]sql.RawBytes, len(values))
	for i, v := range values {
		out[i] = append([]byte(nil), v...)
	}
	return cols, out, nil
}

func pickFirst(cols []string, row []sql.RawBytes, names ...string) string {
	for _, name := range names {
		for i, c := range cols {
			if strings.EqualFold(c, name) {
				return string(row[i])
			}
		}
	}
	return ""
}

func countRows(ctx context.Context, db *sql.DB, q string) (int, error) {
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		n++
	}
	return n, rows.Err()
}

// classifyMySQL 在通用 Classify 之前优先识别 MySQL 专属错误。
func classifyMySQL(err error) ErrorClass {
	if err == nil {
		return ErrNone
	}
	var mErr *mysql.MySQLError
	if errors.As(err, &mErr) {
		switch mErr.Number {
		case 1045, 1044, 1698: // access denied / db access denied / native auth failed
			return ErrAuth
		}
		return ErrProtocol
	}
	return Classify(err)
}
