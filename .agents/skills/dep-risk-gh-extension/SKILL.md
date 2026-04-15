# dep-risk-gh-extension

Use this skill when working on `gh-dep-risk`, the precompiled GitHub CLI extension for on-demand npm dependency PR risk summaries.

## Mission

- keep the product as a single Go binary
- keep the MVP npm-only: `package.json` and `package-lock.json`
- keep the workflow on-demand through `gh dep-risk pr`
- never introduce a server, webhook receiver, GitHub App, queue, database, dashboard, or web UI

## Repository boundaries

- keep GitHub I/O in `internal/github`
- keep orchestration in `internal/app`
- keep deterministic parsing in `internal/npm`
- keep deterministic scoring in `internal/analysis`
- keep output rendering in `internal/render`

## Comment rules

- marker comment is `<!-- gh-dep-risk -->`
- use issue comments on the PR timeline, never review comments
- `--comment` must maintain exactly one marker comment owned by the authenticated user
- if multiple own marker comments exist, update the newest and delete older own duplicates
- never edit or delete another author's marker comment

## Working rules

- keep tests deterministic and network-free
- do not run `gh extension install .` automatically
- do not print tokens or secrets
- add or update tests whenever parser, scoring, rendering, or comment-upsert behavior changes
