# Support Matrix

`gh-dep-risk` prefers GitHub Dependency Review API data whenever GitHub returns
it for the pull request. Local fallback is used only when Dependency Review is
unavailable, such as `403` or `404`.

## Dependency Review Path

When GitHub provides Dependency Review data, `gh-dep-risk` can surface changes
for the ecosystems returned by GitHub, including:

- Cargo
- Composer
- Go modules
- Maven
- npm
- pip
- pnpm
- Poetry
- RubyGems
- Swift Package Manager
- Yarn

Dependency Review data is not merged with local fallback results. If the API is
available, it remains the source of truth.

## Local Fallback Path

| Ecosystem / manager | Local fallback support |
| --- | --- |
| npm | Direct and lockfile-backed changes from `package.json` and `package-lock.json`. |
| pnpm | Direct and lockfile-backed changes from `package.json`, `pnpm-lock.yaml`, and `pnpm-workspace.yaml` workspace discovery. |
| Yarn Classic | Direct and transitive package changes from `package.json` and classic `yarn.lock`. |
| Yarn Berry / modern Yarn | Direct `package.json` declarations matched to modern `yarn.lock`; `.yarnrc.yml` is used only for detection and nodeLinker notes. |
| Bun | Direct `package.json` declarations matched to text `bun.lock`. |
| Python `requirements.txt` | Direct declarations only; unsupported includes, constraints, hashes, editable installs, and bare URLs become unsupported notes. |
| Python PEP 621 | Direct `[project].dependencies` and `[project.optional-dependencies]`; optional `uv.lock` entries can enrich matching direct resolved versions and source metadata. |
| Poetry | Direct `[tool.poetry.dependencies]`, `[tool.poetry.dev-dependencies]`, and `[tool.poetry.group.<name>.dependencies]`; optional `poetry.lock` entries can enrich matching direct resolved versions and source metadata. |
| Go modules | Static `go.mod` `require` and `replace` changes; `go.sum` checksum changes are evidence only. |

## Explicitly Out Of Scope

Local fallback does not perform:

- package manager execution
- resolver behavior
- full transitive graph reconstruction
- registry metadata lookup expansion
- PyPI, Go module proxy, Bun registry, OSV, Socket, or license lookups
- `.pnp.cjs` parsing
- Yarn cache archive, plugin, or constraints interpretation
- binary `bun.lockb` parsing
- SARIF output

Unsupported-only changes return exit code `2` and do not invent a scored
dependency change.

## Live Smoke Coverage

The owned read-only smoke matrix is:

| Repo | PR | Coverage |
| --- | --- | --- |
| `rad1092/gh-dep-risk-smoke-matrix` | `#3` | npm `package.json` + `package-lock.json` |
| `rad1092/gh-dep-risk-smoke-matrix` | `#1` | pnpm `package.json` + `pnpm-lock.yaml` with workspace discovery |
| `rad1092/gh-dep-risk-smoke-matrix` | `#2` | Yarn Classic `package.json` + `yarn.lock` |
| `rad1092/gh-dep-risk-smoke-matrix` | `#4` | Python `requirements.txt` |
| `rad1092/gh-dep-risk-smoke-matrix` | `#5` | Python PEP 621 `pyproject.toml` |
| `rad1092/gh-dep-risk-smoke-matrix` | `#6` | Poetry `pyproject.toml` + `poetry.lock` |
| `rad1092/gh-dep-risk-smoke-matrix` | `#7` | uv `pyproject.toml` + `uv.lock` |
| `rad1092/gh-dep-risk-smoke-matrix` | `#8` | Go modules `go.mod` + `go.sum` checksum evidence |
| `rad1092/gh-dep-risk-smoke-matrix` | `#9` | Yarn Berry / modern Yarn `package.json` + `yarn.lock` |
| `rad1092/gh-dep-risk-smoke-matrix` | `#10` | Bun `package.json` + text `bun.lock` |
| `rad1092/gh-dep-risk-smoke-matrix` | `#11` | Bun `bun.lockb` unsupported-only behavior |
