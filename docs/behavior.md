# Behavior

This document keeps the detailed behavior out of the main README while
preserving the exact operating contract.

## Target Discovery

`gh dep-risk pr` resolves the repository from `--repo`, `GH_REPO`, or the
current git remote. It fetches PR metadata, discovers supported dependency
targets from the base and head repository trees, then prefers GitHub Dependency
Review data.

Supported target shapes include:

- dependency-review targets returned by GitHub
- npm root projects and workspaces using `package-lock.json`
- pnpm root projects and workspaces using `pnpm-lock.yaml`
- Yarn Classic root projects, workspaces, and nested standalone projects
- Yarn Berry / modern Yarn direct fallback targets
- Bun root projects and workspaces using text `bun.lock`
- Python `requirements.txt`
- PEP 621 and Poetry `pyproject.toml`
- Go modules `go.mod`

If one supported target changed, that target is analyzed. If multiple supported
targets changed, the aggregate result contains per-target detail. If no
supported dependency change is found, the command exits `2`.

## Target Filters

Use `--path` to narrow the analysis:

```bash
gh dep-risk pr 123 --path apps/web
gh dep-risk pr 123 --path package.json
gh dep-risk pr 123 --path apps/web --path packages/ui
```

`--path` accepts an exact manifest path or an owning directory when that
directory maps to exactly one detected target.

Use `--list-targets` to print detected targets and exit without running PR
analysis:

```bash
gh dep-risk pr 123 --list-targets
```

## Scoring

The score model is heuristic, deterministic, and intentionally auditable.

- each dependency change is scored from named risk drivers with fixed weights
- the overall PR score is the highest single-change score plus a small capped
  bonus for additional risky changes
- the report keeps the main risk driver explainable instead of hiding it in an
  opaque sum

Risk levels are `low`, `medium`, `high`, and `critical`.

## Registry Lookups

npm-compatible registry publish-age lookups are best effort and limited to npm,
pnpm, and Yarn-style registry packages. They are skipped with `--no-registry`.

Python, Go, Poetry, uv, and Bun local fallback do not perform PyPI, Go module
proxy, or Bun registry publish-age lookups.

## Output Formats

- `human`: concise reviewer-oriented summary
- `json`: machine-readable report with repo, PR metadata, score, level, blast
  radius, dependency review availability, summary bullets, recommended actions,
  notes, changes, and per-target detail
- `markdown`: comment-ready output that starts with `<!-- gh-dep-risk -->`

`--bundle-dir` writes:

- `dep-risk-human.txt`
- `dep-risk.json`
- `dep-risk.md`
- `metadata.json`
- `targets/<safe-target-name>/dep-risk.json` for multi-target runs
- `targets/<safe-target-name>/dep-risk.md` for multi-target runs

## Comment Upsert

`--comment` uses PR timeline issue comments, not review comments.

The marker is:

```html
<!-- gh-dep-risk -->
```

Rules:

- exactly one marker comment owned by the authenticated user is maintained
- if multiple own marker comments exist, the newest is updated and older own
  duplicates are deleted
- another author's marker comment is never edited or deleted
- if another author already has a marker comment, the command warns on stderr
  and only manages the current user's own comment

## Exit Codes

- `0`: success
- `1`: general error
- `2`: no supported dependency change found
- `3`: final score meets or exceeds `--fail-level`
- `4`: authentication required or insufficient permissions

## Version Metadata

`gh dep-risk version` prints human-readable build metadata. Release-quality
builds inject:

- `version`
- `commit`
- `date`

Use `gh dep-risk version --json` for machine-readable metadata.
