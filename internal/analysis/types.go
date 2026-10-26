package analysis

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/rad1092/gh-dependency-risk/internal/npm"
)

type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "low"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelCritical RiskLevel = "critical"
	RiskLevelNone     RiskLevel = "none"
)

type BlastRadius string

const (
	BlastRadiusLow    BlastRadius = "low"
	BlastRadiusMedium BlastRadius = "medium"
	BlastRadiusHigh   BlastRadius = "high"
)

type TargetKind string

const (
	TargetKindRoot       TargetKind = "root"
	TargetKindWorkspace  TargetKind = "workspace"
	TargetKindStandalone TargetKind = "standalone"
)

type DependencyScope string

const (
	ScopeRuntime    DependencyScope = "runtime"
	ScopeDev        DependencyScope = "dev"
	ScopeOptional   DependencyScope = "optional"
	ScopeTransitive DependencyScope = "transitive"
	ScopeUnknown    DependencyScope = "unknown"
)

type ChangeType string

const (
	ChangeAdded   ChangeType = "added"
	ChangeUpdated ChangeType = "updated"
	ChangeRemoved ChangeType = "removed"
)

const (
	DriverKnownVulnerabilities = "known_vulnerabilities"
	DriverAddedDirectRuntime   = "added_direct_runtime_dependency"
	DriverAddedDirectDev       = "added_direct_dev_dependency"
	DriverMajorVersionBump     = "major_version_bump"
	DriverRecentlyPublished    = "recently_published"
	DriverInstallScript        = "install_script_detected"
	DriverPlatformRestricted   = "platform_restricted_package"
	DriverTransitiveFive       = "added_transitive_count_ge_5"
	DriverTransitiveFifteen    = "added_transitive_count_ge_15"
	DriverNameSimilarToPopular = "name_similar_to_popular_package"
	DriverNewPackage           = "package_first_published_recently"
	DriverPublisherChanged     = "publisher_changed"
	DriverProvenanceDropped    = "provenance_dropped"
)

const (
	ActionReviewAdvisories = "review_advisories"
	ActionInspectInstall   = "inspect_install_scripts"
	ActionReviewChangelog  = "review_release_notes"
	ActionInspectTree      = "inspect_dependency_tree"
	ActionRunTargetedTests = "run_targeted_tests"
	ActionValidateSources  = "validate_non_registry_sources"
	ActionVerifyPackage    = "verify_package_identity"
)

const (
	NoteDependencyReviewFallback = "dependency_review_unavailable"
	NoteNonRegistrySource        = "non_registry_source"
	NoteApproximateAttribution   = "approximate_target_attribution"
	NoteUnsupportedDependency    = "unsupported_dependency_entry"
	NoteGoReplaceDirective       = "go_replace_directive"
	NoteGoLocalReplace           = "go_local_replace"
	NoteGoPseudoVersion          = "go_pseudo_version"
	NoteGoChecksumChanged        = "go_checksum_changed"
	NoteGoDirectiveChanged       = "go_directive_changed"
	NoteGoToolchainChanged       = "go_toolchain_directive_changed"
	NoteYarnBerryLockfile        = "yarn_berry_lockfile"
	NoteYarnNodeLinker           = "yarn_node_linker"
	NoteYarnWorkspaceProtocol    = "yarn_workspace_protocol"
	NoteYarnPatchProtocol        = "yarn_patch_protocol"
	NoteYarnPortalProtocol       = "yarn_portal_protocol"
	NoteYarnLinkProtocol         = "yarn_link_protocol"
	NoteYarnFileProtocol         = "yarn_file_protocol"
	NoteYarnGitSource            = "yarn_git_source"
	NoteYarnChecksumChanged      = "yarn_checksum_changed"
	NoteBunLockfile              = "bun_lockfile"
	NoteBunWorkspaceProtocol     = "bun_workspace_protocol"
	NoteBunChecksumChanged       = "bun_checksum_changed"
	NoteBunBinaryLockfile        = "bun_binary_lockfile_unsupported"
)

type AnalysisTarget struct {
	DisplayName       string     `json:"display_name"`
	ManifestPath      string     `json:"manifest_path"`
	LockfilePath      string     `json:"lockfile_path"`
	Kind              TargetKind `json:"kind"`
	WorkspaceRootPath string     `json:"workspace_root_path,omitempty"`
	PackageManager    string     `json:"-"`
	Ecosystem         string     `json:"-"`
	TargetID          string     `json:"-"`
	OwningDirectory   string     `json:"-"`
	LocalFallback     bool       `json:"-"`
	FallbackReason    string     `json:"-"`
}

func (t AnalysisTarget) Directory() string {
	if t.OwningDirectory != "" {
		return t.OwningDirectory
	}
	if t.ManifestPath == "" || t.ManifestPath == "package.json" {
		return ""
	}
	dir := path.Dir(t.ManifestPath)
	if dir == "." {
		return ""
	}
	return dir
}

func (t AnalysisTarget) Key() string {
	if t.TargetID != "" {
		return t.TargetID
	}
	return t.ManifestPath
}

type Vulnerability struct {
	Severity string `json:"severity"`
	GHSAID   string `json:"ghsa_id"`
	Summary  string `json:"summary"`
	URL      string `json:"url"`
}

type ReviewChange struct {
	ChangeType      ChangeType
	Manifest        string
	Name            string
	Version         string
	Ecosystem       string
	PackageManager  string
	Vulnerabilities []Vulnerability
}

type PackageVersion struct {
	Name    string
	Version string
}

type Note struct {
	Code       string `json:"code"`
	Dependency string `json:"dependency,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

// ScorePart is one signal's contribution to a change's score. The parts of a
// change add up to its score unless the sum is over the cap of 100.
type ScorePart struct {
	Signal string `json:"signal"`
	Points int    `json:"points"`
}

type DependencyChange struct {
	Name                 string          `json:"name"`
	Manifest             string          `json:"manifest,omitempty"`
	Target               string          `json:"target,omitempty"`
	ChangeType           ChangeType      `json:"change_type"`
	Scope                DependencyScope `json:"scope"`
	Direct               bool            `json:"direct"`
	FromVersion          string          `json:"from_version,omitempty"`
	ToVersion            string          `json:"to_version,omitempty"`
	FromRequirement      string          `json:"from_requirement,omitempty"`
	ToRequirement        string          `json:"to_requirement,omitempty"`
	Resolved             string          `json:"resolved,omitempty"`
	Score                int             `json:"score"`
	Level                RiskLevel       `json:"level"`
	RiskDrivers          []string        `json:"risk_drivers"`
	ScoreBreakdown       []ScorePart     `json:"score_breakdown"`
	Vulnerabilities      []Vulnerability `json:"vulnerabilities,omitempty"`
	AddedTransitiveCount int             `json:"added_transitive_count"`
	// ImportedBy lists source files in the checkout that import the package.
	// It is filled only when gh-dep-risk runs inside a clone of the repository.
	ImportedBy []string `json:"imported_by,omitempty"`
}

type AnalysisResult struct {
	DependencyReviewAvailable bool                   `json:"dependency_review_available"`
	Score                     int                    `json:"score"`
	Level                     RiskLevel              `json:"level"`
	BlastRadius               BlastRadius            `json:"blast_radius"`
	ChangedDependencies       []DependencyChange     `json:"changed_dependencies"`
	RiskDrivers               []string               `json:"risk_drivers"`
	RecommendedActions        []string               `json:"recommended_actions"`
	QuickCommands             []string               `json:"quick_commands"`
	Notes                     []Note                 `json:"notes,omitempty"`
	AddedTransitiveCount      int                    `json:"added_transitive_count"`
	Targets                   []TargetAnalysisResult `json:"targets,omitempty"`
	addedTransitiveKeys       []string
}

type TargetAnalysisResult struct {
	Target                    AnalysisTarget     `json:"target"`
	DependencyReviewAvailable bool               `json:"dependency_review_available"`
	Score                     int                `json:"score"`
	Level                     RiskLevel          `json:"level"`
	BlastRadius               BlastRadius        `json:"blast_radius"`
	ChangedDependencies       []DependencyChange `json:"changed_dependencies"`
	RiskDrivers               []string           `json:"risk_drivers"`
	RecommendedActions        []string           `json:"recommended_actions"`
	QuickCommands             []string           `json:"quick_commands"`
	Notes                     []Note             `json:"notes,omitempty"`
	AddedTransitiveCount      int                `json:"added_transitive_count"`
	addedTransitiveKeys       []string
}

type Input struct {
	Now                       time.Time
	Target                    AnalysisTarget
	DependencyReviewAvailable bool
	ReviewChanges             []ReviewChange
	BaseManifest              *npm.PackageManifest
	HeadManifest              *npm.PackageManifest
	BaseLockfile              *npm.Lockfile
	HeadLockfile              *npm.Lockfile
	// Facts holds npm registry facts for each change, keyed by the version it
	// moves to and, for an update, the version it replaces. Two targets that
	// move the same package to the same version from different versions get
	// separate comparisons.
	Facts map[RegistryLookup]npm.VersionFacts
}

// RegistryLookup is one package version to look up in the npm registry,
// with the version it replaces when the change is an update.
type RegistryLookup struct {
	Name            string
	Version         string
	PreviousVersion string
}

func ParseRiskLevel(value string) (RiskLevel, error) {
	switch strings.ToLower(value) {
	case string(RiskLevelLow):
		return RiskLevelLow, nil
	case string(RiskLevelMedium):
		return RiskLevelMedium, nil
	case string(RiskLevelHigh):
		return RiskLevelHigh, nil
	case string(RiskLevelCritical):
		return RiskLevelCritical, nil
	case string(RiskLevelNone):
		return RiskLevelNone, nil
	default:
		return "", fmt.Errorf("unsupported risk level %q", value)
	}
}

func (l RiskLevel) Threshold() int {
	switch l {
	case RiskLevelCritical:
		return levelThresholdCritical
	case RiskLevelHigh:
		return levelThresholdHigh
	case RiskLevelMedium:
		return levelThresholdMedium
	case RiskLevelLow:
		return 0
	default:
		return 101
	}
}

func LevelForScore(score int) RiskLevel {
	switch {
	case score >= levelThresholdCritical:
		return RiskLevelCritical
	case score >= levelThresholdHigh:
		return RiskLevelHigh
	case score >= levelThresholdMedium:
		return RiskLevelMedium
	default:
		return RiskLevelLow
	}
}

func CollectRegistryTargets(input Input) []PackageVersion {
	lookups := CollectRegistryLookups(input)
	targets := make([]PackageVersion, 0, len(lookups))
	seen := map[PackageVersion]struct{}{}
	for _, lookup := range lookups {
		key := PackageVersion{Name: lookup.Name, Version: lookup.Version}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		targets = append(targets, key)
	}
	return targets
}

// CollectRegistryLookups lists the registry versions an npm-family target
// moves to, sorted by name and version.
func CollectRegistryLookups(input Input) []RegistryLookup {
	switch input.Target.PackageManager {
	case "npm", "pnpm", "yarn":
	default:
		return nil
	}
	directNames := targetDirectNames(input)
	candidates := collectCandidateSummaries(input, buildTargetLockViews(input, directNames), directNames)
	seen := map[RegistryLookup]struct{}{}
	for _, candidate := range candidates {
		if candidate.ToVersion == "" || candidate.HeadPackage.WorkspaceLocal {
			continue
		}
		lookup := RegistryLookup{Name: candidate.Name, Version: candidate.ToVersion}
		if candidate.ChangeType == ChangeUpdated {
			lookup.PreviousVersion = candidate.FromVersion
		}
		seen[lookup] = struct{}{}
	}
	lookups := make([]RegistryLookup, 0, len(seen))
	for lookup := range seen {
		lookups = append(lookups, lookup)
	}
	sort.Slice(lookups, func(i, j int) bool {
		if lookups[i].Name != lookups[j].Name {
			return lookups[i].Name < lookups[j].Name
		}
		if lookups[i].Version != lookups[j].Version {
			return lookups[i].Version < lookups[j].Version
		}
		return lookups[i].PreviousVersion < lookups[j].PreviousVersion
	})
	return lookups
}
