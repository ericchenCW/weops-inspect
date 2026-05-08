package checker

import (
	"testing"

	"weops-inspect/config"
	"weops-inspect/model"
)

var rmqTestThresholds = config.Thresholds{RabbitMQQueueBacklog: 10000}

func TestCheckRabbitMQ_Nil(t *testing.T) {
	if got := CheckRabbitMQ(nil, rmqTestThresholds); got != nil {
		t.Errorf("want nil, got %v", got)
	}
}

func TestCheckRabbitMQ_AllProblems(t *testing.T) {
	r := &model.RabbitMQStatus{
		Error:               "boom",
		QueuesError:         "queues 500",
		ClusterPartition:    true,
		AbnormalConnections: 2,
		NodeAlarms:          []model.RabbitMQAlarm{{Node: "rabbit@n1", MemAlarm: true, DiskFreeAlarm: true}},
		ExceedingQueues:     []model.RabbitMQQueue{{VHost: "v1", Queue: "celery", MessageCount: 360547}},
		NoConsumerQueues:    []model.RabbitMQQueue{{VHost: "v1", Queue: "default", MessageCount: 8}},
	}
	got := CheckRabbitMQ(r, rmqTestThresholds)
	if len(got) < 7 {
		t.Errorf("want >=7 warns, got %d: %v", len(got), got)
	}
	for _, c := range got {
		if c.Status != model.StatusWarn {
			t.Errorf("want warn, got %v", c)
		}
	}
	if r.PartitionStatus != model.StatusWarn {
		t.Errorf("PartitionStatus = %v", r.PartitionStatus)
	}
	if r.ExceedingQueues[0].MessageStatus != model.StatusWarn {
		t.Errorf("MessageStatus = %v", r.ExceedingQueues[0].MessageStatus)
	}
	if r.NoConsumerQueues[0].ConsumerStatus != model.StatusWarn {
		t.Errorf("ConsumerStatus = %v", r.NoConsumerQueues[0].ConsumerStatus)
	}
}

func TestCheckRabbitMQ_AllHealthy(t *testing.T) {
	r := &model.RabbitMQStatus{}
	got := CheckRabbitMQ(r, rmqTestThresholds)
	if len(got) != 0 {
		t.Errorf("want no warns, got %v", got)
	}
}

func TestCheckRabbitMQ_BacklogFieldFormat(t *testing.T) {
	r := &model.RabbitMQStatus{
		ExceedingQueues: []model.RabbitMQQueue{{VHost: "prod_bk_monitorv3", Queue: "celery", MessageCount: 360547}},
	}
	got := CheckRabbitMQ(r, rmqTestThresholds)
	if len(got) != 1 {
		t.Fatalf("got %v", got)
	}
	if got[0].Field != "rabbitmq.prod_bk_monitorv3.celery.backlog" {
		t.Errorf("field = %q", got[0].Field)
	}
}
