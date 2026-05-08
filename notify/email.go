package notify

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	mail "github.com/wneessen/go-mail"

	"weops-inspect/model"
)

const sendTimeout = 30 * time.Second

// BuildAlertSubject formats the alert email subject.
func BuildAlertSubject(s model.CheckSummary) string {
	return fmt.Sprintf("[WeOps 巡检告警] %d/%d 项异常", s.Warn, s.Total)
}

// BuildRecoverySubject formats the recovery email subject.
func BuildRecoverySubject(s model.CheckSummary) string {
	return fmt.Sprintf("[WeOps 巡检恢复] 全部正常 (%d 项检查)", s.Total)
}

// BuildAlertBody renders a plain-text alert body listing all warn items.
// Unknown items are reflected only in the summary line; details are not listed
// (Notice items don't reach this function at all).
func BuildAlertBody(report *model.InspectReport, items []AlertItem) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[WeOps 巡检告警] %s\n", report.Timestamp)
	fmt.Fprintf(&b, "Summary: 共 %d 项检查，%d 正常，%d 告警，%d 未知\n\n",
		report.Summary.Total, report.Summary.OK, report.Summary.Warn, report.Summary.Unknown)

	b.WriteString("告警明细:\n")
	sorted := make([]AlertItem, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Host != sorted[j].Host {
			return sorted[i].Host < sorted[j].Host
		}
		return sorted[i].Field < sorted[j].Field
	})
	for _, it := range sorted {
		host := it.Host
		if host == "" {
			host = "-"
		}
		if it.Threshold != "" {
			fmt.Fprintf(&b, "  %-16s %-40s = %s  (阈值 %s)\n", host, it.Field, it.Value, it.Threshold)
		} else {
			fmt.Fprintf(&b, "  %-16s %-40s = %s\n", host, it.Field, it.Value)
		}
	}

	b.WriteString("\n详见附件 weops_inspection.html。\n")
	return b.String()
}

// BuildRecoveryBody renders a plain-text recovery body.
func BuildRecoveryBody(report *model.InspectReport) string {
	return fmt.Sprintf(
		"[WeOps 巡检恢复] %s\n\n所有检查项已恢复正常 (共 %d 项)。详见附件。\n",
		report.Timestamp, report.Summary.Total)
}

// Send composes and dispatches a single message via SMTP. When htmlBody is
// non-empty it is added as a text/html alternative alongside the plain text
// body, producing a multipart/alternative payload (further nested under
// multipart/mixed when an attachment is present).
func Send(cfg *Config, subject, body, htmlBody, attachmentPath string) error {
	msg := mail.NewMsg()
	if err := msg.From(cfg.Email.From); err != nil {
		return fmt.Errorf("set From: %w", err)
	}
	if err := msg.To(cfg.Email.To...); err != nil {
		return fmt.Errorf("set To: %w", err)
	}
	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextPlain, body)
	if htmlBody != "" {
		msg.AddAlternativeString(mail.TypeTextHTML, htmlBody)
	}
	if attachmentPath != "" {
		msg.AttachFile(attachmentPath)
	}

	opts := []mail.Option{
		mail.WithPort(cfg.Email.SMTPPort),
		mail.WithTimeout(sendTimeout),
	}
	if cfg.Email.UseTLS {
		opts = append(opts, mail.WithSSL())
	} else {
		opts = append(opts, mail.WithTLSPolicy(mail.TLSOpportunistic))
	}
	if cfg.Email.Username != "" {
		opts = append(opts,
			mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(cfg.Email.Username),
			mail.WithPassword(cfg.Email.Password),
		)
	}

	client, err := mail.NewClient(cfg.Email.SMTPHost, opts...)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()
	if err := client.DialAndSendWithContext(ctx, msg); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}
	return nil
}
