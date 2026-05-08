package notify

import "testing"

func TestSignature_StableAcrossValueChanges(t *testing.T) {
	a := []AlertItem{{Host: "10.0.0.1", Field: "cpu_usage", Value: "76%"}}
	b := []AlertItem{{Host: "10.0.0.1", Field: "cpu_usage", Value: "82%"}}
	if Signature(a) != Signature(b) {
		t.Fatalf("signature must ignore Value changes")
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
