package notify

import (
	"fmt"
	"os"
	"time"

	"weops-inspect/model"
)

// Process is the single entry point invoked from main after the report is on
// disk. It loads state, decides whether to send, sends if needed, and persists
// updated state. Any error is logged to stderr; this function never returns
// one, to protect the inspection exit code.
func Process(cfg *Config, report *model.InspectReport, htmlPath string) {
	if cfg == nil || !cfg.Email.Enabled {
		return
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "notify: 配置不完整，跳过发送: %v\n", err)
		return
	}

	statePath := StatePath(cfg.path)
	prev := LoadState(statePath)

	items := ExtractAlerts(report)
	sigNow := Signature(items)
	cooldown := time.Duration(cfg.Trigger.MinIntervalMinutes) * time.Minute

	now := time.Now()
	action := Decide(now, prev, len(items), sigNow, cooldown)

	if action == ActionSendRecovery && !cfg.Trigger.SendRecovery {
		action = ActionNone
	}

	switch action {
	case ActionNone:
		return

	case ActionSendAlert:
		subject := BuildAlertSubject(report.Summary)
		body := BuildAlertBody(report, items)
		if err := Send(cfg, subject, body, htmlPath); err != nil {
			fmt.Fprintf(os.Stderr, "notify: 告警邮件发送失败: %v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "notify: 已发送告警邮件 (%d 项)\n", len(items))
		_ = SaveState(statePath, &State{
			LastSentAt:    now,
			LastSignature: sigNow,
			LastStatus:    StatusAlert,
		})

	case ActionSendRecovery:
		subject := BuildRecoverySubject(report.Summary)
		body := BuildRecoveryBody(report)
		if err := Send(cfg, subject, body, htmlPath); err != nil {
			fmt.Fprintf(os.Stderr, "notify: 恢复邮件发送失败: %v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "notify: 已发送恢复邮件\n")
		_ = SaveState(statePath, &State{
			LastSentAt:    now,
			LastSignature: "",
			LastStatus:    StatusOK,
		})
	}
}
