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
	client    ghclient.Client
	repo      ghclient.Repo
	rawFiles  map[string][]byte
	manifests map[string]*npm.PackageManifest
	lockfiles map[string]*npm.Lockfile
	trees     map[string][]string
}

func newRepoDataCache(client ghclient.Client, repo ghclient.Repo) *repoDataCache {
	return &repoDataCache{
		client:    client,
		repo:      repo,
		rawFiles:  map[string][]byte{},
		manifests: map[string]*npm.PackageManifest{},
		lockfiles: map[string]*npm.Lockfile{},
		trees:     map[string][]string{},
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

func discoverTargets(ctx context.Context, cache *repoDataCache, baseRef, headRef string) ([]analysis.AnalysisTarget, error) {
	baseFiles, err := cache.listFiles(ctx, baseRef)
	if err != nil {
		return nil, err
	}
	headFiles, err := cache.listFiles(ctx, headRef)
	if err != nil {
		return nil, err
	}

	manifestPaths := unionPaths(filterPaths(baseFiles, "package.json"), filterPaths(headFiles, "package.json"))
	lockfilePaths := pathSet(unionPaths(filterPaths(baseFiles, "package-lock.json"), filterPaths(headFiles, "package-lock.json")))

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

	workspaceRoots := map[string]string{}
	for _, manifestPath := range manifestPaths {
		patterns := workspacePatterns(manifestCache[manifestPath][0], manifestCache[manifestPath][1])
		if len(patterns) == 0 {
			continue
		}
		rootDir := manifestDir(manifestPath)
		lockfilePath := lockfilePathForDir(rootDir)
		if _, ok := lockfilePaths[lockfilePath]; !ok {
			continue
		}
		for _, candidate := range manifestPaths {
			if candidate == manifestPath {
				continue
			}
			if !matchesWorkspaceTarget(rootDir, patterns, candidate) {
				continue
			}
			workspaceRoots[candidate] = rootDir
		}
	}

	targets := make([]analysis.AnalysisTarget, 0, len(manifestPaths))
	for _, manifestPath := range manifestPaths {
		dir := manifestDir(manifestPath)
		lockfilePath := lockfilePathForDir(dir)
		workspaceRoot, isWorkspace := workspaceRoots[manifestPath]
		switch {
		case manifestPath == "package.json":
			if _, ok := lockfilePaths[lockfilePath]; ok {
				targets = append(targets, analysis.AnalysisTarget{
					DisplayName:  "root",
					ManifestPath: manifestPath,
					LockfilePath: lockfilePath,
					Kind:         analysis.TargetKindRoot,
				})
			}
		case isWorkspace:
			targets = append(targets, analysis.AnalysisTarget{
				DisplayName:       dir,
				ManifestPath:      manifestPath,
				LockfilePath:      lockfilePathForDir(workspaceRoot),
				Kind:              analysis.TargetKindWorkspace,
				WorkspaceRootPath: workspaceRoot,
			})
		default:
			if _, ok := lockfilePaths[lockfilePath]; ok {
				targets = append(targets, analysis.AnalysisTarget{
					DisplayName:  dir,
					ManifestPath: manifestPath,
					LockfilePath: lockfilePath,
					Kind:         analysis.TargetKindStandalone,
				})
			}
		}
	}

	sort.Slice(targets, func(i, j int) bool {
		return targets[i].ManifestPath < targets[j].ManifestPath
	})
	return targets, nil
}

func filterTargetsByRequestedPaths(targets []analysis.AnalysisTarget, requested []string) ([]analysis.AnalysisTarget, error) {
	if len(requested) == 0 {
		return append([]analysis.AnalysisTarget(nil), targets...), nil
	}

	index := map[string]analysis.AnalysisTarget{}
	for _, target := range targets {
		index[target.ManifestPath] = target
	}
	selected := make([]analysis.AnalysisTarget, 0, len(requested))
	seen := map[string]struct{}{}
	for _, raw := range requested {
		manifestPath := normalizeRequestedManifestPath(raw)
		target, ok := index[manifestPath]
		if !ok {
			return nil, fmt.Errorf("unknown npm target path %q", raw)
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

func selectChangedTargets(targets []analysis.AnalysisTarget, files []ghclient.PullRequestFile) []analysis.AnalysisTarget {
	changed := map[string]struct{}{}
	for _, file := range files {
		changed[normalizeRepoPath(file.Filename)] = struct{}{}
	}
	selected := make([]analysis.AnalysisTarget, 0, len(targets))
	for _, target := range targets {
		if _, ok := changed[target.ManifestPath]; ok {
			selected = append(selected, target)
			continue
		}
		if _, ok := changed[target.LockfilePath]; ok {
			selected = append(selected, target)
		}
	}
	return selected
}

func formatTargets(targets []analysis.AnalysisTarget) string {
	if len(targets) == 0 {
		return "no supported npm targets detected\n"
	}
	lines := make([]string, 0, len(targets))
	for _, target := range targets {
		line := fmt.Sprintf("%s\t%s\t%s\t%s", target.Kind, target.DisplayName, target.ManifestPath, target.LockfilePath)
		if target.WorkspaceRootPath != "" {
			line += "\t" + target.WorkspaceRootPath
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n") + "\n"
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

func pathSet(paths []string) map[string]struct{} {
	set := make(map[string]struct{}, len(paths))
	for _, filePath := range paths {
		set[filePath] = struct{}{}
	}
	return set
}

func manifestDir(manifestPath string) string {
	cleaned := normalizeRepoPath(manifestPath)
	if cleaned == "package.json" {
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
	for _, pattern := range patterns {
		matched, err := path.Match(pattern, relative)
		if err == nil && matched {
			return true
		}
	}
	return false
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
