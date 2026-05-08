package checker

import (
	"testing"

	"weops-inspect/model"
)

func TestCheckMongo_ErrorWarn(t *testing.T) {
	clusters := []model.MongoCluster{{Instance: "rs0", Error: "boom"}}
	got := CheckMongo(clusters)
	if len(got) != 1 || got[0].Status != model.StatusWarn {
		t.Fatalf("want warn, got %v", got)
	}
}

func TestCheckMongo_UnhealthyMemberNotice(t *testing.T) {
	clusters := []model.MongoCluster{{
		Instance: "rs0",
		Members:  []model.MongoMember{{Name: "1.1.1.1:27017", Health: 0}},
	}}
	got := CheckMongo(clusters)
	if len(got) != 1 || got[0].Status != model.StatusNotice {
		t.Fatalf("want notice, got %v", got)
	}
	if clusters[0].Members[0].HealthStatus != model.StatusNotice {
		t.Errorf("HealthStatus = %v", clusters[0].Members[0].HealthStatus)
	}
}

func TestCheckMongo_AllHealthy(t *testing.T) {
	clusters := []model.MongoCluster{{
		Instance: "rs0",
		Members:  []model.MongoMember{{Name: "1.1.1.1:27017", Health: 1}},
	}}
	got := CheckMongo(clusters)
	if len(got) != 0 {
		t.Fatalf("want no results, got %v", got)
	}
	if clusters[0].Members[0].HealthStatus != model.StatusOK {
		t.Errorf("HealthStatus = %v", clusters[0].Members[0].HealthStatus)
	}
}
