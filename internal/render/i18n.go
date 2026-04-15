package render

import (
	"fmt"

	"gh-dep-risk/internal/analysis"
)

func translator(lang string) func(string) string {
	if lang != "en" {
		return func(key string) string {
			switch key {
			case "repo":
				return "저장소"
			case "pr":
				return "PR"
			case "score":
				return "점수"
			case "blast_radius":
				return "영향 범위"
			case "dependency_review":
				return "Dependency Review 사용 가능"
			case "changed_deps":
				return "변경된 의존성 수"
			case "notes":
				return "참고"
			case "what_changed":
				return "무엇이 바뀌었나"
			case "why_risky":
				return "위험 신호"
			case "recommended_actions":
				return "권장 조치"
			case "quick_commands":
				return "빠른 확인 명령"
			default:
				return key
			}
		}
	}
	return func(key string) string {
		switch key {
		case "repo":
			return "Repository"
		case "pr":
			return "PR"
		case "score":
			return "Score"
		case "blast_radius":
			return "Blast radius"
		case "dependency_review":
			return "Dependency review available"
		case "changed_deps":
			return "Changed dependencies"
		case "notes":
			return "Notes"
		case "what_changed":
			return "What changed"
		case "why_risky":
			return "Why risky"
		case "recommended_actions":
			return "Recommended action"
		case "quick_commands":
			return "Quick commands"
		default:
			return key
		}
	}
}

func localizeDrivers(drivers []string, lang string) []string {
	items := make([]string, 0, len(drivers))
	for _, driver := range drivers {
		if lang == "en" {
			switch driver {
			case analysis.DriverKnownVulnerabilities:
				items = append(items, "Known vulnerabilities were reported for the target version.")
			case analysis.DriverAddedDirectRuntime:
				items = append(items, "A new direct runtime dependency was added.")
			case analysis.DriverAddedDirectDev:
				items = append(items, "A new direct dev dependency was added.")
			case analysis.DriverMajorVersionBump:
				items = append(items, "The dependency crosses a major version boundary.")
			case analysis.DriverRecentlyPublished:
				items = append(items, "The target version was published within the last 7 days.")
			case analysis.DriverInstallScript:
				items = append(items, "The package declares an install script.")
			case analysis.DriverPlatformRestricted:
				items = append(items, "The package is restricted to specific OS/CPU targets.")
			case analysis.DriverTransitiveFive:
				items = append(items, "The PR adds at least 5 new transitive packages.")
			case analysis.DriverTransitiveFifteen:
				items = append(items, "The PR adds at least 15 new transitive packages.")
			}
			continue
		}

		switch driver {
		case analysis.DriverKnownVulnerabilities:
			items = append(items, "대상 버전에 알려진 취약점이 있습니다.")
		case analysis.DriverAddedDirectRuntime:
			items = append(items, "직접 런타임 의존성이 새로 추가되었습니다.")
		case analysis.DriverAddedDirectDev:
			items = append(items, "직접 개발 의존성이 새로 추가되었습니다.")
		case analysis.DriverMajorVersionBump:
			items = append(items, "메이저 버전 경계가 변경되었습니다.")
		case analysis.DriverRecentlyPublished:
			items = append(items, "대상 버전이 최근 7일 이내에 배포되었습니다.")
		case analysis.DriverInstallScript:
			items = append(items, "설치 스크립트가 감지되었습니다.")
		case analysis.DriverPlatformRestricted:
			items = append(items, "특정 OS/CPU 제약이 있는 패키지입니다.")
		case analysis.DriverTransitiveFive:
			items = append(items, "새로운 전이 의존성이 5개 이상 추가되었습니다.")
		case analysis.DriverTransitiveFifteen:
			items = append(items, "새로운 전이 의존성이 15개 이상 추가되었습니다.")
		}
	}
	return items
}

func localizeAction(action, lang string) string {
	if lang == "en" {
		switch action {
		case analysis.ActionInspectInstall:
			return "Inspect install scripts and package tarballs before merging."
		case analysis.ActionInspectTree:
			return "Inspect the dependency tree and lockfile diff."
		case analysis.ActionReviewAdvisories:
			return "Review GHSA advisories before merging."
		case analysis.ActionReviewChangelog:
			return "Read upstream release notes and migration guidance."
		case analysis.ActionRunTargetedTests:
			return "Run targeted tests and smoke checks for affected paths."
		case analysis.ActionValidateSources:
			return "Validate non-registry or git sources explicitly."
		default:
			return action
		}
	}
	switch action {
	case analysis.ActionInspectInstall:
		return "병합 전에 설치 스크립트와 패키지 tarball을 확인하세요."
	case analysis.ActionInspectTree:
		return "의존성 트리와 lockfile diff를 확인하세요."
	case analysis.ActionReviewAdvisories:
		return "병합 전에 GHSA advisory를 검토하세요."
	case analysis.ActionReviewChangelog:
		return "업스트림 릴리즈 노트와 마이그레이션 가이드를 읽으세요."
	case analysis.ActionRunTargetedTests:
		return "영향 경로에 대한 타깃 테스트와 스모크 테스트를 실행하세요."
	case analysis.ActionValidateSources:
		return "레지스트리 외부 또는 git 소스를 별도로 검증하세요."
	default:
		return action
	}
}

func localizeNote(note analysis.Note, lang string) string {
	if lang == "en" {
		switch note.Code {
		case analysis.NoteDependencyReviewFallback:
			return "Dependency review API was unavailable, so lockfile-only fallback analysis was used."
		case analysis.NoteNonRegistrySource:
			return fmt.Sprintf("%s resolves from a non-registry source: %s", note.Dependency, note.Detail)
		default:
			return note.Code
		}
	}
	switch note.Code {
	case analysis.NoteDependencyReviewFallback:
		return "Dependency Review API를 사용할 수 없어 lockfile 기반 fallback 분석을 사용했습니다."
	case analysis.NoteNonRegistrySource:
		return fmt.Sprintf("%s 패키지가 레지스트리 외부 소스로 해석됩니다: %s", note.Dependency, note.Detail)
	default:
		return note.Code
	}
}
