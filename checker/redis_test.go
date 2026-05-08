package checker

import (
	"testing"

	"weops-inspect/config"
	"weops-inspect/model"
)

func defaultRedisThresholds() config.Thresholds {
	return config.Thresholds{RedisCeleryQueue: 1000, RedisMonitorQueue: 10000}
}

func TestCheckRedis_NodeErrorWarn(t *testing.T) {
	nodes := []model.RedisNode{{IP: "1.1.1.1", Error: "connect refused"}}
	got := CheckRedis(nodes, defaultRedisThresholds())
	if len(got) == 0 || got[0].Status != model.StatusWarn {
		t.Fatalf("want warn, got %v", got)
	}
	if nodes[0].ErrorStatus != model.StatusWarn {
		t.Errorf("ErrorStatus = %v", nodes[0].ErrorStatus)
	}
}

func TestCheckRedis_CeleryNotice(t *testing.T) {
	nodes := []model.RedisNode{{IP: "1.1.1.1", CeleryQueue: 5000}}
	got := CheckRedis(nodes, defaultRedisThresholds())
	for _, r := range got {
		if r.Field == "redis.1.1.1.1.celery_queue" && r.Status == model.StatusNotice {
			return
		}
	}
	t.Errorf("celery notice missing: %v", got)
}

func TestCheckRedis_AllOK(t *testing.T) {
	nodes := []model.RedisNode{{IP: "1.1.1.1", CeleryQueue: 0, MonitorQueue: 0}}
	got := CheckRedis(nodes, defaultRedisThresholds())
	for _, r := range got {
		if r.Status != model.StatusOK {
			t.Errorf("unexpected: %v", r)
		}
	}
}

func TestCheckRedisSentinel_AllProblems(t *testing.T) {
	s := &model.SentinelClusterStatus{
		Error:           "boom",
		MasterReachable: false,
		MasterEnvMatch:  "warn",
		Status:          "critical",
		Sentinels: []model.SentinelNodeStatus{{IP: "1.1.1.1", Reachable: false}},
	}
	got := CheckRedisSentinel(s)
	if len(got) < 5 {
		t.Errorf("want >=5 warns, got %v", got)
	}
	for _, r := range got {
		if r.Status != model.StatusWarn {
			t.Errorf("expected warn, got %v", r)
		}
	}
	if s.MasterReachableStatus != model.StatusWarn || s.OverallStatus != model.StatusWarn {
		t.Errorf("statuses not backfilled: %+v", s)
	}
}

func TestCheckRedisSentinel_AllOK(t *testing.T) {
	s := &model.SentinelClusterStatus{
		MasterReachable: true, MasterEnvMatch: "ok", Status: "ok",
		Sentinels: []model.SentinelNodeStatus{{IP: "1.1.1.1", Reachable: true}},
	}
	got := CheckRedisSentinel(s)
	if len(got) != 0 {
		t.Errorf("want no warns, got %v", got)
	}
}

func TestCheckRedisSentinel_Nil(t *testing.T) {
	if got := CheckRedisSentinel(nil); got != nil {
		t.Errorf("want nil, got %v", got)
	}
}
