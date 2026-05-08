package checker

import (
	"fmt"

	"weops-inspect/config"
	"weops-inspect/model"
)

// CheckES walks each ES cluster and produces CheckResults plus per-cell
// render statuses. Mutates the input slice in place to backfill Status fields.
//
// Severity:
//   - cluster.Error                  → Warn (skip downstream node reachability checks)
//   - node.Status == "unreachable"   → Warn
//   - cluster.status != "green"      → Notice (only if cluster.Error is empty)
//   - UnassignedShards > threshold   → Notice
//   - PendingTasks > 0               → Notice
//   - HeapPercent > threshold        → Notice
//   - RAMPercent  > threshold        → Notice
func CheckES(clusters []model.ESCluster, t config.Thresholds) []model.CheckResult {
	var results []model.CheckResult
	for i := range clusters {
		results = append(results, checkOneESCluster(&clusters[i], t)...)
	}
	return results
}

func checkOneESCluster(c *model.ESCluster, t config.Thresholds) []model.CheckResult {
	prefix := "es." + safeInstance(c.Instance) + "."
	var results []model.CheckResult

	// Cluster-level采集失败：直接 Warn 并停止后续判定（节点可达性等也会失真）。
	if c.Error != "" {
		results = append(results, model.CheckResult{
			Field: prefix + "cluster_error", Value: c.Error, Status: model.StatusWarn,
		})
		// HealthStatus mirrors the green/yellow/red cell; on采集失败标 warn 让单元也变红。
		c.HealthStatus = model.StatusWarn
		return results
	}

	// cluster.status 非 green → Notice
	if c.Status != "" && c.Status != "green" {
		c.HealthStatus = model.StatusNotice
		results = append(results, model.CheckResult{
			Field: prefix + "cluster_status", Value: c.Status, Status: model.StatusNotice,
		})
	} else if c.Status == "green" {
		c.HealthStatus = model.StatusOK
	}

	// UnassignedShards > threshold → Notice
	if c.UnassignedShards > t.ESUnassignedShards {
		c.UnassignedStatus = model.StatusNotice
		results = append(results, model.CheckResult{
			Field: prefix + "unassigned_shards",
			Value: fmt.Sprintf("%d", c.UnassignedShards),
			Status: model.StatusNotice,
		})
	} else {
		c.UnassignedStatus = model.StatusOK
	}

	// PendingTasks > 0 → Notice
	if c.PendingTasks > 0 {
		c.PendingStatus = model.StatusNotice
		results = append(results, model.CheckResult{
			Field: prefix + "pending_tasks",
			Value: fmt.Sprintf("%d", c.PendingTasks),
			Status: model.StatusNotice,
		})
	} else {
		c.PendingStatus = model.StatusOK
	}

	// 节点 heap / ram → Notice
	for j := range c.Nodes {
		n := &c.Nodes[j]
		if n.HeapPercent > t.ESHeapPercent {
			n.HeapStatus = model.StatusNotice
			results = append(results, model.CheckResult{
				Field: prefix + n.IP + ".heap",
				Value: fmt.Sprintf("%d%%", n.HeapPercent),
				Status: model.StatusNotice,
			})
		} else {
			n.HeapStatus = model.StatusOK
		}
		if n.RAMPercent > t.ESRAMPercent {
			n.RAMStatus = model.StatusNotice
			results = append(results, model.CheckResult{
				Field: prefix + n.IP + ".ram",
				Value: fmt.Sprintf("%d%%", n.RAMPercent),
				Status: model.StatusNotice,
			})
		} else {
			n.RAMStatus = model.StatusOK
		}
	}

	// 节点可达性
	for j := range c.NodeReachability {
		nr := &c.NodeReachability[j]
		if nr.Status == "unreachable" {
			nr.RenderStatus = model.StatusWarn
			results = append(results, model.CheckResult{
				Field: prefix + nr.IP + ".reachability",
				Value: nr.Status,
				Status: model.StatusWarn,
			})
		} else {
			nr.RenderStatus = model.StatusOK
		}
	}

	return results
}

// safeInstance returns Instance for use as a CheckResult field segment, falling
// back to "unknown" for empty strings to avoid producing fields like "es..foo".
func safeInstance(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}
