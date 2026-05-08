package notify

import (
	"weops-inspect/model"
)

// AlertItem is a normalized warn entry with host context, used by both
// signature calculation and email body rendering.
type AlertItem struct {
	Host  string // may be empty when the check is not host-scoped
	Field string
	Value string
}

// ExtractAlerts walks the report and returns all Warn-status entries, enriched
// with host context where it can be derived from the report structure.
func ExtractAlerts(report *model.InspectReport) []AlertItem {
	if report == nil {
		return nil
	}
	var items []AlertItem

	for _, h := range report.Hosts {
		for _, c := range h.Checks {
			if c.Status == model.StatusWarn {
				items = append(items, AlertItem{
					Host:  h.Metrics.IP,
					Field: c.Field,
					Value: c.Value,
				})
			}
		}
	}

	for _, statuses := range report.Services {
		for _, s := range statuses {
			for _, sm := range s.Services {
				if sm.Status != "active" {
					items = append(items, AlertItem{
						Host:  s.HostIP,
						Field: s.Module + "/" + sm.Module + ".status",
						Value: sm.Status,
					})
				}
				if sm.HealthzAPI != "N/A" && sm.HealthzAPI != "ok" && sm.HealthzAPI != "" {
					items = append(items, AlertItem{
						Host:  s.HostIP,
						Field: s.Module + "/" + sm.Module + ".healthz",
						Value: sm.HealthzAPI,
					})
				}
			}
		}
	}

	if rep := report.Replication; rep != nil {
		for _, m := range rep.MySQLMasters {
			if m.Status != "ok" {
				val := "OFF"
				if m.ReadOnly {
					val = "ON"
				}
				items = append(items, AlertItem{
					Host:  m.IP,
					Field: "mysql_master.read_only",
					Value: val,
				})
			}
		}
		for _, s := range rep.MySQLSlaves {
			if s.Replication != nil && s.Replication.Status != "ok" {
				items = append(items, AlertItem{
					Host:  s.IP,
					Field: "mysql_slave.replication",
					Value: s.Replication.IORunning + "/" + s.Replication.SQLRunning,
				})
			}
		}
		for _, n := range rep.RedisNodes {
			if n.RoleConsistencyStatus != "ok" {
				items = append(items, AlertItem{
					Host:  n.IP,
					Field: "redis.role",
					Value: n.Role,
				})
			}
			if n.Role == "slave" && n.LinkStatus != "" && n.LinkStatus != "N/A" && n.LinkStatus != "ok" {
				items = append(items, AlertItem{
					Host:  n.IP,
					Field: "redis.link",
					Value: n.MasterLinkStatus,
				})
			}
		}
	}

	return items
}
