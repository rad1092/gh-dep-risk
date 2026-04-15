package github

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestResolveCurrentPRUsesCurrentBranch(t *testing.T) {
	originalExec := ghExecContext
	originalBranch := currentGitBranch
	t.Cleanup(func() {
		ghExecContext = originalExec
		currentGitBranch = originalBranch
	})

	currentGitBranch = func(context.Context) (string, error) {
		return "feature/test-branch", nil
	}

	var gotArgs []string
	ghExecContext = func(_ context.Context, args ...string) (bytes.Buffer, bytes.Buffer, error) {
		gotArgs = append([]string{}, args...)
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		stdout.WriteString(`{"number": 17}`)
		return stdout, stderr, nil
	}

	client := NewClient()
	number, err := client.ResolveCurrentPR(context.Background(), Repo{Owner: "owner", Name: "repo"})
	if err != nil {
		t.Fatalf("ResolveCurrentPR returned error: %v", err)
	}
	if number != 17 {
		t.Fatalf("expected PR number 17, got %d", number)
	}

	expected := []string{"pr", "view", "feature/test-branch", "--json", "number", "--repo", "owner/repo"}
	if len(gotArgs) != len(expected) {
		t.Fatalf("unexpected gh args: %#v", gotArgs)
	}
	for i, want := range expected {
		if gotArgs[i] != want {
			t.Fatalf("unexpected gh arg at %d: got %q want %q", i, gotArgs[i], want)
		}
	}
}

func TestResolveCurrentPRBranchError(t *testing.T) {
	originalExec := ghExecContext
	originalBranch := currentGitBranch
	t.Cleanup(func() {
		ghExecContext = originalExec
		currentGitBranch = originalBranch
	})

	currentGitBranch = func(context.Context) (string, error) {
		return "", errors.New("determine current branch: empty branch name")
	}

	client := NewClient()
	_, err := client.ResolveCurrentPR(context.Background(), Repo{Owner: "owner", Name: "repo"})
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "resolve current PR: determine current branch: empty branch name" {
		t.Fatalf("unexpected error: %v", err)
	}
}
