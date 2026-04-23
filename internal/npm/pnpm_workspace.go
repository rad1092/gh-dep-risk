package npm

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

func ParsePNPMWorkspacePatterns(data []byte) ([]string, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var raw struct {
		Packages []string `yaml:"packages"`
	}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse pnpm-workspace.yaml: %w", err)
	}
	return cleanWorkspacePatterns(raw.Packages), nil
}

func MatchWorkspacePatternSet(patterns []string, relative string) bool {
	if relative == "" {
		return false
	}

	matched := false
	for _, pattern := range patterns {
		negated := strings.HasPrefix(pattern, "!")
		glob := strings.TrimPrefix(pattern, "!")
		if glob == "" {
			continue
		}
		ok := matchWorkspaceGlob(glob, relative)
		if !ok {
			continue
		}
		if negated {
			matched = false
			continue
		}
		matched = true
	}
	return matched
}

func matchWorkspaceGlob(pattern, value string) bool {
	patternRunes := []rune(pattern)
	valueRunes := []rune(value)
	type state struct {
		p int
		v int
	}
	memo := map[state]bool{}
	var walk func(int, int) bool
	walk = func(pi, vi int) bool {
		key := state{p: pi, v: vi}
		if cached, ok := memo[key]; ok {
			return cached
		}

		var result bool
		switch {
		case pi == len(patternRunes):
			result = vi == len(valueRunes)
		case patternRunes[pi] == '*':
			if pi+1 < len(patternRunes) && patternRunes[pi+1] == '*' {
				result = walk(pi+2, vi)
				for i := vi; !result && i < len(valueRunes); i++ {
					result = walk(pi+2, i+1)
				}
			} else {
				result = walk(pi+1, vi)
				for i := vi; !result && i < len(valueRunes) && valueRunes[i] != '/'; i++ {
					result = walk(pi+1, i+1)
				}
			}
		case patternRunes[pi] == '?':
			result = vi < len(valueRunes) && valueRunes[vi] != '/' && walk(pi+1, vi+1)
		default:
			result = vi < len(valueRunes) && patternRunes[pi] == valueRunes[vi] && walk(pi+1, vi+1)
		}

		memo[key] = result
		return result
	}
	return walk(0, 0)
}

func normalizeWorkspacePattern(pattern string) string {
	cleaned := strings.TrimSpace(pattern)
	if cleaned == "" {
		return ""
	}
	negated := strings.HasPrefix(cleaned, "!")
	if negated {
		cleaned = strings.TrimSpace(strings.TrimPrefix(cleaned, "!"))
	}
	cleaned = cleanWorkspacePattern(cleaned)
	if cleaned == "" {
		return ""
	}
	if negated {
		return "!" + cleaned
	}
	return cleaned
}

func uniqueWorkspacePatterns(patterns []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		cleaned := normalizeWorkspacePattern(pattern)
		if cleaned == "" {
			continue
		}
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		result = append(result, cleaned)
	}
	sort.Strings(result)
	return result
}
