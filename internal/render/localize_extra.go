package render

import (
	"fmt"

	"gh-dep-risk/internal/analysis"
)

func targetSectionTitle(lang string) string {
	if lang == "en" {
		return "Targets"
	}
	return "타깃별 결과"
}

func localizeSummaryTargets(count int, lang string) string {
	if lang == "en" {
		return fmt.Sprintf("%d npm targets were analyzed.", count)
	}
	return fmt.Sprintf("npm 타깃 %d개를 분석했습니다.", count)
}

func displayTarget(target analysis.AnalysisTarget) string {
	if target.DisplayName != "" {
		return target.DisplayName
	}
	if dir := target.Directory(); dir != "" {
		return dir
	}
	return "root"
}

func localizeNoteMessage(note analysis.Note, lang string) string {
	if note.Code == analysis.NoteApproximateAttribution {
		if lang == "en" {
			return fmt.Sprintf("Target attribution is approximate for %s because shared lockfile changes could not be mapped exactly.", note.Detail)
		}
		return fmt.Sprintf("%s 타깃은 공유 lockfile 변경을 기준으로 근사 분석했습니다.", note.Detail)
	}
	return localizeNote(note, lang)
}
