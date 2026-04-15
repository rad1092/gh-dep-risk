<!-- gh-dep-risk -->
## gh-dep-risk
- Repository: `owner/repo`
- PR: [#123](https://github.com/owner/repo/pull/123) Update dependencies
- Score: `48` (`high`)
- Blast radius: `medium`
- Dependency review available: `false`

### Summary
- 1 npm dependency changes were detected.
- Dependency Review was unavailable, so lockfile-only fallback analysis was used.
- Top risk signals: major version bump, install script.

### Notes
- Dependency review API was unavailable, so lockfile-only fallback analysis was used.

### Targets
- `root` (root, score `48`, level `high`, blast `medium`)
  - `left-pad 1.0.0 -> 2.0.0` (updated/runtime, score `48`)
    - The dependency crosses a major version boundary.
    - The package declares an install script.
  - Dependency review API was unavailable, so lockfile-only fallback analysis was used.

### Why risky
- The dependency crosses a major version boundary.
- The package declares an install script.

### Recommended actions
- Inspect install scripts and package tarballs before merging.
- Read upstream release notes and migration guidance.

### Quick commands
- `npm ls left-pad`

