package render

import (
	"bytes"
	"fmt"

	"weops-inspect/model"
)

// RenderHTML renders the full inspection report to an HTML string.
func RenderHTML(report *model.InspectReport) (string, error) {
	tmpl, err := LoadTemplates()
	if err != nil {
		return "", fmt.Errorf("load templates: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "layout.html.tmpl", report); err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}
	return buf.String(), nil
}
