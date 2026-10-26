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
	scoreNameSimilarToPopular = 20
	scoreNewPackage           = 15
	scorePublisherChanged     = 10
	scoreProvenanceDropped    = 20

	recentPublishWindow        = 7 * 24 * time.Hour
	newPackageWindow           = 30 * 24 * time.Hour
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

// scoreSheet collects the signals that apply to one change, in the order they
// are reported.
type scoreSheet struct {
	parts []ScorePart
}

func (s *scoreSheet) add(condition bool, points int, signal string) {
	if !condition {
		return
	}
	for _, part := range s.parts {
		if part.Signal == signal {
			return
		}
	}
	s.parts = append(s.parts, ScorePart{Signal: signal, Points: points})
}

// result returns the capped score, the signal names and the breakdown.
func (s *scoreSheet) result() (int, []string, []ScorePart) {
	score := 0
	drivers := make([]string, 0, len(s.parts))
	for _, part := range s.parts {
		score += part.Points
		drivers = append(drivers, part.Signal)
	}
	if score > changeScoreCap {
		score = changeScoreCap
	}
	parts := append([]ScorePart{}, s.parts...)
	return score, drivers, parts
}

func scoreChange(input Input, candidate candidateSummary, views targetLockViews, publishedAt map[PackageVersion]time.Time) (int, []string, []ScorePart) {
	var sheet scoreSheet
	addedDirect := candidate.ChangeType == ChangeAdded && candidate.Direct
	updated := candidate.ChangeType == ChangeUpdated

	sheet.add(len(candidate.Vulnerabilities) > 0, scoreKnownVulnerabilities, DriverKnownVulnerabilities)
	sheet.add(addedDirect && SimilarPopularPackage(candidate.Name) != "", scoreNameSimilarToPopular, DriverNameSimilarToPopular)
	sheet.add(addedDirect && (candidate.Scope == ScopeRuntime || candidate.Scope == ScopeOptional), scoreAddedDirectRuntime, DriverAddedDirectRuntime)
	sheet.add(addedDirect && candidate.Scope == ScopeDev, scoreAddedDirectDev, DriverAddedDirectDev)
	sheet.add(isMajorBump(candidate.FromVersion, candidate.ToVersion, candidate.FromRequirement, candidate.ToRequirement), scoreMajorVersionBump, DriverMajorVersionBump)

	key := PackageVersion{Name: candidate.Name, Version: candidate.ToVersion}
	if published, ok := publishedAt[key]; ok && !published.IsZero() {
		sheet.add(input.Now.Sub(published) <= recentPublishWindow, scoreRecentlyPublished, DriverRecentlyPublished)
	}
	lookup := RegistryLookup{Name: candidate.Name, Version: candidate.ToVersion}
	if updated {
		lookup.PreviousVersion = candidate.FromVersion
	}
	if facts, ok := input.Facts[lookup]; ok {
		sheet.add(candidate.ChangeType == ChangeAdded && !facts.PackageCreated.IsZero() && input.Now.Sub(facts.PackageCreated) <= newPackageWindow, scoreNewPackage, DriverNewPackage)
		sheet.add(updated && facts.HasPrevious && facts.PreviousProvenance && !facts.Provenance, scoreProvenanceDropped, DriverProvenanceDropped)
		sheet.add(updated && facts.HasPrevious && facts.Publisher != "" && facts.PreviousPublisher != "" && facts.Publisher != facts.PreviousPublisher, scorePublisherChanged, DriverPublisherChanged)
	}

	sheet.add(candidate.HeadPackage.HasInstallScript, scoreInstallScript, DriverInstallScript)
	sheet.add(len(candidate.HeadPackage.OS) > 0 || len(candidate.HeadPackage.CPU) > 0, scorePlatformRestricted, DriverPlatformRestricted)
	sheet.add(views.AddedTransitive >= transitiveCountThreshold5, scoreTransitiveFive, DriverTransitiveFive)
	sheet.add(views.AddedTransitive >= transitiveCountThreshold15, scoreTransitiveFifteen, DriverTransitiveFifteen)
	return sheet.result()
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
