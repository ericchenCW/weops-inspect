package notify

import (
	"testing"

	"weops-inspect/model"
)

func TestExtractAlerts_OnlyWarn(t *testing.T) {
	report := &model.InspectReport{
		AllChecks: []model.CheckResult{
			{Field: "es.cluster_error", Status: model.StatusWarn, Value: "boom"},
			{Field: "es.heap", Status: model.StatusNotice, Value: "90%"},
			{Field: "service.x.status", Status: model.StatusUnknown, Value: "(empty)"},
			{Field: "redis.x.error", Status: model.StatusOK, Value: ""},
		},
	}
	items := ExtractAlerts(report)
	if len(items) != 1 {
		t.Fatalf("want 1 warn, got %d: %v", len(items), items)
	}
	if items[0].Field != "es.cluster_error" {
		t.Errorf("got %v", items[0])
	}
}

func TestExtractAlerts_HostBackfilled(t *testing.T) {
	report := &model.InspectReport{
		Hosts: []model.HostCheckResult{{
			Metrics: model.HostMetrics{IP: "10.0.0.1"},
			Checks: []model.CheckResult{
				{Field: "cpu_usage", Value: "98%", Status: model.StatusWarn},
			},
		}},
		AllChecks: []model.CheckResult{
			{Field: "cpu_usage", Value: "98%", Status: model.StatusWarn},
		},
	}
	items := ExtractAlerts(report)
	if len(items) != 1 || items[0].Host != "10.0.0.1" {
		t.Errorf("host attribution failed: %v", items)
	}
}

func TestExtractUnknowns_Counts(t *testing.T) {
	report := &model.InspectReport{
		AllChecks: []model.CheckResult{
			{Status: model.StatusUnknown},
			{Status: model.StatusUnknown},
			{Status: model.StatusWarn},
		},
	}
	if got := ExtractUnknowns(report); got != 2 {
		t.Errorf("got %d, want 2", got)
	}
}
