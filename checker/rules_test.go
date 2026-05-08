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
