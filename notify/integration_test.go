package notify

import (
	"path/filepath"
	"testing"
	"time"

	"weops-inspect/model"
)

// makeReport builds a minimal report whose AllChecks contains the given warns
// (each becomes a host-attributable check via report.Hosts). Insertion order
// from `warns` is preserved into report.Hosts, which keeps the host-by-field
// lookup in ExtractAlerts deterministic during tests (same-field-on-multiple-
// hosts will resolve to the *last* listed host).
func makeReport(t *testing.T, warns []AlertItem) *model.InspectReport {
	t.Helper()
	r := &model.InspectReport{
		Timestamp: "2026-05-09 12:00:00",
	}
	hostOrder := []string{}
	hostChecks := map[string][]model.CheckResult{}
	for _, w := range warns {
		c := model.CheckResult{Field: w.Field, Value: w.Value, Status: model.StatusWarn, Threshold: w.Threshold}
		r.AllChecks = append(r.AllChecks, c)
		if w.Host != "" {
			if _, seen := hostChecks[w.Host]; !seen {
				hostOrder = append(hostOrder, w.Host)
			}
			hostChecks[w.Host] = append(hostChecks[w.Host], c)
		}
	}
	for _, ip := range hostOrder {
		r.Hosts = append(r.Hosts, model.HostCheckResult{
			Metrics: model.HostMetrics{IP: ip},
			Checks:  hostChecks[ip],
		})
	}
	r.Summary = recomputeSummary(r.AllChecks)
	return r
}

// fakeCfg returns a minimally valid Config wired to a temp state file.
func fakeCfg(t *testing.T, n int) *Config {
	t.Helper()
	dir := t.TempDir()
	cfg := &Config{
		Email: EmailConfig{
			Enabled:  true,
			SMTPHost: "localhost",
			SMTPPort: 25,
			From:     "a@b",
			To:       []string{"c@d"},
		},
		Trigger:     TriggerConfig{MinIntervalMinutes: 120, SendRecovery: true},
		Persistence: PersistenceConfig{ConsecutiveRuns: n},
		path:        filepath.Join(dir, "config.json"),
	}
	return cfg
}

// Tick is one full Prepare run on a fresh report. We don't actually send mail
// (that requires SMTP); instead we exercise Prepare and inspect the state +
// computed action via a re-implementation of Dispatch's decision.
type tick struct {
	filtered      int
	rawEmpty      bool
	streakAfter   int
	pendingAfter  int
	statusAfter   string // expected status to be saved on Dispatch
	wantAction    Action
}

func runTick(t *testing.T, cfg *Config, statePath string, warns []AlertItem) tick {
	t.Helper()
	report := makeReport(t, warns)
	cfg.path = filepath.Dir(statePath) + "/config.json"

	prev := LoadState(statePath)
	rawItems := ExtractAlerts(report)
	rawEmpty := len(rawItems) == 0
	now := time.Now()
	persRes := ApplyPersistence(rawItems, prev.Pending, cfg.Persistence.ConsecutiveRuns, now)
	streak := UpdateRecoveryStreak(prev.RecoveryStreak, prev.LastStatus, rawEmpty)
	demotePending(report, persRes.PendingKeys)

	cooldown := time.Duration(cfg.Trigger.MinIntervalMinutes) * time.Minute
	sigNow := Signature(persRes.Filtered)
	var action Action
	if prev.LastStatus == StatusAlert && len(persRes.Filtered) == 0 {
		if rawEmpty && streak >= cfg.Persistence.ConsecutiveRuns {
			action = ActionSendRecovery
		} else {
			action = ActionNone
		}
	} else {
		action = Decide(now, prev, len(persRes.Filtered), sigNow, cooldown)
	}

	// Persist state mimicking Dispatch outcomes.
	next := *prev
	next.Pending = persRes.NextPending
	next.RecoveryStreak = streak
	switch action {
	case ActionSendAlert:
		next.LastSentAt = now
		next.LastSignature = sigNow
		next.LastStatus = StatusAlert
		next.RecoveryStreak = 0
	case ActionSendRecovery:
		next.LastSentAt = now
		next.LastSignature = ""
		next.LastStatus = StatusOK
		next.RecoveryStreak = 0
	}
	if err := SaveState(statePath, &next); err != nil {
		t.Fatalf("save state: %v", err)
	}

	return tick{
		filtered:     len(persRes.Filtered),
		rawEmpty:     rawEmpty,
		streakAfter:  next.RecoveryStreak,
		pendingAfter: len(next.Pending),
		statusAfter:  next.LastStatus,
		wantAction:   action,
	}
}

// (a) flap → no email
func TestE2E_FlappingDoesNotFire(t *testing.T) {
	cfg := fakeCfg(t, 2)
	statePath := filepath.Join(filepath.Dir(cfg.path), "state.json")

	// T0: warn appears
	r := runTick(t, cfg, statePath, []AlertItem{{Host: "h1", Field: "cpu"}})
	if r.wantAction != ActionNone {
		t.Fatalf("T0: expected None, got %v", r.wantAction)
	}
	if r.pendingAfter != 1 {
		t.Fatalf("T0: pending should accumulate to 1, got %d", r.pendingAfter)
	}

	// T1: warn vanishes (flap)
	r = runTick(t, cfg, statePath, nil)
	if r.wantAction != ActionNone {
		t.Fatalf("T1: expected None, got %v", r.wantAction)
	}
	if r.pendingAfter != 0 {
		t.Fatalf("T1: pending should reset, got %d", r.pendingAfter)
	}
}

// (b) two consecutive warns → alert
func TestE2E_PersistentWarnFires(t *testing.T) {
	cfg := fakeCfg(t, 2)
	statePath := filepath.Join(filepath.Dir(cfg.path), "state.json")
	warns := []AlertItem{{Host: "h1", Field: "cpu"}}

	r := runTick(t, cfg, statePath, warns)
	if r.wantAction != ActionNone {
		t.Fatalf("T0: expected None (pending), got %v", r.wantAction)
	}

	r = runTick(t, cfg, statePath, warns)
	if r.wantAction != ActionSendAlert {
		t.Fatalf("T1: expected SendAlert (promoted), got %v", r.wantAction)
	}
	if r.statusAfter != StatusAlert {
		t.Fatalf("T1: status should be alert, got %q", r.statusAfter)
	}
}

// (c) alert → single empty run → no recovery
// (d) alert → two empty runs → recovery
func TestE2E_RecoveryNeedsTwoEmpties(t *testing.T) {
	cfg := fakeCfg(t, 2)
	statePath := filepath.Join(filepath.Dir(cfg.path), "state.json")
	warns := []AlertItem{{Host: "h1", Field: "cpu"}}

	// Bring system to firing alert state (two consecutive warns).
	runTick(t, cfg, statePath, warns)
	r := runTick(t, cfg, statePath, warns)
	if r.wantAction != ActionSendAlert {
		t.Fatalf("setup: expected alert, got %v", r.wantAction)
	}

	// First clear: no recovery yet.
	r = runTick(t, cfg, statePath, nil)
	if r.wantAction != ActionNone {
		t.Fatalf("first clear: expected None, got %v", r.wantAction)
	}
	if r.streakAfter != 1 {
		t.Fatalf("first clear: streak should be 1, got %d", r.streakAfter)
	}
	if r.statusAfter != StatusAlert {
		t.Fatalf("first clear: status should remain alert, got %q", r.statusAfter)
	}

	// Second clear: recovery.
	r = runTick(t, cfg, statePath, nil)
	if r.wantAction != ActionSendRecovery {
		t.Fatalf("second clear: expected Recovery, got %v", r.wantAction)
	}
	if r.statusAfter != StatusOK {
		t.Fatalf("second clear: status should be ok, got %q", r.statusAfter)
	}
	if r.streakAfter != 0 {
		t.Fatalf("second clear: streak should reset after send, got %d", r.streakAfter)
	}
}

// (e) alert → clear → re-warn (in pending) → no false recovery
func TestE2E_RawNonEmptyResetsRecoveryStreak(t *testing.T) {
	cfg := fakeCfg(t, 2)
	statePath := filepath.Join(filepath.Dir(cfg.path), "state.json")
	warns := []AlertItem{{Host: "h1", Field: "cpu"}}

	runTick(t, cfg, statePath, warns)
	r := runTick(t, cfg, statePath, warns)
	if r.wantAction != ActionSendAlert {
		t.Fatalf("setup: expected alert, got %v", r.wantAction)
	}

	// First clear: streak=1.
	r = runTick(t, cfg, statePath, nil)
	if r.streakAfter != 1 {
		t.Fatalf("first clear: streak should be 1, got %d", r.streakAfter)
	}

	// Re-warn (raw non-empty even though it's only pending).
	r = runTick(t, cfg, statePath, warns)
	if r.wantAction != ActionNone {
		t.Fatalf("re-warn: expected None (pending + cooldown), got %v", r.wantAction)
	}
	if r.streakAfter != 0 {
		t.Fatalf("re-warn: streak should reset to 0, got %d", r.streakAfter)
	}
	if r.statusAfter != StatusAlert {
		t.Fatalf("re-warn: status should remain alert, got %q", r.statusAfter)
	}
}

// TestE2E_RealWorldFlapping replays the /tmp/a scenario shape: between two
// consecutive runs one alert disappears and a new one appears, alongside a
// stable set. Stable items confirm and fire; the flapping pair (one out, one
// in) stays out of the email. Field names are kept distinct per host because
// the existing ExtractAlerts host-attribution path collapses same-field
// duplicates (pre-existing limitation, unrelated to this change).
func TestE2E_RealWorldFlapping(t *testing.T) {
	cfg := fakeCfg(t, 2)
	statePath := filepath.Join(filepath.Dir(cfg.path), "state.json")

	stable := []AlertItem{
		{Host: "10.10.26.234", Field: "load_average"},
		{Host: "10.10.26.237", Field: "docker.exited"},
		{Host: "10.10.26.235", Field: "redis.235.memory"},
		{Host: "10.10.26.236", Field: "redis.236.memory"},
		{Host: "", Field: "mysql.connectivity"},
		{Host: "", Field: "rabbitmq.bk_bkmonitorv3.celery_cron.no_consumer"},
		{Host: "", Field: "rabbitmq.prod_bk_itsm.default.no_consumer"},
		{Host: "", Field: "rabbitmq.prod_bk_monitorv3.celery.backlog"},
		{Host: "", Field: "rabbitmq.prod_cw_uac_saas.update_alarm_status.no_consumer"},
	}
	flapOut := AlertItem{Host: "", Field: "rabbitmq.prod_weops_saas.celery.no_consumer"}
	flapIn := AlertItem{Host: "10.10.26.237", Field: "redis.237.memory"}

	t0 := append([]AlertItem{}, stable...)
	t0 = append(t0, flapOut)

	t1 := append([]AlertItem{}, stable...)
	t1 = append(t1, flapIn)

	r := runTick(t, cfg, statePath, t0)
	if r.wantAction != ActionNone {
		t.Fatalf("T0: expected None (cold start, all pending), got %v", r.wantAction)
	}

	r = runTick(t, cfg, statePath, t1)
	if r.wantAction != ActionSendAlert {
		t.Fatalf("T1: expected SendAlert (stable items confirmed), got %v", r.wantAction)
	}
	if r.filtered != len(stable) {
		t.Fatalf("T1: filtered=%d, want %d (only the consistently-present items)", r.filtered, len(stable))
	}
	if r.pendingAfter != 1 {
		t.Fatalf("T1: pending should hold the new flap-in item, got %d", r.pendingAfter)
	}
}

func TestDemotePending_RecomputesSummary(t *testing.T) {
	report := makeReport(t, []AlertItem{
		{Host: "h1", Field: "cpu"},
		{Host: "h1", Field: "mem"},
		{Host: "h2", Field: "disk"},
	})
	if report.Summary.Warn != 3 {
		t.Fatalf("setup: expected 3 warns, got %d", report.Summary.Warn)
	}

	pendingKeys := map[string]bool{
		PendingKey("h1", "cpu"):  true,
		PendingKey("h2", "disk"): true,
	}
	demotePending(report, pendingKeys)

	if report.Summary.Warn != 1 {
		t.Fatalf("after demote: expected 1 warn, got %d", report.Summary.Warn)
	}
	// Per-host check status reflects demotion
	for _, h := range report.Hosts {
		for _, c := range h.Checks {
			key := PendingKey(h.Metrics.IP, c.Field)
			if pendingKeys[key] && c.Status != model.StatusNotice {
				t.Fatalf("per-host %s/%s should be Notice, got %q", h.Metrics.IP, c.Field, c.Status)
			}
		}
	}
}
