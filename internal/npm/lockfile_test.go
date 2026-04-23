package npm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseLockfileV3(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "package-lock-v3.json"))
	if err != nil {
		t.Fatal(err)
	}

	lockfile, err := ParseLockfile(data)
	if err != nil {
		t.Fatal(err)
	}

	leftPad, ok := lockfile.TopLevelPackages()["left-pad"]
	if !ok {
		t.Fatalf("expected left-pad top-level package")
	}
	if !leftPad.HasInstallScript {
		t.Fatalf("expected install script flag")
	}
	if !IsTopLevelPackagePath(leftPad.Path) {
		t.Fatalf("expected top-level path")
	}

	tiny := lockfile.FindByName("tiny")
	if len(tiny) != 1 {
		t.Fatalf("expected nested tiny package, got %d", len(tiny))
	}
	if IsRegistrySource(tiny[0].Resolved) {
		t.Fatalf("expected git source to be treated as non-registry")
	}
}

func TestParseLegacyLockfile(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "package-lock-legacy.json"))
	if err != nil {
		t.Fatal(err)
	}

	lockfile, err := ParseLockfile(data)
	if err != nil {
		t.Fatal(err)
	}
	if lockfile.Packages["node_modules/chalk"].Version != "4.1.0" {
		t.Fatalf("unexpected chalk version")
	}
	if !lockfile.Packages["node_modules/chalk/node_modules/ansi-styles"].Optional {
		t.Fatalf("expected nested optional package")
	}
}

func TestAddedTransitiveCountOnlyCountsNewPaths(t *testing.T) {
	base := &Lockfile{
		Packages: map[string]LockPackage{
			"node_modules/direct":                         {Path: "node_modules/direct", Name: "direct", Version: "1.0.0"},
			"node_modules/direct/node_modules/transitive": {Path: "node_modules/direct/node_modules/transitive", Name: "transitive", Version: "1.0.0"},
		},
	}
	head := &Lockfile{
		Packages: map[string]LockPackage{
			"node_modules/direct":                             {Path: "node_modules/direct", Name: "direct", Version: "1.0.0"},
			"node_modules/direct/node_modules/transitive":     {Path: "node_modules/direct/node_modules/transitive", Name: "transitive", Version: "2.0.0"},
			"node_modules/direct/node_modules/new-transitive": {Path: "node_modules/direct/node_modules/new-transitive", Name: "new-transitive", Version: "1.0.0"},
		},
	}

	count := head.AddedTransitiveCount(base, map[string]struct{}{"direct": {}})
	if count != 1 {
		t.Fatalf("expected only newly added transitive paths to count, got %d", count)
	}
}

func TestAddedTransitivePathsForTargetReturnsSortedUniquePaths(t *testing.T) {
	base := &Lockfile{
		Packages: map[string]LockPackage{
			"apps/web":            {Path: "apps/web", Name: "web", Dependencies: map[string]string{"direct": "^1.0.0"}},
			"node_modules/direct": {Path: "node_modules/direct", Name: "direct", Version: "1.0.0", Dependencies: map[string]string{"shared": "^1.0.0"}},
			"node_modules/shared": {Path: "node_modules/shared", Name: "shared", Version: "1.0.0"},
			"node_modules/direct/node_modules/nested-existing": {Path: "node_modules/direct/node_modules/nested-existing", Name: "nested-existing", Version: "1.0.0"},
		},
	}
	head := &Lockfile{
		Packages: map[string]LockPackage{
			"apps/web":            {Path: "apps/web", Name: "web", Dependencies: map[string]string{"direct": "^1.0.0"}},
			"node_modules/direct": {Path: "node_modules/direct", Name: "direct", Version: "1.0.0", Dependencies: map[string]string{"added-b": "^1.0.0", "added-a": "^1.0.0", "shared": "^1.0.0"}},
			"node_modules/shared": {Path: "node_modules/shared", Name: "shared", Version: "1.0.0"},
			"node_modules/direct/node_modules/nested-existing": {Path: "node_modules/direct/node_modules/nested-existing", Name: "nested-existing", Version: "1.0.0"},
			"node_modules/direct/node_modules/added-b":         {Path: "node_modules/direct/node_modules/added-b", Name: "added-b", Version: "1.0.0"},
			"node_modules/direct/node_modules/added-a":         {Path: "node_modules/direct/node_modules/added-a", Name: "added-a", Version: "1.0.0"},
		},
	}

	paths, approximate := head.AddedTransitivePathsForTarget(base, "apps/web", []string{"direct"})
	if approximate {
		t.Fatalf("expected exact target attribution")
	}
	expected := []string{
		"node_modules/direct/node_modules/added-a",
		"node_modules/direct/node_modules/added-b",
	}
	if len(paths) != len(expected) {
		t.Fatalf("expected %d added paths, got %d (%v)", len(expected), len(paths), paths)
	}
	for i := range expected {
		if paths[i] != expected[i] {
			t.Fatalf("expected sorted added path %q at %d, got %q", expected[i], i, paths[i])
		}
	}
}

func TestDetectSourceKind(t *testing.T) {
	tests := map[string]SourceKind{
		"https://registry.npmjs.org/left-pad/-/left-pad-1.0.0.tgz": SourceDefaultRegistry,
		"https://npm.pkg.github.com/download/pkg.tgz":              SourceOtherRegistry,
		"git+https://github.com/npm/cli.git":                       SourceGit,
		"file:../local-package":                                    SourceUnknown,
	}

	for resolved, expected := range tests {
		if actual := DetectSourceKind(resolved); actual != expected {
			t.Fatalf("expected %q to be %s, got %s", resolved, expected, actual)
		}
	}
}

func TestCollectTargetPackagesForWorkspaceUsesSharedRootLockfile(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "workspace.root.head.package-lock.json"))
	if err != nil {
		t.Fatal(err)
	}

	lockfile, err := ParseLockfile(data)
	if err != nil {
		t.Fatal(err)
	}

	view := lockfile.CollectTargetPackages("apps/web", []string{"@acme/ui", "axios", "react"})
	if _, ok := view.Direct["axios"]; !ok {
		t.Fatalf("expected axios to resolve as a direct workspace dependency")
	}
	if _, ok := view.Transitive["node_modules/form-data"]; !ok {
		t.Fatalf("expected shared root transitive dependency to be included")
	}
	if view.Approximate {
		t.Fatalf("expected exact workspace attribution for shared root lockfile fixture")
	}
}

func TestParsePNPMLockfileAndCollectWorkspacePackages(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "pnpm.workspace.head.lock.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	lockfile, err := ParseLockfile(data)
	if err != nil {
		t.Fatal(err)
	}
	if lockfile.Manager != "pnpm" {
		t.Fatalf("expected pnpm manager, got %q", lockfile.Manager)
	}

	view := lockfile.CollectTargetPackages("apps/web", []string{"axios", "react"})
	if _, ok := view.Direct["axios"]; !ok {
		t.Fatalf("expected axios to resolve as a direct pnpm workspace dependency")
	}
	if _, ok := view.Transitive["follow-redirects@1.15.6"]; !ok {
		t.Fatalf("expected pnpm transitive dependency to be included")
	}
	if view.Approximate {
		t.Fatalf("expected exact pnpm workspace attribution")
	}
}

func TestParsePNPMLockfilePreservesWorkspaceLocalLinks(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "pnpm.workspace.head.lock.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	lockfile, err := ParseLockfile(data)
	if err != nil {
		t.Fatal(err)
	}

	view := lockfile.CollectTargetPackages("packages/ui", []string{"@repo/shared", "tailwind-merge"})
	shared, ok := view.Direct["@repo/shared"]
	if !ok {
		t.Fatalf("expected workspace-local direct dependency")
	}
	if !shared.WorkspaceLocal {
		t.Fatalf("expected workspace-local dependency marker")
	}
	if shared.Path != "workspace:@repo/shared" {
		t.Fatalf("unexpected workspace-local path %q", shared.Path)
	}
}
