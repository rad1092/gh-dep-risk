package render

import (
	"strings"
	"testing"
)

func TestRenderMarkdownShowsWhyRiskyNearTop(t *testing.T) {
	output, err := Render(sampleSingleTargetReport(), "markdown", "en")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"- Why risky: left-pad crosses a major version boundary and declares an install script.",
		"- Check release notes and migration guidance for left-pad before merging.",
		"- Inspect install scripts and published tarballs for left-pad before merging.",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected markdown output to contain %q, got %q", expected, output)
		}
	}
}
