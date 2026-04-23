package app

import (
	"encoding/json"
	"testing"

	ghclient "gh-dep-risk/internal/github"
	"gh-dep-risk/internal/render"
	"github.com/cli/go-gh/v2/pkg/api"
)

func TestRunPRTargetShapeMatrix(t *testing.T) {
	testCases := []struct {
		name              string
		client            func(*testing.T) *fakeGitHubClient
		opts              RunPROptions
		wantTargets       []string
		wantQuickCommands []string
		wantReview        bool
	}{
		{
			name:              "npm root",
			client:            newConfiguredFakeGitHubClient,
			opts:              RunPROptions{Format: "json"},
			wantTargets:       []string{"root"},
			wantQuickCommands: []string{"npm ls --all", "npm ls left-pad"},
			wantReview:        true,
		},
		{
			name:              "npm workspace",
			client:            newWorkspaceFakeGitHubClient,
			opts:              RunPROptions{Format: "json", Paths: []string{"apps/web"}},
			wantTargets:       []string{"apps/web"},
			wantQuickCommands: []string{"cd apps/web && npm ls --all", "cd apps/web && npm ls axios"},
			wantReview:        true,
		},
		{
			name: "npm standalone nested project",
			client: func(t *testing.T) *fakeGitHubClient {
				client := newWorkspaceFakeGitHubClient(t)
				client.files = []ghclient.PullRequestFile{
					{Filename: "services/api/package.json", Status: "modified"},
					{Filename: "services/api/package-lock.json", Status: "modified"},
				}
				return client
			},
			opts:              RunPROptions{Format: "json", Paths: []string{"services/api"}},
			wantTargets:       []string{"services/api"},
			wantQuickCommands: []string{"cd services/api && npm ls --all", "cd services/api && npm ls lodash"},
			wantReview:        true,
		},
		{
			name:              "pnpm root",
			client:            newPNPMRootFakeGitHubClient,
			opts:              RunPROptions{Format: "json"},
			wantTargets:       []string{"root"},
			wantQuickCommands: []string{"pnpm list --depth Infinity", "pnpm why axios"},
			wantReview:        true,
		},
		{
			name:              "pnpm workspace",
			client:            newPNPMWorkspaceFakeGitHubClient,
			opts:              RunPROptions{Format: "json", Paths: []string{"apps/web"}},
			wantTargets:       []string{"apps/web"},
			wantQuickCommands: []string{"cd apps/web && pnpm list --depth Infinity", "cd apps/web && pnpm why axios"},
			wantReview:        true,
		},
		{
			name: "pnpm standalone nested project",
			client: func(t *testing.T) *fakeGitHubClient {
				client := newPNPMWorkspaceFakeGitHubClient(t)
				client.files = []ghclient.PullRequestFile{
					{Filename: "tools/cli/package.json", Status: "modified"},
					{Filename: "tools/cli/pnpm-lock.yaml", Status: "modified"},
				}
				return client
			},
			opts:              RunPROptions{Format: "json", Paths: []string{"tools/cli"}},
			wantTargets:       []string{"tools/cli"},
			wantQuickCommands: []string{"cd tools/cli && pnpm list --depth Infinity", "cd tools/cli && pnpm why commander"},
			wantReview:        true,
		},
		{
			name:              "mixed npm and pnpm repo",
			client:            newMixedManagerFakeGitHubClient,
			opts:              RunPROptions{Format: "json"},
			wantTargets:       []string{"root", "tools/cli"},
			wantQuickCommands: []string{"npm ls --all", "cd tools/cli && pnpm list --depth Infinity"},
			wantReview:        true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, _, err := runPRWithClient(t, tc.client(t), tc.opts)
			if err != nil {
				t.Fatal(err)
			}

			var payload render.JSONReport
			if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
				t.Fatal(err)
			}

			if payload.DependencyReviewAvailable != tc.wantReview {
				t.Fatalf("expected dependency review %t, got %#v", tc.wantReview, payload)
			}
			if len(payload.Targets) != len(tc.wantTargets) {
				t.Fatalf("expected targets %v, got %#v", tc.wantTargets, payload.Targets)
			}
			for index, expected := range tc.wantTargets {
				if payload.Targets[index].Target.DisplayName != expected {
					t.Fatalf("expected target %q at %d, got %#v", expected, index, payload.Targets)
				}
			}
			for _, expected := range tc.wantQuickCommands {
				if !containsString(payload.QuickCommands, expected) {
					t.Fatalf("expected quick commands to contain %q, got %#v", expected, payload.QuickCommands)
				}
			}
		})
	}
}

func TestRunPRDependencyReviewModeMatrix(t *testing.T) {
	testCases := []struct {
		name       string
		client     func(*testing.T) *fakeGitHubClient
		opts       RunPROptions
		degrade    func(*fakeGitHubClient)
		wantReview bool
	}{
		{
			name:       "npm dependency review available",
			client:     newConfiguredFakeGitHubClient,
			opts:       RunPROptions{Format: "json"},
			wantReview: true,
		},
		{
			name:   "npm dependency review fallback",
			client: newConfiguredFakeGitHubClient,
			opts:   RunPROptions{Format: "json"},
			degrade: func(client *fakeGitHubClient) {
				client.compareErr = &api.HTTPError{StatusCode: 404, Message: "dependency review disabled"}
			},
			wantReview: false,
		},
		{
			name:       "pnpm dependency review available",
			client:     newPNPMRootFakeGitHubClient,
			opts:       RunPROptions{Format: "json"},
			wantReview: true,
		},
		{
			name:   "pnpm dependency review fallback",
			client: newPNPMRootFakeGitHubClient,
			opts:   RunPROptions{Format: "json"},
			degrade: func(client *fakeGitHubClient) {
				client.compareErr = &api.HTTPError{StatusCode: 404, Message: "dependency review disabled"}
			},
			wantReview: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := tc.client(t)
			if tc.degrade != nil {
				tc.degrade(client)
			}
			stdout, _, err := runPRWithClient(t, client, tc.opts)
			if err != nil {
				t.Fatal(err)
			}

			var payload render.JSONReport
			if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
				t.Fatal(err)
			}
			if payload.DependencyReviewAvailable != tc.wantReview {
				t.Fatalf("expected dependency review %t, got %#v", tc.wantReview, payload)
			}
		})
	}
}

func newPNPMRootFakeGitHubClient(t *testing.T) *fakeGitHubClient {
	t.Helper()
	client := newFakeGitHubClient()
	client.pr = ghclient.PullRequest{
		Title:       "Update pnpm root dependencies",
		Draft:       false,
		Number:      123,
		BaseSHA:     "base-sha",
		HeadSHA:     "head-sha",
		URL:         "https://github.com/owner/repo/pull/123",
		AuthorLogin: "octocat",
	}
	client.files = []ghclient.PullRequestFile{
		{Filename: "package.json", Status: "modified"},
		{Filename: "pnpm-lock.yaml", Status: "modified"},
	}
	client.repositoryFilesByRef["base-sha"] = []string{"package.json", "pnpm-lock.yaml"}
	client.repositoryFilesByRef["head-sha"] = []string{"package.json", "pnpm-lock.yaml"}
	client.filesByKey[fileKey("package.json", "base-sha")] = readFixture(t, "pnpm.root.package.json")
	client.filesByKey[fileKey("package.json", "head-sha")] = readFixture(t, "pnpm.root.package.json")
	client.filesByKey[fileKey("pnpm-lock.yaml", "base-sha")] = readFixture(t, "pnpm.root.base.lock.yaml")
	client.filesByKey[fileKey("pnpm-lock.yaml", "head-sha")] = readFixture(t, "pnpm.root.head.lock.yaml")
	client.compareChanges = []ghclient.DependencyReviewChange{
		{Name: "axios", Manifest: "package.json", Ecosystem: "pnpm", ChangeType: "added", Version: "1.7.0"},
	}
	return client
}
