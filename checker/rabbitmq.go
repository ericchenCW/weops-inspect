package checker

import (
	"fmt"

	"weops-inspect/model"
)

// CheckRabbitMQ produces CheckResults for RabbitMQ. All findings are Warn.
//
// Per design.md D4, ExceedingQueues / NoConsumerQueues are already filtered by
// the collector (with vhost blacklist applied), so the checker just maps each
// element to a Warn CheckResult and backfills per-cell render statuses.
func CheckRabbitMQ(r *model.RabbitMQStatus) []model.CheckResult {
	if r == nil {
		return nil
	}
	var results []model.CheckResult

	if r.Error != "" {
		results = append(results, model.CheckResult{
			Field: "rabbitmq.error", Value: r.Error, Status: model.StatusWarn,
		})
	}

	if r.QueuesError != "" {
		results = append(results, model.CheckResult{
			Field: "rabbitmq.queues_error", Value: r.QueuesError, Status: model.StatusWarn,
		})
	}

	if r.ClusterPartition {
		r.PartitionStatus = model.StatusWarn
		results = append(results, model.CheckResult{
			Field: "rabbitmq.cluster_partition", Value: "true", Status: model.StatusWarn,
		})
	} else {
		r.PartitionStatus = model.StatusOK
	}

	if r.AbnormalConnections > 0 {
		r.AbnormalConnStatus = model.StatusWarn
		results = append(results, model.CheckResult{
			Field:  "rabbitmq.abnormal_connections",
			Value:  fmt.Sprintf("%d", r.AbnormalConnections),
			Status: model.StatusWarn,
		})
	} else {
		r.AbnormalConnStatus = ""
	}

	for i := range r.NodeAlarms {
		a := &r.NodeAlarms[i]
		if a.MemAlarm {
			a.MemStatus = model.StatusWarn
			results = append(results, model.CheckResult{
				Field:  "rabbitmq.node." + a.Node + ".mem_alarm",
				Value:  "true",
				Status: model.StatusWarn,
			})
		} else {
			a.MemStatus = model.StatusOK
		}
		if a.DiskFreeAlarm {
			a.DiskFreeStatus = model.StatusWarn
			results = append(results, model.CheckResult{
				Field:  "rabbitmq.node." + a.Node + ".disk_free_alarm",
				Value:  "true",
				Status: model.StatusWarn,
			})
		} else {
			a.DiskFreeStatus = model.StatusOK
		}
	}

	for i := range r.ExceedingQueues {
		q := &r.ExceedingQueues[i]
		q.MessageStatus = model.StatusWarn
		results = append(results, model.CheckResult{
			Field:  "rabbitmq." + q.VHost + "." + q.Queue + ".backlog",
			Value:  fmt.Sprintf("%d msgs / %d consumers", q.MessageCount, q.Consumers),
			Status: model.StatusWarn,
		})
	}

	for i := range r.NoConsumerQueues {
		q := &r.NoConsumerQueues[i]
		q.ConsumerStatus = model.StatusWarn
		results = append(results, model.CheckResult{
			Field:  "rabbitmq." + q.VHost + "." + q.Queue + ".no_consumer",
			Value:  fmt.Sprintf("%d msgs", q.MessageCount),
			Status: model.StatusWarn,
		})
	}

	return results
}
