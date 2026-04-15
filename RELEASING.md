# Releasing

This document covers the first public release flow for `gh-dep-risk`.

## 1. Verify a clean tree

```bash
git status --short --branch
```

The tree should be clean before cutting a release tag.

## 2. Run tests

```bash
go test ./...
```

## 3. Build a release-quality binary locally

Linux or macOS:

```bash
VERSION=v0.1.0
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
go build -ldflags "-s -w -X gh-dep-risk/cmd.version=${VERSION} -X gh-dep-risk/cmd.commit=${COMMIT} -X gh-dep-risk/cmd.date=${DATE}" -o gh-dep-risk .
./gh-dep-risk version
./gh-dep-risk version --json
```

Windows PowerShell:

```powershell
$version = "v0.1.0"
$commit = git rev-parse --short HEAD
$date = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
go build -ldflags "-s -w -X gh-dep-risk/cmd.version=$version -X gh-dep-risk/cmd.commit=$commit -X gh-dep-risk/cmd.date=$date" -o gh-dep-risk.exe .
.\gh-dep-risk.exe version
.\gh-dep-risk.exe version --json
```

## 4. Optional manual workflow smoke test

If you have GitHub auth and repository access available, run:

```bash
gh workflow run .github/workflows/dep-risk-manual.yml -f pr=123
gh run watch
```

Then verify:

- the workflow summary contains the markdown report
- the uploaded artifact contains the bundle files
- comment mode behaves correctly if you tested `comment=true`

If GitHub auth or repository context is unavailable, skip this step and perform
it later from the default branch after pushing.

## 5. Create and push `v0.1.0`

```bash
git tag v0.1.0
git push origin v0.1.0
```

Do not create the tag until the branch you want to release is already on
`origin/main`.

## 6. Verify release assets

After the `release` workflow finishes:

- open the GitHub release page for `v0.1.0`
- verify precompiled binaries are attached for the expected platforms
- verify the generated release notes look sensible

## 7. Verify remote install

Use a clean shell or another machine if possible:

```bash
gh extension install rad1092/gh-dep-risk
gh dep-risk version
gh dep-risk version --json
```

Verify the version is `v0.1.0` and does not report only `dev`.

## 8. Verify upgrade flow

After the extension is already installed:

```bash
gh extension upgrade dep-risk
gh dep-risk version
```

## 9. Self-hosted runner note

These workflows use Node 24 based GitHub Actions majors. Keep self-hosted
runners current; Actions Runner `v2.327.1+` is the practical minimum baseline
for Node 24 based actions, and older runners should be upgraded before release
validation.
