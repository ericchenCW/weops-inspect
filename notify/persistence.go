package notify

import "time"

// pendingTTL is the maximum age of a stale pending entry before defensive GC
// removes it. Entries are normally pruned every run (keys absent from the
// current items list are dropped), so this only matters if a coding bug ever
// leaves orphans behind.
const pendingTTL = 24 * time.Hour

// PendingKey returns the canonical key used to track an alert's persistence
// counter. The key uses the *raw* Field (no signature normalization) so that
// individual RabbitMQ queues are tracked separately even though signature
// collapses them at the vhost level.
func PendingKey(host, field string) string {
	return host + "|" + field
}

// PersistenceResult carries the outcome of a single persistence pass.
type PersistenceResult struct {
	// Filtered are the items that passed confirmation (firing this run).
	Filtered []AlertItem
	// NextPending is the pending map to persist for the next run. Includes
	// items still accumulating (count < N) and excludes items that just got
	// promoted to firing or that disappeared from the input.
	NextPending map[string]PendingItem
	// PendingKeys is the set of (Host|Field) keys that are present in raw
	// items but did not pass confirmation this run. Callers use it to demote
	// the corresponding rows in the inspection report (so the HTML / email
	// view stays consistent with what was actually notified).
	PendingKeys map[string]bool
}

// ApplyPersistence gates raw alert items behind N consecutive runs. When n<=1
// every item passes through unchanged.
//
// Semantics per item:
//   - First sighting: enter pending(1), do not pass.
//   - Already at pending(N-1): promote to firing this run, drop from pending.
//   - Otherwise: increment pending counter.
//
// Items in prev that are absent from the current input are GC'd (natural
// reset). Items older than pendingTTL are also dropped defensively.
func ApplyPersistence(items []AlertItem, prev map[string]PendingItem, n int, now time.Time) PersistenceResult {
	if n < 1 {
		n = 1
	}
	if prev == nil {
		prev = map[string]PendingItem{}
	}

	res := PersistenceResult{
		NextPending: map[string]PendingItem{},
		PendingKeys: map[string]bool{},
	}

	// Fast path: confirmation disabled. Every item is "firing".
	if n == 1 {
		res.Filtered = append(res.Filtered, items...)
		// We still respect TTL on prev — but with n==1 nothing should be
		// in pending anyway. Carry nothing forward.
		return res
	}

	for _, it := range items {
		key := PendingKey(it.Host, it.Field)
		entry, seen := prev[key]
		newCount := 1
		firstSeen := now
		// Honor prev only when it is fresh; stale entries (older than the TTL)
		// are treated as a fresh start to bound state.json growth and prevent
		// a long-tail flapping key from sneaking past confirmation.
		if seen && now.Sub(entry.FirstSeen) < pendingTTL {
			newCount = entry.Count + 1
			firstSeen = entry.FirstSeen
		}

		if newCount >= n {
			// Promoted to firing. Drop from pending.
			res.Filtered = append(res.Filtered, it)
			continue
		}
		// Still accumulating.
		res.NextPending[key] = PendingItem{Count: newCount, FirstSeen: firstSeen}
		res.PendingKeys[key] = true
	}

	// prev keys absent from `items` are dropped naturally — NextPending only
	// contains keys we walked above.

	return res
}

// UpdateRecoveryStreak applies the streak transition rules:
//   - prev was alert AND raw warns empty this run → +1
//   - raw warns non-empty (even if all in pending) → 0
//   - prev was not alert → 0
//
// Caller is responsible for resetting the streak to 0 after a recovery email
// is actually sent.
func UpdateRecoveryStreak(prevStreak int, prevStatus string, rawWarnsEmpty bool) int {
	if prevStatus != StatusAlert {
		return 0
	}
	if !rawWarnsEmpty {
		return 0
	}
	return prevStreak + 1
}
