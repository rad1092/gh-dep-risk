# AGENTS

- mission: on-demand JavaScript dependency PR risk summary
- JS package manager scope: support `package.json` with `package-lock.json` or
  `pnpm-lock.yaml`, plus `pnpm-workspace.yaml` for pnpm workspace discovery,
  and support classic `yarn.lock` fallback for root, workspace, and nested
  standalone Yarn targets
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
