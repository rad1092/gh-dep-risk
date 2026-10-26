# Contributing

## Local setup

Run tests:

```bash
go test ./...
```

Build a local binary:

```bash
go build -o gh-dep-risk .
./gh-dep-risk version
```

Windows PowerShell:

```powershell
go build -o gh-dep-risk.exe .
.\gh-dep-risk.exe version
```

The supported local fallback scope is documented in
[docs/support-matrix.md](docs/support-matrix.md). Keep new parser behavior
inside that matrix unless a release intentionally expands the support boundary.

Build with explicit metadata:

```bash
make build VERSION=dev-local
```

## Local extension install

Local extension install is manual and should only be run when you want to test
the extension in your own GitHub CLI setup:

```bash
gh extension install .
```

The checkout directory name must start with `gh-` for local install to work.
Do not assume this is part of automated tests or normal unit-test flows.

## Project rules

These also guide AI coding agents working in this repository.

- The job is an on-demand dependency risk summary for one pull request. No
  server, webhook receiver, GitHub App, database or web UI.
- GitHub Dependency Review API first. Local fallback runs only when it is
  unavailable, and stays inside [docs/support-matrix.md](docs/support-matrix.md):
  no resolver, no full transitive graph, no `.pnp.cjs` and no binary
  `bun.lockb` parsing.
- Where code goes: GitHub I/O in `internal/github`, orchestration in
  `internal/app`, deterministic logic in `internal/analysis`, `internal/npm`,
  `internal/python` and `internal/gomod`, rendering in `internal/render`,
  fixtures in `testdata/`.
- Every point in a score comes from a named signal in
  `internal/analysis/scoring.go` and appears in the change's
  `score_breakdown`. Update the table in `docs/behavior.md` and the README
  when a weight changes.
- Comments: the marker is `<!-- gh-dep-risk -->`. Use PR timeline issue
  comments, never review comments. `--comment` keeps exactly one marker
  comment owned by the authenticated user; when there are several, update
  the newest and delete the older ones. Never edit or delete another author's
  marker comment.

## Tests and fixtures

- add or update tests whenever scoring, rendering, parsing, or comment-upsert
  behavior changes
- prefer network-free fixtures under `testdata/`
- `docs/examples/` is rendered from the sample analyses in
  `internal/render/examples_test.go`; regenerate it with
  `go test ./internal/render -run TestCheckedInExamples -update`

## Docs and examples

If behavior changes, update:

- `README.md`
- `docs/support-matrix.md`
- `docs/behavior.md`
- `docs/examples/`
- `docs/smoke-test.md`
- `CHANGELOG.md` when appropriate

## License

By contributing to this repository, you agree that your contributions will be
licensed under the [MIT License](LICENSE).
