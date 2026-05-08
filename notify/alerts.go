package notify

import (
	"weops-inspect/model"
)

// AlertItem is a normalized warn entry, used by both signature calculation and
// email body rendering. Host may be empty when the check is not host-scoped
// (e.g. cluster-level RabbitMQ findings).
type AlertItem struct {
	Host  string
	Field string
	Value string
}

// ExtractAlerts returns all Warn-status entries from the report's flat
// AllChecks list. Unknown and Notice entries are excluded.
//
// Host attribution is best-effort: it parses common field prefixes
// ("redis.<ip>.x", "es.<instance>.<ip>.x", "service.<m>/<sub>.x") to populate
// the Host column when present; otherwise Host is left empty.
func ExtractAlerts(report *model.InspectReport) []AlertItem {
	if report == nil {
		return nil
	}

	// Map host CheckResults back to their host IP via the per-host blocks.
	hostByField := map[string]string{}
	for _, h := range report.Hosts {
		for _, c := range h.Checks {
			hostByField[c.Field] = h.Metrics.IP
		}
	}

	items := make([]AlertItem, 0, len(report.AllChecks))
	for _, c := range report.AllChecks {
		if c.Status != model.StatusWarn {
			continue
		}
		host := hostByField[c.Field]
		items = append(items, AlertItem{
			Host:  host,
			Field: c.Field,
			Value: c.Value,
		})
	}
	return items
}

// ExtractUnknowns returns the count of Unknown entries — used to surface
// "采不到" signals in the email summary line.
func ExtractUnknowns(report *model.InspectReport) int {
	if report == nil {
		return 0
	}
	n := 0
	for _, c := range report.AllChecks {
		if c.Status == model.StatusUnknown {
			n++
		}
	}
	return n
}
