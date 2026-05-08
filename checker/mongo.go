package checker

import (
	"fmt"

	"weops-inspect/model"
)

// CheckMongo handles each MongoDB cluster (replica set):
//   - cluster.Error    → Warn
//   - member.Health!=1 → Notice
func CheckMongo(clusters []model.MongoCluster) []model.CheckResult {
	var results []model.CheckResult
	for i := range clusters {
		c := &clusters[i]
		prefix := "mongo." + safeInstance(c.Instance) + "."

		if c.Error != "" {
			results = append(results, model.CheckResult{
				Field: prefix + "error", Value: c.Error, Status: model.StatusWarn,
			})
			continue
		}
		for j := range c.Members {
			m := &c.Members[j]
			if m.Health != 1 {
				m.HealthStatus = model.StatusNotice
				results = append(results, model.CheckResult{
					Field:  prefix + m.Name + ".health",
					Value:  fmt.Sprintf("%d", m.Health),
					Status: model.StatusNotice,
				})
			} else {
				m.HealthStatus = model.StatusOK
			}
		}
	}
	return results
}
