package model

// DiskUsage represents disk or inode usage for a mount point.
type DiskUsage struct {
	MountPoint string  `json:"mount_point"`
	Usage      string  `json:"usage"`
	UsageFloat float64 `json:"-"` // parsed numeric value for rule checking
}

// NetworkStats holds TCP connection counts by state.
type NetworkStats struct {
	CloseWait   int `json:"CLOSE_WAIT"`
	Established int `json:"ESTABLISHED"`
	Listen      int `json:"LISTEN"`
	TimeWait    int `json:"TIME_WAIT"`
}

// HostMetrics contains all system metrics collected from a host.
type HostMetrics struct {
	IP           string      `json:"ip"`
	CPUUsage     float64     `json:"cpu_usage"`
	MemUsage     float64     `json:"mem_usage"`
	LoadAvg1     float64     `json:"loadavg1"`
	LoadAvg5     float64     `json:"loadavg5"`
	LoadAvg15    float64     `json:"loadavg15"`
	SwapUsage    float64     `json:"swap_usage"`
	DiskUsage    []DiskUsage `json:"disk_usage"`
	InodeUsage   []DiskUsage `json:"innode_usage"`
	MaxOpenFiles int         `json:"max_open_files"`
	ProcessTotal int         `json:"process_total"`
	Ntpd         string      `json:"ntpd"`
	Chronyd      string      `json:"chronyd"`
	SELinux      string      `json:"selinux"`
	Firewalld    string      `json:"firewalld"`
	Proxy        int         `json:"proxy"`
	Iptables     string      `json:"iptables"`
	Network      NetworkStats `json:"network"`
	RunDays      int         `json:"run_days"`
	Version      string      `json:"version"`
	Kernel       string      `json:"kernel"`
	Memory       float64     `json:"memory"`  // total memory in MB
	Swap         float64     `json:"swap"`    // total swap in MB
	Core         int         `json:"core"`    // CPU cores
	Manufacturer string      `json:"manufacturer"`
	Product      string      `json:"product"`
	Serial       string      `json:"serial"`
	Error        string      `json:"error,omitempty"` // set when host is unreachable
}

// ServiceModule represents a single sub-module's status.
type ServiceModule struct {
	Module     string `json:"module"`
	Status     string `json:"status"`
	Resolved   string `json:"resolved"`
	HealthzAPI string `json:"healthz_api"`
	MainPID    int    `json:"main_pid"`
	StartTime  string `json:"start_time"`
	Workers    int    `json:"workers"`
}

// ServiceStatus represents one BlueKing module's status on one host.
type ServiceStatus struct {
	HostIP   string          `json:"host_ip"`
	Module   string          `json:"module"`   // e.g. "paas", "cmdb"
	Services []ServiceModule `json:"services"`
	// Docker container info (for appo/appt)
	ContainersUp     int `json:"containers_up,omitempty"`
	ContainersExited int `json:"containers_exited,omitempty"`
	Error            string `json:"error,omitempty"`
}

// ESNode represents an Elasticsearch node's metrics.
type ESNode struct {
	IP          string  `json:"ip"`
	HeapPercent int     `json:"heap_percent"`
	RAMPercent  int     `json:"ram_percent"`
	CPU         int     `json:"cpu"`
	Load1m      float64 `json:"load_1m"`
	Load5m      float64 `json:"load_5m"`
	Load15m     float64 `json:"load_15m"`
	Role        string  `json:"role"`
}

// ESCluster represents an Elasticsearch cluster's health and nodes.
type ESCluster struct {
	Instance              string   `json:"instance"`
	ClusterName           string   `json:"cluster_name"`
	Status                string   `json:"status"` // green/yellow/red
	NumberOfNodes         int      `json:"number_of_nodes"`
	NumberOfDataNodes     int      `json:"number_of_data_nodes"`
	ActivePrimaryShards   int      `json:"active_primary_shards"`
	ActiveShards          int      `json:"active_shards"`
	UnassignedShards      int      `json:"unassigned_shards"`
	PendingTasks          int      `json:"number_of_pending_tasks"`
	ActiveShardsPercent   float64  `json:"active_shards_percent_as_number"`
	Nodes                 []ESNode `json:"nodes"`
	Error                 string   `json:"error,omitempty"`
}

// MySQLNode represents a MySQL node's configuration and status.
type MySQLNode struct {
	IP                   string `json:"ip"`
	Version              string `json:"version"`
	MaxConnections       int    `json:"max_connections"`
	ExpireLogsDays       int    `json:"expire_logs_days"`
	MaxAllowedPacket     int64  `json:"max_allowed_packet"`
	Role                 string `json:"role"` // master/slave
	BinlogCount          int    `json:"binlog_count"`
	SlaveIOState         string `json:"slave_io_state"`
	SlaveSQLState        string `json:"slave_sql_state"`
	SlowQueryLog         string `json:"slow_query_log"`
	CharacterSet         string `json:"character_set"`
	BufferPoolSize       int64  `json:"buffer_pool_size"`
	BufferPoolInstances  int    `json:"buffer_pool_instances"`
	InnodbIOCapacity     int    `json:"innodb_io_capacity"`
	InnodbReadIOThreads  int    `json:"innodb_read_io_threads"`
	InnodbWriteIOThreads int    `json:"innodb_write_io_threads"`
	InteractiveTimeout   int    `json:"interactive_timeout"`
	TableOpenCache       int    `json:"table_open_cache"`
	WaitTimeout          int    `json:"wait_timeout"`
	Error                string `json:"error,omitempty"`
}

// MySQLCluster represents a MySQL cluster instance.
type MySQLCluster struct {
	Instance string      `json:"instance"`
	Nodes    []MySQLNode `json:"nodes"`
	Error    string      `json:"error,omitempty"`
}

// RedisNode represents a Redis node's metrics.
type RedisNode struct {
	IP              string `json:"ip"`
	Role            string `json:"role"`
	ClusterEnabled  string `json:"cluster_enabled"`
	Version         string `json:"version"`
	UsedMemory      string `json:"used_memory"`
	MaxMemory       string `json:"max_memory"`
	UptimeDays      string `json:"uptime_days"`
	ConnectedClients string `json:"connected_clients"`
	BlockedClients  string `json:"blocked_clients"`
	CeleryQueue     int    `json:"celery_queue"`
	MonitorQueue    int    `json:"monitor_queue"`
	Error           string `json:"error,omitempty"`
}

// RedisCluster represents a Redis cluster instance.
type RedisCluster struct {
	Instance string      `json:"instance"`
	Nodes    []RedisNode `json:"nodes"`
	Error    string      `json:"error,omitempty"`
}

// MongoMember represents a MongoDB replica set member.
type MongoMember struct {
	Name           string `json:"name"`
	Health         int    `json:"health"`
	StateStr       string `json:"stateStr"`
	Uptime         int64  `json:"uptime"`
	SyncingTo      string `json:"syncingTo"`
	SyncSourceHost string `json:"syncSourceHost"`
}

// MongoCluster represents a MongoDB cluster instance.
type MongoCluster struct {
	Instance string        `json:"instance"`
	Members  []MongoMember `json:"members"`
	Error    string        `json:"error,omitempty"`
}

// RabbitMQAlarm represents a node alarm in RabbitMQ.
type RabbitMQAlarm struct {
	Node          string `json:"node"`
	MemAlarm      bool   `json:"mem_alarm"`
	DiskFreeAlarm bool   `json:"disk_free_alarm"`
}

// RabbitMQQueue represents a problematic queue.
type RabbitMQQueue struct {
	VHost        string `json:"vhost"`
	Queue        string `json:"queue"`
	MessageCount int    `json:"message_count"`
	Consumers    int    `json:"consumers"`
	Durable      bool   `json:"durable"`
}

// RabbitMQStatus represents the RabbitMQ cluster status.
type RabbitMQStatus struct {
	ClusterPartition    bool            `json:"cluster_partition"`
	Uptime              string          `json:"uptime"`
	TotalConnections    int             `json:"total_connections"`
	AbnormalConnections int             `json:"abnormal_connections"`
	TotalChannels       int             `json:"total_channels"`
	NodeAlarms          []RabbitMQAlarm `json:"nodes_alarms"`
	ExceedingQueues     []RabbitMQQueue `json:"queues_exceeding_message_threshold"`
	NoConsumerQueues    []RabbitMQQueue `json:"queues_with_no_consumers"`
	Error               string          `json:"error,omitempty"`
}

// CheckStatus represents the status of a rule check.
type CheckStatus string

const (
	StatusOK   CheckStatus = "ok"
	StatusWarn CheckStatus = "warn"
)

// CheckResult represents the result of checking one field against a rule.
type CheckResult struct {
	Field  string      `json:"field"`
	Value  string      `json:"value"`
	Status CheckStatus `json:"status"`
}

// CheckSummary holds aggregated rule check results.
type CheckSummary struct {
	Total   int `json:"total"`
	OK      int `json:"ok"`
	Warn    int `json:"warn"`
}

// HostCheckResult holds a host's metrics with its rule check results.
type HostCheckResult struct {
	Metrics HostMetrics   `json:"metrics"`
	Checks  []CheckResult `json:"checks"`
}

// InspectReport is the top-level report containing all inspection data.
type InspectReport struct {
	Timestamp      string                      `json:"timestamp"`
	Hosts          []HostCheckResult            `json:"hosts"`
	Services       map[string][]ServiceStatus   `json:"services"` // module -> statuses
	ES             []ESCluster                  `json:"elasticsearch"`
	MySQL          []MySQLCluster               `json:"mysql"`
	Redis          []RedisCluster               `json:"redis"`
	MongoDB        []MongoCluster               `json:"mongodb"`
	RabbitMQ       *RabbitMQStatus              `json:"rabbitmq"`
	Summary        CheckSummary                 `json:"summary"`
}
