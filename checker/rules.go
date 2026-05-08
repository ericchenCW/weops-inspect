package checker

import (
	"fmt"
	"strconv"
	"strings"

	"weops-inspect/config"
	"weops-inspect/model"
)

// CheckHost applies threshold rules to a host's metrics and returns check results.
func CheckHost(h model.HostMetrics, thresholds config.Thresholds) []model.CheckResult {
	var results []model.CheckResult

	add := func(field string, value interface{}, status model.CheckStatus) {
		results = append(results, model.CheckResult{
			Field:  field,
			Value:  fmt.Sprintf("%v", value),
			Status: status,
		})
	}

	// CPU usage
	if h.CPUUsage >= thresholds.CPUUsage {
		add("cpu_usage", fmt.Sprintf("%.2f%%", h.CPUUsage), model.StatusWarn)
	} else {
		add("cpu_usage", fmt.Sprintf("%.2f%%", h.CPUUsage), model.StatusOK)
	}

	// Memory usage
	if h.MemUsage >= thresholds.MemUsage {
		add("mem_usage", fmt.Sprintf("%.2f%%", h.MemUsage), model.StatusWarn)
	} else {
		add("mem_usage", fmt.Sprintf("%.2f%%", h.MemUsage), model.StatusOK)
	}

	// Disk usage
	for _, du := range h.DiskUsage {
		val := du.UsageFloat
		if val == 0 {
			numStr := strings.TrimSuffix(du.Usage, "%")
			val, _ = strconv.ParseFloat(numStr, 64)
		}
		if val >= thresholds.DiskUsage {
			add("disk_usage("+du.MountPoint+")", du.Usage, model.StatusWarn)
		} else {
			add("disk_usage("+du.MountPoint+")", du.Usage, model.StatusOK)
		}
	}

	// Inode usage
	for _, iu := range h.InodeUsage {
		val := iu.UsageFloat
		if val == 0 {
			numStr := strings.TrimSuffix(iu.Usage, "%")
			val, _ = strconv.ParseFloat(numStr, 64)
		}
		if val >= thresholds.InodeUsage {
			add("inode_usage("+iu.MountPoint+")", iu.Usage, model.StatusWarn)
		} else {
			add("inode_usage("+iu.MountPoint+")", iu.Usage, model.StatusOK)
		}
	}

	// Max open files
	if h.MaxOpenFiles < thresholds.MaxOpenFiles {
		add("max_open_files", h.MaxOpenFiles, model.StatusWarn)
	} else {
		add("max_open_files", h.MaxOpenFiles, model.StatusOK)
	}

	// SELinux
	if h.SELinux != "Disabled" && h.SELinux != "N/A" {
		add("selinux", h.SELinux, model.StatusWarn)
	} else {
		add("selinux", h.SELinux, model.StatusOK)
	}

	// Firewalld
	if h.Firewalld != "inactive" && h.Firewalld != "N/A" {
		add("firewalld", h.Firewalld, model.StatusWarn)
	} else {
		add("firewalld", h.Firewalld, model.StatusOK)
	}

	// Chronyd
	if h.Chronyd != "active" && h.Chronyd != "N/A" {
		add("chronyd", h.Chronyd, model.StatusWarn)
	} else {
		add("chronyd", h.Chronyd, model.StatusOK)
	}

	// Load average anomaly: loadavg1 > loadavg5 > loadavg15 > cores
	if h.LoadAvg1 > h.LoadAvg5 && h.LoadAvg5 > h.LoadAvg15 && h.LoadAvg15 > float64(h.Core) {
		add("load_average", fmt.Sprintf("%.2f/%.2f/%.2f (cores: %d)", h.LoadAvg1, h.LoadAvg5, h.LoadAvg15, h.Core), model.StatusWarn)
	} else {
		add("load_average", fmt.Sprintf("%.2f/%.2f/%.2f (cores: %d)", h.LoadAvg1, h.LoadAvg5, h.LoadAvg15, h.Core), model.StatusOK)
	}

	return results
}

// CheckService checks service status fields against expected values and
// backfills RenderStatus / HealthzRenderStatus on the module for HTML coloring.
// An empty Status (sub-module not registered on the host) yields a single
// Unknown CheckResult.
func CheckService(sm *model.ServiceModule, hostIP, moduleKey string) []model.CheckResult {
	var results []model.CheckResult
	prefix := "service." + moduleKey + "/" + sm.Module + "."

	// Status check.
	switch sm.Status {
	case "":
		sm.RenderStatus = model.StatusUnknown
		results = append(results, model.CheckResult{
			Field:  prefix + "status", Value: "(empty)", Status: model.StatusUnknown,
		})
	case "active":
		sm.RenderStatus = model.StatusOK
		results = append(results, model.CheckResult{
			Field: prefix + "status", Value: sm.Status, Status: model.StatusOK,
		})
	default:
		sm.RenderStatus = model.StatusWarn
		results = append(results, model.CheckResult{
			Field: prefix + "status", Value: sm.Status, Status: model.StatusWarn,
		})
	}

	// Healthz check (skip when N/A).
	if sm.HealthzAPI == "" {
		sm.HealthzRenderStatus = ""
	} else if sm.HealthzAPI == "N/A" {
		sm.HealthzRenderStatus = ""
	} else if sm.HealthzAPI == "ok" {
		sm.HealthzRenderStatus = model.StatusOK
		results = append(results, model.CheckResult{
			Field: prefix + "healthz", Value: sm.HealthzAPI, Status: model.StatusOK,
		})
	} else {
		sm.HealthzRenderStatus = model.StatusWarn
		results = append(results, model.CheckResult{
			Field: prefix + "healthz", Value: sm.HealthzAPI, Status: model.StatusWarn,
		})
	}

	_ = hostIP // currently unused in field (host-scoped CheckResult would duplicate hostIP);
	// host context is attached at notify layer via the surrounding ServiceStatus.
	return results
}

// CheckServiceContainers handles per-host Docker container counts as a Notice
// when ContainersExited exceeds the threshold. ContainersUp is informational.
func CheckServiceContainers(s *model.ServiceStatus, t config.Thresholds) []model.CheckResult {
	if s.ContainersUp == 0 && s.ContainersExited == 0 {
		return nil
	}
	if s.ContainersExited > t.ServiceContainersExited {
		s.ExitedRenderStatus = model.StatusNotice
		return []model.CheckResult{{
			Field:  "service." + s.Module + ".docker.exited",
			Value:  fmt.Sprintf("%d", s.ContainersExited),
			Status: model.StatusNotice,
		}}
	}
	s.ExitedRenderStatus = model.StatusOK
	return nil
}

// CheckServiceCollectError reports a Notice when a service-section probe failed
// (e.g. SSH unreachable or systemctl errored).
func CheckServiceCollectError(s *model.ServiceStatus) []model.CheckResult {
	if s.Error == "" {
		return nil
	}
	return []model.CheckResult{{
		Field:  "service." + s.Module + ".collect_error",
		Value:  s.Error,
		Status: model.StatusNotice,
	}}
}

// CheckReplication produces check results for a ReplicationReport so that
// replication health rolls up into the overall summary.
func CheckReplication(rep *model.ReplicationReport) []model.CheckResult {
	if rep == nil {
		return nil
	}
	var results []model.CheckResult
	add := func(field, value string, status model.CheckStatus) {
		results = append(results, model.CheckResult{Field: field, Value: value, Status: status})
	}

	for _, m := range rep.MySQLMasters {
		field := "mysql_master(" + m.IP + ").read_only"
		val := "OFF"
		if m.ReadOnly {
			val = "ON"
		}
		if m.Status == "ok" {
			add(field, val, model.StatusOK)
		} else {
			add(field, val, model.StatusWarn)
		}
	}

	for _, s := range rep.MySQLSlaves {
		if s.Replication == nil {
			continue
		}
		r := s.Replication
		field := "mysql_slave(" + s.IP + ").replication"
		val := r.IORunning + "/" + r.SQLRunning + " lag=" + strconvI(r.SecondsBehindMaster) + "s"
		if r.Status == "ok" {
			add(field, val, model.StatusOK)
		} else {
			add(field, val, model.StatusWarn)
		}
	}

	for _, n := range rep.RedisNodes {
		field := "redis(" + n.IP + ").role"
		if n.RoleConsistencyStatus == "ok" {
			add(field, n.Role, model.StatusOK)
		} else {
			add(field, n.Role, model.StatusWarn)
		}
		if n.Role == "slave" && n.LinkStatus != "" && n.LinkStatus != "N/A" {
			lf := "redis(" + n.IP + ").link"
			lv := n.MasterLinkStatus + " io=" + strconvI(n.MasterLastIOSeconds) + "s"
			if n.LinkStatus == "ok" {
				add(lf, lv, model.StatusOK)
			} else {
				add(lf, lv, model.StatusWarn)
			}
		}
	}
	return results
}

func strconvI(i int) string { return strconv.Itoa(i) }

// Summarize counts OK / Warn / Unknown from a list of check results.
// Total includes OK + Warn + Unknown; Notice items are excluded from every bucket.
func Summarize(results []model.CheckResult) model.CheckSummary {
	var s model.CheckSummary
	for _, r := range results {
		switch r.Status {
		case model.StatusOK:
			s.OK++
			s.Total++
		case model.StatusWarn:
			s.Warn++
			s.Total++
		case model.StatusUnknown:
			s.Unknown++
			s.Total++
		case model.StatusNotice:
			// excluded
		}
	}
	return s
}
