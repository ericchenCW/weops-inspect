package checker

import (
	"weops-inspect/model"
)

// CheckBKDeps maps each bkmonitorv3 dependency probe result to a CheckResult:
//   - "ok"   → OK
//   - "skip" → no result (collector chose to skip this probe)
//   - other  → Notice (本轮不进 Summary.Warn 不进邮件，仅 HTML 着色)
func CheckBKDeps(s *model.BKMonitorV3Section) []model.CheckResult {
	if s == nil {
		return nil
	}
	var results []model.CheckResult
	for i := range s.Dependencies {
		d := &s.Dependencies[i]
		switch d.Status {
		case "ok":
			d.RenderStatus = model.StatusOK
			results = append(results, model.CheckResult{
				Field: "bkdeps." + d.Item + ".status", Value: d.Status, Status: model.StatusOK,
			})
		case "skip":
			d.RenderStatus = ""
		default:
			d.RenderStatus = model.StatusNotice
			results = append(results, model.CheckResult{
				Field: "bkdeps." + d.Item + ".status", Value: d.Status, Status: model.StatusNotice,
			})
		}
	}
	return results
}
