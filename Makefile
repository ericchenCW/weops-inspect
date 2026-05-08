.PHONY: test lint lint-templates build

build:
	go build ./...

test:
	go test ./...

lint-templates:
	@hits=$$(grep -RnE '\{\{if (gt|eq) \.' render/templates/ || true); \
	if [ -n "$$hits" ]; then \
		echo "Inline {{if gt|eq .X}} comparisons are forbidden in templates."; \
		echo "Color via Status fields filled by checker. Hits:"; \
		echo "$$hits"; \
		exit 1; \
	fi

lint: lint-templates
