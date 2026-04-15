# gh-dep-risk

`gh-dep-risk` is a precompiled GitHub CLI extension that reviewers run on demand to summarize npm dependency risk in pull requests. It is a CLI extension instead of a server so the workflow stays local, uses existing `gh` authentication, does not require webhooks or background infrastructure, and can be invoked exactly when a reviewer wants an opinionated dependency summary.

## Scope

- npm-only MVP
- supported manifests: `package.json`, `package-lock.json`
- one Go binary, no server, no dashboard, no webhook receiver
- dependency review API first, lockfile-only fallback when the API is unavailable

## Install

1. Authenticate GitHub CLI:

```bash
gh auth login
```

2. Install locally from the repository root:

```bash
go build -o gh-dep-risk .
gh extension install .
```

On Windows, build `gh-dep-risk.exe` instead:

```powershell
go build -o gh-dep-risk.exe .
gh extension install .
```

You can also provide credentials with `GH_TOKEN` and pin the repository context with `GH_REPO=OWNER/REPO`.

## Usage

```bash
gh dep-risk pr 123
gh dep-risk pr https://github.com/OWNER/REPO/pull/123
gh dep-risk pr --format json
gh dep-risk pr --comment
gh dep-risk pr --fail-level high
```

### Command summary

- `gh dep-risk pr [<number>|<url>]`
- `gh dep-risk version`

### Flags

- `--repo owner/repo`
- `--format human|json|markdown`
- `--lang ko|en`
- `--comment`
- `--fail-level low|medium|high|critical|none`
- `--no-registry`

### Authentication

- `gh auth login` is the default path.
- `GH_TOKEN` or `GITHUB_TOKEN` also work through `go-gh`.
- `GH_REPO` overrides repository detection when you are not inside a git checkout.

## Behavior

The command resolves the current pull request when no PR argument is supplied. It fetches PR metadata, changed files, dependency review data, and the base/head `package.json` plus `package-lock.json`.

If the dependency review API returns `403` or `404`, `gh-dep-risk` falls back to lockfile-only analysis and marks `dependency_review_available=false`. Registry publish-age lookups are best effort and are skipped with `--no-registry`.

`--comment` manages exactly one timeline comment owned by the authenticated user. The marker is `<!-- gh-dep-risk -->`. If duplicates owned by the current user exist, the newest is updated and older duplicates are deleted. Comments from other authors are never edited or deleted.

## Exit codes

- `0` success
- `1` general error
- `2` no supported npm dependency change found
- `3` final score meets or exceeds `--fail-level`
- `4` authentication required or insufficient permissions

## Local development

```bash
go test ./...
go build -o gh-dep-risk .
gh extension install .
gh dep-risk version
```

On Windows, replace the binary name with `gh-dep-risk.exe`.

## Release

Push a `v*` tag to trigger `.github/workflows/release.yml`. The workflow uses `cli/gh-extension-precompile@v2` to publish precompiled binaries.
