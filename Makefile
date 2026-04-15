build:
	go build -o gh-dep-risk .

test:
	go test ./...

workflow-example:
	@printf '%s\n' \
		'gh workflow run .github/workflows/dep-risk-manual.yml -f pr=123' \
		'gh workflow run .github/workflows/dep-risk-manual.yml -f pr=https://github.com/OWNER/REPO/pull/123 -f comment=true' \
		'gh run watch'
