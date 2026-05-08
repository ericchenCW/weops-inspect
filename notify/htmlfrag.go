package notify

import "strings"

// extractBodyFragment pulls the inline <style> block and the inner <body>
// content out of an HTML document so they can be embedded as the HTML
// alternative of an email. Returns ok=false when <body> is missing; a missing
// <style> is tolerated and returned as an empty string.
func extractBodyFragment(html string) (body, style string, ok bool) {
	style = extractStyle(html)
	body, ok = extractBody(html)
	return body, style, ok
}

func extractStyle(html string) string {
	open := strings.Index(html, "<style")
	if open < 0 {
		return ""
	}
	gt := strings.Index(html[open:], ">")
	if gt < 0 {
		return ""
	}
	contentStart := open + gt + 1
	close := strings.Index(html[contentStart:], "</style>")
	if close < 0 {
		return ""
	}
	return html[contentStart : contentStart+close]
}

func extractBody(html string) (string, bool) {
	open := strings.Index(html, "<body")
	if open < 0 {
		return "", false
	}
	gt := strings.Index(html[open:], ">")
	if gt < 0 {
		return "", false
	}
	contentStart := open + gt + 1
	close := strings.Index(html[contentStart:], "</body>")
	if close < 0 {
		return "", false
	}
	return html[contentStart : contentStart+close], true
}
