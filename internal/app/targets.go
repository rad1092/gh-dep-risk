package app

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"gh-dep-risk/internal/analysis"
	ghclient "gh-dep-risk/internal/github"
	"gh-dep-risk/internal/npm"
)

type repoDataCache struct {
	client     ghclient.Client
	repo       ghclient.Repo
	rawFiles   map[string][]byte
	manifests  map[string]*npm.PackageManifest
	lockfiles  map[string]*npm.Lockfile
	trees      map[string][]string
	workspaces map[string][]string
}

type discoveredTarget struct {
	ManifestPath string
	DisplayName  string
	Variants     []analysis.AnalysisTarget
}

func newRepoDataCache(client ghclient.Client, repo ghclient.Repo) *repoDataCache {
	return &repoDataCache{
		client:     client,
		repo:       repo,
		rawFiles:   map[string][]byte{},
		manifests:  map[string]*npm.PackageManifest{},
		lockfiles:  map[string]*npm.Lockfile{},
		trees:      map[string][]string{},
		workspaces: map[string][]string{},
	}
}

func (c *repoDataCache) listFiles(ctx context.Context, ref string) ([]string, error) {
	if files, ok := c.trees[ref]; ok {
		return append([]string(nil), files...), nil
	}
	files, err := c.client.ListRepositoryFiles(ctx, c.repo, ref)
	if err != nil {
		return nil, err
	}
	sorted := append([]string(nil), files...)
	sort.Strings(sorted)
	c.trees[ref] = sorted
	return append([]string(nil), sorted...), nil
}

func (c *repoDataCache) manifest(ctx context.Context, ref, manifestPath string) (*npm.PackageManifest, error) {
	key := cacheKey(ref, manifestPath)
	if manifest, ok := c.manifests[key]; ok {
		return manifest, nil
	}
	data, err := c.file(ctx, ref, manifestPath)
	if err != nil {
		return nil, err
	}
	manifest, err := npm.ParsePackageManifest(data)
	if err != nil {
		return nil, err
	}
	c.manifests[key] = manifest
	return manifest, nil
}

func (c *repoDataCache) lockfile(ctx context.Context, ref, lockfilePath string) (*npm.Lockfile, error) {
	key := cacheKey(ref, lockfilePath)
	if lockfile, ok := c.lockfiles[key]; ok {
		return lockfile, nil
	}
	data, err := c.file(ctx, ref, lockfilePath)
	if err != nil {
		return nil, err
	}
	lockfile, err := npm.ParseLockfile(data)
	if err != nil {
		return nil, err
	}
	c.lockfiles[key] = lockfile
	return lockfile, nil
}

func (c *repoDataCache) pnpmWorkspacePatterns(ctx context.Context, ref, workspacePath string) ([]string, error) {
	key := cacheKey(ref, workspacePath)
	if patterns, ok := c.workspaces[key]; ok {
		return append([]string(nil), patterns...), nil
	}
	data, err := c.file(ctx, ref, workspacePath)
	if err != nil {
		return nil, err
	}
	patterns, err := npm.ParsePNPMWorkspacePatterns(data)
	if err != nil {
		return nil, err
	}
	c.workspaces[key] = append([]string(nil), patterns...)
	return append([]string(nil), patterns...), nil
}

func (c *repoDataCache) file(ctx context.Context, ref, filePath string) ([]byte, error) {
	key := cacheKey(ref, filePath)
	if data, ok := c.rawFiles[key]; ok {
		return append([]byte(nil), data...), nil
	}
	data, err := c.client.GetRepositoryFile(ctx, c.repo, filePath, ref)
	if err != nil {
		if errors.Is(err, ghclient.ErrNotFound) {
			c.rawFiles[key] = nil
			return nil, nil
		}
		return nil, err
	}
	c.rawFiles[key] = append([]byte(nil), data...)
	return append([]byte(nil), data...), nil
}

func discoverTargets(ctx context.Context, cache *repoDataCache, baseRef, headRef string) ([]discoveredTarget, error) {
	baseFiles, err := cache.listFiles(ctx, baseRef)
	if err != nil {
		return nil, err
	}
	headFiles, err := cache.listFiles(ctx, headRef)
	if err != nil {
		return nil, err
	}

	manifestPaths := unionPaths(filterPaths(baseFiles, "package.json"), filterPaths(headFiles, "package.json"))
	npmLockfilePaths := pathSet(unionPaths(filterPaths(baseFiles, "package-lock.json"), filterPaths(headFiles, "package-lock.json")))
	pnpmLockfilePaths := pathSet(unionPaths(filterPaths(baseFiles, "pnpm-lock.yaml"), filterPaths(headFiles, "pnpm-lock.yaml")))
	pnpmWorkspacePaths := unionPaths(filterPaths(baseFiles, "pnpm-workspace.yaml"), filterPaths(headFiles, "pnpm-workspace.yaml"))

	manifestCache := map[string][2]*npm.PackageManifest{}
	for _, manifestPath := range manifestPaths {
		baseManifest, err := cache.manifest(ctx, baseRef, manifestPath)
		if err != nil {
			return nil, err
		}
		headManifest, err := cache.manifest(ctx, headRef, manifestPath)
		if err != nil {
			return nil, err
		}
		manifestCache[manifestPath] = [2]*npm.PackageManifest{baseManifest, headManifest}
	}

	npmWorkspaceRoots := map[string]string{}
	for _, manifestPath := range manifestPaths {
		patterns := workspacePatterns(manifestCache[manifestPath][0], manifestCache[manifestPath][1])
		if len(patterns) == 0 {
			continue
		}
		rootDir := manifestDir(manifestPath)
		lockfilePath := lockfilePathForDir(rootDir)
		if _, ok := npmLockfilePaths[lockfilePath]; !ok {
			continue
		}
		for _, candidate := range manifestPaths {
			if candidate == manifestPath {
				continue
			}
			if !matchesWorkspaceTarget(rootDir, patterns, candidate) {
				continue
			}
			npmWorkspaceRoots[candidate] = rootDir
		}
	}

	pnpmWorkspaceRoots := map[string]string{}
	for _, workspacePath := range pnpmWorkspacePaths {
		rootDir := manifestDir(workspacePath)
		lockfilePath := pnpmLockfilePathForDir(rootDir)
		if _, ok := pnpmLockfilePaths[lockfilePath]; !ok {
			continue
		}
		basePatterns, err := cache.pnpmWorkspacePatterns(ctx, baseRef, workspacePath)
		if err != nil {
			return nil, err
		}
		headPatterns, err := cache.pnpmWorkspacePatterns(ctx, headRef, workspacePath)
		if err != nil {
			return nil, err
		}
		patterns := unionStrings(basePatterns, headPatterns)
		if len(patterns) == 0 {
			continue
		}
		rootManifestPath := manifestPathForDir(rootDir)
		for _, candidate := range manifestPaths {
			if candidate == rootManifestPath {
				continue
			}
			if !matchesWorkspaceTarget(rootDir, patterns, candidate) {
				continue
			}
			pnpmWorkspaceRoots[candidate] = rootDir
		}
	}

	grouped := map[string][]analysis.AnalysisTarget{}
	for _, manifestPath := range manifestPaths {
		dir := manifestDir(manifestPath)
		npmLockfilePath := lockfilePathForDir(dir)
		pnpmLockfilePath := pnpmLockfilePathForDir(dir)

		if workspaceRoot, ok := npmWorkspaceRoots[manifestPath]; ok {
			grouped[manifestPath] = append(grouped[manifestPath], analysis.AnalysisTarget{
				DisplayName:       displayNameForManifest(manifestPath),
				ManifestPath:      manifestPath,
				LockfilePath:      lockfilePathForDir(workspaceRoot),
				Kind:              analysis.TargetKindWorkspace,
				WorkspaceRootPath: workspaceRoot,
				PackageManager:    "npm",
			})
		} else if _, ok := npmLockfilePaths[npmLockfilePath]; ok {
			grouped[manifestPath] = append(grouped[manifestPath], analysis.AnalysisTarget{
				DisplayName:    displayNameForManifest(manifestPath),
				ManifestPath:   manifestPath,
				LockfilePath:   npmLockfilePath,
				Kind:           kindForManifest(manifestPath),
				PackageManager: "npm",
			})
		}

		if workspaceRoot, ok := pnpmWorkspaceRoots[manifestPath]; ok {
			grouped[manifestPath] = append(grouped[manifestPath], analysis.AnalysisTarget{
				DisplayName:       displayNameForManifest(manifestPath),
				ManifestPath:      manifestPath,
				LockfilePath:      pnpmLockfilePathForDir(workspaceRoot),
				Kind:              analysis.TargetKindWorkspace,
				WorkspaceRootPath: workspaceRoot,
				PackageManager:    "pnpm",
			})
		} else if _, ok := pnpmLockfilePaths[pnpmLockfilePath]; ok {
			grouped[manifestPath] = append(grouped[manifestPath], analysis.AnalysisTarget{
				DisplayName:    displayNameForManifest(manifestPath),
				ManifestPath:   manifestPath,
				LockfilePath:   pnpmLockfilePath,
				Kind:           kindForManifest(manifestPath),
				PackageManager: "pnpm",
			})
		}
	}

	targets := make([]discoveredTarget, 0, len(grouped))
	for manifestPath, variants := range grouped {
		sort.Slice(variants, func(i, j int) bool {
			if variants[i].PackageManager == variants[j].PackageManager {
				return variants[i].LockfilePath < variants[j].LockfilePath
			}
			return variants[i].PackageManager < variants[j].PackageManager
		})
		targets = append(targets, discoveredTarget{
			ManifestPath: manifestPath,
			DisplayName:  displayNameForManifest(manifestPath),
			Variants:     variants,
		})
	}

	sort.Slice(targets, func(i, j int) bool {
		return targets[i].ManifestPath < targets[j].ManifestPath
	})
	return targets, nil
}

func filterTargetsByRequestedPaths(targets []discoveredTarget, requested []string) ([]discoveredTarget, error) {
	if len(requested) == 0 {
		return append([]discoveredTarget(nil), targets...), nil
	}

	index := map[string]discoveredTarget{}
	for _, target := range targets {
		index[target.ManifestPath] = target
	}
	selected := make([]discoveredTarget, 0, len(requested))
	seen := map[string]struct{}{}
	for _, raw := range requested {
		manifestPath := normalizeRequestedManifestPath(raw)
		target, ok := index[manifestPath]
		if !ok {
			return nil, fmt.Errorf("unknown dependency target path %q. Run --list-targets to inspect detected targets, or try one of: %s", raw, targetPathExamples(targets))
		}
		if _, ok := seen[target.ManifestPath]; ok {
			continue
		}
		seen[target.ManifestPath] = struct{}{}
		selected = append(selected, target)
	}
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].ManifestPath < selected[j].ManifestPath
	})
	return selected, nil
}

func selectChangedTargets(targets []discoveredTarget, files []ghclient.PullRequestFile) ([]analysis.AnalysisTarget, error) {
	changed := map[string]struct{}{}
	for _, file := range files {
		changed[normalizeRepoPath(file.Filename)] = struct{}{}
	}
	selected := make([]analysis.AnalysisTarget, 0, len(targets))
	for _, target := range targets {
		if !targetIsChanged(target, changed) {
			continue
		}
		resolved, err := resolveTargetVariant(target, changed)
		if err != nil {
			return nil, err
		}
		selected = append(selected, resolved)
	}
	return selected, nil
}

func formatTargets(targets []discoveredTarget) string {
	if len(targets) == 0 {
		return "Detected JS package targets:\n- none\n"
	}
	lines := make([]string, 0, len(targets)+1)
	lines = append(lines, "Detected JS package targets:")
	for _, target := range targets {
		if target.ambiguous() {
			lines = append(lines, fmt.Sprintf("- %s [ambiguous]\n  manifest: %s\n  lockfiles: %s\n  package managers: %s\n  note: analysis will use the single changed lockfile if the PR makes this target unambiguous", displayTargetName(target), target.ManifestPath, strings.Join(target.lockfiles(), ", "), strings.Join(target.packageManagers(), ", ")))
			continue
		}
		variant := target.Variants[0]
		line := fmt.Sprintf("- %s [%s, %s]\n  manifest: %s\n  lockfile: %s", displayTargetName(target), variant.Kind, variant.PackageManager, target.ManifestPath, variant.LockfilePath)
		if variant.WorkspaceRootPath != "" {
			line += fmt.Sprintf("\n  workspace root: %s", variant.WorkspaceRootPath)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n") + "\n"
}

func targetPathExamples(targets []discoveredTarget) string {
	paths := make([]string, 0, len(targets))
	for _, target := range targets {
		paths = append(paths, target.ManifestPath)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return "package.json"
	}
	if len(paths) > 4 {
		paths = paths[:4]
	}
	return strings.Join(paths, ", ")
}

func displayTargetName(target discoveredTarget) string {
	if target.DisplayName != "" {
		return target.DisplayName
	}
	if dir := manifestDir(target.ManifestPath); dir != "" {
		return dir
	}
	return "root"
}

func cacheKey(ref, filePath string) string {
	return ref + "@" + filePath
}

func filterPaths(paths []string, base string) []string {
	filtered := make([]string, 0)
	for _, filePath := range paths {
		if path.Base(filePath) == base {
			filtered = append(filtered, normalizeRepoPath(filePath))
		}
	}
	sort.Strings(filtered)
	return filtered
}

func unionPaths(left, right []string) []string {
	set := map[string]struct{}{}
	for _, filePath := range append(append([]string(nil), left...), right...) {
		set[normalizeRepoPath(filePath)] = struct{}{}
	}
	paths := make([]string, 0, len(set))
	for filePath := range set {
		paths = append(paths, filePath)
	}
	sort.Strings(paths)
	return paths
}

func unionStrings(left, right []string) []string {
	set := map[string]struct{}{}
	for _, value := range append(append([]string(nil), left...), right...) {
		if strings.TrimSpace(value) == "" {
			continue
		}
		set[value] = struct{}{}
	}
	values := make([]string, 0, len(set))
	for value := range set {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func pathSet(paths []string) map[string]struct{} {
	set := make(map[string]struct{}, len(paths))
	for _, filePath := range paths {
		set[filePath] = struct{}{}
	}
	return set
}

func manifestDir(manifestPath string) string {
	cleaned := normalizeRepoPath(manifestPath)
	if cleaned == "package.json" || cleaned == "pnpm-workspace.yaml" {
		return ""
	}
	dir := path.Dir(cleaned)
	if dir == "." {
		return ""
	}
	return dir
}

func manifestPathForDir(dir string) string {
	cleaned := normalizeRepoPath(dir)
	if cleaned == "" {
		return "package.json"
	}
	return cleaned + "/package.json"
}

func lockfilePathForDir(dir string) string {
	cleaned := normalizeRepoPath(dir)
	if cleaned == "" {
		return "package-lock.json"
	}
	return cleaned + "/package-lock.json"
}

func pnpmLockfilePathForDir(dir string) string {
	cleaned := normalizeRepoPath(dir)
	if cleaned == "" {
		return "pnpm-lock.yaml"
	}
	return cleaned + "/pnpm-lock.yaml"
}

func normalizeRepoPath(value string) string {
	cleaned := path.Clean(strings.TrimSpace(strings.ReplaceAll(value, "\\", "/")))
	switch cleaned {
	case ".", "/":
		return ""
	default:
		return strings.TrimPrefix(cleaned, "./")
	}
}

func normalizeRequestedManifestPath(value string) string {
	cleaned := normalizeRepoPath(value)
	if cleaned == "" {
		return "package.json"
	}
	if strings.HasSuffix(cleaned, "/package.json") || cleaned == "package.json" {
		return cleaned
	}
	return manifestPathForDir(cleaned)
}

func workspacePatterns(base, head *npm.PackageManifest) []string {
	set := map[string]struct{}{}
	for _, manifest := range []*npm.PackageManifest{base, head} {
		if manifest == nil {
			continue
		}
		for _, pattern := range manifest.Workspaces {
			set[pattern] = struct{}{}
		}
	}
	patterns := make([]string, 0, len(set))
	for pattern := range set {
		patterns = append(patterns, pattern)
	}
	sort.Strings(patterns)
	return patterns
}

func matchesWorkspaceTarget(rootDir string, patterns []string, manifestPath string) bool {
	dir := manifestDir(manifestPath)
	if dir == "" {
		return false
	}
	relative, ok := relativeToRoot(rootDir, dir)
	if !ok || relative == "" {
		return false
	}
	return npm.MatchWorkspacePatternSet(patterns, relative)
}

func relativeToRoot(rootDir, targetDir string) (string, bool) {
	root := normalizeRepoPath(rootDir)
	target := normalizeRepoPath(targetDir)
	if root == "" {
		return target, true
	}
	if target == root {
		return "", true
	}
	prefix := root + "/"
	if !strings.HasPrefix(target, prefix) {
		return "", false
	}
	return strings.TrimPrefix(target, prefix), true
}

func displayNameForManifest(manifestPath string) string {
	if dir := manifestDir(manifestPath); dir != "" {
		return dir
	}
	return "root"
}

func kindForManifest(manifestPath string) analysis.TargetKind {
	if manifestPath == "package.json" {
		return analysis.TargetKindRoot
	}
	return analysis.TargetKindStandalone
}

func (t discoveredTarget) ambiguous() bool {
	return len(t.Variants) > 1
}

func (t discoveredTarget) lockfiles() []string {
	lockfiles := make([]string, 0, len(t.Variants))
	for _, variant := range t.Variants {
		lockfiles = append(lockfiles, variant.LockfilePath)
	}
	sort.Strings(lockfiles)
	return lockfiles
}

func (t discoveredTarget) packageManagers() []string {
	seen := map[string]struct{}{}
	managers := make([]string, 0, len(t.Variants))
	for _, variant := range t.Variants {
		if _, ok := seen[variant.PackageManager]; ok {
			continue
		}
		seen[variant.PackageManager] = struct{}{}
		managers = append(managers, variant.PackageManager)
	}
	sort.Strings(managers)
	return managers
}

func targetIsChanged(target discoveredTarget, changed map[string]struct{}) bool {
	if _, ok := changed[target.ManifestPath]; ok {
		return true
	}
	for _, variant := range target.Variants {
		if _, ok := changed[variant.LockfilePath]; ok {
			return true
		}
	}
	return false
}

func resolveTargetVariant(target discoveredTarget, changed map[string]struct{}) (analysis.AnalysisTarget, error) {
	if len(target.Variants) == 1 {
		return target.Variants[0], nil
	}

	changedVariants := make([]analysis.AnalysisTarget, 0, len(target.Variants))
	for _, variant := range target.Variants {
		if _, ok := changed[variant.LockfilePath]; ok {
			changedVariants = append(changedVariants, variant)
		}
	}
	switch len(changedVariants) {
	case 1:
		return changedVariants[0], nil
	case 0:
		return analysis.AnalysisTarget{}, fmt.Errorf("target %q is ambiguous because both %s are present. Change exactly one lockfile in the PR or remove the unused lockfile, then rerun with --path %s", target.ManifestPath, strings.Join(target.lockfiles(), " and "), target.ManifestPath)
	default:
		return analysis.AnalysisTarget{}, fmt.Errorf("target %q is ambiguous because multiple supported lockfiles changed in the same directory (%s). Keep only one package manager lockfile per target or narrow the PR before rerunning", target.ManifestPath, strings.Join(target.lockfiles(), ", "))
	}
}
