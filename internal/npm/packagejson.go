package npm

import (
	"encoding/json"
	"fmt"
	"sort"
)

type PackageManifest struct {
	Name                 string            `json:"name"`
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
}

func ParsePackageManifest(data []byte) (*PackageManifest, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var manifest PackageManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse package.json: %w", err)
	}
	if manifest.Dependencies == nil {
		manifest.Dependencies = map[string]string{}
	}
	if manifest.DevDependencies == nil {
		manifest.DevDependencies = map[string]string{}
	}
	if manifest.OptionalDependencies == nil {
		manifest.OptionalDependencies = map[string]string{}
	}
	return &manifest, nil
}

func (m *PackageManifest) Scope(name string) (string, bool) {
	if m == nil {
		return "", false
	}
	if _, ok := m.Dependencies[name]; ok {
		return "runtime", true
	}
	if _, ok := m.OptionalDependencies[name]; ok {
		return "optional", true
	}
	if _, ok := m.DevDependencies[name]; ok {
		return "dev", true
	}
	return "", false
}

func (m *PackageManifest) Requirement(name string) string {
	if m == nil {
		return ""
	}
	if value, ok := m.Dependencies[name]; ok {
		return value
	}
	if value, ok := m.OptionalDependencies[name]; ok {
		return value
	}
	if value, ok := m.DevDependencies[name]; ok {
		return value
	}
	return ""
}

func (m *PackageManifest) DirectNames() []string {
	if m == nil {
		return nil
	}
	set := map[string]struct{}{}
	for name := range m.Dependencies {
		set[name] = struct{}{}
	}
	for name := range m.DevDependencies {
		set[name] = struct{}{}
	}
	for name := range m.OptionalDependencies {
		set[name] = struct{}{}
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
