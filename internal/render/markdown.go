package render

import (
	"fmt"
	"strings"

	"github.com/rad1092/gh-dependency-risk/internal/analysis"
)

func renderMarkdown(report Report, lang string) string {
	var b strings.Builder
	tr := translator(lang)

	b.WriteString("<!-- gh-dep-risk -->\n")
	b.WriteString("## gh-dep-risk\n")
	fmt.Fprintf(&b, "- %s: `%s`\n", tr("repo"), report.Repo)
	fmt.Fprintf(&b, "- %s: [#%d](%s) %s\n", tr("pr"), report.PR.Number, report.PR.URL, report.PR.Title)
	fmt.Fprintf(&b, "- %s: `%d` (`%s`)", tr("score"), report.Analysis.Score, report.Analysis.Level)
	if source := scoreSource(report, lang); source != "" {
		fmt.Fprintf(&b, ", %s", source)
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "- %s: `%s`\n", tr("blast_radius"), report.Analysis.BlastRadius)
	fmt.Fprintf(&b, "- %s: `%t`\n", tr("dependency_review"), report.Analysis.DependencyReviewAvailable)
	if why := whyRiskyLine(report, lang); why != "" {
		fmt.Fprintf(&b, "- %s: %s\n", tr("why_risky"), why)
	}

	if checks := checkFirstLines(report, lang); len(checks) > 0 {
		b.WriteString("\n### ")
		b.WriteString(tr("check_first"))
		b.WriteString("\n")
		for i, line := range checks {
			fmt.Fprintf(&b, "%d. %s\n", i+1, line)
		}
	}

	b.WriteString("\n### ")
	b.WriteString(tr("summary"))
	b.WriteString("\n")
	for _, line := range summaryBullets(report, lang) {
		fmt.Fprintf(&b, "- %s\n", line)
	}

	if len(report.Analysis.Notes) > 0 {
		b.WriteString("\n### ")
		b.WriteString(tr("notes"))
		b.WriteString("\n")
		for _, note := range report.Analysis.Notes {
			fmt.Fprintf(&b, "- %s\n", localizeNoteMessage(note, lang))
		}
	}

	if len(report.Analysis.Targets) > 0 {
		b.WriteString("\n### ")
		b.WriteString(targetSectionTitle(lang))
		b.WriteString("\n")
		for _, target := range report.Analysis.Targets {
			fmt.Fprintf(&b, "- `%s` (%s, score `%d`, level `%s`, blast `%s`)\n", displayTarget(target.Target), targetContext(target.Target), target.Score, target.Level, target.BlastRadius)
			for _, change := range target.ChangedDependencies {
				writeMarkdownChange(&b, "  ", change, lang)
			}
			for _, note := range target.Notes {
				fmt.Fprintf(&b, "  - %s\n", localizeNoteMessage(note, lang))
			}
		}
	} else if len(report.Analysis.ChangedDependencies) > 0 {
		b.WriteString("\n### ")
		b.WriteString(tr("what_changed"))
		b.WriteString("\n")
		for _, change := range report.Analysis.ChangedDependencies {
			writeMarkdownChange(&b, "", change, lang)
		}
	}

	if len(report.Analysis.RecommendedActions) > 0 {
		b.WriteString("\n### ")
		b.WriteString(tr("recommended_actions"))
		b.WriteString("\n")
		for _, action := range recommendedActionLines(report, lang) {
			fmt.Fprintf(&b, "- %s\n", action)
		}
	}

	if len(report.Analysis.QuickCommands) > 0 {
		b.WriteString("\n### ")
		b.WriteString(tr("quick_commands"))
		b.WriteString("\n")
		for _, command := range report.Analysis.QuickCommands {
			fmt.Fprintf(&b, "- `%s`\n", command)
		}
	}

	return b.String() + "\n"
}

// writeMarkdownChange prints one change and each signal with its points.
func writeMarkdownChange(b *strings.Builder, indent string, change analysis.DependencyChange, lang string) {
	fmt.Fprintf(b, "%s- `%s` (%s/%s, score `%d`)\n", indent, displayChange(change), change.ChangeType, change.Scope, change.Score)
	sum := 0
	if len(change.ScoreBreakdown) > 0 {
		for _, part := range change.ScoreBreakdown {
			text := localizeDrivers([]string{part.Signal}, lang)[0]
			fmt.Fprintf(b, "%s  - `+%d` %s\n", indent, part.Points, text)
			sum += part.Points
		}
	} else {
		for _, driver := range localizeDrivers(change.RiskDrivers, lang) {
			fmt.Fprintf(b, "%s  - %s\n", indent, driver)
		}
	}
	if sum > change.Score {
		if lang != "en" {
			fmt.Fprintf(b, "%s  - 합계 %d, 최대 %d점\n", indent, sum, change.Score)
		} else {
			fmt.Fprintf(b, "%s  - Total %d, capped at %d\n", indent, sum, change.Score)
		}
	}
	if files := fileList(change.ImportedBy, lang); files != "" {
		fmt.Fprintf(b, "%s  - %s: %s\n", indent, translator(lang)("imported_by"), files)
	}
}
