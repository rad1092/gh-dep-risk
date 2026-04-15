# gh-dep-risk

`gh-dep-risk` is a precompiled GitHub CLI extension that reviewers run on demand to summarize npm dependency risk in pull requests.

It is a GitHub CLI extension instead of a server so the workflow stays local to the reviewer, reuses existing `gh` authentication, avoids webhooks and background infrastructure, and runs only when someone explicitly asks for a dependency-risk summary.

## Scope

- npm-only MVP
- supported manifests: `package.json`, `package-lock.json`
- one Go binary
- no server, webhook receiver, GitHub App, queue, database, or dashboard
- dependency review API first, lockfile-only fallback when dependency review is unavailable

## Install

### Authenticate first

```bash
gh auth login
```

`go-gh` also respects:

- `GH_TOKEN`
- `GITHUB_TOKEN`
- `GH_HOST`
- `GH_REPO`

`GH_REPO=OWNER/REPO` is useful outside a git checkout. `GH_HOST` is useful for GitHub Enterprise.

### Install from a remote repository

```bash
gh extension install OWNER/gh-dep-risk
```

Upgrade later with:

```bash
gh extension upgrade dep-risk
```

### Install locally from this repo

Linux or macOS:

```bash
go build -o gh-dep-risk .
gh extension install .
```

Windows PowerShell:

```powershell
go build -o gh-dep-risk.exe .
gh extension install .
```

This repo does not install itself automatically. Build the binary at the repository root first, then run `gh extension install .` manually.

## Usage

```bash
gh dep-risk pr 123
gh dep-risk pr https://github.com/OWNER/REPO/pull/123
gh dep-risk pr --format json
gh dep-risk pr 123 --list-targets
gh dep-risk pr 123 --path apps/web
gh dep-risk pr 123 --path package.json --comment
gh dep-risk pr --bundle-dir ./dep-risk-bundle
gh dep-risk pr --comment
gh dep-risk pr --fail-level high
gh dep-risk version
```

## Command shape

- `gh dep-risk pr [<number>|<url>]`
- `gh dep-risk version`

If the PR argument is omitted, `gh dep-risk pr` resolves the PR for the current branch.

## Flags

- `--repo owner/repo`
- `--format human|json|markdown`
- `--lang ko|en`
- `--comment`
- `--fail-level low|medium|high|critical|none`
- `--no-registry`
- `--bundle-dir <dir>`
- `--path <repo-relative-dir-or-package.json>` repeatable
- `--list-targets`

## Output formats

- `human`: concise reviewer-oriented summary
- `json`: stable machine-readable schema with repo, PR metadata, score, level, blast radius, dependency-review availability, summary bullets, recommended actions, notes, detailed changes, and a `targets` array
- `markdown`: comment-ready output that always starts with `<!-- gh-dep-risk -->`

Korean is the default language. Use `--lang en` for English.

`--bundle-dir` writes a reusable output bundle with:

- `dep-risk-human.txt`
- `dep-risk.json`
- `dep-risk.md`
- `metadata.json`

When multiple npm targets are analyzed, the same bundle also includes per-target files under:

- `targets/<safe-target-name>/dep-risk.json`
- `targets/<safe-target-name>/dep-risk.md`

`metadata.json` includes the detected targets, target count, overall score, level, blast radius, and `dependency_review_available`.

## Behavior

`gh dep-risk pr` resolves the repository from `GH_REPO` or the current git remote, fetches PR metadata, lists changed files, discovers supported npm targets from the base/head repository trees, and analyzes only the changed targets unless `--path` narrows the set explicitly.

Supported target shapes:

- repo root projects with `package.json` and `package-lock.json`
- npm workspaces with a shared root `package-lock.json`
- nested standalone subprojects with their own `package.json` and `package-lock.json`

### npm monorepos and workspaces

Default behavior:

- if one supported npm target changed, `gh-dep-risk` analyzes that target
- if multiple supported targets changed, `gh-dep-risk` analyzes all of them once and produces one aggregate result plus per-target detail
- if no supported npm target changed, the command exits with code `2`

Useful commands:

```bash
gh dep-risk pr 123 --list-targets
gh dep-risk pr 123 --path apps/web
gh dep-risk pr 123 --path package.json --comment
gh dep-risk pr 123 --bundle-dir ./out
```

Notes:

- `--path` accepts either a directory or a `package.json` path and can be repeated
- `--list-targets` prints detected targets and exits without running analysis
- npm workspaces reuse the shared root `package-lock.json`; per-workspace attribution is best effort and is called out in notes when lockfile-only changes cannot be mapped exactly
- npm-only remains the current limit; pnpm and yarn are intentionally out of scope

If dependency review returns `403` or `404`, `gh-dep-risk` falls back to lockfile-only analysis and explicitly reports `dependency_review_available=false`. Registry publish-age lookups are best effort and are skipped with `--no-registry`.

If there is no meaningful npm manifest or lockfile dependency change, the command exits with code `2`.

### Comment upsert rules

`--comment` uses PR timeline issue comments, not review comments.

The marker comment is:

```html
<!-- gh-dep-risk -->
```

Behavior:

- exactly one marker comment owned by the authenticated user is maintained
- if multiple own marker comments exist, the newest is updated and older own duplicates are deleted
- another author's marker comment is never edited or deleted
- if another author already has a marker comment, `gh-dep-risk` warns on stderr and only manages the current user's own comment

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
./gh-dep-risk version
```

Windows PowerShell:

```powershell
go test ./...
go build -o gh-dep-risk.exe .
.\gh-dep-risk.exe version
```

Local extension install remains manual:

```bash
gh extension install .
```

## Run without local install

You can run the existing CLI engine from GitHub Actions without installing the extension locally.

### From the Actions tab

Use the `dep-risk-manual` workflow and provide:

- `pr`: required PR number or full PR URL
- `repo`: optional repository override
- `lang`
- `fail_level`
- `comment`
- `no_registry`

The workflow file must exist on the default branch for the **Run workflow** button to appear.

### From GitHub CLI

```bash
gh workflow run .github/workflows/dep-risk-manual.yml -f pr=123
gh workflow run .github/workflows/dep-risk-manual.yml -f pr=https://github.com/OWNER/REPO/pull/123 -f comment=true
gh run watch
```

### Results

Each manual run:

- builds and tests the current repo
- runs `gh-dep-risk` once
- uploads a bundle artifact containing the aggregate files plus any per-target bundle files
- appends the markdown report to the workflow job summary

Find the artifact and step summary on the workflow run page.

If `comment=true`, comment ownership follows the workflow-authenticated identity backed by `GITHUB_TOKEN`.

## Release

Push a `v*` tag to trigger `.github/workflows/release.yml`.

The release workflow:

- runs `go test ./...`
- uses `cli/gh-extension-precompile@v2`
- publishes precompiled binaries for GitHub CLI extension installs
