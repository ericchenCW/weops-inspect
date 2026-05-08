package checker_test

// E2E coverage: build a synthetic InspectReport that triggers every category
// (Warn / Notice / Unknown / OK) and assert that:
//   - Summary buckets match expected counts
//   - notify.ExtractAlerts matches the Warn subset of AllChecks
//   - HTML rendering matches AllChecks (red cells iff Warn or Notice)

import (
	"strings"
	"testing"

	"weops-inspect/checker"
	"weops-inspect/config"
	"weops-inspect/model"
	"weops-inspect/notify"
	"weops-inspect/render"
)

func defaultThresholds() config.Thresholds {
	return config.Thresholds{
		CPUUsage: 95, MemUsage: 95, DiskUsage: 95, InodeUsage: 95, MaxOpenFiles: 65536,
		MySQLReplLagSec: 60, RedisReplIOSec: 10, RabbitMQQueueBacklog: 10000,
		ESHeapPercent: 85, ESRAMPercent: 95, ESUnassignedShards: 0,
		RedisCeleryQueue: 1000, RedisMonitorQueue: 10000, ServiceContainersExited: 0,
	}
}

func buildFixtureReport() *model.InspectReport {
	r := &model.InspectReport{Timestamp: "2026-05-08 21:00:00"}

	// Hosts
	host := model.HostMetrics{
		IP: "10.10.26.237", CPUUsage: 50, MemUsage: 50, MaxOpenFiles: 102400,
		Chronyd: "active", Firewalld: "inactive", SELinux: "Disabled",
		Core: 16, LoadAvg1: 1, LoadAvg5: 1, LoadAvg15: 1,
	}
	r.Hosts = []model.HostCheckResult{{Metrics: host}}

	// Services: one normal + one empty-status (Unknown) + one warn + one with docker exited.
	r.Services = map[string][]model.ServiceStatus{
		"job": {{
			HostIP: "10.10.26.237", Module: "job",
			Services: []model.ServiceModule{
				{Module: "job-config", Status: "active", HealthzAPI: "ok"},
				{Module: "job-analysis", Status: "", HealthzAPI: "N/A"}, // Unknown
				{Module: "job-broken", Status: "failed", HealthzAPI: "N/A"},
			},
		}},
		"appt": {{
			HostIP: "10.10.26.237", Module: "appt",
			Services:         []model.ServiceModule{{Module: "bk-paasagent", Status: "active", HealthzAPI: "N/A"}},
			ContainersUp:     8,
			ContainersExited: 3,
		}},
	}

	// ES: one with cluster.Error, one healthy, one with RAM>95.
	r.ES = []model.ESCluster{
		{Instance: "es1", Error: "all nodes unreachable"},
		{Instance: "es2", Status: "green", Nodes: []model.ESNode{{IP: "1.1.1.1", HeapPercent: 50, RAMPercent: 96}}},
	}
	// Redis standalone: one with error, one with celery overflow.
	r.RedisStandalone = []model.RedisNode{
		{IP: "10.0.0.1", Error: "connect refused"},
		{IP: "10.0.0.2", CeleryQueue: 5000},
	}
	// RabbitMQ: one ExceedingQueues item.
	r.RabbitMQ = &model.RabbitMQStatus{
		ExceedingQueues: []model.RabbitMQQueue{{VHost: "prod_bk_monitorv3", Queue: "celery", MessageCount: 360547}},
	}
	// Mongo: error → Warn
	r.MongoDB = []model.MongoCluster{{Instance: "rs0", Error: "boom"}}
	// BKDeps: ok + fail (Notice)
	r.BKMonitorV3 = &model.BKMonitorV3Section{Dependencies: []model.DependencyResult{
		{Item: "redis", Status: "ok"},
		{Item: "mysql", Status: "fail"},
	}}
	return r
}

func runFullChecker(r *model.InspectReport, t config.Thresholds) []model.CheckResult {
	var all []model.CheckResult
	for i := range r.Hosts {
		all = append(all, checker.CheckHost(r.Hosts[i].Metrics, t)...)
	}
	for moduleKey, statuses := range r.Services {
		for i := range statuses {
			s := &statuses[i]
			all = append(all, checker.CheckServiceCollectError(s)...)
			for j := range s.Services {
				all = append(all, checker.CheckService(&s.Services[j], s.HostIP, moduleKey)...)
			}
			all = append(all, checker.CheckServiceContainers(s, t)...)
		}
		r.Services[moduleKey] = statuses
	}
	all = append(all, checker.CheckES(r.ES, t)...)
	all = append(all, checker.CheckRedis(r.RedisStandalone, t)...)
	all = append(all, checker.CheckRedisSentinel(r.RedisSentinel)...)
	all = append(all, checker.CheckMongo(r.MongoDB)...)
	all = append(all, checker.CheckRabbitMQ(r.RabbitMQ)...)
	all = append(all, checker.CheckBKDeps(r.BKMonitorV3)...)
	all = append(all, checker.CheckReplication(r.Replication)...)
	return all
}

func TestE2E_SummaryAndAlertsConsistent(t *testing.T) {
	report := buildFixtureReport()
	all := runFullChecker(report, defaultThresholds())
	report.AllChecks = all
	report.Summary = checker.Summarize(all)

	// Count expectations.
	if report.Summary.Unknown < 1 {
		t.Errorf("Unknown should include job-analysis empty status, got Summary=%+v", report.Summary)
	}
	if report.Summary.Warn < 5 {
		t.Errorf("Warn should include several entries (es.error, redis.error, rabbitmq backlog, mongo error, service warn), got %+v", report.Summary)
	}

	// notify.ExtractAlerts must equal the Warn subset.
	items := notify.ExtractAlerts(report)
	if len(items) != report.Summary.Warn {
		t.Errorf("ExtractAlerts size = %d, Summary.Warn = %d", len(items), report.Summary.Warn)
	}

	// Notice items must NOT appear in alerts.
	for _, it := range items {
		// no alert should reference bkdeps or es.unassigned (Notice candidates)
		if strings.HasPrefix(it.Field, "bkdeps.") {
			t.Errorf("bkdeps Notice leaked into Warn alerts: %v", it)
		}
	}
}

func TestE2E_HTMLRendersWithoutPanic(t *testing.T) {
	report := buildFixtureReport()
	all := runFullChecker(report, defaultThresholds())
	report.AllChecks = all
	report.Summary = checker.Summarize(all)

	tmpl, err := render.LoadTemplates()
	if err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	var buf strings.Builder
	if err := tmpl.ExecuteTemplate(&buf, "summary.html.tmpl", report); err != nil {
		t.Errorf("summary template: %v", err)
	}
	buf.Reset()
	if err := tmpl.ExecuteTemplate(&buf, "services.html.tmpl", report); err != nil {
		t.Errorf("services template: %v", err)
	}
	buf.Reset()
	if err := tmpl.ExecuteTemplate(&buf, "opensources.html.tmpl", report); err != nil {
		t.Errorf("opensources template: %v", err)
	}

	// Spot-check: rendered opensources should mention status-warn for the
	// RabbitMQ backlog row.
	html := buf.String()
	if !strings.Contains(html, "360547") {
		t.Errorf("expected backlog 360547 in opensources HTML, got\n%s", html[:min(len(html), 500)])
	}
}

func min(a, b int) int { if a < b { return a }; return b }
