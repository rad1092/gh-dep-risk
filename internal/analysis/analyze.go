package analysis

import (
	"sort"
	"strings"
	"time"

	"gh-dep-risk/internal/npm"
)

type candidateSummary struct {
	Name            string
	Manifest        string
	ChangeType      ChangeType
	Scope           DependencyScope
	Direct          bool
	FromVersion     string
	ToVersion       string
	FromRequirement string
	ToRequirement   string
	Resolved        string
	Vulnerabilities []Vulnerability
}

func Analyze(input Input, publishedAt map[PackageVersion]time.Time) AnalysisResult {
	candidates := collectCandidateSummaries(input)
	directNames := directNameSet(input.BaseManifest, input.HeadManifest, input.BaseLockfile, input.HeadLockfile)
	addedTransitiveCount := 0
	if input.HeadLockfile != nil {
		addedTransitiveCount = input.HeadLockfile.AddedTransitiveCount(input.BaseLockfile, directNames)
	}

	changes := make([]DependencyChange, 0, len(candidates))
	notes := make([]Note, 0)
	for _, candidate := range candidates {
		change := DependencyChange{
			Name:                 candidate.Name,
			Manifest:             candidate.Manifest,
			ChangeType:           candidate.ChangeType,
			Scope:                candidate.Scope,
			Direct:               candidate.Direct,
			FromVersion:          candidate.FromVersion,
			ToVersion:            candidate.ToVersion,
			FromRequirement:      candidate.FromRequirement,
			ToRequirement:        candidate.ToRequirement,
			Resolved:             candidate.Resolved,
			Vulnerabilities:      append([]Vulnerability(nil), candidate.Vulnerabilities...),
			AddedTransitiveCount: addedTransitiveCount,
		}

		score := 0
		drivers := make([]string, 0)
		if len(candidate.Vulnerabilities) > 0 {
			score += 35
			drivers = append(drivers, DriverKnownVulnerabilities)
		}
		if candidate.ChangeType == ChangeAdded && candidate.Direct {
			switch candidate.Scope {
			case ScopeRuntime, ScopeOptional:
				score += 12
				drivers = append(drivers, DriverAddedDirectRuntime)
			case ScopeDev:
				score += 6
				drivers = append(drivers, DriverAddedDirectDev)
			}
		}
		if isMajorBump(candidate.FromVersion, candidate.ToVersion, candidate.FromRequirement, candidate.ToRequirement) {
			score += 10
			drivers = append(drivers, DriverMajorVersionBump)
		}
		if published, ok := publishedAt[PackageVersion{Name: candidate.Name, Version: candidate.ToVersion}]; ok && !published.IsZero() {
			if input.Now.Sub(published) <= 7*24*time.Hour {
				score += 18
				drivers = append(drivers, DriverRecentlyPublished)
			}
		}

		headPkg := headPackage(input.HeadLockfile, candidate.Name, candidate.Direct)
		if headPkg.HasInstallScript {
			score += 20
			drivers = append(drivers, DriverInstallScript)
		}
		if len(headPkg.OS) > 0 || len(headPkg.CPU) > 0 {
			score += 6
			drivers = append(drivers, DriverPlatformRestricted)
		}
		if addedTransitiveCount >= 5 {
			score += 12
			drivers = append(drivers, DriverTransitiveFive)
		}
		if addedTransitiveCount >= 15 {
			score += 8
			drivers = append(drivers, DriverTransitiveFifteen)
		}
		if score > 100 {
			score = 100
		}

		change.Score = score
		change.Level = LevelForScore(score)
		change.RiskDrivers = uniqueStrings(drivers)
		if candidate.Resolved != "" && !npm.IsRegistrySource(candidate.Resolved) {
			notes = append(notes, Note{
				Code:       NoteNonRegistrySource,
				Dependency: candidate.Name,
				Detail:     npm.DescribeSource(candidate.Resolved),
			})
		}
		changes = append(changes, change)
	}

	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Score == changes[j].Score {
			return changes[i].Name < changes[j].Name
		}
		return changes[i].Score > changes[j].Score
	})

	if !input.DependencyReviewAvailable {
		notes = append(notes, Note{Code: NoteDependencyReviewFallback})
	}

	score := aggregateScore(changes)
	return AnalysisResult{
		DependencyReviewAvailable: input.DependencyReviewAvailable,
		Score:                     score,
		Level:                     LevelForScore(score),
		BlastRadius:               deriveBlastRadius(changes, addedTransitiveCount),
		ChangedDependencies:       changes,
		RiskDrivers:               collectDrivers(changes),
		RecommendedActions:        recommendedActions(changes, notes),
		QuickCommands:             quickCommands(changes),
		Notes:                     uniqueNotes(notes),
		AddedTransitiveCount:      addedTransitiveCount,
	}
}

func collectCandidateSummaries(input Input) []candidateSummary {
	reviewMap := map[string]candidateSummary{}
	for _, change := range input.ReviewChanges {
		if change.Name == "" {
			continue
		}
		current := reviewMap[change.Name]
		current.Name = change.Name
		current.Manifest = change.Manifest
		switch change.ChangeType {
		case ChangeAdded:
			current.ToVersion = change.Version
			if current.ChangeType == ChangeRemoved {
				current.ChangeType = ChangeUpdated
			} else if current.ChangeType == "" {
				current.ChangeType = ChangeAdded
			}
			current.Vulnerabilities = append(current.Vulnerabilities, change.Vulnerabilities...)
		case ChangeRemoved:
			current.FromVersion = change.Version
			if current.ChangeType == ChangeAdded {
				current.ChangeType = ChangeUpdated
			} else if current.ChangeType == "" {
				current.ChangeType = ChangeRemoved
			}
		}
		reviewMap[change.Name] = current
	}

	names := map[string]candidateSummary{}
	if len(reviewMap) > 0 {
		for name, current := range reviewMap {
			fillCandidateFromManifests(&current, input)
			names[name] = current
		}
	} else {
		for name, current := range collectManifestAndLockCandidates(input) {
			fillCandidateFromManifests(&current, input)
			names[name] = current
		}
	}

	candidates := make([]candidateSummary, 0, len(names))
	for _, candidate := range names {
		if candidate.ChangeType == "" {
			continue
		}
		candidates = append(candidates, candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Name < candidates[j].Name
	})
	return candidates
}

func collectManifestAndLockCandidates(input Input) map[string]candidateSummary {
	candidates := map[string]candidateSummary{}
	allNames := map[string]struct{}{}
	for _, name := range input.BaseManifest.DirectNames() {
		allNames[name] = struct{}{}
	}
	for _, name := range input.HeadManifest.DirectNames() {
		allNames[name] = struct{}{}
	}
	if input.BaseLockfile != nil {
		for name := range input.BaseLockfile.TopLevelPackages() {
			allNames[name] = struct{}{}
		}
	}
	if input.HeadLockfile != nil {
		for name := range input.HeadLockfile.TopLevelPackages() {
			allNames[name] = struct{}{}
		}
	}
	for name := range allNames {
		candidate := candidateSummary{Name: name}
		fillCandidateFromManifests(&candidate, input)
		if candidate.ChangeType != "" {
			candidates[name] = candidate
		}
	}
	for name, candidate := range collectTransitiveCandidates(input) {
		if _, ok := candidates[name]; !ok {
			candidates[name] = candidate
		}
	}
	return candidates
}

func collectTransitiveCandidates(input Input) map[string]candidateSummary {
	candidates := map[string]candidateSummary{}
	basePackages := map[string]npm.LockPackage{}
	headPackages := map[string]npm.LockPackage{}
	if input.BaseLockfile != nil {
		for path, pkg := range input.BaseLockfile.Packages {
			if path == "" || npm.IsTopLevelPackagePath(path) {
				continue
			}
			basePackages[path] = pkg
		}
	}
	if input.HeadLockfile != nil {
		for path, pkg := range input.HeadLockfile.Packages {
			if path == "" || npm.IsTopLevelPackagePath(path) {
				continue
			}
			headPackages[path] = pkg
		}
	}
	for path, headPkg := range headPackages {
		basePkg, ok := basePackages[path]
		if ok && basePkg.Version == headPkg.Version {
			continue
		}
		current := candidates[headPkg.Name]
		current.Name = headPkg.Name
		current.Scope = ScopeTransitive
		current.Direct = false
		current.ToVersion = headPkg.Version
		current.Resolved = headPkg.Resolved
		if ok {
			current.ChangeType = ChangeUpdated
			current.FromVersion = basePkg.Version
		} else {
			current.ChangeType = ChangeAdded
		}
		candidates[headPkg.Name] = current
	}
	for path, basePkg := range basePackages {
		if _, ok := headPackages[path]; ok {
			continue
		}
		current := candidates[basePkg.Name]
		current.Name = basePkg.Name
		current.Scope = ScopeTransitive
		current.Direct = false
		current.ChangeType = ChangeRemoved
		current.FromVersion = basePkg.Version
		candidates[basePkg.Name] = current
	}
	return candidates
}

func fillCandidateFromManifests(candidate *candidateSummary, input Input) {
	baseScope, baseDirect := manifestScope(candidate.Name, input.BaseManifest)
	headScope, headDirect := manifestScope(candidate.Name, input.HeadManifest)
	if headDirect {
		candidate.Scope = headScope
		candidate.Direct = true
	} else if baseDirect {
		candidate.Scope = baseScope
		candidate.Direct = true
	}
	if candidate.Scope == "" && candidate.Direct {
		candidate.Scope = ScopeUnknown
	}
	if candidate.Scope == "" {
		candidate.Scope = ScopeTransitive
	}

	if candidate.Manifest == "" {
		candidate.Manifest = "package-lock.json"
		if candidate.Direct {
			candidate.Manifest = "package.json"
		}
	}

	baseRequirement := input.BaseManifest.Requirement(candidate.Name)
	headRequirement := input.HeadManifest.Requirement(candidate.Name)
	basePkg, basePkgOK := topLevelPackage(input.BaseLockfile, candidate.Name)
	headPkg, headPkgOK := topLevelPackage(input.HeadLockfile, candidate.Name)
	if !candidate.Direct {
		basePkg, basePkgOK = anyPackage(input.BaseLockfile, candidate.Name)
		headPkg, headPkgOK = anyPackage(input.HeadLockfile, candidate.Name)
	}

	if candidate.FromRequirement == "" {
		candidate.FromRequirement = baseRequirement
	}
	if candidate.ToRequirement == "" {
		candidate.ToRequirement = headRequirement
	}
	if candidate.FromVersion == "" && basePkgOK {
		candidate.FromVersion = basePkg.Version
	}
	if candidate.ToVersion == "" && headPkgOK {
		candidate.ToVersion = headPkg.Version
	}
	if candidate.Resolved == "" && headPkgOK {
		candidate.Resolved = headPkg.Resolved
	}

	if candidate.ChangeType == "" {
		switch {
		case !baseDirect && !basePkgOK && (headDirect || headPkgOK):
			candidate.ChangeType = ChangeAdded
		case (baseDirect || basePkgOK) && !headDirect && !headPkgOK:
			candidate.ChangeType = ChangeRemoved
		case valuesDiffer(candidate.FromVersion, candidate.ToVersion) || valuesDiffer(candidate.FromRequirement, candidate.ToRequirement):
			candidate.ChangeType = ChangeUpdated
		}
	}
}

func valuesDiffer(left, right string) bool {
	return strings.TrimSpace(left) != strings.TrimSpace(right)
}

func manifestScope(name string, manifest *npm.PackageManifest) (DependencyScope, bool) {
	scope, ok := manifest.Scope(name)
	if !ok {
		return "", false
	}
	switch scope {
	case "runtime":
		return ScopeRuntime, true
	case "dev":
		return ScopeDev, true
	case "optional":
		return ScopeOptional, true
	default:
		return ScopeUnknown, true
	}
}

func directNameSet(baseManifest, headManifest *npm.PackageManifest, baseLock, headLock *npm.Lockfile) map[string]struct{} {
	names := map[string]struct{}{}
	for _, name := range baseManifest.DirectNames() {
		names[name] = struct{}{}
	}
	for _, name := range headManifest.DirectNames() {
		names[name] = struct{}{}
	}
	if len(names) == 0 {
		if baseLock != nil {
			for name := range baseLock.TopLevelPackages() {
				names[name] = struct{}{}
			}
		}
		if headLock != nil {
			for name := range headLock.TopLevelPackages() {
				names[name] = struct{}{}
			}
		}
	}
	return names
}

func topLevelPackage(lockfile *npm.Lockfile, name string) (npm.LockPackage, bool) {
	if lockfile == nil {
		return npm.LockPackage{}, false
	}
	pkg, ok := lockfile.TopLevelPackages()[name]
	return pkg, ok
}

func anyPackage(lockfile *npm.Lockfile, name string) (npm.LockPackage, bool) {
	if lockfile == nil {
		return npm.LockPackage{}, false
	}
	packages := lockfile.FindByName(name)
	if len(packages) == 0 {
		return npm.LockPackage{}, false
	}
	return packages[0], true
}

func headPackage(lockfile *npm.Lockfile, name string, direct bool) npm.LockPackage {
	if direct {
		if pkg, ok := topLevelPackage(lockfile, name); ok {
			return pkg
		}
	}
	pkg, _ := anyPackage(lockfile, name)
	return pkg
}

func isMajorBump(fromVersion, toVersion, fromRequirement, toRequirement string) bool {
	left := fromVersion
	if left == "" {
		left = fromRequirement
	}
	right := toVersion
	if right == "" {
		right = toRequirement
	}
	fromMajor, fromOK := npm.MajorVersion(left)
	toMajor, toOK := npm.MajorVersion(right)
	return fromOK && toOK && toMajor > fromMajor
}

func aggregateScore(changes []DependencyChange) int {
	if len(changes) == 0 {
		return 0
	}
	maxScore := changes[0].Score
	bonus := 0
	for _, change := range changes[1:] {
		if change.Score >= 20 {
			bonus += 4
		} else if change.Score > 0 {
			bonus++
		}
		if bonus >= 15 {
			bonus = 15
			break
		}
	}
	score := maxScore + bonus
	if score > 100 {
		score = 100
	}
	return score
}

func deriveBlastRadius(changes []DependencyChange, addedTransitiveCount int) BlastRadius {
	hasRuntime := false
	hasManyChanges := len(changes) >= 3
	for _, change := range changes {
		if change.Scope == ScopeRuntime || change.Scope == ScopeOptional {
			hasRuntime = true
		}
		if (change.Scope == ScopeRuntime || change.Scope == ScopeOptional) && change.ChangeType == ChangeAdded && change.Score >= 40 {
			return BlastRadiusHigh
		}
	}
	switch {
	case hasRuntime && addedTransitiveCount >= 15:
		return BlastRadiusHigh
	case hasRuntime || addedTransitiveCount >= 5 || hasManyChanges:
		return BlastRadiusMedium
	default:
		return BlastRadiusLow
	}
}

func collectDrivers(changes []DependencyChange) []string {
	set := map[string]struct{}{}
	for _, change := range changes {
		for _, driver := range change.RiskDrivers {
			set[driver] = struct{}{}
		}
	}
	return sortedKeys(set)
}

func recommendedActions(changes []DependencyChange, notes []Note) []string {
	set := map[string]struct{}{}
	for _, change := range changes {
		for _, driver := range change.RiskDrivers {
			switch driver {
			case DriverKnownVulnerabilities:
				set[ActionReviewAdvisories] = struct{}{}
			case DriverInstallScript:
				set[ActionInspectInstall] = struct{}{}
			case DriverMajorVersionBump:
				set[ActionReviewChangelog] = struct{}{}
				set[ActionRunTargetedTests] = struct{}{}
			case DriverTransitiveFive, DriverTransitiveFifteen:
				set[ActionInspectTree] = struct{}{}
			default:
				if change.Direct {
					set[ActionRunTargetedTests] = struct{}{}
				}
			}
		}
	}
	for _, note := range notes {
		if note.Code == NoteNonRegistrySource {
			set[ActionValidateSources] = struct{}{}
		}
	}
	return sortedKeys(set)
}

func quickCommands(changes []DependencyChange) []string {
	commands := []string{"npm ls --all"}
	if len(changes) > 0 {
		target := changes[0]
		commands = append(commands, "npm ls "+target.Name)
		if target.ToVersion != "" {
			commands = append(commands, "npm view "+target.Name+"@"+target.ToVersion+" time --json")
		}
	}
	return uniqueStrings(commands)
}

func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func uniqueStrings(values []string) []string {
	set := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := set[value]; ok {
			continue
		}
		set[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func uniqueNotes(notes []Note) []Note {
	seen := map[string]struct{}{}
	result := make([]Note, 0, len(notes))
	for _, note := range notes {
		key := note.Code + "|" + note.Dependency + "|" + note.Detail
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, note)
	}
	sort.Slice(result, func(i, j int) bool {
		left := result[i].Code + result[i].Dependency + result[i].Detail
		right := result[j].Code + result[j].Dependency + result[j].Detail
		return left < right
	})
	return result
}
