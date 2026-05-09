package notify

import (
	"fmt"
	"os"
	"time"

	"weops-inspect/model"
)

// loadHTMLBody reads the on-disk HTML report and returns an HTML fragment
// suitable for embedding as an email alternative body. On any failure it
// returns an empty string so the caller falls back to plain text + attachment;
// failures are logged to stderr but never propagated.
func loadHTMLBody(htmlPath string) string {
	if htmlPath == "" {
		return ""
	}
	raw, err := os.ReadFile(htmlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "notify: 解析 HTML 报告失败，退化为纯文本+附件: %v\n", err)
		return ""
	}
	body, style, ok := extractBodyFragment(string(raw))
	if !ok {
		fmt.Fprintf(os.Stderr, "notify: 解析 HTML 报告失败，退化为纯文本+附件: %v\n",
			fmt.Errorf("missing <body> in %s", htmlPath))
		return ""
	}
	if style == "" {
		return body
	}
	return "<style>" + style + "</style>\n" + body
}

// PrepContext is the precomputed state produced by Prepare. It carries the
// filtered alert list, persistence map, and recovery streak forward to the
// post-output Dispatch step. The report has already been mutated in place by
// the time this is returned, so any subsequent rendering will reflect filter
// decisions.
type PrepContext struct {
	StatePath     string
	Prev          *State
	Filtered      []AlertItem
	NextPending   map[string]PendingItem
	NewStreak     int
	RawWarnsEmpty bool
	Now           time.Time
}

// Prepare validates config, loads previous state, applies persistence
// confirmation, and demotes pending warns in the report (Summary recomputed).
// Returns nil and logs to stderr if config is disabled or invalid; the report
// is left untouched in those cases.
//
// Callers must invoke Prepare BEFORE writing the HTML report so the on-disk
// artifact matches what eventually gets emailed.
func Prepare(cfg *Config, report *model.InspectReport) *PrepContext {
	if cfg == nil || !cfg.Email.Enabled {
		return nil
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "notify: 配置不完整，跳过发送: %v\n", err)
		return nil
	}

	statePath := StatePath(cfg.path)
	prev := LoadState(statePath)

	rawItems := ExtractAlerts(report)
	rawWarnsEmpty := len(rawItems) == 0

	now := time.Now()
	persRes := ApplyPersistence(rawItems, prev.Pending, cfg.Persistence.ConsecutiveRuns, now)
	streak := UpdateRecoveryStreak(prev.RecoveryStreak, prev.LastStatus, rawWarnsEmpty)

	demotePending(report, persRes.PendingKeys)

	return &PrepContext{
		StatePath:     statePath,
		Prev:          prev,
		Filtered:      persRes.Filtered,
		NextPending:   persRes.NextPending,
		NewStreak:     streak,
		RawWarnsEmpty: rawWarnsEmpty,
		Now:           now,
	}
}

// Dispatch executes the decision matrix and persists state. Must be called
// AFTER the HTML report has been written, so the email body and the attached
// HTML are consistent.
func Dispatch(ctx *PrepContext, cfg *Config, report *model.InspectReport, htmlPath string) {
	if ctx == nil {
		return
	}

	cooldown := time.Duration(cfg.Trigger.MinIntervalMinutes) * time.Minute
	sigNow := Signature(ctx.Filtered)
	n := cfg.Persistence.ConsecutiveRuns

	var action Action
	switch {
	case ctx.Prev.LastStatus == StatusAlert && len(ctx.Filtered) == 0:
		// No firing items this run. Two sub-cases:
		//  - raw warns empty AND streak ≥ N → genuine recovery
		//  - otherwise (raw non-empty but all pending, OR streak still
		//    accumulating) → suppress; do not let Decide misfire a recovery
		if ctx.RawWarnsEmpty && ctx.NewStreak >= n {
			action = ActionSendRecovery
		} else {
			action = ActionNone
		}
	default:
		action = Decide(ctx.Now, ctx.Prev, len(ctx.Filtered), sigNow, cooldown)
	}

	if action == ActionSendRecovery && !cfg.Trigger.SendRecovery {
		action = ActionNone
	}

	htmlBody := loadHTMLBody(htmlPath)

	switch action {
	case ActionNone:
		// Suppressed; do not touch last_*, but persist updated pending and
		// recovery streak so the gating state advances.
		next := *ctx.Prev
		next.Pending = ctx.NextPending
		next.RecoveryStreak = ctx.NewStreak
		_ = SaveState(ctx.StatePath, &next)
		return

	case ActionSendAlert:
		subject := BuildAlertSubject(report.Summary)
		body := BuildAlertBody(report, ctx.Filtered)
		if err := Send(cfg, subject, body, htmlBody, htmlPath); err != nil {
			fmt.Fprintf(os.Stderr, "notify: 告警邮件发送失败: %v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "notify: 已发送告警邮件 (%d 项)\n", len(ctx.Filtered))
		_ = SaveState(ctx.StatePath, &State{
			LastSentAt:     ctx.Now,
			LastSignature:  sigNow,
			LastStatus:     StatusAlert,
			Pending:        ctx.NextPending,
			RecoveryStreak: 0,
		})

	case ActionSendRecovery:
		subject := BuildRecoverySubject(report.Summary)
		body := BuildRecoveryBody(report)
		if err := Send(cfg, subject, body, htmlBody, htmlPath); err != nil {
			fmt.Fprintf(os.Stderr, "notify: 恢复邮件发送失败: %v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "notify: 已发送恢复邮件\n")
		_ = SaveState(ctx.StatePath, &State{
			LastSentAt:     ctx.Now,
			LastSignature:  "",
			LastStatus:     StatusOK,
			Pending:        ctx.NextPending,
			RecoveryStreak: 0,
		})
	}
}

// Process is a convenience entry point that runs Prepare immediately followed
// by Dispatch. main.go uses Prepare + Dispatch separately so the report can be
// mutated before HTML is written; Process remains for callers (and tests) that
// don't care about that ordering.
func Process(cfg *Config, report *model.InspectReport, htmlPath string) {
	ctx := Prepare(cfg, report)
	Dispatch(ctx, cfg, report, htmlPath)
}

// demotePending changes the Status of any Warn check matching a key in
// pendingKeys (host|field) to StatusNotice and recomputes Summary. The same
// host attribution heuristic as ExtractAlerts is used for AllChecks; per-host
// Checks are walked directly so each host's view is updated even when the
// same field name appears on multiple hosts.
func demotePending(report *model.InspectReport, pendingKeys map[string]bool) {
	if len(pendingKeys) == 0 {
		return
	}

	for i := range report.Hosts {
		ip := report.Hosts[i].Metrics.IP
		for j := range report.Hosts[i].Checks {
			c := &report.Hosts[i].Checks[j]
			if c.Status != model.StatusWarn {
				continue
			}
			if pendingKeys[PendingKey(ip, c.Field)] {
				c.Status = model.StatusNotice
			}
		}
	}

	hostByField := map[string]string{}
	for _, h := range report.Hosts {
		for _, c := range h.Checks {
			hostByField[c.Field] = h.Metrics.IP
		}
	}
	for i := range report.AllChecks {
		c := &report.AllChecks[i]
		if c.Status != model.StatusWarn {
			continue
		}
		host := hostByField[c.Field]
		if pendingKeys[PendingKey(host, c.Field)] {
			c.Status = model.StatusNotice
		}
	}

	report.Summary = recomputeSummary(report.AllChecks)
}

// recomputeSummary mirrors checker.Summarize to avoid pulling the checker
// package into notify (would be a fine import but keeps the dep graph
// notify→model only).
func recomputeSummary(results []model.CheckResult) model.CheckSummary {
	var s model.CheckSummary
	for _, r := range results {
		switch r.Status {
		case model.StatusOK:
			s.OK++
			s.Total++
		case model.StatusWarn:
			s.Warn++
			s.Total++
		case model.StatusUnknown:
			s.Unknown++
			s.Total++
		}
	}
	return s
}
