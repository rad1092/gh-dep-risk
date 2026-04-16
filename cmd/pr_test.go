package cmd

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizePRArgsAllowsFlagsAfterPRNumber(t *testing.T) {
	args := []string{"123", "--comment", "--fail-level", "medium"}
	got := normalizePRArgs(args)
	want := []string{"--comment", "--fail-level", "medium", "123"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected normalized args: got %#v want %#v", got, want)
	}
}

func TestNormalizePRArgsAllowsRepoFlagBeforePRNumber(t *testing.T) {
	args := []string{"--repo", "owner/repo", "123", "--bundle-dir", "out"}
	got := normalizePRArgs(args)
	want := []string{"--repo", "owner/repo", "--bundle-dir", "out", "123"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected normalized args: got %#v want %#v", got, want)
	}
}

func TestFlagConsumesValue(t *testing.T) {
	for _, token := range []string{"--repo", "--format=json", "--lang", "--fail-level", "--bundle-dir", "--path"} {
		if !flagConsumesValue(token) {
			t.Fatalf("expected %s to consume a value", token)
		}
	}
	for _, token := range []string{"--comment", "--list-targets", "--no-registry"} {
		if flagConsumesValue(token) {
			t.Fatalf("expected %s to be treated as a boolean flag", token)
		}
	}
}

func TestPrintPRUsageShowsEnglishDefaultLanguage(t *testing.T) {
	var output bytes.Buffer
	printPRUsage(&output)
	if !strings.Contains(output.String(), `output language: ko|en (default "en")`) {
		t.Fatalf("expected help output to mention english default, got %q", output.String())
	}
}
