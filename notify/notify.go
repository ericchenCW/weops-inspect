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

	htmlBody := loadHTMLBody(htmlPath)

	switch action {
	case ActionNone:
		return

	case ActionSendAlert:
		subject := BuildAlertSubject(report.Summary)
		body := BuildAlertBody(report, items)
		if err := Send(cfg, subject, body, htmlBody, htmlPath); err != nil {
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
		if err := Send(cfg, subject, body, htmlBody, htmlPath); err != nil {
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
