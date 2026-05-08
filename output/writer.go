package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"weops-inspect/model"
	"weops-inspect/render"
)

// Write generates and writes HTML and JSON report files. Returns the absolute
// HTML path so callers (e.g. notification subsystem) can attach it.
func Write(report *model.InspectReport, outputDir string) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	// Write HTML
	htmlContent, err := render.RenderHTML(report)
	if err != nil {
		return "", fmt.Errorf("render HTML: %w", err)
	}
	htmlPath := filepath.Join(outputDir, "weops_inspection.html")
	if err := os.WriteFile(htmlPath, []byte(htmlContent), 0644); err != nil {
		return "", fmt.Errorf("write HTML: %w", err)
	}
	fmt.Fprintf(os.Stderr, "HTML 报告已生成: %s\n", htmlPath)

	// Write JSON
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal JSON: %w", err)
	}
	jsonPath := filepath.Join(outputDir, "weops_inspection.json")
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		return "", fmt.Errorf("write JSON: %w", err)
	}
	fmt.Fprintf(os.Stderr, "JSON 数据已生成: %s\n", jsonPath)

	absPath, err := filepath.Abs(htmlPath)
	if err != nil {
		return htmlPath, nil
	}
	return absPath, nil
}
