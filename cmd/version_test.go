package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunVersionHumanOutput(t *testing.T) {
	restore := setBuildInfoForTest("0.1.0", "abc1234", "2026-04-15T12:00:00Z")
	defer restore()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runVersion(&stdout, &stderr, nil)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
	expected := "gh-dep-risk 0.1.0 (commit abc1234, built 2026-04-15T12:00:00Z)\n"
	if stdout.String() != expected {
		t.Fatalf("unexpected human version output: %q", stdout.String())
	}
}

func TestRunVersionJSONOutput(t *testing.T) {
	restore := setBuildInfoForTest("0.1.0", "abc1234", "2026-04-15T12:00:00Z")
	defer restore()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runVersion(&stdout, &stderr, []string{"--json"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	var payload map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{
		"version": "0.1.0",
		"commit":  "abc1234",
		"date":    "2026-04-15T12:00:00Z",
	}
	if len(payload) != len(expected) {
		t.Fatalf("unexpected JSON keys: %#v", payload)
	}
	for key, want := range expected {
		if got := payload[key]; got != want {
			t.Fatalf("unexpected %s: got %q want %q", key, got, want)
		}
	}
}

func TestRunVersionDefaults(t *testing.T) {
	restore := setBuildInfoForTest("dev", "none", "unknown")
	defer restore()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runVersion(&stdout, &stderr, nil)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "gh-dep-risk dev") {
		t.Fatalf("expected dev default output, got %q", stdout.String())
	}
}

func TestRunVersionRejectsPositionalArgs(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runVersion(&stdout, &stderr, []string{"unexpected"})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "version does not accept positional arguments") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func setBuildInfoForTest(nextVersion, nextCommit, nextDate string) func() {
	originalVersion, originalCommit, originalDate := version, commit, date
	version, commit, date = nextVersion, nextCommit, nextDate
	return func() {
		version, commit, date = originalVersion, originalCommit, originalDate
	}
}
