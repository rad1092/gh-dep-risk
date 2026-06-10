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

## Tests and fixtures

- keep GitHub I/O behind interfaces in `internal/github`
- keep deterministic analysis and parser logic in `internal/analysis` and
  `internal/npm`
- add or update tests whenever scoring, rendering, parsing, or comment-upsert
  behavior changes
- prefer network-free fixtures under `testdata/`

## Docs and examples

If behavior changes, update:

- `README.md`
- `docs/support-matrix.md`
- `docs/behavior.md`
- `docs/examples/`
- `docs/smoke-test.md`
- `CHANGELOG.md` when appropriate

## Contributor credit

Keep public credit concise and factual. The current README credits `rad1092`
and `Codex` for AI-assisted implementation, docs, and release validation.

## License

By contributing to this repository, you agree that your contributions will be
licensed under the [MIT License](LICENSE).
