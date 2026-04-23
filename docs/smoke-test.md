# Smoke Test

These smoke tests are designed for release validation. They assume you are in
the repository root and have a built binary or an installed extension.

On Windows PowerShell, use `.\gh-dep-risk.exe` instead of `./gh-dep-risk`.
If you want isolated extension-install testing, set a temporary `GH_CONFIG_DIR`
before running the install commands.

## 1. Local CLI run against a real PR

```bash
gh auth login
export GH_REPO=OWNER/REPO
go build -o gh-dep-risk .
./gh-dep-risk pr 123
```

Verify:

- the command exits `0`, `2`, `3`, or `4` as documented
- the report includes repo, PR, score, blast radius, and recommended actions
- before release, run at least one smaller PR and one larger PR instead of
  relying only on fixture-backed tests
- if the target repository has a very large lockfile, verify the run does not
  fail on a GitHub contents API `encoding: none` response

## 2. Workflow dispatch run

```bash
gh workflow run .github/workflows/dep-risk-manual.yml -f pr=123
gh run watch
```

Verify:

- the workflow summary contains the aggregate markdown output
- the artifact includes `dep-risk-human.txt`, `dep-risk.json`, `dep-risk.md`,
  `metadata.json`
- if multiple targets changed, the artifact also includes `targets/...`
- for private cross-repo targets, verify the workflow token can read the target
  PR repository; otherwise the run can fail before comment upsert or artifact
  upload

## 3. Remote install smoke

```bash
gh extension install rad1092/gh-dep-risk --force
gh dep-risk version
gh dep-risk version --json
```

Verify:

- the install succeeds from the published release assets
- the reported version matches the latest tag
- the command does not report only `dev`

## 4. Comment mode

```bash
./gh-dep-risk pr 123 --comment
```

Verify:

- exactly one marker comment owned by the current authenticated user remains
- older duplicate comments owned by the same user are removed
- marker comments from other authors are not edited or deleted

## 5. Fail-level mode

```bash
./gh-dep-risk pr 123 --fail-level high
```

Verify:

- exit code `3` is returned only when the final score meets or exceeds the
  threshold
- the report still renders before the exit code is surfaced

## 6. Monorepo target selection

```bash
./gh-dep-risk pr 123 --list-targets
./gh-dep-risk pr 123 --path apps/web
./gh-dep-risk pr 123 --path package.json --bundle-dir ./out
```

Verify:

- `--list-targets` exits `0` and prints detected targets with clear ecosystem
  and manager context
- `--path` restricts analysis to the selected target or targets by exact
  manifest path or by owning directory when that is unambiguous
- aggregate and per-target bundle files are written when `--bundle-dir` is set
