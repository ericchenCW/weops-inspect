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

	// Run days
	if h.RunDays >= thresholds.RunDays {
		add("run_days", h.RunDays, model.StatusWarn)
	} else {
		add("run_days", h.RunDays, model.StatusOK)
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

// CheckService checks service status fields against expected values.
func CheckService(sm model.ServiceModule) []model.CheckResult {
	var results []model.CheckResult

	// Status check
	status := model.StatusOK
	if sm.Status != "active" {
		status = model.StatusWarn
	}
	results = append(results, model.CheckResult{
		Field: "status", Value: sm.Status, Status: status,
	})

	// Healthz check
	if sm.HealthzAPI != "N/A" {
		hzStatus := model.StatusOK
		if sm.HealthzAPI != "ok" {
			hzStatus = model.StatusWarn
		}
		results = append(results, model.CheckResult{
			Field: "healthz_api", Value: sm.HealthzAPI, Status: hzStatus,
		})
	}

	return results
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

// Summarize counts OK and WARN from a list of check results.
func Summarize(results []model.CheckResult) model.CheckSummary {
	s := model.CheckSummary{Total: len(results)}
	for _, r := range results {
		if r.Status == model.StatusOK {
			s.OK++
		} else {
			s.Warn++
		}
	}
	return s
}
