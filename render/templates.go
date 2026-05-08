package render

import (
	"embed"
	"html/template"

	"weops-inspect/model"
)

//go:embed templates/*.html.tmpl
var templateFS embed.FS

// LoadTemplates parses all embedded templates.
func LoadTemplates() (*template.Template, error) {
	funcMap := template.FuncMap{
		"statusColor": func(status string) string {
			if status == "ok" {
				return "green"
			}
			return "red"
		},
		"checkColor": func(status string) string {
			if status == "ok" {
				return "#27ae60"
			}
			return "#e74c3c"
		},
		"esStatusColor": func(status string) string {
			switch status {
			case "green":
				return "#27ae60"
			case "yellow":
				return "#f39c12"
			default:
				return "#e74c3c"
			}
		},
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i
			}
			return s
		},
		"add": func(a, b int) int {
			return a + b
		},
		"findCheck": func(checks []model.CheckResult, field string) model.CheckResult {
			for _, c := range checks {
				if c.Field == field {
					return c
				}
			}
			return model.CheckResult{Field: field, Value: "N/A", Status: model.StatusOK}
		},
		"list": func(args ...string) []string {
			return args
		},
		// statusClass maps a CheckStatus to a CSS class for HTML coloring.
		// Warn and Notice both render as red (semantic difference is in
		// alerting, not visuals). Unknown is gray. Empty produces no class.
		"statusClass": func(s model.CheckStatus) string {
			switch s {
			case model.StatusOK:
				return "status-ok"
			case model.StatusWarn, model.StatusNotice:
				return "status-warn"
			case model.StatusUnknown:
				return "status-na"
			default:
				return ""
			}
		},
		// okClass: "ok" → status-ok, anything else → status-warn.
		// Used for legacy string-typed Status fields (replication, dependency).
		"okClass": func(s string) string {
			if s == "ok" {
				return "status-ok"
			}
			return "status-warn"
		},
		// yesClass: "Yes" → status-ok, anything else → status-warn.
		// Used for MySQL slave IO/SQL running flags.
		"yesClass": func(s string) string {
			if s == "Yes" {
				return "status-ok"
			}
			return "status-warn"
		},
		// linkStatusClass: ok → status-ok, N/A → "", else → status-warn.
		"linkStatusClass": func(s string) string {
			switch s {
			case "ok":
				return "status-ok"
			case "N/A":
				return ""
			default:
				return "status-warn"
			}
		},
		// depStatusClass: ok → status-ok, skip → "", else → status-warn.
		"depStatusClass": func(s string) string {
			switch s {
			case "ok":
				return "status-ok"
			case "skip":
				return ""
			default:
				return "status-warn"
			}
		},
		// mongoHealthLabel: 1 → 健康, else → 异常.
		"mongoHealthLabel": func(h int) string {
			if h == 1 {
				return "健康"
			}
			return "异常"
		},
		// boolLabel: true → trueLabel, else → falseLabel. Avoids inline `{{if eq}}`.
		"boolLabel": func(b bool, trueLabel, falseLabel string) string {
			if b {
				return trueLabel
			}
			return falseLabel
		},
		// boolClassOK: true → status-ok, false → status-warn.
		"boolClassOK": func(b bool) string {
			if b {
				return "status-ok"
			}
			return "status-warn"
		},
		// warnCardClass: "warn" if n>0 else "ok". Used by summary cards which
		// use a different CSS palette from per-cell status classes.
		"warnCardClass": func(n int) string {
			if n > 0 {
				return "warn"
			}
			return "ok"
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html.tmpl")
	if err != nil {
		return nil, err
	}
	return tmpl, nil
}
