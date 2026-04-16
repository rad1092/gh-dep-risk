package npm

import (
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
)

type Lockfile struct {
	LockfileVersion int
	Packages        map[string]LockPackage
}

type LockPackage struct {
	Path             string
	Name             string
	Version          string
	Resolved         string
	Integrity        string
	HasInstallScript bool
	Dev              bool
	Optional         bool
	DevOptional      bool
	OS               []string
	CPU              []string
	Dependencies     map[string]string
}

type TargetPackages struct {
	Direct      map[string]LockPackage
	All         map[string]LockPackage
	Transitive  map[string]LockPackage
	Approximate bool
}

type SourceKind string

const (
	SourceDefaultRegistry SourceKind = "default_registry"
	SourceOtherRegistry   SourceKind = "other_registry"
	SourceGit             SourceKind = "git"
	SourceUnknown         SourceKind = "unknown"
)

func ParseLockfile(data []byte) (*Lockfile, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var raw struct {
		LockfileVersion int `json:"lockfileVersion"`
		Packages        map[string]struct {
			Name             string            `json:"name"`
			Version          string            `json:"version"`
			Resolved         string            `json:"resolved"`
			Integrity        string            `json:"integrity"`
			HasInstallScript bool              `json:"hasInstallScript"`
			Dev              bool              `json:"dev"`
			Optional         bool              `json:"optional"`
			DevOptional      bool              `json:"devOptional"`
			OS               []string          `json:"os"`
			CPU              []string          `json:"cpu"`
			Dependencies     map[string]string `json:"dependencies"`
		} `json:"packages"`
		Dependencies map[string]legacyDependency `json:"dependencies"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse package-lock.json: %w", err)
	}

	lockfile := &Lockfile{
		LockfileVersion: raw.LockfileVersion,
		Packages:        map[string]LockPackage{},
	}

	if len(raw.Packages) > 0 {
		for path, entry := range raw.Packages {
			name := entry.Name
			if name == "" && path != "" {
				name = PackageNameFromPath(path)
			}
			lockfile.Packages[path] = LockPackage{
				Path:             path,
				Name:             name,
				Version:          entry.Version,
				Resolved:         entry.Resolved,
				Integrity:        entry.Integrity,
				HasInstallScript: entry.HasInstallScript,
				Dev:              entry.Dev,
				Optional:         entry.Optional,
				DevOptional:      entry.DevOptional,
				OS:               append([]string(nil), entry.OS...),
				CPU:              append([]string(nil), entry.CPU...),
				Dependencies:     cloneMap(entry.Dependencies),
			}
		}
		return lockfile, nil
	}

	flattenLegacyDependencies(lockfile.Packages, "", raw.Dependencies)
	return lockfile, nil
}

type legacyDependency struct {
	Version          string                      `json:"version"`
	Resolved         string                      `json:"resolved"`
	Integrity        string                      `json:"integrity"`
	HasInstallScript bool                        `json:"hasInstallScript"`
	Dev              bool                        `json:"dev"`
	Optional         bool                        `json:"optional"`
	DevOptional      bool                        `json:"devOptional"`
	OS               []string                    `json:"os"`
	CPU              []string                    `json:"cpu"`
	Requires         map[string]string           `json:"requires"`
	Dependencies     map[string]legacyDependency `json:"dependencies"`
}

func flattenLegacyDependencies(target map[string]LockPackage, parent string, deps map[string]legacyDependency) {
	for name, dep := range deps {
		path := "node_modules/" + name
		if parent != "" {
			path = parent + "/node_modules/" + name
		}
		target[path] = LockPackage{
			Path:             path,
			Name:             name,
			Version:          dep.Version,
			Resolved:         dep.Resolved,
			Integrity:        dep.Integrity,
			HasInstallScript: dep.HasInstallScript,
			Dev:              dep.Dev,
			Optional:         dep.Optional,
			DevOptional:      dep.DevOptional,
			OS:               append([]string(nil), dep.OS...),
			CPU:              append([]string(nil), dep.CPU...),
			Dependencies:     cloneMap(dep.Requires),
		}
		flattenLegacyDependencies(target, path, dep.Dependencies)
	}
}

func cloneMap(source map[string]string) map[string]string {
	if source == nil {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func PackageNameFromPath(path string) string {
	if path == "" {
		return ""
	}
	index := strings.LastIndex(path, "node_modules/")
	if index < 0 {
		return ""
	}
	return path[index+len("node_modules/"):]
}

func IsTopLevelPackagePath(path string) bool {
	if !strings.HasPrefix(path, "node_modules/") {
		return false
	}
	trimmed := strings.TrimPrefix(path, "node_modules/")
	return !strings.Contains(trimmed, "/node_modules/")
}

func (l *Lockfile) TopLevelPackages() map[string]LockPackage {
	result := map[string]LockPackage{}
	if l == nil {
		return result
	}
	for path, pkg := range l.Packages {
		if IsTopLevelPackagePath(path) {
			result[pkg.Name] = pkg
		}
	}
	return result
}

func (l *Lockfile) FindByName(name string) []LockPackage {
	if l == nil {
		return nil
	}
	packages := make([]LockPackage, 0)
	for _, pkg := range l.Packages {
		if pkg.Name == name {
			packages = append(packages, pkg)
		}
	}
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Path < packages[j].Path
	})
	return packages
}

func (l *Lockfile) AddedTransitiveCount(base *Lockfile, directNames map[string]struct{}) int {
	if l == nil {
		return 0
	}
	if base == nil {
		base = &Lockfile{Packages: map[string]LockPackage{}}
	}
	count := 0
	for path, pkg := range l.Packages {
		if path == "" || IsTopLevelPackagePath(path) {
			continue
		}
		if _, ok := directNames[pkg.Name]; ok {
			continue
		}
		if _, ok := base.Packages[path]; ok {
			continue
		}
		count++
	}
	return count
}

func (l *Lockfile) PackageAt(packagePath string) (LockPackage, bool) {
	if l == nil {
		return LockPackage{}, false
	}
	pkg, ok := l.Packages[cleanLockPath(packagePath)]
	return pkg, ok
}

func (l *Lockfile) TargetRootDependencies(targetDir string) map[string]string {
	if l == nil {
		return map[string]string{}
	}
	if pkg, ok := l.PackageAt(targetDir); ok {
		return cloneMap(pkg.Dependencies)
	}
	return map[string]string{}
}

func (l *Lockfile) ResolvePackage(basePath, name string) (LockPackage, bool, bool) {
	if l == nil || strings.TrimSpace(name) == "" {
		return LockPackage{}, false, false
	}
	for _, base := range resolutionBases(basePath) {
		candidate := joinNodeModules(base, name)
		if pkg, ok := l.Packages[candidate]; ok {
			return pkg, true, false
		}
	}
	packages := l.FindByName(name)
	if len(packages) == 0 {
		return LockPackage{}, false, false
	}
	return packages[0], true, true
}

func (l *Lockfile) CollectTargetPackages(targetDir string, directNames []string) TargetPackages {
	result := TargetPackages{
		Direct:     map[string]LockPackage{},
		All:        map[string]LockPackage{},
		Transitive: map[string]LockPackage{},
	}
	if l == nil {
		return result
	}

	names := append([]string(nil), directNames...)
	sort.Strings(names)
	queue := make([]LockPackage, 0, len(names))
	for _, name := range names {
		pkg, ok, approximate := l.ResolvePackage(targetDir, name)
		if !ok {
			continue
		}
		if approximate {
			result.Approximate = true
		}
		result.Direct[name] = pkg
		if _, seen := result.All[pkg.Path]; seen {
			continue
		}
		result.All[pkg.Path] = pkg
		queue = append(queue, pkg)
	}

	for len(queue) > 0 {
		pkg := queue[0]
		queue = queue[1:]
		for _, depName := range sortedDependencyNames(pkg.Dependencies) {
			dep, ok, approximate := l.ResolvePackage(pkg.Path, depName)
			if !ok {
				continue
			}
			if approximate {
				result.Approximate = true
			}
			if _, seen := result.All[dep.Path]; seen {
				continue
			}
			result.All[dep.Path] = dep
			queue = append(queue, dep)
		}
	}

	for _, pkgPath := range sortedDependencyNames(pathsAsDependencyMap(l.Packages)) {
		pkg := l.Packages[pkgPath]
		if _, seen := result.All[pkg.Path]; seen {
			continue
		}
		for existingPath := range result.All {
			if strings.HasPrefix(pkg.Path, existingPath+"/node_modules/") {
				result.All[pkg.Path] = pkg
				break
			}
		}
	}

	directPaths := map[string]struct{}{}
	for _, pkg := range result.Direct {
		directPaths[pkg.Path] = struct{}{}
	}
	for pkgPath, pkg := range result.All {
		if _, ok := directPaths[pkgPath]; ok {
			continue
		}
		result.Transitive[pkgPath] = pkg
	}
	return result
}

func (l *Lockfile) AddedTransitiveCountForTarget(base *Lockfile, targetDir string, directNames []string) (int, bool) {
	paths, approximate := l.AddedTransitivePathsForTarget(base, targetDir, directNames)
	return len(paths), approximate
}

func (l *Lockfile) AddedTransitivePathsForTarget(base *Lockfile, targetDir string, directNames []string) ([]string, bool) {
	headView := l.CollectTargetPackages(targetDir, directNames)
	if base == nil {
		paths := sortedPackagePaths(headView.Transitive)
		return paths, headView.Approximate
	}
	baseView := base.CollectTargetPackages(targetDir, directNames)
	added := make([]string, 0, len(headView.Transitive))
	for pkgPath := range headView.Transitive {
		if _, ok := baseView.Transitive[pkgPath]; ok {
			continue
		}
		added = append(added, pkgPath)
	}
	sort.Strings(added)
	return added, headView.Approximate || baseView.Approximate
}

func StripVersionPrefix(version string) string {
	trimmed := strings.TrimSpace(version)
	for len(trimmed) > 0 {
		r := trimmed[0]
		if (r >= '0' && r <= '9') || r == 'v' {
			break
		}
		trimmed = trimmed[1:]
	}
	return strings.TrimPrefix(trimmed, "v")
}

func MajorVersion(version string) (int, bool) {
	trimmed := StripVersionPrefix(version)
	if trimmed == "" {
		return 0, false
	}
	end := strings.IndexAny(trimmed, ".-+")
	if end == -1 {
		end = len(trimmed)
	}
	value := trimmed[:end]
	if value == "" {
		return 0, false
	}
	var major int
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return 0, false
		}
		major = major*10 + int(value[i]-'0')
	}
	return major, true
}

func IsRegistrySource(resolved string) bool {
	return DetectSourceKind(resolved) == SourceDefaultRegistry
}

func DetectSourceKind(resolved string) SourceKind {
	if resolved == "" {
		return SourceDefaultRegistry
	}
	lower := strings.ToLower(resolved)
	if strings.HasPrefix(lower, "git+") || strings.HasPrefix(lower, "git://") || strings.HasPrefix(lower, "github:") {
		return SourceGit
	}
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		if strings.Contains(lower, "registry.npmjs.org") {
			return SourceDefaultRegistry
		}
		return SourceOtherRegistry
	}
	return SourceUnknown
}

func DescribeSource(resolved string) string {
	switch DetectSourceKind(resolved) {
	case SourceGit:
		return "git source: " + resolved
	case SourceOtherRegistry:
		return "non-default registry: " + resolved
	case SourceUnknown:
		return "non-default source: " + resolved
	default:
		return resolved
	}
}

func cleanLockPath(value string) string {
	cleaned := path.Clean(strings.TrimSpace(strings.ReplaceAll(value, "\\", "/")))
	switch cleaned {
	case ".", "/":
		return ""
	default:
		return strings.TrimPrefix(cleaned, "./")
	}
}

func sortedDependencyNames(deps map[string]string) []string {
	names := make([]string, 0, len(deps))
	for name := range deps {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedPackagePaths(packages map[string]LockPackage) []string {
	paths := make([]string, 0, len(packages))
	for pkgPath := range packages {
		paths = append(paths, pkgPath)
	}
	sort.Strings(paths)
	return paths
}

func pathsAsDependencyMap(packages map[string]LockPackage) map[string]string {
	paths := make(map[string]string, len(packages))
	for pkgPath := range packages {
		paths[pkgPath] = pkgPath
	}
	return paths
}

func joinNodeModules(basePath, name string) string {
	cleanBase := cleanLockPath(basePath)
	if cleanBase == "" {
		return "node_modules/" + name
	}
	return cleanBase + "/node_modules/" + name
}

func resolutionBases(basePath string) []string {
	current := cleanLockPath(basePath)
	result := make([]string, 0, 8)
	seen := map[string]struct{}{}
	for {
		if _, ok := seen[current]; !ok {
			seen[current] = struct{}{}
			result = append(result, current)
		}
		if current == "" {
			break
		}
		if stripped, ok := stripLastNodeModulesSegment(current); ok {
			current = stripped
			continue
		}
		parent := path.Dir(current)
		switch parent {
		case ".", "/":
			current = ""
		default:
			current = parent
		}
	}
	return result
}

func stripLastNodeModulesSegment(value string) (string, bool) {
	const marker = "/node_modules/"
	cleaned := cleanLockPath(value)
	index := strings.LastIndex(cleaned, marker)
	if index < 0 {
		if strings.HasPrefix(cleaned, "node_modules/") {
			return "", true
		}
		return "", false
	}
	return strings.Trim(cleaned[:index], "/"), true
}
