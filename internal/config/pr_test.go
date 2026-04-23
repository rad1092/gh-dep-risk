package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPRConfigMissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfg, found, err := LoadPRConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatalf("expected missing config to report found=false")
	}
	if cfg.ConfigPath != "" {
		t.Fatalf("expected empty config path, got %q", cfg.ConfigPath)
	}
}

func TestLoadPRConfigParsesValidFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, PRConfigFileName)
	if err := os.WriteFile(configPath, []byte("lang: ko\nfail_level: high\ncomment: true\npath:\n  - apps/web\n  - package.json\nno_registry: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, found, err := LoadPRConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatalf("expected config to be found")
	}
	if cfg.ConfigPath != configPath {
		t.Fatalf("unexpected config path %q", cfg.ConfigPath)
	}
	if cfg.Lang == nil || *cfg.Lang != "ko" {
		t.Fatalf("unexpected lang %#v", cfg.Lang)
	}
	if cfg.FailLevel == nil || *cfg.FailLevel != "high" {
		t.Fatalf("unexpected fail level %#v", cfg.FailLevel)
	}
	if cfg.Comment == nil || !*cfg.Comment {
		t.Fatalf("unexpected comment %#v", cfg.Comment)
	}
	if !cfg.Paths.Set || len(cfg.Paths.Values) != 2 || cfg.Paths.Values[0] != "apps/web" || cfg.Paths.Values[1] != "package.json" {
		t.Fatalf("unexpected path values %#v", cfg.Paths)
	}
	if cfg.NoRegistry == nil || !*cfg.NoRegistry {
		t.Fatalf("unexpected no_registry %#v", cfg.NoRegistry)
	}
}

func TestLoadPRConfigAcceptsSinglePathString(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, PRConfigFileName)
	if err := os.WriteFile(configPath, []byte("path: apps/web\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, found, err := LoadPRConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatalf("expected config to be found")
	}
	if !cfg.Paths.Set || len(cfg.Paths.Values) != 1 || cfg.Paths.Values[0] != "apps/web" {
		t.Fatalf("unexpected path values %#v", cfg.Paths)
	}
}

func TestLoadPRConfigRejectsUnknownKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, PRConfigFileName)
	if err := os.WriteFile(configPath, []byte("lang: en\nunknown_key: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, found, err := LoadPRConfig(dir)
	if !found {
		t.Fatalf("expected config to be found")
	}
	if err == nil {
		t.Fatalf("expected unknown key error")
	}
	if !strings.Contains(err.Error(), configPath) {
		t.Fatalf("expected error to include config path, got %v", err)
	}
	if !strings.Contains(err.Error(), "field unknown_key not found") {
		t.Fatalf("expected unknown key details, got %v", err)
	}
}
