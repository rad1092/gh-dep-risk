package render

import (
	"fmt"
	"strings"

	"github.com/rad1092/gh-dependency-risk/internal/analysis"
	"github.com/rad1092/gh-dependency-risk/internal/npm"
)

const checkFirstLimit = 3

// checkPriority orders the signals that turn into a concrete check.
var checkPriority = []string{
	analysis.DriverKnownVulnerabilities,
	analysis.DriverNameSimilarToPopular,
	analysis.DriverProvenanceDropped,
	analysis.DriverInstallScript,
	analysis.DriverNewPackage,
	analysis.DriverPublisherChanged,
	analysis.DriverRecentlyPublished,
	analysis.DriverMajorVersionBump,
	analysis.DriverTransitiveFifteen,
	analysis.DriverTransitiveFive,
	analysis.DriverAddedDirectRuntime,
	analysis.DriverPlatformRestricted,
	analysis.DriverAddedDirectDev,
}

// checkFirstLines returns up to three concrete things to check. The first
// pass takes the most important check of each change scored medium or
// higher, so one package does not crowd out another risky one. The second
// pass fills the rest from every change, riskiest first.
func checkFirstLines(report Report, lang string) []string {
	lines := make([]string, 0, checkFirstLimit)
	seen := map[string]struct{}{}
	try := func(change analysis.DependencyChange, signal string) bool {
		if !contains(change.RiskDrivers, signal) {
			return false
		}
		key := checkKey(signal, change)
		if _, ok := seen[key]; ok {
			return false
		}
		line := checkLine(signal, change, npmTarget(report, change), lang)
		if line == "" {
			return false
		}
		seen[key] = struct{}{}
		lines = append(lines, line)
		return true
	}
	changes := report.Analysis.ChangedDependencies
	for _, change := range changes {
		if analysis.LevelForScore(change.Score) == analysis.RiskLevelLow {
			continue
		}
		for _, signal := range checkPriority {
			if len(lines) >= checkFirstLimit {
				return lines
			}
			if try(change, signal) {
				break
			}
		}
	}
	for _, change := range changes {
		if change.Score == 0 {
			continue
		}
		for _, signal := range checkPriority {
			if len(lines) >= checkFirstLimit {
				return lines
			}
			try(change, signal)
		}
	}
	return lines
}

func checkKey(signal string, change analysis.DependencyChange) string {
	switch signal {
	case analysis.DriverTransitiveFive, analysis.DriverTransitiveFifteen:
		return "transitive|" + change.Target
	}
	return signal + "|" + change.Name
}

func npmTarget(report Report, change analysis.DependencyChange) bool {
	for _, target := range report.Analysis.Targets {
		if displayTarget(target.Target) == change.Target || target.Target.DisplayName == change.Target {
			switch target.Target.PackageManager {
			case "npm", "pnpm", "yarn", "bun":
				return true
			}
			return target.Target.Ecosystem == "npm"
		}
	}
	return strings.HasSuffix(change.Manifest, "package.json") || strings.HasSuffix(change.Manifest, "package-lock.json")
}

func checkLine(signal string, change analysis.DependencyChange, npmFamily bool, lang string) string {
	name := change.Name
	to := change.ToVersion
	if to == "" {
		to = change.ToRequirement
	}
	at := name
	if to != "" {
		at = name + "@" + npm.StripVersionPrefix(to)
	}
	from := change.FromVersion
	if from == "" {
		from = change.FromRequirement
	}
	command := func(format string, args ...any) string {
		if !npmFamily {
			return ""
		}
		return " `" + fmt.Sprintf(format, args...) + "`"
	}
	ko := lang != "en"

	switch signal {
	case analysis.DriverKnownVulnerabilities:
		ids := advisoryIDs(change.Vulnerabilities)
		if ko {
			return fmt.Sprintf("%s의 advisory%s를 읽고, 고쳐진 버전이 있으면 그 버전으로 올리세요.", at, parenthesized(ids))
		}
		return fmt.Sprintf("Read the advisories for %s%s and move to a fixed version if there is one.", at, parenthesized(ids))
	case analysis.DriverNameSimilarToPopular:
		popular := analysis.SimilarPopularPackage(name)
		if ko {
			return fmt.Sprintf("%s는 인기 패키지 %s와 이름이 한 글자만 다릅니다. 의도한 패키지가 맞는지 확인하세요:%s", name, popular, command("npm view %s repository.url", name))
		}
		return fmt.Sprintf("%s is one edit away from the popular package %s. Check that it is the package you meant:%s", name, popular, command("npm view %s repository.url", name))
	case analysis.DriverProvenanceDropped:
		if ko {
			return fmt.Sprintf("%s %s에는 provenance 증명이 있었는데 %s에는 없습니다. 프로젝트의 릴리스 워크플로에서 나온 버전인지 확인하세요.", name, from, to)
		}
		return fmt.Sprintf("%s %s had a provenance attestation and %s does not. Check that %s came from the project's release workflow.", name, from, to, to)
	case analysis.DriverInstallScript:
		if ko {
			return fmt.Sprintf("%s는 설치할 때 스크립트를 실행합니다. 설치 전에 읽어 보세요:%s", at, command("npm view %s scripts", at))
		}
		return fmt.Sprintf("%s runs a script when it is installed. Read it before installing:%s", at, command("npm view %s scripts", at))
	case analysis.DriverNewPackage:
		if ko {
			return fmt.Sprintf("%s는 처음 배포된 지 30일이 안 됐습니다. 누가 관리하는지 확인하세요:%s", name, command("npm view %s maintainers repository.url", name))
		}
		return fmt.Sprintf("%s was first published less than 30 days ago. Check who maintains it:%s", name, command("npm view %s maintainers repository.url", name))
	case analysis.DriverPublisherChanged:
		if ko {
			return fmt.Sprintf("%s %s는 %s와 다른 npm 계정이 배포했습니다. 관리자 목록에 있는 계정인지 확인하세요:%s", name, to, from, command("npm view %s maintainers", name))
		}
		return fmt.Sprintf("%s %s was published by a different npm account than %s. Check that it is one of the maintainers:%s", name, to, from, command("npm view %s maintainers", name))
	case analysis.DriverRecentlyPublished:
		if ko {
			return fmt.Sprintf("%s는 최근 7일 안에 배포됐습니다. 급하지 않으면 며칠 기다렸다가 병합하세요.", at)
		}
		return fmt.Sprintf("%s was published in the last 7 days. If the update isn't urgent, wait a few days before merging.", at)
	case analysis.DriverMajorVersionBump:
		files := fileList(change.ImportedBy, lang)
		if ko {
			line := fmt.Sprintf("%s가 %s에서 %s로 메이저 버전이 바뀝니다. 릴리스 노트에서 호환되지 않는 변경을 확인하세요.", name, from, to)
			if files != "" {
				line += fmt.Sprintf(" 이 패키지를 쓰는 파일: %s.", files)
			}
			return line
		}
		line := fmt.Sprintf("%s moves from %s to %s. Read the release notes for breaking changes.", name, from, to)
		if files != "" {
			line += fmt.Sprintf(" It is used in %s.", files)
		}
		return line
	case analysis.DriverTransitiveFive, analysis.DriverTransitiveFifteen:
		if ko {
			return fmt.Sprintf("%s의 lockfile diff를 확인하세요. 직접 추가하지 않은 패키지 %d개가 새로 들어옵니다.", targetLabel(change), change.AddedTransitiveCount)
		}
		return fmt.Sprintf("Read the lockfile diff for %s: it adds %d packages you did not add directly.", targetLabel(change), change.AddedTransitiveCount)
	case analysis.DriverAddedDirectRuntime, analysis.DriverAddedDirectDev:
		files := fileList(change.ImportedBy, lang)
		if ko {
			line := fmt.Sprintf("%s가 새 의존성으로 추가됩니다. 의도한 패키지가 맞는지 확인하세요:%s", name, command("npm view %s repository.url", name))
			if files != "" {
				line += fmt.Sprintf(" 쓰는 파일: %s.", files)
			}
			return line
		}
		line := fmt.Sprintf("%s is a new dependency. Check that it is the package you meant:%s", name, command("npm view %s repository.url", name))
		if files != "" {
			line += fmt.Sprintf(" Used in %s.", files)
		}
		return line
	case analysis.DriverPlatformRestricted:
		if ko {
			return fmt.Sprintf("%s는 일부 OS/CPU에서만 설치됩니다. CI와 배포 환경이 포함되는지 확인하세요.", at)
		}
		return fmt.Sprintf("%s installs only on some OS/CPU targets. Check that CI and production machines are among them.", at)
	}
	return ""
}

func targetLabel(change analysis.DependencyChange) string {
	if change.Target == "" {
		return "root"
	}
	return change.Target
}

func advisoryIDs(vulnerabilities []analysis.Vulnerability) []string {
	ids := make([]string, 0, 2)
	for _, vulnerability := range vulnerabilities {
		if vulnerability.GHSAID == "" || contains(ids, vulnerability.GHSAID) {
			continue
		}
		ids = append(ids, vulnerability.GHSAID)
		if len(ids) == 2 {
			break
		}
	}
	return ids
}

func parenthesized(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return " (" + strings.Join(values, ", ") + ")"
}

// fileList names up to two files and counts the rest.
func fileList(files []string, lang string) string {
	if len(files) == 0 {
		return ""
	}
	shown := files
	if len(shown) > 2 {
		shown = shown[:2]
	}
	quoted := make([]string, 0, len(shown))
	for _, file := range shown {
		quoted = append(quoted, "`"+file+"`")
	}
	rest := len(files) - len(shown)
	if lang != "en" {
		text := strings.Join(quoted, ", ")
		if rest > 0 {
			text += fmt.Sprintf(" 외 %d개", rest)
		}
		return text
	}
	if rest > 0 {
		return fmt.Sprintf("%s and %d more", strings.Join(quoted, ", "), rest)
	}
	return joinList(quoted)
}

// breakdownText spells out a change's score, for example
// "10 major version bump + 20 install script".
func breakdownText(change analysis.DependencyChange, lang string) string {
	if len(change.ScoreBreakdown) == 0 {
		return ""
	}
	parts := make([]string, 0, len(change.ScoreBreakdown))
	sum := 0
	for _, part := range change.ScoreBreakdown {
		parts = append(parts, fmt.Sprintf("%d %s", part.Points, localizeDriverLabel(part.Signal, lang)))
		sum += part.Points
	}
	text := strings.Join(parts, " + ")
	if sum > change.Score {
		if lang != "en" {
			text += fmt.Sprintf(" = %d, 최대 %d", sum, change.Score)
		} else {
			text += fmt.Sprintf(" = %d, capped at %d", sum, change.Score)
		}
	}
	return text
}

// scoreSource says where the overall score comes from: the riskiest change,
// plus a small bonus for the other risky changes.
func scoreSource(report Report, lang string) string {
	change, ok := strongestChange(report.Analysis)
	if !ok || change.Score == 0 || report.Analysis.Score == 0 {
		return ""
	}
	subject := riskSubject(change, len(report.Analysis.Targets) > 1)
	bonus := report.Analysis.Score - change.Score
	if lang != "en" {
		if bonus > 0 {
			return fmt.Sprintf("%s %d + 나머지 변경 %d", subject, change.Score, bonus)
		}
		return subject + " 기준"
	}
	if bonus > 0 {
		return fmt.Sprintf("%d from %s + %d for the other changes", change.Score, subject, bonus)
	}
	return "from " + subject
}

func scoreHeadline(report Report, lang string) string {
	headline := fmt.Sprintf("%d (%s)", report.Analysis.Score, report.Analysis.Level)
	if source := scoreSource(report, lang); source != "" {
		headline += ", " + source
	}
	return headline
}
