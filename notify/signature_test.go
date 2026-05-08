package notify

import "testing"

func TestSignature_StableAcrossValueChanges(t *testing.T) {
	a := []AlertItem{{Host: "10.0.0.1", Field: "cpu_usage", Value: "76%"}}
	b := []AlertItem{{Host: "10.0.0.1", Field: "cpu_usage", Value: "82%"}}
	if Signature(a) != Signature(b) {
		t.Fatalf("signature must ignore Value changes")
	}
}

func TestSignature_StableAcrossThresholdChanges(t *testing.T) {
	a := []AlertItem{{Host: "10.0.0.1", Field: "cpu_usage", Value: "96%", Threshold: "≥ 95%"}}
	b := []AlertItem{{Host: "10.0.0.1", Field: "cpu_usage", Value: "96%", Threshold: "≥ 90%"}}
	if Signature(a) != Signature(b) {
		t.Fatalf("signature must ignore Threshold changes (env retune must not flush suppression)")
	}
}

func TestSignature_OrderInsensitive(t *testing.T) {
	a := []AlertItem{
		{Host: "10.0.0.1", Field: "cpu_usage"},
		{Host: "10.0.0.2", Field: "mem_usage"},
	}
	b := []AlertItem{
		{Host: "10.0.0.2", Field: "mem_usage"},
		{Host: "10.0.0.1", Field: "cpu_usage"},
	}
	if Signature(a) != Signature(b) {
		t.Fatalf("signature must be order-insensitive")
	}
}

func TestSignature_ChangesWhenSetChanges(t *testing.T) {
	a := []AlertItem{{Host: "10.0.0.1", Field: "cpu_usage"}}
	b := []AlertItem{
		{Host: "10.0.0.1", Field: "cpu_usage"},
		{Host: "10.0.0.2", Field: "cpu_usage"},
	}
	if Signature(a) == Signature(b) {
		t.Fatalf("adding an alert must change the signature")
	}
}

func TestSignature_EmptyReturnsEmpty(t *testing.T) {
	if Signature(nil) != "" {
		t.Fatalf("empty input should yield empty signature")
	}
}

func TestSignature_RabbitMQQueueNameDrift(t *testing.T) {
	// Real-world sample: bk_bkmonitorv3 no-consumer queue set rotated between
	// the 22:44 and 23:02 inspection runs, but the underlying problem is the
	// same vhost.
	at2244 := []AlertItem{
		{Field: "rabbitmq.bk_bkmonitorv3.celery_api_cron.no_consumer"},
		{Field: "rabbitmq.bk_bkmonitorv3.celery_cron.no_consumer"},
		{Field: "rabbitmq.bk_bkmonitorv3.celery_service.no_consumer"},
	}
	at2302 := []AlertItem{
		{Field: "rabbitmq.bk_bkmonitorv3.celery_alert_builder.no_consumer"},
		{Field: "rabbitmq.bk_bkmonitorv3.celery_cron.no_consumer"},
		{Field: "rabbitmq.bk_bkmonitorv3.celery_service_access_event.no_consumer"},
	}
	if Signature(at2244) != Signature(at2302) {
		t.Fatalf("queue-name drift within same vhost must not change signature")
	}
}

func TestSignature_RabbitMQQueueCountChange(t *testing.T) {
	three := []AlertItem{
		{Field: "rabbitmq.bk_bkmonitorv3.q1.no_consumer"},
		{Field: "rabbitmq.bk_bkmonitorv3.q2.no_consumer"},
		{Field: "rabbitmq.bk_bkmonitorv3.q3.no_consumer"},
	}
	five := []AlertItem{
		{Field: "rabbitmq.bk_bkmonitorv3.q1.no_consumer"},
		{Field: "rabbitmq.bk_bkmonitorv3.q2.no_consumer"},
		{Field: "rabbitmq.bk_bkmonitorv3.q3.no_consumer"},
		{Field: "rabbitmq.bk_bkmonitorv3.q4.no_consumer"},
		{Field: "rabbitmq.bk_bkmonitorv3.q5.no_consumer"},
	}
	if Signature(three) != Signature(five) {
		t.Fatalf("queue-count change within same vhost must not change signature")
	}
}

func TestSignature_RabbitMQCrossVhost(t *testing.T) {
	a := []AlertItem{
		{Field: "rabbitmq.vhostA.q1.no_consumer"},
	}
	b := []AlertItem{
		{Field: "rabbitmq.vhostA.q1.no_consumer"},
		{Field: "rabbitmq.vhostB.q1.no_consumer"},
	}
	if Signature(a) == Signature(b) {
		t.Fatalf("adding a new vhost must change the signature")
	}
}

func TestSignature_RabbitMQBacklogVsNoConsumerDistinct(t *testing.T) {
	a := []AlertItem{{Field: "rabbitmq.vhostA.q1.no_consumer"}}
	b := []AlertItem{{Field: "rabbitmq.vhostA.q1.backlog"}}
	if Signature(a) == Signature(b) {
		t.Fatalf("backlog and no_consumer must not collapse together")
	}
}

func TestSignature_RabbitMQClusterLevelFieldsNotCollapsed(t *testing.T) {
	// Cluster-level fields keep their identity verbatim.
	a := []AlertItem{
		{Field: "rabbitmq.error", Value: "x"},
		{Field: "rabbitmq.cluster_partition"},
		{Field: "rabbitmq.node.es-1.mem_alarm"},
	}
	b := []AlertItem{
		{Field: "rabbitmq.error", Value: "y"}, // value drift only — same sig
		{Field: "rabbitmq.cluster_partition"},
		{Field: "rabbitmq.node.es-1.mem_alarm"},
	}
	c := []AlertItem{
		{Field: "rabbitmq.error"},
		{Field: "rabbitmq.cluster_partition"},
		{Field: "rabbitmq.node.es-1.mem_alarm"},
		{Field: "rabbitmq.node.es-2.mem_alarm"}, // genuinely new node alert
	}
	if Signature(a) != Signature(b) {
		t.Fatalf("cluster-level fields with only value drift must keep same signature")
	}
	if Signature(a) == Signature(c) {
		t.Fatalf("a new cluster-level field (extra node alarm) must change signature")
	}
}

func TestSignature_NonRabbitMQUnaffected(t *testing.T) {
	a := []AlertItem{
		{Host: "10.0.0.1", Field: "cpu_usage"},
		{Host: "10.0.0.2", Field: "es.heap_percent"},
		{Field: "redis.10.0.0.3.celery_queue"},
	}
	b := []AlertItem{
		{Host: "10.0.0.1", Field: "cpu_usage"},
		{Host: "10.0.0.2", Field: "es.heap_percent"},
		{Field: "redis.10.0.0.3.celery_queue"},
	}
	if Signature(a) != Signature(b) {
		t.Fatalf("non-rabbitmq fields must hash deterministically")
	}
	c := []AlertItem{
		{Host: "10.0.0.1", Field: "cpu_usage"},
	}
	if Signature(a) == Signature(c) {
		t.Fatalf("dropping non-rabbitmq fields must change signature")
	}
}
