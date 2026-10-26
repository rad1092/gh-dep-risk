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

Each dependency change gets points for every signal that applies. The
report shows the points next to each change, so a score can always be added
up by hand.

| Signal | Points | When |
| --- | ---: | --- |
| Known vulnerabilities | 35 | Dependency Review reports an advisory for the new version |
| Name like a popular package | 20 | A new direct npm package is one edit away from a widely used one, for example `lodahs` or `crossenv` |
| Provenance dropped | 20 | The previous version had an npm provenance attestation and the new one does not |
| Install script | 20 | The lockfile says the package runs an install script |
| Recently published | 18 | The new version is less than 7 days old |
| New package | 15 | A newly added package was first published less than 30 days ago |
| 5+ new transitive packages | 12 | The target gains at least 5 packages it did not add directly |
| New direct runtime dependency | 12 | Added to `dependencies` or `optionalDependencies` |
| Major version bump | 10 | The major version goes up |
| Different publisher | 10 | Another npm account published the new version than the previous one |
| 15+ new transitive packages | 8 | On top of the 5+ points |
| New direct dev dependency | 6 | Added to `devDependencies` |
| Platform restriction | 6 | The package only installs on some OS or CPU targets |

A change's score is capped at 100. The PR score is the highest change score
plus a bonus for the other changes: 4 for each other change scored 20 or
more, 1 for each other change above 0, at most 15. The score line says how
much came from where, for example `29 (medium), 28 from apps/web /
react-router-dom + 1 for the other changes`.

Levels: `low` below 20, `medium` from 20, `high` from 40, `critical` from 70.

## Check First

Every report starts with up to three concrete checks, riskiest change first,
such as reading an install script with `npm view <package>@<version> scripts`
or reading a major version's release notes. When `gh dep-risk` runs inside a
clone of the repository that has the PR's commits, it also names the files
that import each changed npm package (found with `git grep`, nothing is
fetched) and points the test advice at them.

## Registry Lookups

For npm, pnpm and Yarn lockfile targets, `gh dep-risk` reads each new
version's metadata from the npm registry: its publish date, when the package
was first published, which account published it, and whether it has a
provenance attestation. Lookups are best effort and skipped with
`--no-registry`.

Python, Go, Poetry, uv, and Bun local fallback do not perform PyPI, Go module
proxy, or Bun registry lookups.

## Output Formats

- `human`: concise reviewer-oriented summary
- `json`: machine-readable report with repo, PR metadata, score, level, blast
  radius, dependency review availability, summary bullets, `check_first`,
  recommended actions, notes, changes with their `score_breakdown`, and
  per-target detail
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
