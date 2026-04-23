package npm

import (
	"path"
	"sort"
	"strings"
)

// The shared JS graph adapter below keeps analysis-facing traversal separate
// from npm- and pnpm-specific parsing details. Manager-specific parsers populate
// Lockfile/LockPackage, and these helpers expose a consistent target graph view
// to internal/analysis.

func (l *Lockfile) TopLevelPackages() map[string]LockPackage {
	result := map[string]LockPackage{}
	if l == nil {
		return result
	}
	if l.Manager == "pnpm" {
		for _, importer := range l.Importers {
			for name, dep := range importer.allDependencies() {
				pkg, ok, _ := l.resolvePNPMPackage(name, dep.Version)
				if !ok {
					continue
				}
				result[name] = pkg
			}
		}
		return result
	}
	for pkgPath, pkg := range l.Packages {
		if IsTopLevelPackagePath(pkgPath) {
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
	for pkgPath, pkg := range l.Packages {
		if pkgPath == "" || IsTopLevelPackagePath(pkgPath) {
			continue
		}
		if _, ok := directNames[pkg.Name]; ok {
			continue
		}
		if _, ok := base.Packages[pkgPath]; ok {
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
	if l.Manager == "pnpm" {
		if importer, ok := l.importerForTarget(targetDir); ok {
			return importer.requirements()
		}
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
	if l.Manager == "pnpm" {
		return l.resolvePNPMPackage(name, "")
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
	if l.Manager == "pnpm" {
		return l.collectPNPMTargetPackages(targetDir, directNames)
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
		if pkg.WorkspaceLocal {
			continue
		}
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
			if dep.WorkspaceLocal {
				continue
			}
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
