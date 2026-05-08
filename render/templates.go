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
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html.tmpl")
	if err != nil {
		return nil, err
	}
	return tmpl, nil
}
