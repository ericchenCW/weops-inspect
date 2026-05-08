package checker

import (
	"fmt"

	"weops-inspect/config"
	"weops-inspect/model"
)

// CheckRedis covers standalone Redis nodes:
//   - node.Error                       → Warn
//   - CeleryQueue > threshold          → Notice
//   - MonitorQueue > threshold         → Notice
func CheckRedis(nodes []model.RedisNode, t config.Thresholds) []model.CheckResult {
	var results []model.CheckResult
	for i := range nodes {
		n := &nodes[i]
		prefix := "redis." + n.IP + "."

		if n.Error != "" {
			n.ErrorStatus = model.StatusWarn
			results = append(results, model.CheckResult{
				Field: prefix + "error", Value: n.Error, Status: model.StatusWarn,
			})
		} else {
			n.ErrorStatus = ""
		}

		if n.CeleryQueue > t.RedisCeleryQueue {
			n.CeleryQueueStatus = model.StatusNotice
			results = append(results, model.CheckResult{
				Field: prefix + "celery_queue",
				Value: fmt.Sprintf("%d", n.CeleryQueue),
				Status: model.StatusNotice,
			})
		} else {
			n.CeleryQueueStatus = model.StatusOK
		}

		if n.MonitorQueue > t.RedisMonitorQueue {
			n.MonitorQueueStatus = model.StatusNotice
			results = append(results, model.CheckResult{
				Field: prefix + "monitor_queue",
				Value: fmt.Sprintf("%d", n.MonitorQueue),
				Status: model.StatusNotice,
			})
		} else {
			n.MonitorQueueStatus = model.StatusOK
		}
	}
	return results
}

// CheckRedisSentinel covers Sentinel cluster status. All findings are Warn.
func CheckRedisSentinel(s *model.SentinelClusterStatus) []model.CheckResult {
	if s == nil {
		return nil
	}
	var results []model.CheckResult

	if s.Error != "" {
		results = append(results, model.CheckResult{
			Field: "redis_sentinel.error", Value: s.Error, Status: model.StatusWarn,
		})
	}

	if !s.MasterReachable {
		s.MasterReachableStatus = model.StatusWarn
		results = append(results, model.CheckResult{
			Field: "redis_sentinel.master_reachable", Value: "false", Status: model.StatusWarn,
		})
	} else {
		s.MasterReachableStatus = model.StatusOK
	}

	switch s.MasterEnvMatch {
	case "warn":
		s.MasterEnvMatchStatus = model.StatusWarn
		results = append(results, model.CheckResult{
			Field: "redis_sentinel.master_env_match", Value: s.MasterEnvMatch, Status: model.StatusWarn,
		})
	case "ok":
		s.MasterEnvMatchStatus = model.StatusOK
	default:
		s.MasterEnvMatchStatus = ""
	}

	if s.Status != "ok" {
		s.OverallStatus = model.StatusWarn
		results = append(results, model.CheckResult{
			Field: "redis_sentinel.status", Value: s.Status, Status: model.StatusWarn,
		})
	} else {
		s.OverallStatus = model.StatusOK
	}

	for i := range s.Sentinels {
		sn := &s.Sentinels[i]
		if !sn.Reachable {
			sn.RenderStatus = model.StatusWarn
			results = append(results, model.CheckResult{
				Field: "redis_sentinel." + sn.IP + ".reachable", Value: "false", Status: model.StatusWarn,
			})
		} else {
			sn.RenderStatus = model.StatusOK
		}
	}
	return results
}
