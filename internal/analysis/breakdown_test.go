package analysis

import (
	"testing"
	"time"

	"github.com/rad1092/gh-dependency-risk/internal/npm"
)

func TestScoreBreakdownAddsUpToTheScore(t *testing.T) {
	now := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	input := rootInput(
		now,
		manifest(runtimeDeps("left-pad", "^1.0.0"), nil, nil),
		manifest(runtimeDeps("left-pad", "^2.0.0"), nil, nil),
		lockfile(pkg("node_modules/left-pad", "left-pad", "1.0.0", nil)),
		lockfile(pkg("node_modules/left-pad", "left-pad", "2.0.0", nil, withInstallScript())),
		nil,
	)
	result := Analyze(input, map[PackageVersion]time.Time{{Name: "left-pad", Version: "2.0.0"}: now.Add(-48 * time.Hour)})
	if len(result.ChangedDependencies) != 1 {
		t.Fatalf("changes: %+v", result.ChangedDependencies)
	}
	change := result.ChangedDependencies[0]
	want := []ScorePart{
		{Signal: DriverMajorVersionBump, Points: scoreMajorVersionBump},
		{Signal: DriverRecentlyPublished, Points: scoreRecentlyPublished},
		{Signal: DriverInstallScript, Points: scoreInstallScript},
	}
	if len(change.ScoreBreakdown) != len(want) {
		t.Fatalf("breakdown = %+v", change.ScoreBreakdown)
	}
	sum := 0
	for i, part := range change.ScoreBreakdown {
		if part != want[i] {
			t.Fatalf("part %d = %+v, want %+v", i, part, want[i])
		}
		if change.RiskDrivers[i] != part.Signal {
			t.Fatalf("drivers %v do not follow the breakdown", change.RiskDrivers)
		}
		sum += part.Points
	}
	if sum != change.Score || change.Score != 48 {
		t.Fatalf("score %d, breakdown sums to %d", change.Score, sum)
	}
}

func TestScoreBreakdownKeepsEveryPartWhenTheScoreIsCapped(t *testing.T) {
	now := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	head := append([]npm.LockPackage{}, transitiveExpansion("heavy", 16)...)
	head[0].HasInstallScript = true
	head[0].OS = []string{"linux"}
	input := rootInput(
		now,
		manifest(nil, nil, nil),
		manifest(runtimeDeps("heavy", "^1.0.0"), nil, nil),
		lockfile(),
		lockfile(head...),
		[]ReviewChange{{Name: "heavy", ChangeType: ChangeAdded, Version: "1.0.0", Vulnerabilities: []Vulnerability{{GHSAID: "GHSA-x"}}}},
	)
	result := Analyze(input, map[PackageVersion]time.Time{{Name: "heavy", Version: "1.0.0"}: now})
	change := result.ChangedDependencies[0]
	sum := 0
	for _, part := range change.ScoreBreakdown {
		sum += part.Points
	}
	if change.Score != 100 || sum <= 100 {
		t.Fatalf("score %d, sum %d", change.Score, sum)
	}
}

func TestNameOneEditFromAPopularPackageIsFlagged(t *testing.T) {
	now := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		added   string
		flagged bool
	}{
		{"swapped letters", "lodahs", true},
		{"missing hyphen", "crossenv", true},
		{"extra letter", "expresss", true},
		{"the popular package itself", "lodash", false},
		{"a popular neighbour", "mysql2", false},
		{"scoped", "@acme/lodahs", false},
		{"short names are skipped", "chal", false},
		{"two edits away", "lodaahs", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := rootInput(
				now,
				manifest(nil, nil, nil),
				manifest(runtimeDeps(tc.added, "^1.0.0"), nil, nil),
				lockfile(),
				lockfile(pkg("node_modules/"+tc.added, tc.added, "1.0.0", nil)),
				nil,
			)
			change := Analyze(input, nil).ChangedDependencies[0]
			if got := contains(change.RiskDrivers, DriverNameSimilarToPopular); got != tc.flagged {
				t.Fatalf("%s flagged=%t, drivers %v", tc.added, got, change.RiskDrivers)
			}
		})
	}
}

func TestUpdatingAnExistingLookalikeIsNotFlagged(t *testing.T) {
	now := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	input := rootInput(
		now,
		manifest(runtimeDeps("lodahs", "^1.0.0"), nil, nil),
		manifest(runtimeDeps("lodahs", "^1.1.0"), nil, nil),
		lockfile(pkg("node_modules/lodahs", "lodahs", "1.0.0", nil)),
		lockfile(pkg("node_modules/lodahs", "lodahs", "1.1.0", nil)),
		nil,
	)
	if drivers := Analyze(input, nil).ChangedDependencies[0].RiskDrivers; contains(drivers, DriverNameSimilarToPopular) {
		t.Fatalf("drivers %v", drivers)
	}
}

func TestRegistryFactsSignals(t *testing.T) {
	now := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
	update := rootInput(
		now,
		manifest(runtimeDeps("widget", "^1.0.0"), nil, nil),
		manifest(runtimeDeps("widget", "^1.0.0"), nil, nil),
		lockfile(pkg("node_modules/widget", "widget", "1.0.0", nil)),
		lockfile(pkg("node_modules/widget", "widget", "1.0.1", nil)),
		nil,
	)
	update.Facts = map[PackageVersion]npm.VersionFacts{
		{Name: "widget", Version: "1.0.1"}: {
			PackageCreated:     now.AddDate(-3, 0, 0),
			Publisher:          "mallory",
			HasPrevious:        true,
			PreviousPublisher:  "alice",
			PreviousProvenance: true,
		},
	}
	lookups := CollectRegistryLookups(update)
	if len(lookups) != 1 || lookups[0] != (RegistryLookup{Name: "widget", Version: "1.0.1", PreviousVersion: "1.0.0"}) {
		t.Fatalf("lookups = %+v", lookups)
	}
	change := Analyze(update, nil).ChangedDependencies[0]
	assertStringSetEqual(t, change.RiskDrivers, []string{DriverProvenanceDropped, DriverPublisherChanged})
	if change.Score != scoreProvenanceDropped+scorePublisherChanged {
		t.Fatalf("score = %d", change.Score)
	}

	added := rootInput(
		now,
		manifest(nil, nil, nil),
		manifest(runtimeDeps("fresh-thing", "^0.1.0"), nil, nil),
		lockfile(),
		lockfile(pkg("node_modules/fresh-thing", "fresh-thing", "0.1.0", nil)),
		nil,
	)
	added.Facts = map[PackageVersion]npm.VersionFacts{
		{Name: "fresh-thing", Version: "0.1.0"}: {PackageCreated: now.AddDate(0, 0, -3), Publisher: "someone"},
	}
	change = Analyze(added, nil).ChangedDependencies[0]
	assertStringSetEqual(t, change.RiskDrivers, []string{DriverAddedDirectRuntime, DriverNewPackage})
}

func TestWithinOneEdit(t *testing.T) {
	cases := map[[2]string]bool{
		{"react", "raect"}:     true,
		{"react", "reacts"}:    true,
		{"react", "reat"}:      true,
		{"react", "rexct"}:     true,
		{"react", "react"}:     false,
		{"react", "raecx"}:     false,
		{"react", "tcaer"}:     false,
		{"lodash", "lodashxx"}: false,
	}
	for pair, want := range cases {
		if got := withinOneEdit(pair[0], pair[1]); got != want {
			t.Errorf("withinOneEdit(%q, %q) = %t", pair[0], pair[1], got)
		}
	}
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
