package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rad1092/gh-dependency-risk/internal/analysis"
	"github.com/rad1092/gh-dependency-risk/internal/npm"
)

func gitRepo(t *testing.T, files map[string]string) (string, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "-q")
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("add", "-A")
	run("commit", "-q", "-m", "init")
	return dir, run("rev-parse", "HEAD")
}

func TestGitImportFinderFindsImportsAndRequires(t *testing.T) {
	dir, head := gitRepo(t, map[string]string{
		"apps/web/src/App.tsx":             "import { BrowserRouter } from \"react-router-dom\";\n",
		"apps/web/src/lazy.ts":             "const r = await import('react-router-dom/server');\n",
		"apps/web/src/legacy.js":           "const router = require( 'react-router-dom' );\n",
		"apps/web/src/other.ts":            "import x from 'react-router-dom-extra';\n",
		"apps/web/node_modules/a/index.js": "require('react-router-dom')\n",
		"packages/ui/src/index.ts":         "import 'react-router-dom';\n",
		"apps/web/README.md":               "import from 'react-router-dom'\n",
	})
	finder := GitImportFinder{Dir: dir}
	if !finder.HasCommit(context.Background(), head) || finder.HasCommit(context.Background(), strings.Repeat("0", 40)) {
		t.Fatal("HasCommit should only report commits in the clone")
	}
	files, err := finder.FilesImporting(context.Background(), head, "apps/web", "react-router-dom", 5)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"apps/web/src/App.tsx", "apps/web/src/lazy.ts", "apps/web/src/legacy.js"}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("files = %v, want %v", files, want)
	}
	files, err = finder.FilesImporting(context.Background(), head, "", "react-router-dom", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("limit not applied: %v", files)
	}
	files, err = finder.FilesImporting(context.Background(), head, "", "left-pad", 5)
	if err != nil || len(files) != 0 {
		t.Fatalf("files = %v, err = %v", files, err)
	}
}

type fakeImportFinder struct {
	commits map[string]bool
	calls   []string
}

func (f *fakeImportFinder) HasCommit(_ context.Context, commit string) bool { return f.commits[commit] }

func (f *fakeImportFinder) FilesImporting(_ context.Context, commit, dir, name string, _ int) ([]string, error) {
	f.calls = append(f.calls, commit+" "+dir+" "+name)
	return []string{dir + "/src/" + name + ".ts"}, nil
}

func TestAddImportedBySearchesTheBaseWhenTheHeadIsMissing(t *testing.T) {
	results := []analysis.TargetAnalysisResult{{
		Target: analysis.AnalysisTarget{DisplayName: "apps/web", ManifestPath: "apps/web/package.json", PackageManager: "npm"},
		ChangedDependencies: []analysis.DependencyChange{
			{Name: "updated", ChangeType: analysis.ChangeUpdated, Direct: true},
			{Name: "added", ChangeType: analysis.ChangeAdded, Direct: true},
			{Name: "transitive", ChangeType: analysis.ChangeUpdated},
		},
	}, {
		Target:              analysis.AnalysisTarget{DisplayName: "service", ManifestPath: "service/go.mod", PackageManager: "go"},
		ChangedDependencies: []analysis.DependencyChange{{Name: "golang.org/x/text", ChangeType: analysis.ChangeUpdated, Direct: true}},
	}}
	finder := &fakeImportFinder{commits: map[string]bool{"base": true}}
	addImportedBy(context.Background(), finder, results, "base", "head")
	if want := []string{"base apps/web updated"}; !reflect.DeepEqual(finder.calls, want) {
		t.Fatalf("calls = %v", finder.calls)
	}
	if got := results[0].ChangedDependencies[0].ImportedBy; !reflect.DeepEqual(got, []string{"apps/web/src/updated.ts"}) {
		t.Fatalf("ImportedBy = %v", got)
	}

	finder = &fakeImportFinder{commits: map[string]bool{}}
	addImportedBy(context.Background(), finder, results, "base", "head")
	if len(finder.calls) != 0 {
		t.Fatalf("a clone without the PR's commits should not be searched: %v", finder.calls)
	}
}

type fakeFactsRegistry struct {
	lookups []string
}

func (f *fakeFactsRegistry) PublishedAt(context.Context, string, string) (time.Time, error) {
	panic("PublishedAt should not be used when VersionFacts is available")
}

func (f *fakeFactsRegistry) VersionFacts(_ context.Context, name, version, previous string) (npm.VersionFacts, error) {
	f.lookups = append(f.lookups, name+"@"+version+"<"+previous)
	return npm.VersionFacts{Published: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Publisher: "b", HasPrevious: previous != "", PreviousPublisher: "a"}, nil
}

func TestLookupRegistryFactsUsesOneLookupPerVersion(t *testing.T) {
	input := analysis.Input{
		Target:       analysis.AnalysisTarget{DisplayName: "root", ManifestPath: "package.json", LockfilePath: "package-lock.json", PackageManager: "npm"},
		BaseManifest: &npm.PackageManifest{Dependencies: map[string]string{"widget": "^1.0.0"}},
		HeadManifest: &npm.PackageManifest{Dependencies: map[string]string{"widget": "^1.0.0"}},
		BaseLockfile: &npm.Lockfile{Manager: "npm", Packages: map[string]npm.LockPackage{"node_modules/widget": {Path: "node_modules/widget", Name: "widget", Version: "1.0.0"}}},
		HeadLockfile: &npm.Lockfile{Manager: "npm", Packages: map[string]npm.LockPackage{"node_modules/widget": {Path: "node_modules/widget", Name: "widget", Version: "1.0.1"}}},
	}
	registry := &fakeFactsRegistry{}
	published := map[analysis.PackageVersion]time.Time{}
	facts := lookupRegistryFacts(context.Background(), registry, []analysis.Input{input, input}, published)
	if want := []string{"widget@1.0.1<1.0.0"}; !reflect.DeepEqual(registry.lookups, want) {
		t.Fatalf("lookups = %v", registry.lookups)
	}
	key := analysis.PackageVersion{Name: "widget", Version: "1.0.1"}
	if !facts[key].HasPrevious || published[key].IsZero() {
		t.Fatalf("facts = %+v, published = %v", facts, published)
	}
	input.Facts = facts
	drivers := analysis.Analyze(input, published).ChangedDependencies[0].RiskDrivers
	if !containsString(drivers, analysis.DriverPublisherChanged) {
		t.Fatalf("drivers = %v", drivers)
	}
}
