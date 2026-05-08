package collector

// SubModule defines a sub-module within a BlueKing service.
type SubModule struct {
	Name        string
	ServiceUnit string // systemctl unit name, e.g. "bk-cmdb-api"
	ProcessName string // for ps grep, e.g. "cmdb_api"
	Port        int    // healthz port
	HealthzPath string // healthz URL path, e.g. "/healthz"
	HealthzType string // "http_status" | "json_ok" | "json_up" | "none"
}

// ModuleRegistry maps module names to their sub-module definitions.
var ModuleRegistry = map[string][]SubModule{
	"paas": {
		{Name: "appengine", ServiceUnit: "bk-paas-appengine", ProcessName: "uwsgi-open_paas-appengine.ini", Port: 8000, HealthzPath: "/v1/healthz/", HealthzType: "http_status"},
		{Name: "console", ServiceUnit: "bk-paas-console", ProcessName: "uwsgi-open_paas-console.ini", Port: 8004, HealthzPath: "/console/healthz/", HealthzType: "http_status"},
		{Name: "esb", ServiceUnit: "bk-paas-esb", ProcessName: "uwsgi-open_paas-esb.ini", Port: 8002, HealthzPath: "/healthz/", HealthzType: "http_status"},
		{Name: "login", ServiceUnit: "bk-paas-login", ProcessName: "uwsgi-open_paas-login.ini", Port: 8003, HealthzPath: "/healthz/", HealthzType: "http_status"},
		{Name: "paas", ServiceUnit: "bk-paas-paas", ProcessName: "uwsgi-open_paas-paas.ini", Port: 8001, HealthzPath: "/healthz/", HealthzType: "http_status"},
	},
	"cmdb": {
		{Name: "cmdb-admin", ServiceUnit: "bk-cmdb-admin", ProcessName: "cmdb_admin", Port: 9000, HealthzPath: "/healthz", HealthzType: "json_ok"},
		{Name: "cmdb-api", ServiceUnit: "bk-cmdb-api", ProcessName: "cmdb_api", Port: 9001, HealthzPath: "/healthz", HealthzType: "json_ok"},
		{Name: "cmdb-auth", ServiceUnit: "bk-cmdb-auth", ProcessName: "cmdb_auth", Port: 9002, HealthzPath: "/healthz", HealthzType: "json_ok"},
		{Name: "cmdb-cloud", ServiceUnit: "bk-cmdb-cloud", ProcessName: "cmdb_cloud", Port: 9003, HealthzPath: "/healthz", HealthzType: "json_ok"},
		{Name: "cmdb-core", ServiceUnit: "bk-cmdb-core", ProcessName: "cmdb_core", Port: 9004, HealthzPath: "/healthz", HealthzType: "json_ok"},
		{Name: "cmdb-datacollection", ServiceUnit: "bk-cmdb-datacollection", ProcessName: "cmdb_datacollection", Port: 9005, HealthzPath: "/healthz", HealthzType: "json_ok"},
		{Name: "cmdb-event", ServiceUnit: "bk-cmdb-event", ProcessName: "cmdb_event", Port: 9006, HealthzPath: "/healthz", HealthzType: "json_ok"},
		{Name: "cmdb-host", ServiceUnit: "bk-cmdb-host", ProcessName: "cmdb_host", Port: 9007, HealthzPath: "/healthz", HealthzType: "json_ok"},
		{Name: "cmdb-operation", ServiceUnit: "bk-cmdb-operation", ProcessName: "cmdb_operation", Port: 9008, HealthzPath: "/healthz", HealthzType: "json_ok"},
		{Name: "cmdb-proc", ServiceUnit: "bk-cmdb-proc", ProcessName: "cmdb_proc", Port: 9009, HealthzPath: "/healthz", HealthzType: "json_ok"},
		{Name: "cmdb-task", ServiceUnit: "bk-cmdb-task", ProcessName: "cmdb_task", Port: 9011, HealthzPath: "/healthz", HealthzType: "json_ok"},
		{Name: "cmdb-topo", ServiceUnit: "bk-cmdb-topo", ProcessName: "cmdb_topo", Port: 9012, HealthzPath: "/healthz", HealthzType: "json_ok"},
		{Name: "cmdb-web", ServiceUnit: "bk-cmdb-web", ProcessName: "cmdb_web", Port: 9013, HealthzPath: "/healthz", HealthzType: "json_ok"},
		{Name: "cmdb-cache", ServiceUnit: "bk-cmdb-cache", ProcessName: "cmdb_cache", Port: 9014, HealthzPath: "/healthz", HealthzType: "json_ok"},
	},
	"job": {
		{Name: "job-backup", ServiceUnit: "bk-job-backup", ProcessName: "job-backup", Port: 10507, HealthzPath: "/actuator/health", HealthzType: "json_up"},
		{Name: "job-config", ServiceUnit: "bk-job-config", ProcessName: "job-config", Port: 10500, HealthzPath: "/actuator/health", HealthzType: "json_up"},
		{Name: "job-crontab", ServiceUnit: "bk-job-crontab", ProcessName: "job-crontab", Port: 10501, HealthzPath: "/actuator/health", HealthzType: "json_up"},
		{Name: "job-execute", ServiceUnit: "bk-job-execute", ProcessName: "job-execute", Port: 10502, HealthzPath: "/actuator/health", HealthzType: "json_up"},
		{Name: "job-logsvr", ServiceUnit: "bk-job-logsvr", ProcessName: "job-logsvr", Port: 10504, HealthzPath: "/actuator/health", HealthzType: "json_up"},
		{Name: "job-manage", ServiceUnit: "bk-job-manage", ProcessName: "job-manage", Port: 10505, HealthzPath: "/actuator/health", HealthzType: "json_up"},
		{Name: "job-analysis", ServiceUnit: "bk-job-analysis", ProcessName: "job-analysis", Port: 10508, HealthzPath: "/actuator/health", HealthzType: "json_up"},
		{Name: "job-gateway", ServiceUnit: "bk-job-gateway", ProcessName: "job-gateway", Port: 10503, HealthzPath: "/actuator/health", HealthzType: "json_up"},
	},
	"gse": {
		{Name: "gse-alarm", ServiceUnit: "bk-gse-alarm", ProcessName: "gse_alarm", Port: 0, HealthzPath: "", HealthzType: "none"},
		{Name: "gse-api", ServiceUnit: "bk-gse-api", ProcessName: "gse_api", Port: 0, HealthzPath: "", HealthzType: "none"},
		{Name: "gse-btsvr", ServiceUnit: "bk-gse-btsvr", ProcessName: "gse_btsvr", Port: 0, HealthzPath: "", HealthzType: "none"},
		{Name: "gse-data", ServiceUnit: "bk-gse-data", ProcessName: "gse_data", Port: 0, HealthzPath: "", HealthzType: "none"},
		{Name: "gse-dba", ServiceUnit: "bk-gse-dba", ProcessName: "gse_dba", Port: 0, HealthzPath: "", HealthzType: "none"},
		{Name: "gse-procmgr", ServiceUnit: "bk-gse-procmgr", ProcessName: "gse_procmgr", Port: 0, HealthzPath: "", HealthzType: "none"},
		{Name: "gse-config", ServiceUnit: "bk-gse-config", ProcessName: "gse_config", Port: 0, HealthzPath: "", HealthzType: "none"},
		{Name: "gse-task", ServiceUnit: "bk-gse-task", ProcessName: "gse_task", Port: 0, HealthzPath: "", HealthzType: "none"},
	},
	"appo": {
		{Name: "bk-paasagent", ServiceUnit: "bk-paasagent", ProcessName: "paasagent", Port: 0, HealthzPath: "", HealthzType: "none"},
	},
	"appt": {
		{Name: "bk-paasagent", ServiceUnit: "bk-paasagent", ProcessName: "paasagent", Port: 0, HealthzPath: "", HealthzType: "none"},
	},
	"iam": {
		{Name: "bkiam", ServiceUnit: "bk-iam", ProcessName: "bkiam", Port: 5001, HealthzPath: "/healthz", HealthzType: "http_status"},
	},
	"usermgr": {
		// Django + gunicorn 的 /healthz 会 301 → /healthz/,直接打带斜杠的目标 URL 拿 200。
		{Name: "usermgr", ServiceUnit: "bk-usermgr", ProcessName: "usermgr", Port: 8009, HealthzPath: "/healthz/", HealthzType: "http_status"},
	},
	"nodeman": {
		{Name: "nodeman", ServiceUnit: "bk-nodeman", ProcessName: "nodeman", Port: 10300, HealthzPath: "/", HealthzType: "http_alive"},
	},
	// bkmonitorv3 拆为 4 个独立 module key,每个 key 只含本角色的 SubModule。
	// 这样 service.go 流水线在某主机上只会探测该主机实际部署的角色,
	// 避免跨角色 not-found / unreachable 误报。
	"bkmonitorv3-monitor": {
		// monitor 走 supervisord 拉起多个 Python 进程,故 ProcessName 取 supervisord
		// (worker 数反映 supervisor 自身,子进程不计)。
		{Name: "monitor", ServiceUnit: "bk-monitor", ProcessName: "supervisord", Port: 10204, HealthzPath: "/", HealthzType: "http_alive"},
	},
	"bkmonitorv3-influxdb-proxy": {
		{Name: "influxdb-proxy", ServiceUnit: "bk-influxdb-proxy", ProcessName: "influxdb-proxy", Port: 10203, HealthzPath: "/", HealthzType: "http_alive"},
	},
	"bkmonitorv3-transfer": {
		{Name: "transfer", ServiceUnit: "bk-transfer", ProcessName: "transfer", Port: 10202, HealthzPath: "/", HealthzType: "http_alive"},
	},
	"bkmonitorv3-unify-query": {
		{Name: "unify-query", ServiceUnit: "bk-unify-query", ProcessName: "unify-query", Port: 10206, HealthzPath: "/", HealthzType: "http_alive"},
	},
}
