package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ModuleHosts maps a module name to its list of host IPs.
type ModuleHosts struct {
	Module string
	IPs    []string
}

// BKMonitorV3DepConfig holds endpoints + credentials used by the bkmonitorv3
// dependency probe. Mirrors the checks in bk-install/health_check/deploy_check.py.
type BKMonitorV3DepConfig struct {
	RedisHost     string
	RedisPort     string
	RedisPassword string

	// 蓝鲸生产环境 PaaS 与 Monitor 共用同一套 MySQL,统一通过 Consul 暴露为
	// mysql.service.consul:3306。这里只保留单组字段,加载顺序见 Load()。
	MySQLHost     string
	MySQLPort     string
	MySQLUser     string
	MySQLPassword string

	RabbitMQHost     string
	RabbitMQPort     string
	RabbitMQUser     string
	RabbitMQPassword string
	RabbitMQVHost    string

	ZKHost string
	ZKPort string

	ES7Host     string
	ES7RestPort string
	ES7User     string
	ES7Password string

	InfluxDBHost string
	InfluxDBPort string
}

// Credentials holds database/service credentials.
type Credentials struct {
	MySQLUser        string
	MySQLPassword    string
	RedisPassword    string
	MongoDBUser      string
	MongoDBPassword  string
	ES7Password      string
	RabbitMQUser     string
	RabbitMQPassword string
}

// Thresholds holds rule check thresholds with defaults.
type Thresholds struct {
	CPUUsage        float64
	DiskUsage       float64
	InodeUsage      float64
	MemUsage        float64
	MaxOpenFiles    int
	MySQLReplLagSec       int // Seconds_Behind_Master warn threshold
	RedisReplIOSec        int // master_last_io_seconds_ago warn threshold
	RabbitMQQueueBacklog  int // RabbitMQ queue messages count warn threshold
	// vhosts whose queues are exempt from the "0 consumers" alert (queue backlog
	// alert still applies). env-set value fully replaces the default.
	RabbitMQNoConsumerVHostBlacklist []string

	// Notice-level thresholds: trigger HTML coloring but NOT email alerts and NOT
	// Summary.Warn. Migrated from inline template constants.
	ESHeapPercent             int // > → Notice
	ESRAMPercent              int // > → Notice
	ESUnassignedShards        int // > → Notice
	RedisCeleryQueue          int // > → Notice
	RedisMonitorQueue         int // > → Notice
	ServiceContainersExited   int // > → Notice
}

// Config is the top-level configuration.
type Config struct {
	// BlueKing module host mappings
	PaaSIPs    []string
	CMDBIPs    []string
	JobIPs     []string
	GSEIPs     []string
	APPOIPs    []string
	APPTIPs    []string
	IAMIPs     []string
	UserMgrIPs []string
	NodeManIPs []string
	MonitorV3IPs []string

	// bkmonitorv3 子角色独立 IP 列表。生产中各角色常被拆到不同主机部署,这里允许
	// 按角色分别声明部署主机;空列表回退使用 MonitorV3IPs(由 GetModuleHosts() 处理)。
	MonitorV3MonitorIPs        []string
	MonitorV3InfluxDBProxyIPs  []string
	MonitorV3TransferIPs       []string
	MonitorV3UnifyQueryIPs     []string

	// Open source component hosts
	ES7IPs           []string
	MySQLIPs         []string
	MySQLMasterIPs   []string
	MySQLSlaveIPs    []string
	MySQLPort        string
	RedisIPs         []string
	RedisMasterIPs   []string
	RedisSlaveIPs    []string
	RedisSentinelIPs []string
	RedisPort        string
	MongoDBIPs       []string
	MongoDBPort      string
	MongoRSName      string
	RabbitMQIPs      []string

	// bkmonitorv3 dependency endpoints (read from BK_MONITOR_* / BK_PAAS_MYSQL_* /
	// BK_GSE_ZK_* / BK_INFLUXDB_*). Used by the bkmonitorv3 dependency probe; any
	// missing field is left as "" and the corresponding probe is skipped.
	BKMonitorV3 BKMonitorV3DepConfig

	// All unique host IPs (deduplicated, BK modules + infra nodes)
	AllHosts []string

	// Credentials
	Creds Credentials

	// Thresholds
	Thresholds Thresholds

	// SSH settings
	SSHUser     string
	SSHPort     int
	SSHKeyPath  string
	SSHUseSudo  bool
	SSHTimeout  int // seconds

	// RabbitMQ Management API curl timeout (seconds). Applies to all /api/* calls.
	// /api/queues on busy clusters can be slow; default 60s.
	RabbitMQAPITimeoutSec int

	// Output settings
	OutputDir string

	// Mount paths to check. Empty = collect all "real" filesystems (filtered by
	// fs-type allow/block lists). Non-empty = colon-separated list of mount
	// points to match exactly.
	CheckMountPath string

	// DiskIncludeNFS, when true, includes nfs/nfs4/cifs/smbfs/smb3 mounts in the
	// default (CheckMountPath empty) collection. NFS is otherwise excluded by
	// default to avoid network-flap induced noise.
	DiskIncludeNFS bool
}

// parseIPList splits a comma-separated IP list, trimming whitespace.
func parseIPList(val string) []string {
	if val == "" {
		return nil
	}
	parts := strings.Split(val, ",")
	var ips []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			ips = append(ips, p)
		}
	}
	return ips
}

// Load reads BK_* environment variables and builds the Config.
func Load(outputDir string) (*Config, error) {
	c := &Config{OutputDir: outputDir}

	// BK module IPs
	c.PaaSIPs = parseIPList(os.Getenv("BK_PAAS_IP_COMMA"))
	c.CMDBIPs = parseIPList(os.Getenv("BK_CMDB_IP_COMMA"))
	c.JobIPs = parseIPList(os.Getenv("BK_JOB_IP_COMMA"))
	c.GSEIPs = parseIPList(os.Getenv("BK_GSE_IP_COMMA"))
	c.APPOIPs = parseIPList(os.Getenv("BK_APPO_IP_COMMA"))
	c.APPTIPs = parseIPList(os.Getenv("BK_APPT_IP_COMMA"))
	c.IAMIPs = parseIPList(os.Getenv("BK_IAM_IP_COMMA"))
	c.UserMgrIPs = parseIPList(os.Getenv("BK_USERMGR_IP_COMMA"))
	c.NodeManIPs = parseIPList(os.Getenv("BK_NODEMAN_IP_COMMA"))
	c.MonitorV3IPs = parseIPList(os.Getenv("BK_MONITORV3_IP_COMMA"))
	c.MonitorV3MonitorIPs = parseIPList(os.Getenv("BK_MONITORV3_MONITOR_IP_COMMA"))
	c.MonitorV3InfluxDBProxyIPs = parseIPList(os.Getenv("BK_MONITORV3_INFLUXDB_PROXY_IP_COMMA"))
	c.MonitorV3TransferIPs = parseIPList(os.Getenv("BK_MONITORV3_TRANSFER_IP_COMMA"))
	c.MonitorV3UnifyQueryIPs = parseIPList(os.Getenv("BK_MONITORV3_UNIFY_QUERY_IP_COMMA"))

	// bkmonitorv3 dependency endpoints (used only by the bkmonitorv3 dep probe).
	c.BKMonitorV3 = BKMonitorV3DepConfig{
		RedisHost:     os.Getenv("BK_MONITOR_REDIS_HOST"),
		RedisPort:     os.Getenv("BK_MONITOR_REDIS_PORT"),
		RedisPassword: os.Getenv("BK_MONITOR_REDIS_PASSWORD"),

		MySQLHost:     firstNonEmpty(os.Getenv("BK_MONITOR_MYSQL_HOST"), os.Getenv("BK_PAAS_MYSQL_HOST"), "mysql.service.consul"),
		MySQLPort:     firstNonEmpty(os.Getenv("BK_MONITOR_MYSQL_PORT"), os.Getenv("BK_PAAS_MYSQL_PORT"), "3306"),
		MySQLUser:     firstNonEmpty(os.Getenv("BK_MONITOR_MYSQL_USER"), os.Getenv("BK_PAAS_MYSQL_USER")),
		MySQLPassword: firstNonEmpty(os.Getenv("BK_MONITOR_MYSQL_PASSWORD"), os.Getenv("BK_PAAS_MYSQL_PASSWORD")),

		RabbitMQHost:     os.Getenv("BK_MONITOR_RABBITMQ_HOST"),
		RabbitMQPort:     os.Getenv("BK_MONITOR_RABBITMQ_PORT"),
		RabbitMQUser:     os.Getenv("BK_MONITOR_RABBITMQ_USERNAME"),
		RabbitMQPassword: os.Getenv("BK_MONITOR_RABBITMQ_PASSWORD"),
		RabbitMQVHost:    os.Getenv("BK_MONITOR_RABBITMQ_VHOST"),

		ZKHost: os.Getenv("BK_GSE_ZK_HOST"),
		ZKPort: os.Getenv("BK_GSE_ZK_PORT"),

		ES7Host:     os.Getenv("BK_MONITOR_ES7_HOST"),
		ES7RestPort: os.Getenv("BK_MONITOR_ES7_REST_PORT"),
		ES7User:     os.Getenv("BK_MONITOR_ES7_USER"),
		ES7Password: os.Getenv("BK_MONITOR_ES7_PASSWORD"),

		InfluxDBHost: os.Getenv("BK_INFLUXDB_IP0"),
		InfluxDBPort: os.Getenv("BK_MONITOR_INFLUXDB_PORT"),
	}

	// Open source components — read from *_IP_COMMA (the array-literal *_IP form
	// in bk.env is not exportable as a scalar env var).
	c.ES7IPs = parseIPList(os.Getenv("BK_ES7_IP_COMMA"))
	c.MySQLIPs = parseIPList(os.Getenv("BK_MYSQL_IP_COMMA"))
	c.MySQLMasterIPs = parseIPList(os.Getenv("BK_MYSQL_MASTER_IP_COMMA"))
	c.MySQLSlaveIPs = parseIPList(os.Getenv("BK_MYSQL_SLAVE_IP_COMMA"))
	c.RedisIPs = parseIPList(os.Getenv("BK_REDIS_IP_COMMA"))
	c.RedisMasterIPs = parseIPList(os.Getenv("BK_REDIS_MASTER_IP_COMMA"))
	c.RedisSlaveIPs = parseIPList(os.Getenv("BK_REDIS_SLAVE_IP_COMMA"))
	c.RedisSentinelIPs = parseIPList(os.Getenv("BK_REDIS_SENTINEL_IP_COMMA"))
	c.MongoDBIPs = parseIPList(os.Getenv("BK_MONGODB_IP_COMMA"))
	c.RabbitMQIPs = parseIPList(os.Getenv("BK_RABBITMQ_IP_COMMA"))

	// Ports — bk.env has no global *_PORT for these, so use INSPECT_* with defaults.
	c.MySQLPort = envOrDefault("INSPECT_MYSQL_PORT", "3306")
	c.RedisPort = envOrDefault("INSPECT_REDIS_PORT", "6379")
	c.MongoDBPort = envOrDefault("INSPECT_MONGODB_PORT", "27017")
	c.MongoRSName = envOrDefault("INSPECT_MONGO_RS_NAME", "rs0")

	// Credentials
	c.Creds = Credentials{
		MySQLUser:        envOrDefault("BK_MYSQL_ADMIN_USER", "root"),
		MySQLPassword:    os.Getenv("BK_MYSQL_ADMIN_PASSWORD"),
		RedisPassword:    os.Getenv("BK_REDIS_ADMIN_PASSWORD"),
		MongoDBUser:      envOrDefault("BK_MONGODB_ADMIN_USER", "root"),
		MongoDBPassword:  os.Getenv("BK_MONGODB_ADMIN_PASSWORD"),
		ES7Password:      os.Getenv("BK_ES7_ADMIN_PASSWORD"),
		RabbitMQUser:     envOrDefault("BK_RABBITMQ_ADMIN_USER", "admin"),
		RabbitMQPassword: os.Getenv("BK_RABBITMQ_ADMIN_PASSWORD"),
	}

	// Thresholds — env-overridable, parse failures abort.
	cpu, err := parseFloatEnv("INSPECT_CPU_THRESHOLD", 95)
	if err != nil {
		return nil, err
	}
	disk, err := parseFloatEnv("INSPECT_DISK_THRESHOLD", 95)
	if err != nil {
		return nil, err
	}
	inode, err := parseFloatEnv("INSPECT_INODE_THRESHOLD", 95)
	if err != nil {
		return nil, err
	}
	mem, err := parseFloatEnv("INSPECT_MEM_THRESHOLD", 95)
	if err != nil {
		return nil, err
	}
	maxOpen, err := parseIntEnv("INSPECT_MAX_OPEN_FILES", 65536)
	if err != nil {
		return nil, err
	}
	mysqlLag, err := parseIntEnv("INSPECT_MYSQL_REPL_LAG_THRESHOLD", 60)
	if err != nil {
		return nil, err
	}
	redisIO, err := parseIntEnv("INSPECT_REDIS_REPL_IO_THRESHOLD", 10)
	if err != nil {
		return nil, err
	}
	rmqBacklog, err := parseIntEnv("INSPECT_RABBITMQ_QUEUE_BACKLOG_THRESHOLD", 10000)
	if err != nil {
		return nil, err
	}
	esHeap, err := parseIntEnv("INSPECT_ES_HEAP_THRESHOLD", 85)
	if err != nil {
		return nil, err
	}
	esRAM, err := parseIntEnv("INSPECT_ES_RAM_THRESHOLD", 95)
	if err != nil {
		return nil, err
	}
	esUnassigned, err := parseIntEnv("INSPECT_ES_UNASSIGNED_SHARDS_THRESHOLD", 0)
	if err != nil {
		return nil, err
	}
	redisCelery, err := parseIntEnv("INSPECT_REDIS_CELERY_QUEUE_THRESHOLD", 1000)
	if err != nil {
		return nil, err
	}
	redisMonitor, err := parseIntEnv("INSPECT_REDIS_MONITOR_QUEUE_THRESHOLD", 10000)
	if err != nil {
		return nil, err
	}
	dockerExited, err := parseIntEnv("INSPECT_DOCKER_EXITED_THRESHOLD", 0)
	if err != nil {
		return nil, err
	}
	c.Thresholds = Thresholds{
		CPUUsage:                         cpu,
		DiskUsage:                        disk,
		InodeUsage:                       inode,
		MemUsage:                         mem,
		MaxOpenFiles:                     maxOpen,
		MySQLReplLagSec:                  mysqlLag,
		RedisReplIOSec:                   redisIO,
		RabbitMQQueueBacklog:             rmqBacklog,
		RabbitMQNoConsumerVHostBlacklist: parseVHostBlacklist(os.Getenv("INSPECT_RABBITMQ_NO_CONSUMER_VHOST_BLACKLIST"), []string{"bk_bknodeman"}),
		ESHeapPercent:                    esHeap,
		ESRAMPercent:                     esRAM,
		ESUnassignedShards:               esUnassigned,
		RedisCeleryQueue:                 redisCelery,
		RedisMonitorQueue:                redisMonitor,
		ServiceContainersExited:          dockerExited,
	}

	// SSH settings
	c.SSHUser = envOrDefault("INSPECT_SSH_USER", "root")
	port, err := parseIntEnv("INSPECT_SSH_PORT", 22)
	if err != nil {
		return nil, err
	}
	c.SSHPort = port
	c.SSHKeyPath = os.Getenv("INSPECT_SSH_KEY_PATH")
	c.SSHUseSudo = parseBoolEnv("INSPECT_SSH_USE_SUDO", false)
	c.SSHTimeout = 30

	rmqAPITimeout, err := parseIntEnv("INSPECT_RABBITMQ_API_TIMEOUT_SEC", 60)
	if err != nil {
		return nil, err
	}
	c.RabbitMQAPITimeoutSec = rmqAPITimeout

	// Mount paths. Default empty -> collect all real filesystems (see
	// collector/host.go fs-type filtering). Set CHECK_MOUNT_PATH=/data:/var
	// for the legacy exact-match behavior.
	c.CheckMountPath = os.Getenv("CHECK_MOUNT_PATH")
	c.DiskIncludeNFS = parseBoolEnv("INSPECT_DISK_INCLUDE_NFS", false)

	// Build deduplicated host list (BK modules + infra)
	c.AllHosts = c.buildAllHosts()

	return c, nil
}

// Validate checks that essential configuration is present.
func (c *Config) Validate() error {
	if len(c.AllHosts) == 0 {
		return fmt.Errorf("no hosts found: set at least one BK_{MODULE}_IP_COMMA environment variable")
	}
	return nil
}

// GetModuleHosts returns the ordered list of BK modules and their hosts.
//
// bkmonitorv3 被拆为 4 个独立角色 module key:每个 key 的 IP 来源优先取角色专属变量
// (BK_MONITORV3_<ROLE>_IP_COMMA),为空时回退到 BK_MONITORV3_IP_COMMA(MonitorV3IPs),
// 兼容旧的"四角色同主机"部署。
func (c *Config) GetModuleHosts() []ModuleHosts {
	roleIPs := func(role []string) []string {
		if len(role) > 0 {
			return role
		}
		return c.MonitorV3IPs
	}
	return []ModuleHosts{
		{Module: "paas", IPs: c.PaaSIPs},
		{Module: "cmdb", IPs: c.CMDBIPs},
		{Module: "job", IPs: c.JobIPs},
		{Module: "gse", IPs: c.GSEIPs},
		{Module: "appo", IPs: c.APPOIPs},
		{Module: "appt", IPs: c.APPTIPs},
		{Module: "iam", IPs: c.IAMIPs},
		{Module: "usermgr", IPs: c.UserMgrIPs},
		{Module: "nodeman", IPs: c.NodeManIPs},
		{Module: "bkmonitorv3-monitor", IPs: roleIPs(c.MonitorV3MonitorIPs)},
		{Module: "bkmonitorv3-influxdb-proxy", IPs: roleIPs(c.MonitorV3InfluxDBProxyIPs)},
		{Module: "bkmonitorv3-transfer", IPs: roleIPs(c.MonitorV3TransferIPs)},
		{Module: "bkmonitorv3-unify-query", IPs: roleIPs(c.MonitorV3UnifyQueryIPs)},
	}
}

// buildAllHosts collects and deduplicates all IPs across BK modules and
// infrastructure components (ES7 / RabbitMQ / MySQL / MongoDB / Redis*),
// so that infra-only nodes also receive OS-level metric collection.
func (c *Config) buildAllHosts() []string {
	seen := make(map[string]bool)
	var hosts []string

	allIPs := [][]string{
		c.PaaSIPs, c.CMDBIPs, c.JobIPs, c.GSEIPs,
		c.APPOIPs, c.APPTIPs, c.IAMIPs, c.UserMgrIPs,
		c.NodeManIPs, c.MonitorV3IPs,
		c.MonitorV3MonitorIPs, c.MonitorV3InfluxDBProxyIPs,
		c.MonitorV3TransferIPs, c.MonitorV3UnifyQueryIPs,
		c.ES7IPs, c.RabbitMQIPs, c.MySQLIPs, c.MongoDBIPs,
		c.RedisIPs, c.RedisSentinelIPs,
	}

	for _, ips := range allIPs {
		for _, ip := range ips {
			if !seen[ip] {
				seen[ip] = true
				hosts = append(hosts, ip)
			}
		}
	}
	return hosts
}

// firstNonEmpty 按顺序返回首个非空字符串。常用于 env 优先级回退,例如
// `BK_MONITOR_MYSQL_HOST` → `BK_PAAS_MYSQL_HOST` → 默认值。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// parseFloatEnv reads a float64 from env, returning the default if unset.
// Returns an error (with the variable name) if the value is not a valid number.
func parseFloatEnv(key string, def float64) (float64, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid float for %s: %q", key, v)
	}
	return f, nil
}

// parseIntEnv reads an int from env, returning the default if unset.
// Returns an error (with the variable name) if the value is not a valid integer.
func parseIntEnv(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid int for %s: %q", key, v)
	}
	return n, nil
}

// parseVHostBlacklist parses a comma-separated vhost blacklist with full-replace
// semantics: env unset → use def; env set (even to "") → fully replaces def.
// Empty/whitespace-only entries are dropped, so `INSPECT_..._BLACKLIST=` disables
// the blacklist entirely.
func parseVHostBlacklist(envVal string, def []string) []string {
	if _, present := os.LookupEnv("INSPECT_RABBITMQ_NO_CONSUMER_VHOST_BLACKLIST"); !present {
		return def
	}
	var out []string
	for _, p := range strings.Split(envVal, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseBoolEnv reads a bool from env (1/true/yes → true), default if unset.
func parseBoolEnv(key string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
