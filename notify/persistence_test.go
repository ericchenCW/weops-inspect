package notify

import (
	"testing"
	"time"
)

func TestApplyPersistence_FirstSightingEntersPending(t *testing.T) {
	now := time.Now()
	items := []AlertItem{{Host: "h1", Field: "cpu"}}
	res := ApplyPersistence(items, nil, 2, now)

	if len(res.Filtered) != 0 {
		t.Fatalf("first sighting must not pass, got %d filtered", len(res.Filtered))
	}
	if got := res.NextPending[PendingKey("h1", "cpu")].Count; got != 1 {
		t.Fatalf("pending count = %d, want 1", got)
	}
	if !res.PendingKeys[PendingKey("h1", "cpu")] {
		t.Fatalf("pending key must be flagged for report demotion")
	}
}

func TestApplyPersistence_PromoteAtN(t *testing.T) {
	now := time.Now()
	prev := map[string]PendingItem{
		PendingKey("h1", "cpu"): {Count: 1, FirstSeen: now.Add(-5 * time.Minute)},
	}
	items := []AlertItem{{Host: "h1", Field: "cpu"}}
	res := ApplyPersistence(items, prev, 2, now)

	if len(res.Filtered) != 1 {
		t.Fatalf("must promote at N=2, got %d filtered", len(res.Filtered))
	}
	if _, still := res.NextPending[PendingKey("h1", "cpu")]; still {
		t.Fatalf("promoted item must leave pending")
	}
}

func TestApplyPersistence_FlapResetsPending(t *testing.T) {
	now := time.Now()
	prev := map[string]PendingItem{
		PendingKey("h1", "cpu"): {Count: 1, FirstSeen: now.Add(-5 * time.Minute)},
	}
	res := ApplyPersistence(nil, prev, 2, now)

	if len(res.Filtered) != 0 {
		t.Fatalf("absent items must not fire, got %d", len(res.Filtered))
	}
	if len(res.NextPending) != 0 {
		t.Fatalf("absent items must drop from pending, got %d entries", len(res.NextPending))
	}
}

func TestApplyPersistence_NDisabledPassesThrough(t *testing.T) {
	now := time.Now()
	items := []AlertItem{
		{Host: "h1", Field: "cpu"},
		{Host: "h2", Field: "mem"},
	}
	res := ApplyPersistence(items, nil, 1, now)

	if len(res.Filtered) != 2 {
		t.Fatalf("N=1 must pass everything, got %d", len(res.Filtered))
	}
	if len(res.NextPending) != 0 {
		t.Fatalf("N=1 must not accumulate pending, got %d", len(res.NextPending))
	}
}

func TestApplyPersistence_StaleEntryGCd(t *testing.T) {
	now := time.Now()
	prev := map[string]PendingItem{
		PendingKey("h1", "cpu"): {Count: 1, FirstSeen: now.Add(-25 * time.Hour)},
	}
	items := []AlertItem{{Host: "h1", Field: "cpu"}}
	res := ApplyPersistence(items, prev, 2, now)

	// Stale prev is treated as cold; key starts at count=1 again.
	if len(res.Filtered) != 0 {
		t.Fatalf("stale entry must not promote on first reappearance, got %d filtered", len(res.Filtered))
	}
	got := res.NextPending[PendingKey("h1", "cpu")]
	if got.Count != 1 {
		t.Fatalf("after TTL reset, count = %d, want 1", got.Count)
	}
	if got.FirstSeen.Before(now) {
		t.Fatalf("after TTL reset, FirstSeen must be refreshed to now")
	}
}

func TestApplyPersistence_RawFieldNotNormalized(t *testing.T) {
	// Two RabbitMQ queues in the same vhost. Signature would collapse them;
	// pending must NOT — they each accumulate independently.
	now := time.Now()
	items := []AlertItem{
		{Host: "", Field: "rabbitmq.vh.q1.no_consumer"},
		{Host: "", Field: "rabbitmq.vh.q2.no_consumer"},
	}
	res := ApplyPersistence(items, nil, 2, now)

	if len(res.NextPending) != 2 {
		t.Fatalf("each raw queue field must occupy its own pending entry, got %d", len(res.NextPending))
	}
}

func TestUpdateRecoveryStreak(t *testing.T) {
	cases := []struct {
		name          string
		prevStreak    int
		prevStatus    string
		rawWarnsEmpty bool
		want          int
	}{
		{"alert + empty → +1 from 0", 0, StatusAlert, true, 1},
		{"alert + empty → +1 from 1", 1, StatusAlert, true, 2},
		{"alert + non-empty → reset", 3, StatusAlert, false, 0},
		{"ok status → 0 regardless", 5, StatusOK, true, 0},
		{"empty status → 0 regardless", 5, "", true, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := UpdateRecoveryStreak(c.prevStreak, c.prevStatus, c.rawWarnsEmpty)
			if got != c.want {
				t.Fatalf("got %d, want %d", got, c.want)
			}
		})
	}
}
