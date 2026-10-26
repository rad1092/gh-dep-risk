package app

import (
	"bytes"
	"context"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/rad1092/gh-dependency-risk/internal/analysis"
)

// ImportFinder lists source files that import a package at a commit.
type ImportFinder interface {
	FilesImporting(ctx context.Context, commit, dir, packageName string, limit int) ([]string, error)
}

const (
	importFilesPerPackage = 5
	importPackagesPerRun  = 20
)

// jsSourceGlobs are the files searched for npm imports.
var jsSourceGlobs = []string{"*.js", "*.jsx", "*.mjs", "*.cjs", "*.ts", "*.tsx", "*.mts", "*.cts", "*.vue", "*.svelte", "*.astro"}

// GitImportFinder searches a local clone with git grep. It never fetches.
type GitImportFinder struct {
	Dir string
}

func (f GitImportFinder) FilesImporting(ctx context.Context, commit, dir, packageName string, limit int) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	pattern := `(from|import|require)[[:space:]]*\(?[[:space:]]*['"]` + regexp.QuoteMeta(packageName) + `(['"]|/)`
	args := []string{"-C", f.Dir, "grep", "-l", "-I", "-E", pattern, commit, "--"}
	prefix := ""
	if dir != "" {
		prefix = strings.TrimSuffix(dir, "/") + "/"
	}
	for _, glob := range jsSourceGlobs {
		args = append(args, ":(glob)"+prefix+"**/"+glob)
	}
	args = append(args, ":(exclude,glob)**/node_modules/**", ":(exclude,glob)**/dist/**", ":(exclude,glob)**/*.min.js")
	cmd := exec.CommandContext(ctx, "git", args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		// git grep exits 1 when nothing matches.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}
	files := make([]string, 0)
	for _, line := range strings.Split(strings.TrimSpace(stdout.String()), "\n") {
		line = strings.TrimPrefix(line, commit+":")
		if line == "" {
			continue
		}
		files = append(files, line)
	}
	sort.Strings(files)
	if len(files) > limit {
		files = files[:limit]
	}
	return files, nil
}

// HasCommit reports whether the clone has commit without fetching it.
func (f GitImportFinder) HasCommit(ctx context.Context, commit string) bool {
	if commit == "" {
		return false
	}
	cmd := exec.CommandContext(ctx, "git", "-C", f.Dir, "cat-file", "-e", commit+"^{commit}")
	return cmd.Run() == nil
}

// commitChecker is implemented by finders that can tell whether a commit is
// available locally, so a run outside the right clone does nothing.
type commitChecker interface {
	HasCommit(ctx context.Context, commit string) bool
}

// addImportedBy fills ImportedBy for direct npm-family changes. It searches
// the PR head when the clone has it, and the base otherwise; packages added
// by the PR only exist in the head.
func addImportedBy(ctx context.Context, finder ImportFinder, results []analysis.TargetAnalysisResult, baseSHA, headSHA string) {
	if finder == nil {
		return
	}
	hasHead, hasBase := true, true
	if checker, ok := finder.(commitChecker); ok {
		hasHead = checker.HasCommit(ctx, headSHA)
		hasBase = checker.HasCommit(ctx, baseSHA)
	}
	if !hasHead && !hasBase {
		return
	}
	searched := 0
	for t := range results {
		target := results[t].Target
		if !npmFamilyTarget(target) {
			continue
		}
		for c := range results[t].ChangedDependencies {
			change := &results[t].ChangedDependencies[c]
			if !change.Direct || searched >= importPackagesPerRun {
				continue
			}
			commit := headSHA
			if !hasHead {
				if change.ChangeType == analysis.ChangeAdded {
					continue
				}
				commit = baseSHA
			}
			if change.ChangeType == analysis.ChangeRemoved && hasBase {
				commit = baseSHA
			}
			searched++
			files, err := finder.FilesImporting(ctx, commit, target.Directory(), change.Name, importFilesPerPackage)
			if err != nil || len(files) == 0 {
				continue
			}
			change.ImportedBy = files
		}
	}
}

func npmFamilyTarget(target analysis.AnalysisTarget) bool {
	switch target.PackageManager {
	case "npm", "pnpm", "yarn", "bun":
		return true
	}
	return target.Ecosystem == "npm"
}
