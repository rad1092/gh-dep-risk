# AGENTS

- mission: on-demand npm dependency PR risk summary
- npm-only scope: support only `package.json` and `package-lock.json`
- keep GitHub I/O in `internal/github`
- keep orchestration in `internal/app`
- keep deterministic logic in `internal/analysis` and `internal/npm`
- keep rendering in `internal/render`
- keep fixtures in `testdata`
- marker comment is `<!-- gh-dep-risk -->`
- PR timeline comments must use issue comments, never review comments
- `--comment` must maintain exactly one marker comment owned by the authenticated user
- if multiple own marker comments exist, update the newest and delete older own duplicates
- never edit or delete another author's marker comment
- never build a server or web UI
- add tests whenever parser, scoring, rendering, or comment-upsert rules change
