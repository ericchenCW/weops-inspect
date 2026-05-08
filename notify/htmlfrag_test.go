package notify

import "testing"

func TestExtractBodyFragment(t *testing.T) {
	tests := []struct {
		name      string
		html      string
		wantBody  string
		wantStyle string
		wantOK    bool
	}{
		{
			name: "typical report",
			html: `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<style>body { color: red; }</style>
</head>
<body>
<h1>Hi</h1>
</body>
</html>`,
			wantBody:  "\n<h1>Hi</h1>\n",
			wantStyle: "body { color: red; }",
			wantOK:    true,
		},
		{
			name:      "body with attributes",
			html:      `<html><body class="x" data-y="1"><p>x</p></body></html>`,
			wantBody:  "<p>x</p>",
			wantStyle: "",
			wantOK:    true,
		},
		{
			name:      "missing body",
			html:      `<html><head><style>a{}</style></head></html>`,
			wantBody:  "",
			wantStyle: "a{}",
			wantOK:    false,
		},
		{
			name:      "missing style",
			html:      `<html><body>x</body></html>`,
			wantBody:  "x",
			wantStyle: "",
			wantOK:    true,
		},
		{
			name:      "empty input",
			html:      "",
			wantBody:  "",
			wantStyle: "",
			wantOK:    false,
		},
		{
			name:      "style with type attribute",
			html:      `<html><head><style type="text/css">.a{color:red}</style></head><body>y</body></html>`,
			wantBody:  "y",
			wantStyle: ".a{color:red}",
			wantOK:    true,
		},
		{
			name:      "body present but unclosed",
			html:      `<html><body><p>oops`,
			wantBody:  "",
			wantStyle: "",
			wantOK:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, style, ok := extractBodyFragment(tc.html)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if body != tc.wantBody {
				t.Errorf("body = %q, want %q", body, tc.wantBody)
			}
			if style != tc.wantStyle {
				t.Errorf("style = %q, want %q", style, tc.wantStyle)
			}
		})
	}
}
