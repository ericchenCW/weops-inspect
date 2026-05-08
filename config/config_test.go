package config

import (
	"os"
	"reflect"
	"testing"
)

// unsetEnv 在测试结束时恢复原值; 与 t.Setenv("") 不同, 它真正 LookupEnv 不可见。
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	prev, hadPrev := os.LookupEnv(key)
	os.Unsetenv(key)
	t.Cleanup(func() {
		if hadPrev {
			os.Setenv(key, prev)
		} else {
			os.Unsetenv(key)
		}
	})
}

func TestThresholds_Defaults(t *testing.T) {
	t.Setenv("BK_PAAS_IP_COMMA", "10.0.0.1")
	// 数值阈值 env 走 os.Getenv: ""=未设置走默认。
	for _, k := range []string{
		"INSPECT_CPU_THRESHOLD", "INSPECT_MEM_THRESHOLD",
		"INSPECT_DISK_THRESHOLD", "INSPECT_INODE_THRESHOLD",
		"INSPECT_MAX_OPEN_FILES",
	} {
		t.Setenv(k, "")
	}
	c, err := Load("/tmp")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Thresholds.CPUUsage != 95 {
		t.Errorf("CPUUsage default = %v, want 95", c.Thresholds.CPUUsage)
	}
	if c.Thresholds.MemUsage != 95 {
		t.Errorf("MemUsage default = %v, want 95", c.Thresholds.MemUsage)
	}
	if c.Thresholds.DiskUsage != 95 {
		t.Errorf("DiskUsage default = %v, want 95", c.Thresholds.DiskUsage)
	}
	if c.Thresholds.InodeUsage != 95 {
		t.Errorf("InodeUsage default = %v, want 95", c.Thresholds.InodeUsage)
	}
	if c.Thresholds.MaxOpenFiles != 65536 {
		t.Errorf("MaxOpenFiles default = %v, want 65536", c.Thresholds.MaxOpenFiles)
	}
}

func TestNoticeThresholds_Defaults(t *testing.T) {
	t.Setenv("BK_PAAS_IP_COMMA", "10.0.0.1")
	for _, k := range []string{
		"INSPECT_ES_HEAP_THRESHOLD", "INSPECT_ES_RAM_THRESHOLD",
		"INSPECT_ES_UNASSIGNED_SHARDS_THRESHOLD",
		"INSPECT_REDIS_CELERY_QUEUE_THRESHOLD", "INSPECT_REDIS_MONITOR_QUEUE_THRESHOLD",
		"INSPECT_DOCKER_EXITED_THRESHOLD",
	} {
		unsetEnv(t, k)
	}
	c, err := Load("/tmp")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"ESHeapPercent", c.Thresholds.ESHeapPercent, 85},
		{"ESRAMPercent", c.Thresholds.ESRAMPercent, 95},
		{"ESUnassignedShards", c.Thresholds.ESUnassignedShards, 0},
		{"RedisCeleryQueue", c.Thresholds.RedisCeleryQueue, 1000},
		{"RedisMonitorQueue", c.Thresholds.RedisMonitorQueue, 10000},
		{"ServiceContainersExited", c.Thresholds.ServiceContainersExited, 0},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s default = %d, want %d", c.name, c.got, c.want)
		}
	}
}

func TestNoticeThresholds_EnvOverride(t *testing.T) {
	t.Setenv("BK_PAAS_IP_COMMA", "10.0.0.1")
	t.Setenv("INSPECT_ES_HEAP_THRESHOLD", "90")
	t.Setenv("INSPECT_ES_RAM_THRESHOLD", "98")
	t.Setenv("INSPECT_REDIS_CELERY_QUEUE_THRESHOLD", "5000")
	t.Setenv("INSPECT_DOCKER_EXITED_THRESHOLD", "5")
	c, err := Load("/tmp")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Thresholds.ESHeapPercent != 90 {
		t.Errorf("ESHeapPercent = %d, want 90", c.Thresholds.ESHeapPercent)
	}
	if c.Thresholds.ESRAMPercent != 98 {
		t.Errorf("ESRAMPercent = %d, want 98", c.Thresholds.ESRAMPercent)
	}
	if c.Thresholds.RedisCeleryQueue != 5000 {
		t.Errorf("RedisCeleryQueue = %d, want 5000", c.Thresholds.RedisCeleryQueue)
	}
	if c.Thresholds.ServiceContainersExited != 5 {
		t.Errorf("ServiceContainersExited = %d, want 5", c.Thresholds.ServiceContainersExited)
	}
}

func TestNoticeThresholds_InvalidNumber(t *testing.T) {
	t.Setenv("BK_PAAS_IP_COMMA", "10.0.0.1")
	t.Setenv("INSPECT_ES_HEAP_THRESHOLD", "not-a-number")
	if _, err := Load("/tmp"); err == nil {
		t.Fatal("Load should fail on invalid threshold")
	}
}

func TestRabbitMQNoConsumerVHostBlacklist_DefaultUnset(t *testing.T) {
	t.Setenv("BK_PAAS_IP_COMMA", "10.0.0.1")
	unsetEnv(t, "INSPECT_RABBITMQ_NO_CONSUMER_VHOST_BLACKLIST")
	c, err := Load("/tmp")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"bk_bknodeman"}
	if !reflect.DeepEqual(c.Thresholds.RabbitMQNoConsumerVHostBlacklist, want) {
		t.Errorf("default blacklist = %v, want %v", c.Thresholds.RabbitMQNoConsumerVHostBlacklist, want)
	}
}

func TestRabbitMQNoConsumerVHostBlacklist_EnvOverride(t *testing.T) {
	t.Setenv("BK_PAAS_IP_COMMA", "10.0.0.1")
	t.Setenv("INSPECT_RABBITMQ_NO_CONSUMER_VHOST_BLACKLIST", "foo, bar ,baz")
	c, err := Load("/tmp")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"foo", "bar", "baz"}
	if !reflect.DeepEqual(c.Thresholds.RabbitMQNoConsumerVHostBlacklist, want) {
		t.Errorf("env override = %v, want %v", c.Thresholds.RabbitMQNoConsumerVHostBlacklist, want)
	}
}

func TestRabbitMQNoConsumerVHostBlacklist_EmptyDisablesBlacklist(t *testing.T) {
	t.Setenv("BK_PAAS_IP_COMMA", "10.0.0.1")
	t.Setenv("INSPECT_RABBITMQ_NO_CONSUMER_VHOST_BLACKLIST", "")
	c, err := Load("/tmp")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Thresholds.RabbitMQNoConsumerVHostBlacklist) != 0 {
		t.Errorf("empty env should disable blacklist, got %v", c.Thresholds.RabbitMQNoConsumerVHostBlacklist)
	}
}

// withEnv 设置一组 env 并在测试结束时恢复。
func withEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for k, v := range kv {
		t.Setenv(k, v)
	}
}

// clearMySQLEnv 清空 BK_*_MYSQL_* 全部相关 env,避免 host 机器残留干扰子测试。
func clearMySQLEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"BK_MONITOR_MYSQL_HOST", "BK_MONITOR_MYSQL_PORT",
		"BK_MONITOR_MYSQL_USER", "BK_MONITOR_MYSQL_PASSWORD",
		"BK_PAAS_MYSQL_HOST", "BK_PAAS_MYSQL_PORT",
		"BK_PAAS_MYSQL_USER", "BK_PAAS_MYSQL_PASSWORD",
	} {
		t.Setenv(k, "")
	}
}

func TestBKMonitorV3MySQL_Default(t *testing.T) {
	clearMySQLEnv(t)
	t.Setenv("BK_MONITORV3_IP_COMMA", "10.0.0.1") // 让加载继续

	c, err := Load("/tmp")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := c.BKMonitorV3.MySQLHost, "mysql.service.consul"; got != want {
		t.Errorf("MySQLHost = %q, want %q", got, want)
	}
	if got, want := c.BKMonitorV3.MySQLPort, "3306"; got != want {
		t.Errorf("MySQLPort = %q, want %q", got, want)
	}
}

func TestBKMonitorV3MySQL_MonitorOverride(t *testing.T) {
	clearMySQLEnv(t)
	withEnv(t, map[string]string{
		"BK_MONITOR_MYSQL_HOST":     "10.10.26.235",
		"BK_MONITOR_MYSQL_PORT":     "3307",
		"BK_MONITOR_MYSQL_USER":     "monitor_user",
		"BK_MONITOR_MYSQL_PASSWORD": "monitor_pwd",
		"BK_PAAS_MYSQL_HOST":        "should-not-win",
		"BK_PAAS_MYSQL_USER":        "paas_user",
		"BK_MONITORV3_IP_COMMA":     "10.0.0.1",
	})
	c, err := Load("/tmp")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	dep := c.BKMonitorV3
	if dep.MySQLHost != "10.10.26.235" || dep.MySQLPort != "3307" {
		t.Errorf("monitor override failed: host=%q port=%q", dep.MySQLHost, dep.MySQLPort)
	}
	if dep.MySQLUser != "monitor_user" || dep.MySQLPassword != "monitor_pwd" {
		t.Errorf("monitor creds: user=%q pwd=%q", dep.MySQLUser, dep.MySQLPassword)
	}
}

// 找到 GetModuleHosts() 中指定 module 的 IP 列表,简化各场景断言。
func moduleIPs(t *testing.T, c *Config, module string) []string {
	t.Helper()
	for _, mh := range c.GetModuleHosts() {
		if mh.Module == module {
			return mh.IPs
		}
	}
	t.Fatalf("module %s not found", module)
	return nil
}

func clearMonitorV3Env(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"BK_MONITORV3_IP_COMMA",
		"BK_MONITORV3_MONITOR_IP_COMMA",
		"BK_MONITORV3_INFLUXDB_PROXY_IP_COMMA",
		"BK_MONITORV3_TRANSFER_IP_COMMA",
		"BK_MONITORV3_UNIFY_QUERY_IP_COMMA",
	} {
		t.Setenv(k, "")
	}
}

func TestMonitorV3Roles_LegacyFallback(t *testing.T) {
	clearMonitorV3Env(t)
	t.Setenv("BK_MONITORV3_IP_COMMA", "10.97.20.18")

	c, err := Load("/tmp")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, m := range []string{
		"bkmonitorv3-monitor", "bkmonitorv3-influxdb-proxy",
		"bkmonitorv3-transfer", "bkmonitorv3-unify-query",
	} {
		ips := moduleIPs(t, c, m)
		if len(ips) != 1 || ips[0] != "10.97.20.18" {
			t.Errorf("%s IPs = %v, want [10.97.20.18]", m, ips)
		}
	}
}

func TestMonitorV3Roles_PerRole(t *testing.T) {
	clearMonitorV3Env(t)
	t.Setenv("BK_MONITORV3_MONITOR_IP_COMMA", "10.10.26.235")
	t.Setenv("BK_MONITORV3_INFLUXDB_PROXY_IP_COMMA", "10.10.26.235")
	t.Setenv("BK_MONITORV3_TRANSFER_IP_COMMA", "10.10.26.236")
	t.Setenv("BK_MONITORV3_UNIFY_QUERY_IP_COMMA", "10.10.26.236")

	c, err := Load("/tmp")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := moduleIPs(t, c, "bkmonitorv3-monitor"); len(got) != 1 || got[0] != "10.10.26.235" {
		t.Errorf("monitor IPs = %v", got)
	}
	if got := moduleIPs(t, c, "bkmonitorv3-transfer"); len(got) != 1 || got[0] != "10.10.26.236" {
		t.Errorf("transfer IPs = %v", got)
	}
	// AllHosts 应包含两个 IP(各 1 次)
	seen235, seen236 := 0, 0
	for _, ip := range c.AllHosts {
		if ip == "10.10.26.235" {
			seen235++
		}
		if ip == "10.10.26.236" {
			seen236++
		}
	}
	if seen235 != 1 || seen236 != 1 {
		t.Errorf("AllHosts dedup mismatch: 235=%d 236=%d, hosts=%v", seen235, seen236, c.AllHosts)
	}
}

func TestMonitorV3Roles_MixedFallback(t *testing.T) {
	// 仅 transfer 单独配,其它角色与 legacy 共用 BK_MONITORV3_IP_COMMA
	clearMonitorV3Env(t)
	t.Setenv("BK_MONITORV3_IP_COMMA", "10.97.20.18")
	t.Setenv("BK_MONITORV3_TRANSFER_IP_COMMA", "10.10.26.236")

	c, err := Load("/tmp")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := moduleIPs(t, c, "bkmonitorv3-transfer"); len(got) != 1 || got[0] != "10.10.26.236" {
		t.Errorf("transfer should use role var, got %v", got)
	}
	if got := moduleIPs(t, c, "bkmonitorv3-monitor"); len(got) != 1 || got[0] != "10.97.20.18" {
		t.Errorf("monitor should fall back to legacy IP, got %v", got)
	}
}

func TestMonitorV3Roles_AllEmpty(t *testing.T) {
	// 所有相关变量全空 → 4 个 module IP 列表都为空,采集器各角色自然跳过
	clearMonitorV3Env(t)
	t.Setenv("BK_PAAS_IP_COMMA", "10.0.0.1") // 满足 Validate(): AllHosts 非空

	c, err := Load("/tmp")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, m := range []string{
		"bkmonitorv3-monitor", "bkmonitorv3-influxdb-proxy",
		"bkmonitorv3-transfer", "bkmonitorv3-unify-query",
	} {
		if ips := moduleIPs(t, c, m); len(ips) != 0 {
			t.Errorf("%s should be empty, got %v", m, ips)
		}
	}
}

func TestBKMonitorV3MySQL_PaaSFallback(t *testing.T) {
	clearMySQLEnv(t)
	withEnv(t, map[string]string{
		"BK_PAAS_MYSQL_HOST":     "10.10.26.236",
		"BK_PAAS_MYSQL_USER":     "root",
		"BK_PAAS_MYSQL_PASSWORD": "paas_pwd",
		"BK_MONITORV3_IP_COMMA":  "10.0.0.1",
	})
	c, err := Load("/tmp")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	dep := c.BKMonitorV3
	if dep.MySQLHost != "10.10.26.236" {
		t.Errorf("paas fallback host: got %q", dep.MySQLHost)
	}
	if dep.MySQLPort != "3306" {
		t.Errorf("paas fallback port should default to 3306, got %q", dep.MySQLPort)
	}
	if dep.MySQLUser != "root" || dep.MySQLPassword != "paas_pwd" {
		t.Errorf("paas fallback creds: user=%q pwd=%q", dep.MySQLUser, dep.MySQLPassword)
	}
}
