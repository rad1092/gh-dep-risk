package render

import (
	"strings"
	"testing"
)

func TestRenderHumanShowsWhyRiskyAndOperationalActions(t *testing.T) {
	output, err := Render(sampleSingleTargetReport(), "human", "en")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"Why risky: left-pad crosses a major version boundary and declares an install script.",
		"Inspect install scripts and published tarballs for left-pad before merging.",
		"Check release notes and migration guidance for left-pad before merging.",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected human output to contain %q, got %q", expected, output)
		}
	}
}
