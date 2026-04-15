# AGENTS

- mission: on-demand npm dependency PR risk summary
- npm-only MVP scope
- keep GitHub I/O in `internal/github`
- keep deterministic logic in `internal/analysis` and `internal/npm`
- marker comment is `<!-- gh-dep-risk -->`
- never build a server or web UI
- add tests whenever parser, scoring, or rendering changes
