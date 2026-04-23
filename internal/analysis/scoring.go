package analysis

import (
	"sort"
	"time"
)

const (
	scoreKnownVulnerabilities = 35
	scoreAddedDirectRuntime   = 12
	scoreAddedDirectDev       = 6
	scoreMajorVersionBump     = 10
	scoreRecentlyPublished    = 18
	scoreInstallScript        = 20
	scorePlatformRestricted   = 6
	scoreTransitiveFive       = 12
	scoreTransitiveFifteen    = 8

	recentPublishWindow        = 7 * 24 * time.Hour
	transitiveCountThreshold5  = 5
	transitiveCountThreshold15 = 15
	changeScoreCap             = 100

	levelThresholdMedium   = 20
	levelThresholdHigh     = 40
	levelThresholdCritical = 70

	// Aggregate scoring keeps the model explainable: the riskiest single
	// dependency change sets the baseline, and additional risky changes add a
	// small capped bonus so multi-target PRs can rise without drowning out the
	// strongest single signal.
	aggregateHighRiskChangeThreshold = levelThresholdMedium
	aggregateHighRiskBonus           = 4
	aggregateAnyRiskBonus            = 1
	aggregateBonusCap                = 15
)

func scoreChange(input Input, candidate candidateSummary, views targetLockViews, publishedAt map[PackageVersion]time.Time) (int, []string) {
	score := 0
	drivers := make([]string, 0, 4)
	add := func(condition bool, points int, driver string) {
		if !condition {
			return
		}
		score += points
		drivers = append(drivers, driver)
	}

	add(len(candidate.Vulnerabilities) > 0, scoreKnownVulnerabilities, DriverKnownVulnerabilities)
	add(candidate.ChangeType == ChangeAdded && candidate.Direct && (candidate.Scope == ScopeRuntime || candidate.Scope == ScopeOptional), scoreAddedDirectRuntime, DriverAddedDirectRuntime)
	add(candidate.ChangeType == ChangeAdded && candidate.Direct && candidate.Scope == ScopeDev, scoreAddedDirectDev, DriverAddedDirectDev)
	add(isMajorBump(candidate.FromVersion, candidate.ToVersion, candidate.FromRequirement, candidate.ToRequirement), scoreMajorVersionBump, DriverMajorVersionBump)

	if published, ok := publishedAt[PackageVersion{Name: candidate.Name, Version: candidate.ToVersion}]; ok && !published.IsZero() {
		add(input.Now.Sub(published) <= recentPublishWindow, scoreRecentlyPublished, DriverRecentlyPublished)
	}

	add(candidate.HeadPackage.HasInstallScript, scoreInstallScript, DriverInstallScript)
	add(len(candidate.HeadPackage.OS) > 0 || len(candidate.HeadPackage.CPU) > 0, scorePlatformRestricted, DriverPlatformRestricted)
	add(views.AddedTransitive >= transitiveCountThreshold5, scoreTransitiveFive, DriverTransitiveFive)
	add(views.AddedTransitive >= transitiveCountThreshold15, scoreTransitiveFifteen, DriverTransitiveFifteen)

	if score > changeScoreCap {
		score = changeScoreCap
	}
	return score, uniqueStrings(drivers)
}

func aggregateScore(changes []DependencyChange) int {
	if len(changes) == 0 {
		return 0
	}
	scores := make([]int, 0, len(changes))
	for _, change := range changes {
		scores = append(scores, change.Score)
	}
	return aggregateTargetScore(scores)
}

func aggregateTargetScore(scores []int) int {
	if len(scores) == 0 {
		return 0
	}
	sorted := append([]int(nil), scores...)
	sort.Sort(sort.Reverse(sort.IntSlice(sorted)))
	maxScore := sorted[0]
	bonus := 0
	for _, score := range sorted[1:] {
		switch {
		case score >= aggregateHighRiskChangeThreshold:
			bonus += aggregateHighRiskBonus
		case score > 0:
			bonus += aggregateAnyRiskBonus
		}
		if bonus >= aggregateBonusCap {
			bonus = aggregateBonusCap
			break
		}
	}
	total := maxScore + bonus
	if total > changeScoreCap {
		return changeScoreCap
	}
	return total
}
