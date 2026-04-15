package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gh-dep-risk/internal/analysis"
)

func TestWriteBundle(t *testing.T) {
	t.Setenv("GITHUB_SERVER_URL", "https://github.com")
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")
	t.Setenv("GITHUB_RUN_ID", "12345")

	report := Report{
		Repo: "owner/repo",
		PR: PullRequestMetadata{
			Number:      123,
			URL:         "https://github.com/owner/repo/pull/123",
			Title:       "Update dependencies",
			BaseSHA:     "base",
			HeadSHA:     "head",
			AuthorLogin: "octocat",
		},
		Analysis: analysis.AnalysisResult{
			DependencyReviewAvailable: true,
			Score:                     48,
			Level:                     analysis.RiskLevelHigh,
			BlastRadius:               analysis.BlastRadiusMedium,
			ChangedDependencies: []analysis.DependencyChange{
				{
					Name:        "left-pad",
					ChangeType:  analysis.ChangeUpdated,
					Scope:       analysis.ScopeRuntime,
					Score:       48,
					RiskDrivers: []string{analysis.DriverMajorVersionBump},
					FromVersion: "1.0.0",
					ToVersion:   "2.0.0",
				},
			},
		},
	}

	dir := t.TempDir()
	paths, err := WriteBundle(report, "en", dir)
	if err != nil {
		t.Fatal(err)
	}

	before := readBundleFiles(t, paths)
	if !strings.HasPrefix(before.Markdown, "<!-- gh-dep-risk -->") {
		t.Fatalf("expected markdown bundle to start with marker comment")
	}

	var metadata BundleMetadata
	if err := json.Unmarshal([]byte(before.Metadata), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.WorkflowRunURL != "https://github.com/owner/repo/actions/runs/12345" {
		t.Fatalf("unexpected workflow URL: %q", metadata.WorkflowRunURL)
	}
	if metadata.Score != 48 || metadata.Level != analysis.RiskLevelHigh {
		t.Fatalf("unexpected metadata: %#v", metadata)
	}

	if _, err := WriteBundle(report, "en", dir); err != nil {
		t.Fatal(err)
	}
	after := readBundleFiles(t, paths)
	if before != after {
		t.Fatalf("expected deterministic bundle outputs")
	}
}

type bundleContents struct {
	Human    string
	JSON     string
	Markdown string
	Metadata string
}

func readBundleFiles(t *testing.T, paths BundlePaths) bundleContents {
	t.Helper()
	return bundleContents{
		Human:    readFile(t, paths.Human),
		JSON:     readFile(t, paths.JSON),
		Markdown: readFile(t, paths.Markdown),
		Metadata: readFile(t, paths.Metadata),
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
