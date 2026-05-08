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
	RunDays         int
	MySQLReplLagSec int // Seconds_Behind_Master warn threshold
	RedisReplIOSec  int // master_last_io_seconds_ago warn threshold
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

	// Output settings
	OutputDir string

	// Mount paths to check
	CheckMountPath string
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
	cpu, err := parseFloatEnv("INSPECT_CPU_THRESHOLD", 75)
	if err != nil {
		return nil, err
	}
	disk, err := parseFloatEnv("INSPECT_DISK_THRESHOLD", 75)
	if err != nil {
		return nil, err
	}
	inode, err := parseFloatEnv("INSPECT_INODE_THRESHOLD", 75)
	if err != nil {
		return nil, err
	}
	mem, err := parseFloatEnv("INSPECT_MEM_THRESHOLD", 75)
	if err != nil {
		return nil, err
	}
	maxOpen, err := parseIntEnv("INSPECT_MAX_OPEN_FILES", 102400)
	if err != nil {
		return nil, err
	}
	runDays, err := parseIntEnv("INSPECT_RUN_DAYS", 365)
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
	c.Thresholds = Thresholds{
		CPUUsage:        cpu,
		DiskUsage:       disk,
		InodeUsage:      inode,
		MemUsage:        mem,
		MaxOpenFiles:    maxOpen,
		RunDays:         runDays,
		MySQLReplLagSec: mysqlLag,
		RedisReplIOSec:  redisIO,
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

	// Mount paths
	c.CheckMountPath = envOrDefault("CHECK_MOUNT_PATH", "/data")

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
func (c *Config) GetModuleHosts() []ModuleHosts {
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
		c.NodeManIPs,
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
