package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestExecuteHelpPR(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := execute(&stdout, &stderr, []string{"help", "pr"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "gh dep-risk pr [<number>|<url>]") {
		t.Fatalf("expected PR help output, got %q", stdout.String())
	}
}

func TestExecuteVersionHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := execute(&stdout, &stderr, []string{"version", "--help"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "gh dep-risk version") {
		t.Fatalf("expected version help output, got %q", stdout.String())
	}
}

func TestExecuteVersionJSONHelpExample(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := execute(&stdout, &stderr, []string{"--help"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "gh dep-risk version --json") {
		t.Fatalf("expected root help to mention version --json, got %q", stdout.String())
	}
}

func TestExecutePRRejectsUnsupportedFormat(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := execute(&stdout, &stderr, []string{"pr", "--format", "xml"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), `unsupported format "xml"`) {
		t.Fatalf("expected unsupported format error, got %q", stderr.String())
	}
}
