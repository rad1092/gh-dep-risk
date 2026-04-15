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
