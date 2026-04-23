<!-- gh-dep-risk -->
## gh-dep-risk
- Repository: `owner/repo`
- PR: [#456](https://github.com/owner/repo/pull/456) Update workspace dependencies
- Score: `52` (`high`)
- Blast radius: `medium`
- Dependency review available: `true`
- Why risky: apps/web / axios adds a new direct runtime dependency.

### Summary
- 2 npm dependency changes were detected.
- 2 npm targets were analyzed.
- Top risk signals: direct runtime addition.

### Targets
- `apps/web` (workspace, score `48`, level `high`, blast `medium`)
  - `axios -> 1.7.0` (added/runtime, score `48`)
    - A new direct runtime dependency was added.
- `packages/ui` (workspace, score `22`, level `medium`, blast `low`)
  - `tailwind-merge -> 2.3.0` (added/runtime, score `22`)
    - A new direct runtime dependency was added.

### Recommended actions
- Run targeted smoke checks for apps/web and packages/ui and the code paths that import the changed packages.

### Quick commands
- `cd apps/web && npm ls --all`
- `cd apps/web && npm ls axios`
- `cd packages/ui && npm ls --all`

