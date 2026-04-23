package npm

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Lockfile struct {
	Manager         string
	LockfileVersion int
	Packages        map[string]LockPackage
	Importers       map[string]LockImporter
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
	WorkspaceLocal   bool
}

type LockImporter struct {
	Dependencies         map[string]LockDependency
	DevDependencies      map[string]LockDependency
	OptionalDependencies map[string]LockDependency
}

type LockDependency struct {
	Specifier      string
	Version        string
	WorkspaceLocal bool
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

type UnsupportedYarnFallbackError struct {
	Reason string
}

func (e UnsupportedYarnFallbackError) Error() string {
	if strings.TrimSpace(e.Reason) == "" {
		return "yarn local fallback is unsupported for this lockfile"
	}
	return e.Reason
}

func IsUnsupportedYarnFallback(err error) bool {
	var unsupported UnsupportedYarnFallbackError
	return errors.As(err, &unsupported)
}

func ParseLockfile(data []byte) (*Lockfile, error) {
	if len(data) == 0 {
		return nil, nil
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, nil
	}
	if strings.HasPrefix(trimmed, "{") {
		return parseNPMLockfile(data)
	}
	if looksLikePNPMLockfile(trimmed) {
		return parsePNPMLockfile(data)
	}
	return parseYarnLockfile(data)
}

func parseNPMLockfile(data []byte) (*Lockfile, error) {

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
		Manager:         "npm",
		LockfileVersion: raw.LockfileVersion,
		Packages:        map[string]LockPackage{},
		Importers:       map[string]LockImporter{},
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

func parsePNPMLockfile(data []byte) (*Lockfile, error) {
	var raw struct {
		LockfileVersion any `yaml:"lockfileVersion"`
		Importers       map[string]struct {
			Dependencies         map[string]pnpmDependency `yaml:"dependencies"`
			DevDependencies      map[string]pnpmDependency `yaml:"devDependencies"`
			OptionalDependencies map[string]pnpmDependency `yaml:"optionalDependencies"`
		} `yaml:"importers"`
		Packages map[string]struct {
			Version      string            `yaml:"version"`
			Dependencies map[string]string `yaml:"dependencies"`
			OS           []string          `yaml:"os"`
			CPU          []string          `yaml:"cpu"`
			Resolution   struct {
				Integrity string `yaml:"integrity"`
				Tarball   string `yaml:"tarball"`
				Repo      string `yaml:"repo"`
				Type      string `yaml:"type"`
			} `yaml:"resolution"`
		} `yaml:"packages"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse pnpm-lock.yaml: %w", err)
	}

	lockfile := &Lockfile{
		Manager:   "pnpm",
		Packages:  map[string]LockPackage{},
		Importers: map[string]LockImporter{},
	}
	for importerPath, importer := range raw.Importers {
		key := normalizeImporterPath(importerPath)
		lockfile.Importers[key] = LockImporter{
			Dependencies:         normalizePNPMDependencies(importer.Dependencies),
			DevDependencies:      normalizePNPMDependencies(importer.DevDependencies),
			OptionalDependencies: normalizePNPMDependencies(importer.OptionalDependencies),
		}
	}
	for rawPath, pkg := range raw.Packages {
		name, version := parsePNPMPackageKey(rawPath)
		lockfile.Packages[normalizePNPMPackagePath(rawPath)] = LockPackage{
			Path:             normalizePNPMPackagePath(rawPath),
			Name:             name,
			Version:          coalesceVersion(pkg.Version, version),
			Resolved:         pnpmResolved(pkg.Resolution.Tarball, pkg.Resolution.Repo, pkg.Resolution.Type),
			Integrity:        pkg.Resolution.Integrity,
			OS:               append([]string(nil), pkg.OS...),
			CPU:              append([]string(nil), pkg.CPU...),
			Dependencies:     cloneMap(pkg.Dependencies),
			WorkspaceLocal:   strings.HasPrefix(pkg.Version, "link:") || strings.HasPrefix(pkg.Version, "workspace:"),
			HasInstallScript: false,
		}
	}
	return lockfile, nil
}

func parseYarnLockfile(data []byte) (*Lockfile, error) {
	normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
	trimmed := strings.TrimSpace(normalized)
	switch {
	case strings.Contains(trimmed, "__metadata:"):
		return nil, UnsupportedYarnFallbackError{Reason: "Yarn Berry / Plug'n'Play lockfiles are not supported for local fallback in this release; rely on GitHub dependency review or use a Yarn Classic node_modules lockfile"}
	case !strings.Contains(trimmed, "yarn lockfile v1"):
		return nil, UnsupportedYarnFallbackError{Reason: "unsupported yarn.lock format for local fallback; only Yarn Classic v1 lockfiles are supported in this release"}
	}

	lockfile := &Lockfile{
		Manager:   "yarn",
		Packages:  map[string]LockPackage{},
		Importers: map[string]LockImporter{},
	}
	seen := map[string]string{}
	lines := strings.Split(normalized, "\n")
	for i := 0; i < len(lines); {
		line := strings.TrimRight(lines[i], " \t")
		trimmedLine := strings.TrimSpace(line)
		switch {
		case trimmedLine == "", strings.HasPrefix(trimmedLine, "#"):
			i++
			continue
		case strings.HasPrefix(line, " "):
			i++
			continue
		case !strings.HasSuffix(trimmedLine, ":"):
			i++
			continue
		}

		selectors := strings.TrimSuffix(trimmedLine, ":")
		name := yarnPackageName(selectors)
		if name == "" {
			i++
			continue
		}

		entry := LockPackage{
			Name:         name,
			Dependencies: map[string]string{},
		}
		i++
		inDependencies := false
		for i < len(lines) {
			childLine := strings.TrimRight(lines[i], " \t")
			trimmedChild := strings.TrimSpace(childLine)
			if trimmedChild == "" {
				i++
				continue
			}
			if !strings.HasPrefix(childLine, "  ") {
				break
			}
			if strings.HasPrefix(childLine, "    ") && inDependencies {
				depName, depVersion, ok := parseYarnDependencyLine(trimmedChild)
				if ok {
					entry.Dependencies[depName] = depVersion
				}
				i++
				continue
			}

			inDependencies = false
			switch {
			case strings.HasPrefix(trimmedChild, "version "):
				entry.Version = yarnPropertyValue(trimmedChild, "version")
			case strings.HasPrefix(trimmedChild, "resolved "):
				entry.Resolved = yarnPropertyValue(trimmedChild, "resolved")
			case strings.HasPrefix(trimmedChild, "integrity "):
				entry.Integrity = yarnPropertyValue(trimmedChild, "integrity")
			case strings.HasPrefix(trimmedChild, "dependencies:"):
				inDependencies = true
			}
			i++
		}
		if entry.Version == "" {
			continue
		}
		key := entry.Name + "@" + entry.Version
		if existingPath, ok := seen[key]; ok {
			existing := lockfile.Packages[existingPath]
			for depName, depVersion := range entry.Dependencies {
				existing.Dependencies[depName] = depVersion
			}
			if existing.Resolved == "" {
				existing.Resolved = entry.Resolved
			}
			if existing.Integrity == "" {
				existing.Integrity = entry.Integrity
			}
			lockfile.Packages[existingPath] = existing
			continue
		}
		entry.Path = yarnPackagePath(lockfile.Packages, entry.Name, entry.Version)
		lockfile.Packages[entry.Path] = entry
		seen[key] = entry.Path
	}
	return lockfile, nil
}

type pnpmDependency struct {
	Specifier string `yaml:"specifier"`
	Version   string `yaml:"version"`
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

func looksLikePNPMLockfile(value string) bool {
	return strings.Contains(value, "lockfileVersion:") || strings.Contains(value, "importers:")
}

func yarnPackageName(selectors string) string {
	first := selectors
	if index := strings.Index(first, ","); index >= 0 {
		first = first[:index]
	}
	first = strings.Trim(strings.TrimSpace(first), "\"")
	if index := strings.Index(first, "@npm:"); index > 0 {
		return first[:index]
	}
	if strings.HasPrefix(first, "@") {
		slash := strings.Index(first, "/")
		if slash >= 0 {
			rest := first[slash+1:]
			if offset := strings.LastIndex(rest, "@"); offset >= 0 {
				return first[:slash+1+offset]
			}
		}
	}
	if index := strings.LastIndex(first, "@"); index > 0 {
		return first[:index]
	}
	return first
}

func yarnPropertyValue(line, key string) string {
	value := strings.TrimSpace(strings.TrimPrefix(line, key))
	return strings.Trim(value, "\"")
}

func parseYarnDependencyLine(line string) (string, string, bool) {
	parts := strings.SplitN(line, " ", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.Trim(strings.TrimSpace(parts[1]), "\""), true
}

func yarnPackagePath(existing map[string]LockPackage, name, version string) string {
	canonical := joinNodeModules("", name)
	if _, ok := existing[canonical]; !ok {
		return canonical
	}
	candidate := path.Clean("node_modules/.yarn/" + strings.ReplaceAll(strings.TrimPrefix(name, "@"), "/", "+") + "@" + version)
	if _, ok := existing[candidate]; !ok {
		return candidate
	}
	for index := 2; ; index++ {
		next := fmt.Sprintf("%s#%d", candidate, index)
		if _, ok := existing[next]; !ok {
			return next
		}
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

func (l *Lockfile) importerForTarget(targetDir string) (LockImporter, bool) {
	if l == nil {
		return LockImporter{}, false
	}
	key := normalizeImporterPath(targetDir)
	importer, ok := l.Importers[key]
	return importer, ok
}

func (l *Lockfile) collectPNPMTargetPackages(targetDir string, directNames []string) TargetPackages {
	result := TargetPackages{
		Direct:     map[string]LockPackage{},
		All:        map[string]LockPackage{},
		Transitive: map[string]LockPackage{},
	}
	importer, ok := l.importerForTarget(targetDir)
	if !ok {
		return result
	}

	queue := make([]LockPackage, 0, len(directNames))
	for _, name := range directNames {
		dependency, ok := importer.lookup(name)
		if !ok {
			continue
		}
		pkg, found, approximate := l.resolvePNPMPackage(name, dependency.Version)
		if !found {
			continue
		}
		if approximate {
			result.Approximate = true
		}
		result.Direct[name] = pkg
		if pkg.WorkspaceLocal {
			result.All[pkg.Path] = pkg
			continue
		}
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
			dep, ok, approximate := l.resolvePNPMPackage(depName, pkg.Dependencies[depName])
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

func (l *Lockfile) resolvePNPMPackage(name, versionRef string) (LockPackage, bool, bool) {
	if strings.TrimSpace(name) == "" {
		return LockPackage{}, false, false
	}
	if isPNPMWorkspaceLink(versionRef) {
		return LockPackage{
			Path:           "workspace:" + name,
			Name:           name,
			Version:        versionRef,
			WorkspaceLocal: true,
			Dependencies:   map[string]string{},
		}, true, false
	}
	if versionRef != "" {
		exactKey := normalizePNPMPackagePath(name + "@" + versionRef)
		if pkg, ok := l.Packages[exactKey]; ok {
			return pkg, true, false
		}
	}

	normalizedVersion := normalizePNPMVersion(versionRef)
	matches := make([]LockPackage, 0)
	for _, pkg := range l.Packages {
		if pkg.Name != name {
			continue
		}
		if normalizedVersion != "" && normalizePNPMVersion(pkg.Version) != normalizedVersion {
			continue
		}
		matches = append(matches, pkg)
	}
	if len(matches) == 0 {
		return LockPackage{}, false, false
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Path < matches[j].Path
	})
	return matches[0], true, len(matches) > 1 || versionRef == ""
}

func normalizePNPMDependencies(source map[string]pnpmDependency) map[string]LockDependency {
	if source == nil {
		return map[string]LockDependency{}
	}
	result := make(map[string]LockDependency, len(source))
	for name, dep := range source {
		result[name] = LockDependency{
			Specifier:      dep.Specifier,
			Version:        dep.Version,
			WorkspaceLocal: isPNPMWorkspaceLink(dep.Version) || strings.HasPrefix(dep.Specifier, "workspace:"),
		}
	}
	return result
}

func (i LockImporter) lookup(name string) (LockDependency, bool) {
	if dep, ok := i.Dependencies[name]; ok {
		return dep, true
	}
	if dep, ok := i.OptionalDependencies[name]; ok {
		return dep, true
	}
	if dep, ok := i.DevDependencies[name]; ok {
		return dep, true
	}
	return LockDependency{}, false
}

func (i LockImporter) allDependencies() map[string]LockDependency {
	result := map[string]LockDependency{}
	for name, dep := range i.Dependencies {
		result[name] = dep
	}
	for name, dep := range i.OptionalDependencies {
		result[name] = dep
	}
	for name, dep := range i.DevDependencies {
		result[name] = dep
	}
	return result
}

func (i LockImporter) requirements() map[string]string {
	result := map[string]string{}
	for name, dep := range i.Dependencies {
		result[name] = dep.Version
	}
	for name, dep := range i.OptionalDependencies {
		result[name] = dep.Version
	}
	for name, dep := range i.DevDependencies {
		result[name] = dep.Version
	}
	return result
}

func normalizeImporterPath(value string) string {
	cleaned := cleanLockPath(value)
	if cleaned == "" {
		return "."
	}
	return cleaned
}

func normalizePNPMPackagePath(value string) string {
	return strings.TrimPrefix(cleanLockPath(value), "/")
}

func parsePNPMPackageKey(key string) (string, string) {
	cleaned := normalizePNPMPackagePath(key)
	if cleaned == "" {
		return "", ""
	}
	versionStart := strings.Index(cleaned, "@")
	if strings.HasPrefix(cleaned, "@") {
		slash := strings.Index(cleaned, "/")
		if slash >= 0 {
			rest := cleaned[slash+1:]
			offset := strings.Index(rest, "@")
			if offset >= 0 {
				versionStart = slash + 1 + offset
			}
		}
	}
	if versionStart <= 0 || versionStart >= len(cleaned)-1 {
		return cleaned, ""
	}
	return cleaned[:versionStart], normalizePNPMVersion(cleaned[versionStart+1:])
}

func normalizePNPMVersion(version string) string {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return ""
	}
	if index := strings.Index(trimmed, "("); index >= 0 {
		return trimmed[:index]
	}
	return trimmed
}

func coalesceVersion(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func isPNPMWorkspaceLink(version string) bool {
	lower := strings.ToLower(strings.TrimSpace(version))
	return strings.HasPrefix(lower, "link:") || strings.HasPrefix(lower, "workspace:")
}

func pnpmResolved(tarball, repo, resolutionType string) string {
	switch {
	case strings.TrimSpace(tarball) != "":
		return tarball
	case strings.TrimSpace(repo) != "":
		return repo
	case strings.TrimSpace(resolutionType) == "directory":
		return "workspace-local"
	default:
		return ""
	}
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
