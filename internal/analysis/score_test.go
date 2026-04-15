package analysis

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gh-dep-risk/internal/npm"
)

func TestAnalyzeScoresAndCaps(t *testing.T) {
	baseManifestData, _ := os.ReadFile(filepath.Join("..", "..", "testdata", "base.package.json"))
	headManifestData, _ := os.ReadFile(filepath.Join("..", "..", "testdata", "head.package.json"))
	baseLockData, _ := os.ReadFile(filepath.Join("..", "..", "testdata", "base.package-lock.json"))
	headLockData, _ := os.ReadFile(filepath.Join("..", "..", "testdata", "head.package-lock.json"))

	baseManifest, _ := npm.ParsePackageManifest(baseManifestData)
	headManifest, _ := npm.ParsePackageManifest(headManifestData)
	baseLockfile, _ := npm.ParseLockfile(baseLockData)
	headLockfile, _ := npm.ParseLockfile(headLockData)

	now := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	result := Analyze(Input{
		Now:                       now,
		DependencyReviewAvailable: true,
		ReviewChanges: []ReviewChange{
			{Name: "left-pad", Manifest: "package.json", ChangeType: ChangeRemoved, Version: "1.0.0"},
			{Name: "left-pad", Manifest: "package.json", ChangeType: ChangeAdded, Version: "2.0.0", Vulnerabilities: []Vulnerability{{GHSAID: "GHSA-1", Severity: "high", Summary: "demo", URL: "https://example.com"}}},
			{Name: "chalk", Manifest: "package.json", ChangeType: ChangeAdded, Version: "5.0.0"},
		},
		BaseManifest: baseManifest,
		HeadManifest: headManifest,
		BaseLockfile: baseLockfile,
		HeadLockfile: headLockfile,
	}, map[PackageVersion]time.Time{
		{Name: "left-pad", Version: "2.0.0"}: now.Add(-48 * time.Hour),
		{Name: "chalk", Version: "5.0.0"}:    now.Add(-24 * time.Hour),
	})

	if result.Score < 70 {
		t.Fatalf("expected critical score, got %d", result.Score)
	}
	if result.Level != RiskLevelCritical {
		t.Fatalf("expected critical level, got %s", result.Level)
	}
	if len(result.ChangedDependencies) != 2 {
		t.Fatalf("expected 2 dependency changes, got %d", len(result.ChangedDependencies))
	}
	if result.ChangedDependencies[0].Score != 100 {
		t.Fatalf("expected score cap at 100, got %d", result.ChangedDependencies[0].Score)
	}
}
