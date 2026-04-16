# Changelog

All notable changes to `gh-dep-risk` will be documented in this file.

## Unreleased

- No unreleased changes yet.

## v0.1.2

- Hardened large-repo target discovery so truncated Git tree responses no
  longer silently drop npm targets.
- Fixed dependency-review auth classification and workflow input handling so
  auth and permission failures surface consistently and safely.
- Deduplicated aggregate transitive dependency counts across workspace targets
  that share a root lockfile.

## v0.1.1

- Added the cross-platform `install-smoke` workflow to validate released
  extension install and execution on Linux, macOS, and Windows.
- Extended install smoke coverage to run `gh dep-risk pr` against a public test
  PR instead of checking version output only.
- Documented that workflow-driven cross-repo comment mode can be blocked by the
  scope of `GITHUB_TOKEN`, which surfaces as exit code `4`.

## v0.1.0

- Added `gh dep-risk pr` for on-demand npm dependency risk summaries on GitHub
  pull requests.
- Added human, JSON, and markdown output formats with Korean as the default
  language.
- Added `--comment` marker-comment upsert behavior using PR timeline issue
  comments.
- Added `--fail-level` support with deterministic exit codes for CI and workflow
  gating.
- Added best-effort npm registry publish-age checks with `--no-registry` opt
  out.
- Added npm workspace and nested standalone subproject support with `--path`
  and `--list-targets`.
- Added reusable output bundle generation and a manual GitHub Actions workflow
  for no-local-install runs.
- Added precompiled release workflow support for GitHub CLI extension installs.
