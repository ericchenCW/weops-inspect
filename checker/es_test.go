package checker

import (
	"testing"

	"weops-inspect/config"
	"weops-inspect/model"
)

func defaultESThresholds() config.Thresholds {
	return config.Thresholds{ESHeapPercent: 85, ESRAMPercent: 95, ESUnassignedShards: 0}
}

func TestCheckES_ClusterErrorIsWarn(t *testing.T) {
	clusters := []model.ESCluster{{Instance: "es1", Error: "all nodes unreachable"}}
	got := CheckES(clusters, defaultESThresholds())
	if len(got) != 1 || got[0].Status != model.StatusWarn {
		t.Fatalf("want 1 warn, got %v", got)
	}
	if clusters[0].HealthStatus != model.StatusWarn {
		t.Errorf("HealthStatus = %v", clusters[0].HealthStatus)
	}
}

func TestCheckES_NodeUnreachableIsWarn(t *testing.T) {
	clusters := []model.ESCluster{{
		Instance:         "es1",
		Status:           "green",
		NodeReachability: []model.ESNodeReach{{IP: "1.1.1.1", Status: "unreachable"}},
	}}
	got := CheckES(clusters, defaultESThresholds())
	found := false
	for _, r := range got {
		if r.Status == model.StatusWarn {
			found = true
		}
	}
	if !found {
		t.Errorf("expected at least one warn, got %v", got)
	}
}

func TestCheckES_HealthStatusYellowIsNotice(t *testing.T) {
	clusters := []model.ESCluster{{Instance: "es1", Status: "yellow"}}
	got := CheckES(clusters, defaultESThresholds())
	if len(got) == 0 || got[0].Status != model.StatusNotice {
		t.Errorf("want notice, got %v", got)
	}
}

func TestCheckES_UnassignedShardsNotice(t *testing.T) {
	clusters := []model.ESCluster{{Instance: "es1", Status: "green", UnassignedShards: 3}}
	got := CheckES(clusters, defaultESThresholds())
	for _, r := range got {
		if r.Field == "es.es1.unassigned_shards" && r.Status == model.StatusNotice {
			return
		}
	}
	t.Errorf("expected unassigned notice, got %v", got)
}

func TestCheckES_HeapAndRAMNotice(t *testing.T) {
	clusters := []model.ESCluster{{
		Instance: "es1", Status: "green",
		Nodes: []model.ESNode{{IP: "1.1.1.1", HeapPercent: 90, RAMPercent: 96}},
	}}
	got := CheckES(clusters, defaultESThresholds())
	heap, ram := false, false
	for _, r := range got {
		if r.Field == "es.es1.1.1.1.1.heap" {
			heap = true
		}
		if r.Field == "es.es1.1.1.1.1.ram" {
			ram = true
		}
	}
	if !heap || !ram {
		t.Errorf("heap/ram missing: heap=%v ram=%v %v", heap, ram, got)
	}
	if clusters[0].Nodes[0].HeapStatus != model.StatusNotice {
		t.Errorf("HeapStatus = %v", clusters[0].Nodes[0].HeapStatus)
	}
}

func TestCheckES_PendingTasksNotice(t *testing.T) {
	clusters := []model.ESCluster{{Instance: "es1", Status: "green", PendingTasks: 5}}
	got := CheckES(clusters, defaultESThresholds())
	for _, r := range got {
		if r.Field == "es.es1.pending_tasks" && r.Status == model.StatusNotice {
			return
		}
	}
	t.Errorf("pending notice missing: %v", got)
}

func TestCheckES_AllGreenNoNotices(t *testing.T) {
	clusters := []model.ESCluster{{
		Instance: "es1", Status: "green",
		Nodes: []model.ESNode{{IP: "1.1.1.1", HeapPercent: 50, RAMPercent: 80}},
	}}
	got := CheckES(clusters, defaultESThresholds())
	for _, r := range got {
		if r.Status != model.StatusOK {
			t.Errorf("unexpected non-ok: %v", r)
		}
	}
}
