package npm

import "testing"

func TestParsePNPMWorkspacePatterns(t *testing.T) {
	patterns, err := ParsePNPMWorkspacePatterns([]byte("packages:\n  - apps/*\n  - packages/*\n  - '!**/test/**'\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) != 3 {
		t.Fatalf("expected 3 patterns, got %#v", patterns)
	}
	if !MatchWorkspacePatternSet(patterns, "apps/web") {
		t.Fatalf("expected apps/web to match pnpm workspace patterns")
	}
	if !MatchWorkspacePatternSet(patterns, "packages/ui") {
		t.Fatalf("expected packages/ui to match pnpm workspace patterns")
	}
	if MatchWorkspacePatternSet(patterns, "packages/test/helpers") {
		t.Fatalf("expected excluded path to be filtered out")
	}
}
