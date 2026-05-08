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

	add := func(field string, value interface{}, status model.CheckStatus, threshold string) {
		results = append(results, model.CheckResult{
			Field:     field,
			Value:     fmt.Sprintf("%v", value),
			Status:    status,
			Threshold: threshold,
		})
	}

	// CPU usage
	cpuThr := fmt.Sprintf("≥ %.0f%%", thresholds.CPUUsage)
	if h.CPUUsage >= thresholds.CPUUsage {
		add("cpu_usage", fmt.Sprintf("%.2f%%", h.CPUUsage), model.StatusWarn, cpuThr)
	} else {
		add("cpu_usage", fmt.Sprintf("%.2f%%", h.CPUUsage), model.StatusOK, cpuThr)
	}

	// Memory usage
	memThr := fmt.Sprintf("≥ %.0f%%", thresholds.MemUsage)
	if h.MemUsage >= thresholds.MemUsage {
		add("mem_usage", fmt.Sprintf("%.2f%%", h.MemUsage), model.StatusWarn, memThr)
	} else {
		add("mem_usage", fmt.Sprintf("%.2f%%", h.MemUsage), model.StatusOK, memThr)
	}

	// Disk usage
	diskThr := fmt.Sprintf("≥ %.0f%%", thresholds.DiskUsage)
	for _, du := range h.DiskUsage {
		val := du.UsageFloat
		if val == 0 {
			numStr := strings.TrimSuffix(du.Usage, "%")
			val, _ = strconv.ParseFloat(numStr, 64)
		}
		if val >= thresholds.DiskUsage {
			add("disk_usage("+du.MountPoint+")", du.Usage, model.StatusWarn, diskThr)
		} else {
			add("disk_usage("+du.MountPoint+")", du.Usage, model.StatusOK, diskThr)
		}
	}

	// Inode usage
	inodeThr := fmt.Sprintf("≥ %.0f%%", thresholds.InodeUsage)
	for _, iu := range h.InodeUsage {
		val := iu.UsageFloat
		if val == 0 {
			numStr := strings.TrimSuffix(iu.Usage, "%")
			val, _ = strconv.ParseFloat(numStr, 64)
		}
		if val >= thresholds.InodeUsage {
			add("inode_usage("+iu.MountPoint+")", iu.Usage, model.StatusWarn, inodeThr)
		} else {
			add("inode_usage("+iu.MountPoint+")", iu.Usage, model.StatusOK, inodeThr)
		}
	}

	// Max open files
	maxOpenThr := fmt.Sprintf("< %d", thresholds.MaxOpenFiles)
	if h.MaxOpenFiles < thresholds.MaxOpenFiles {
		add("max_open_files", h.MaxOpenFiles, model.StatusWarn, maxOpenThr)
	} else {
		add("max_open_files", h.MaxOpenFiles, model.StatusOK, maxOpenThr)
	}

	// SELinux
	if h.SELinux != "Disabled" && h.SELinux != "N/A" {
		add("selinux", h.SELinux, model.StatusWarn, "期望 Disabled")
	} else {
		add("selinux", h.SELinux, model.StatusOK, "期望 Disabled")
	}

	// Firewalld
	if h.Firewalld != "inactive" && h.Firewalld != "N/A" {
		add("firewalld", h.Firewalld, model.StatusWarn, "期望 inactive")
	} else {
		add("firewalld", h.Firewalld, model.StatusOK, "期望 inactive")
	}

	// Chronyd
	if h.Chronyd != "active" && h.Chronyd != "N/A" {
		add("chronyd", h.Chronyd, model.StatusWarn, "期望 active")
	} else {
		add("chronyd", h.Chronyd, model.StatusOK, "期望 active")
	}

	// Load average anomaly: loadavg1 > loadavg5 > loadavg15 > cores.
	// Relational rule — no single-value threshold; leave Threshold empty.
	if h.LoadAvg1 > h.LoadAvg5 && h.LoadAvg5 > h.LoadAvg15 && h.LoadAvg15 > float64(h.Core) {
		add("load_average", fmt.Sprintf("%.2f/%.2f/%.2f (cores: %d)", h.LoadAvg1, h.LoadAvg5, h.LoadAvg15, h.Core), model.StatusWarn, "")
	} else {
		add("load_average", fmt.Sprintf("%.2f/%.2f/%.2f (cores: %d)", h.LoadAvg1, h.LoadAvg5, h.LoadAvg15, h.Core), model.StatusOK, "")
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
			Field: prefix + "status", Value: "(empty)", Status: model.StatusUnknown,
			Threshold: "期望 active",
		})
	case "active":
		sm.RenderStatus = model.StatusOK
		results = append(results, model.CheckResult{
			Field: prefix + "status", Value: sm.Status, Status: model.StatusOK,
			Threshold: "期望 active",
		})
	default:
		sm.RenderStatus = model.StatusWarn
		results = append(results, model.CheckResult{
			Field: prefix + "status", Value: sm.Status, Status: model.StatusWarn,
			Threshold: "期望 active",
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
			Threshold: "期望 ok",
		})
	} else {
		sm.HealthzRenderStatus = model.StatusWarn
		results = append(results, model.CheckResult{
			Field: prefix + "healthz", Value: sm.HealthzAPI, Status: model.StatusWarn,
			Threshold: "期望 ok",
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
			Field:     "service." + s.Module + ".docker.exited",
			Value:     fmt.Sprintf("%d", s.ContainersExited),
			Status:    model.StatusNotice,
			Threshold: fmt.Sprintf("> %d", t.ServiceContainersExited),
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
func CheckReplication(rep *model.ReplicationReport, t config.Thresholds) []model.CheckResult {
	if rep == nil {
		return nil
	}
	var results []model.CheckResult
	add := func(field, value string, status model.CheckStatus, threshold string) {
		results = append(results, model.CheckResult{
			Field: field, Value: value, Status: status, Threshold: threshold,
		})
	}

	// MySQL master read_only: relational/multi-condition (collector decides
	// status from read_only flag + reachability) — leave Threshold empty.
	for _, m := range rep.MySQLMasters {
		field := "mysql_master(" + m.IP + ").read_only"
		val := "OFF"
		if m.ReadOnly {
			val = "ON"
		}
		if m.Status == "ok" {
			add(field, val, model.StatusOK, "")
		} else {
			add(field, val, model.StatusWarn, "")
		}
	}

	mysqlLagThr := fmt.Sprintf("lag > %ds", t.MySQLReplLagSec)
	for _, s := range rep.MySQLSlaves {
		if s.Replication == nil {
			continue
		}
		r := s.Replication
		field := "mysql_slave(" + s.IP + ").replication"
		val := r.IORunning + "/" + r.SQLRunning + " lag=" + strconvI(r.SecondsBehindMaster) + "s"
		if r.Status == "ok" {
			add(field, val, model.StatusOK, mysqlLagThr)
		} else {
			add(field, val, model.StatusWarn, mysqlLagThr)
		}
	}

	redisIOThr := fmt.Sprintf("io > %ds", t.RedisReplIOSec)
	for _, n := range rep.RedisNodes {
		// Role consistency is a relational check (master/slave declaration vs
		// observed) — no single threshold; leave empty.
		field := "redis(" + n.IP + ").role"
		if n.RoleConsistencyStatus == "ok" {
			add(field, n.Role, model.StatusOK, "")
		} else {
			add(field, n.Role, model.StatusWarn, "")
		}
		if n.Role == "slave" && n.LinkStatus != "" && n.LinkStatus != "N/A" {
			lf := "redis(" + n.IP + ").link"
			lv := n.MasterLinkStatus + " io=" + strconvI(n.MasterLastIOSeconds) + "s"
			if n.LinkStatus == "ok" {
				add(lf, lv, model.StatusOK, redisIOThr)
			} else {
				add(lf, lv, model.StatusWarn, redisIOThr)
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
