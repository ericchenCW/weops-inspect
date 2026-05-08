package config

import (
	"fmt"
	"os"
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
	CPUUsage     float64
	DiskUsage    float64
	InodeUsage   float64
	MemUsage     float64
	MaxOpenFiles int
	RunDays      int
}

// Config is the top-level configuration.
type Config struct {
	// BlueKing module host mappings
	PaaSIPs     []string
	CMDBIPs     []string
	JobIPs      []string
	GSEIPs      []string
	APPOIPs     []string
	APPTIPs     []string
	IAMIPs      []string
	UserMgrIPs  []string
	NodeManIPs  []string

	// Open source component hosts
	ES7IPs      []string
	MySQLIP     string
	MySQLPort   string
	RedisIP     string
	RedisPort   string
	MongoDBIP   string
	MongoDBPort string
	RabbitMQIPs []string

	// All unique host IPs (deduplicated)
	AllHosts []string

	// Credentials
	Creds Credentials

	// Thresholds
	Thresholds Thresholds

	// SSH settings
	SSHUser    string
	SSHTimeout int // seconds

	// Output settings
	OutputDir string

	// Mount paths to check
	CheckMountPath string
}

// bkModules maps BK env var prefixes to Config fields for host discovery.
var bkModules = []struct {
	envVar string
	label  string
}{
	{"BK_PAAS_IP_COMMA", "paas"},
	{"BK_CMDB_IP_COMMA", "cmdb"},
	{"BK_JOB_IP_COMMA", "job"},
	{"BK_GSE_IP_COMMA", "gse"},
	{"BK_APPO_IP_COMMA", "appo"},
	{"BK_APPT_IP_COMMA", "appt"},
	{"BK_IAM_IP_COMMA", "iam"},
	{"BK_USERMGR_IP_COMMA", "usermgr"},
	{"BK_NODEMAN_IP_COMMA", "nodeman"},
	{"BK_ES7_IP_COMMA", "es7"},
	{"BK_RABBITMQ_IP_COMMA", "rabbitmq"},
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
	c := &Config{
		OutputDir: outputDir,
	}

	// Module IPs
	c.PaaSIPs = parseIPList(os.Getenv("BK_PAAS_IP_COMMA"))
	c.CMDBIPs = parseIPList(os.Getenv("BK_CMDB_IP_COMMA"))
	c.JobIPs = parseIPList(os.Getenv("BK_JOB_IP_COMMA"))
	c.GSEIPs = parseIPList(os.Getenv("BK_GSE_IP_COMMA"))
	c.APPOIPs = parseIPList(os.Getenv("BK_APPO_IP_COMMA"))
	c.APPTIPs = parseIPList(os.Getenv("BK_APPT_IP_COMMA"))
	c.IAMIPs = parseIPList(os.Getenv("BK_IAM_IP_COMMA"))
	c.UserMgrIPs = parseIPList(os.Getenv("BK_USERMGR_IP_COMMA"))
	c.NodeManIPs = parseIPList(os.Getenv("BK_NODEMAN_IP_COMMA"))

	// Open source components
	c.ES7IPs = parseIPList(os.Getenv("BK_ES7_IP_COMMA"))
	c.MySQLIP = os.Getenv("BK_MYSQL_IP")
	c.MySQLPort = envOrDefault("BK_MYSQL_PORT", "3306")
	c.RedisIP = os.Getenv("BK_REDIS_IP")
	c.RedisPort = envOrDefault("BK_REDIS_PORT", "6379")
	c.MongoDBIP = os.Getenv("BK_MONGODB_IP")
	c.MongoDBPort = envOrDefault("BK_MONGODB_PORT", "27017")
	c.RabbitMQIPs = parseIPList(os.Getenv("BK_RABBITMQ_IP_COMMA"))

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

	// Thresholds with defaults
	c.Thresholds = Thresholds{
		CPUUsage:     75,
		DiskUsage:    75,
		InodeUsage:   75,
		MemUsage:     75,
		MaxOpenFiles: 102400,
		RunDays:      365,
	}

	// SSH settings
	c.SSHUser = envOrDefault("INSPECT_SSH_USER", "root")
	c.SSHTimeout = 30

	// Mount paths
	c.CheckMountPath = envOrDefault("CHECK_MOUNT_PATH", "/data")

	// Build deduplicated host list
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

// GetModuleHosts returns the ordered list of modules and their hosts.
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

// buildAllHosts collects and deduplicates all IPs across all modules.
func (c *Config) buildAllHosts() []string {
	seen := make(map[string]bool)
	var hosts []string

	allIPs := [][]string{
		c.PaaSIPs, c.CMDBIPs, c.JobIPs, c.GSEIPs,
		c.APPOIPs, c.APPTIPs, c.IAMIPs, c.UserMgrIPs,
		c.NodeManIPs,
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
