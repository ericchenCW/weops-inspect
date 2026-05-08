package notify

import (
	"strings"
	"testing"

	"weops-inspect/model"
)

func TestBuildAlertBody_RendersThreshold(t *testing.T) {
	report := &model.InspectReport{
		Timestamp: "2026-05-08T12:00:00Z",
		Summary:   model.CheckSummary{Total: 10, OK: 7, Warn: 3, Unknown: 0},
	}
	items := []AlertItem{
		{Host: "10.0.0.1", Field: "cpu_usage", Value: "96.50%", Threshold: "≥ 95%"},
		{Host: "10.0.0.2", Field: "load_average", Value: "5.0/4.0/3.0 (cores: 2)", Threshold: ""},
		{Host: "", Field: "rabbitmq.v.q.backlog", Value: "20000 msgs / 1 consumers", Threshold: "> 10000 msgs"},
	}

	body := BuildAlertBody(report, items)

	if !strings.Contains(body, "(阈值 ≥ 95%)") {
		t.Errorf("CPU row missing threshold label:\n%s", body)
	}
	if !strings.Contains(body, "(阈值 > 10000 msgs)") {
		t.Errorf("rabbitmq row missing threshold label:\n%s", body)
	}
	// load_average has no Threshold; ensure no stray "(阈值 )" placeholder.
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, "load_average") && strings.Contains(line, "阈值") {
			t.Errorf("load_average row should not show threshold marker: %q", line)
		}
	}
}

func TestBuildAlertBody_NoThresholdOnAnyItem(t *testing.T) {
	report := &model.InspectReport{
		Timestamp: "2026-05-08T12:00:00Z",
		Summary:   model.CheckSummary{Total: 1, Warn: 1},
	}
	items := []AlertItem{{Host: "h", Field: "f", Value: "v"}}
	body := BuildAlertBody(report, items)
	if strings.Contains(body, "阈值") {
		t.Errorf("body must not contain '阈值' when no item carries one:\n%s", body)
	}
}
