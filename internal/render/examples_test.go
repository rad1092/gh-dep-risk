package render

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rad1092/gh-dependency-risk/internal/analysis"
	"github.com/rad1092/gh-dependency-risk/internal/npm"
)

var updateExamples = flag.Bool("update", false, "rewrite docs/examples from the sample analyses")

// The examples in docs/examples are rendered from real analyses of the sample
// pull requests below, so every score in them is what the scoring code gives.
// Regenerate them with: go test ./internal/render -run TestCheckedInExamples -update
func TestCheckedInExamplesStayInSync(t *testing.T) {
	testCases := []struct {
		name   string
		report Report
	}{
		{name: "single-target", report: sampleSingleTargetReport(t)},
		{name: "multi-target", report: sampleMultiTargetReport(t)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{
				"human":    tc.name + "-human.txt",
				"markdown": tc.name + ".md",
				"json":     tc.name + ".json",
			}
			for format, file := range files {
				examplePath := filepath.Join("..", "..", "docs", "examples", file)
				got, err := Render(tc.report, format, "en")
				if err != nil {
					t.Fatal(err)
				}
				if *updateExamples {
					if err := os.WriteFile(examplePath, []byte(got), 0o644); err != nil {
						t.Fatal(err)
					}
					continue
				}
				wantBytes, err := os.ReadFile(filepath.Clean(examplePath))
				if err != nil {
					t.Fatal(err)
				}
				if got != string(wantBytes) {
					t.Fatalf("example %s (%s) is out of sync; run go test ./internal/render -run TestCheckedInExamples -update", examplePath, format)
				}
			}
		})
	}
}

func TestExampleScoresMatchTheirBreakdowns(t *testing.T) {
	for _, report := range []Report{sampleSingleTargetReport(t), sampleMultiTargetReport(t)} {
		for _, change := range report.Analysis.ChangedDependencies {
			sum := 0
			for _, part := range change.ScoreBreakdown {
				sum += part.Points
			}
			if sum != change.Score {
				t.Fatalf("%s: score %d, breakdown %v", change.Name, change.Score, change.ScoreBreakdown)
			}
		}
	}
}

var exampleNow = time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)

func mustManifest(t *testing.T, data string) *npm.PackageManifest {
	t.Helper()
	manifest, err := npm.ParsePackageManifest([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

func mustLockfile(t *testing.T, data string) *npm.Lockfile {
	t.Helper()
	lockfile, err := npm.ParseLockfile([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	return lockfile
}

// sampleSingleTargetReport is an npm repository without Dependency Review,
// where a PR moves left-pad to a new major version that was published two
// days ago and runs an install script.
func sampleSingleTargetReport(t *testing.T) Report {
	target := analysis.AnalysisTarget{DisplayName: "root", ManifestPath: "package.json", LockfilePath: "package-lock.json", Kind: analysis.TargetKindRoot, PackageManager: "npm", Ecosystem: "npm"}
	input := analysis.Input{
		Now:          exampleNow,
		Target:       target,
		BaseManifest: mustManifest(t, `{"name":"app","dependencies":{"left-pad":"^1.0.0"}}`),
		HeadManifest: mustManifest(t, `{"name":"app","dependencies":{"left-pad":"^2.0.0"}}`),
		BaseLockfile: mustLockfile(t, `{"name":"app","lockfileVersion":3,"packages":{
			"":{"name":"app","dependencies":{"left-pad":"^1.0.0"}},
			"node_modules/left-pad":{"version":"1.0.0","resolved":"https://registry.npmjs.org/left-pad/-/left-pad-1.0.0.tgz"}}}`),
		HeadLockfile: mustLockfile(t, `{"name":"app","lockfileVersion":3,"packages":{
			"":{"name":"app","dependencies":{"left-pad":"^2.0.0"}},
			"node_modules/left-pad":{"version":"2.0.0","resolved":"https://registry.npmjs.org/left-pad/-/left-pad-2.0.0.tgz","hasInstallScript":true}}}`),
	}
	published := map[analysis.PackageVersion]time.Time{
		{Name: "left-pad", Version: "2.0.0"}: exampleNow.Add(-50 * time.Hour),
	}
	result := analysis.Analyze(input, published)
	return Report{
		Repo: "owner/repo",
		PR: PullRequestMetadata{
			Number:      123,
			URL:         "https://github.com/owner/repo/pull/123",
			Title:       "Update dependencies",
			BaseSHA:     "base-sha",
			HeadSHA:     "head-sha",
			AuthorLogin: "octocat",
		},
		Analysis: analysis.AggregateResults([]analysis.TargetAnalysisResult{analysis.TargetResult(target, result)}),
	}
}

// sampleMultiTargetReport is an npm workspace with Dependency Review. One PR
// moves apps/web to react-router-dom 7 and adds tailwind-merge to packages/ui.
// The importing files are what the git grep step finds in a clone.
func sampleMultiTargetReport(t *testing.T) Report {
	web := analysis.AnalysisTarget{DisplayName: "apps/web", ManifestPath: "apps/web/package.json", LockfilePath: "package-lock.json", Kind: analysis.TargetKindWorkspace, PackageManager: "npm", Ecosystem: "npm"}
	ui := analysis.AnalysisTarget{DisplayName: "packages/ui", ManifestPath: "packages/ui/package.json", LockfilePath: "package-lock.json", Kind: analysis.TargetKindWorkspace, PackageManager: "npm", Ecosystem: "npm"}

	webResult := analysis.Analyze(analysis.Input{
		Now:                       exampleNow,
		Target:                    web,
		DependencyReviewAvailable: true,
		BaseManifest:              mustManifest(t, `{"name":"web","dependencies":{"react-router-dom":"^6.26.0"}}`),
		HeadManifest:              mustManifest(t, `{"name":"web","dependencies":{"react-router-dom":"^7.1.0"}}`),
		ReviewChanges: []analysis.ReviewChange{
			{Name: "react-router-dom", Manifest: "package-lock.json", ChangeType: analysis.ChangeRemoved, Version: "6.26.0"},
			{Name: "react-router-dom", Manifest: "package-lock.json", ChangeType: analysis.ChangeAdded, Version: "7.1.0"},
		},
	}, map[analysis.PackageVersion]time.Time{
		{Name: "react-router-dom", Version: "7.1.0"}: exampleNow.Add(-72 * time.Hour),
	})
	webTarget := analysis.TargetResult(web, webResult)
	webTarget.ChangedDependencies[0].ImportedBy = []string{"apps/web/src/App.tsx", "apps/web/src/routes.tsx", "apps/web/src/pages/Settings.tsx"}

	uiResult := analysis.Analyze(analysis.Input{
		Now:                       exampleNow,
		Target:                    ui,
		DependencyReviewAvailable: true,
		BaseManifest:              mustManifest(t, `{"name":"ui","dependencies":{}}`),
		HeadManifest:              mustManifest(t, `{"name":"ui","dependencies":{"tailwind-merge":"^2.3.0"}}`),
		ReviewChanges: []analysis.ReviewChange{
			{Name: "tailwind-merge", Manifest: "package-lock.json", ChangeType: analysis.ChangeAdded, Version: "2.3.0"},
		},
	}, nil)

	return Report{
		Repo: "owner/repo",
		PR: PullRequestMetadata{
			Number:      456,
			URL:         "https://github.com/owner/repo/pull/456",
			Title:       "Update workspace dependencies",
			BaseSHA:     "base-workspace",
			HeadSHA:     "head-workspace",
			AuthorLogin: "octocat",
		},
		Analysis: analysis.AggregateResults([]analysis.TargetAnalysisResult{webTarget, analysis.TargetResult(ui, uiResult)}),
	}
}
