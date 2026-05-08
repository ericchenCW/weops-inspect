package checker

import (
	"testing"

	"weops-inspect/config"
	"weops-inspect/model"
)

func TestCheckService_EmptyStatusIsUnknown(t *testing.T) {
	sm := &model.ServiceModule{Module: "job-analysis", Status: "", HealthzAPI: "N/A"}
	results := CheckService(sm, "10.0.0.1", "job")
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	if results[0].Status != model.StatusUnknown {
		t.Errorf("status = %v, want unknown", results[0].Status)
	}
	if sm.RenderStatus != model.StatusUnknown {
		t.Errorf("RenderStatus = %v, want unknown", sm.RenderStatus)
	}
}

func TestCheckService_ActiveOK(t *testing.T) {
	sm := &model.ServiceModule{Module: "paas", Status: "active", HealthzAPI: "ok"}
	results := CheckService(sm, "10.0.0.1", "paas")
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Status != model.StatusOK {
			t.Errorf("expected ok, got %v for %s", r.Status, r.Field)
		}
	}
	if sm.RenderStatus != model.StatusOK || sm.HealthzRenderStatus != model.StatusOK {
		t.Errorf("RenderStatus=%v HealthzRenderStatus=%v", sm.RenderStatus, sm.HealthzRenderStatus)
	}
}

func TestCheckService_NonActiveWarn(t *testing.T) {
	sm := &model.ServiceModule{Module: "x", Status: "failed", HealthzAPI: "N/A"}
	results := CheckService(sm, "1.1.1.1", "m")
	if len(results) != 1 || results[0].Status != model.StatusWarn {
		t.Fatalf("want 1 warn result, got %v", results)
	}
}

func TestCheckServiceContainers_NoticeOnExited(t *testing.T) {
	s := &model.ServiceStatus{Module: "appo", ContainersUp: 18, ContainersExited: 3}
	results := CheckServiceContainers(s, config.Thresholds{ServiceContainersExited: 0})
	if len(results) != 1 || results[0].Status != model.StatusNotice {
		t.Fatalf("want 1 notice, got %v", results)
	}
	if s.ExitedRenderStatus != model.StatusNotice {
		t.Errorf("ExitedRenderStatus = %v", s.ExitedRenderStatus)
	}
}

func TestCheckServiceContainers_OKWhenWithinThreshold(t *testing.T) {
	s := &model.ServiceStatus{Module: "appo", ContainersUp: 18, ContainersExited: 0}
	results := CheckServiceContainers(s, config.Thresholds{ServiceContainersExited: 0})
	if len(results) != 0 {
		t.Fatalf("want no results, got %v", results)
	}
	if s.ExitedRenderStatus != model.StatusOK {
		t.Errorf("ExitedRenderStatus = %v, want ok", s.ExitedRenderStatus)
	}
}

func TestCheckServiceCollectError_Notice(t *testing.T) {
	s := &model.ServiceStatus{Module: "paas", Error: "ssh timeout"}
	results := CheckServiceCollectError(s)
	if len(results) != 1 || results[0].Status != model.StatusNotice {
		t.Fatalf("want 1 notice, got %v", results)
	}
}

func TestCheckHost_Thresholds(t *testing.T) {
	thr := config.Thresholds{
		CPUUsage: 95, MemUsage: 95, DiskUsage: 95, InodeUsage: 95, MaxOpenFiles: 65536,
	}
	h := model.HostMetrics{
		CPUUsage: 96, MemUsage: 50, MaxOpenFiles: 1024,
		DiskUsage:  []model.DiskUsage{{MountPoint: "/", Usage: "97%", UsageFloat: 97}},
		InodeUsage: []model.DiskUsage{{MountPoint: "/", Usage: "12%", UsageFloat: 12}},
		SELinux:    "enforcing", Firewalld: "active", Chronyd: "inactive",
	}
	results := CheckHost(h, thr)

	want := map[string]string{
		"cpu_usage":      "≥ 95%",
		"mem_usage":      "≥ 95%",
		"disk_usage(/)":  "≥ 95%",
		"inode_usage(/)": "≥ 95%",
		"max_open_files": "< 65536",
		"selinux":        "期望 Disabled",
		"firewalld":      "期望 inactive",
		"chronyd":        "期望 active",
	}
	got := map[string]model.CheckResult{}
	for _, r := range results {
		got[r.Field] = r
	}
	for field, wantThr := range want {
		r, ok := got[field]
		if !ok {
			t.Errorf("missing field %q", field)
			continue
		}
		if r.Threshold != wantThr {
			t.Errorf("%s: Threshold = %q, want %q", field, r.Threshold, wantThr)
		}
	}
	// load_average is relational, should leave Threshold empty.
	if la, ok := got["load_average"]; ok && la.Threshold != "" {
		t.Errorf("load_average Threshold = %q, want empty", la.Threshold)
	}
}

func TestCheckService_ThresholdLabels(t *testing.T) {
	cases := []struct {
		name      string
		sm        *model.ServiceModule
		wantStatus string
		wantHealth string
	}{
		{"warn", &model.ServiceModule{Status: "failed", HealthzAPI: "fail"}, "期望 active", "期望 ok"},
		{"ok", &model.ServiceModule{Status: "active", HealthzAPI: "ok"}, "期望 active", "期望 ok"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			results := CheckService(c.sm, "1.1.1.1", "m")
			for _, r := range results {
				if r.Field == "service.m/.status" && r.Threshold != c.wantStatus {
					t.Errorf("status Threshold = %q, want %q", r.Threshold, c.wantStatus)
				}
				if r.Field == "service.m/.healthz" && r.Threshold != c.wantHealth {
					t.Errorf("healthz Threshold = %q, want %q", r.Threshold, c.wantHealth)
				}
			}
		})
	}
}

func TestCheckRabbitMQ_BacklogThreshold(t *testing.T) {
	r := &model.RabbitMQStatus{
		ExceedingQueues: []model.RabbitMQQueue{{VHost: "v", Queue: "q", MessageCount: 20000}},
	}
	results := CheckRabbitMQ(r, config.Thresholds{RabbitMQQueueBacklog: 10000})
	if len(results) != 1 {
		t.Fatalf("got %v", results)
	}
	if results[0].Threshold != "> 10000 msgs" {
		t.Errorf("Threshold = %q", results[0].Threshold)
	}
}

func TestCheckReplication_Thresholds(t *testing.T) {
	rep := &model.ReplicationReport{
		MySQLSlaves: []model.MySQLSlaveStatus{{
			IP: "1.1.1.1",
			Replication: &model.MySQLReplicationStatus{
				IORunning: "Yes", SQLRunning: "Yes", SecondsBehindMaster: 120, Status: "lag",
			},
		}},
		RedisNodes: []model.RedisReplicationStatus{{
			IP: "2.2.2.2", Role: "slave", LinkStatus: "stale",
			MasterLinkStatus: "down", MasterLastIOSeconds: 50,
		}},
	}
	results := CheckReplication(rep, config.Thresholds{MySQLReplLagSec: 60, RedisReplIOSec: 10})
	got := map[string]string{}
	for _, r := range results {
		got[r.Field] = r.Threshold
	}
	if got["mysql_slave(1.1.1.1).replication"] != "lag > 60s" {
		t.Errorf("mysql Threshold = %q", got["mysql_slave(1.1.1.1).replication"])
	}
	if got["redis(2.2.2.2).link"] != "io > 10s" {
		t.Errorf("redis link Threshold = %q", got["redis(2.2.2.2).link"])
	}
}

func TestSummarize_NoticeExcluded(t *testing.T) {
	results := []model.CheckResult{
		{Status: model.StatusOK},
		{Status: model.StatusOK},
		{Status: model.StatusWarn},
		{Status: model.StatusUnknown},
		{Status: model.StatusNotice},
		{Status: model.StatusNotice},
	}
	s := Summarize(results)
	if s.Total != 4 || s.OK != 2 || s.Warn != 1 || s.Unknown != 1 {
		t.Errorf("Summary = %+v", s)
	}
}
