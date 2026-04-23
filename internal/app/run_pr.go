package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"gh-dep-risk/internal/analysis"
	ghclient "gh-dep-risk/internal/github"
	"gh-dep-risk/internal/npm"
	"gh-dep-risk/internal/render"
)

type RunPROptions struct {
	PRArg       string
	Repo        string
	Format      string
	Lang        string
	BundleDir   string
	Comment     bool
	FailLevel   analysis.RiskLevel
	NoRegistry  bool
	Paths       []string
	ListTargets bool
}

type RunPRDependencies struct {
	GitHub   ghclient.Client
	Registry npm.RegistryClient
	Stdout   io.Writer
	Stderr   io.Writer
}

type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit %d", e.Code)
	}
	return e.Err.Error()
}

func RunPR(ctx context.Context, deps RunPRDependencies, opts RunPROptions) error {
	if deps.GitHub == nil {
		return &ExitError{Code: 1, Err: errors.New("missing GitHub client")}
	}
	if deps.Stdout == nil || deps.Stderr == nil {
		return &ExitError{Code: 1, Err: errors.New("stdout/stderr writers are required")}
	}

	repo, prNumber, err := resolveTarget(ctx, deps.GitHub, opts)
	if err != nil {
		return wrapGitHubError(err)
	}

	pr, err := deps.GitHub.GetPullRequest(ctx, repo, prNumber)
	if err != nil {
		return wrapGitHubError(err)
	}

	cache := newRepoDataCache(deps.GitHub, repo)
	targets, err := discoverTargets(ctx, cache, pr.BaseSHA, pr.HeadSHA)
	if err != nil {
		return wrapGitHubError(err)
	}
	selectedTargets, err := filterTargetsByRequestedPaths(targets, opts.Paths)
	if err != nil {
		return &ExitError{Code: 1, Err: err}
	}
	if opts.ListTargets {
		if _, err := io.WriteString(deps.Stdout, formatTargets(selectedTargets)); err != nil {
			return &ExitError{Code: 1, Err: err}
		}
		return nil
	}

	files, err := deps.GitHub.ListPullRequestFiles(ctx, repo, prNumber)
	if err != nil {
		return wrapGitHubError(err)
	}
	resolvedTargets, err := selectChangedTargets(selectedTargets, files)
	if err != nil {
		return &ExitError{Code: 1, Err: err}
	}
	if len(resolvedTargets) == 0 {
		return &ExitError{Code: 2, Err: errors.New("no supported npm or pnpm dependency change found")}
	}

	now := time.Now().UTC()
	inputs := make([]analysis.Input, 0, len(resolvedTargets))
	for _, target := range resolvedTargets {
		reviewChanges, dependencyReviewAvailable, err := compareTargetDependencies(ctx, deps.GitHub, repo, pr.BaseSHA, pr.HeadSHA, target)
		if err != nil {
			return wrapGitHubError(err)
		}

		baseManifest, err := cache.manifest(ctx, pr.BaseSHA, target.ManifestPath)
		if err != nil {
			return &ExitError{Code: 1, Err: err}
		}
		headManifest, err := cache.manifest(ctx, pr.HeadSHA, target.ManifestPath)
		if err != nil {
			return &ExitError{Code: 1, Err: err}
		}
		baseLockfile, err := cache.lockfile(ctx, pr.BaseSHA, target.LockfilePath)
		if err != nil {
			return &ExitError{Code: 1, Err: err}
		}
		headLockfile, err := cache.lockfile(ctx, pr.HeadSHA, target.LockfilePath)
		if err != nil {
			return &ExitError{Code: 1, Err: err}
		}

		inputs = append(inputs, analysis.Input{
			Now:                       now,
			Target:                    target,
			DependencyReviewAvailable: dependencyReviewAvailable,
			ReviewChanges:             reviewChanges,
			BaseManifest:              baseManifest,
			HeadManifest:              headManifest,
			BaseLockfile:              baseLockfile,
			HeadLockfile:              headLockfile,
		})
	}

	publishedAt := map[analysis.PackageVersion]time.Time{}
	if !opts.NoRegistry && deps.Registry != nil {
		registryTargets := collectRegistryTargets(inputs)
		for _, target := range registryTargets {
			published, err := deps.Registry.PublishedAt(ctx, target.Name, target.Version)
			if err != nil {
				continue
			}
			publishedAt[target] = published
		}
	}

	targetResults := make([]analysis.TargetAnalysisResult, 0, len(inputs))
	for _, input := range inputs {
		result := analysis.Analyze(input, publishedAt)
		if !analysis.HasMeaningfulChange(result) {
			continue
		}
		targetResults = append(targetResults, analysis.TargetResult(input.Target, result))
	}
	if len(targetResults) == 0 {
		return &ExitError{Code: 2, Err: errors.New("no supported npm or pnpm dependency change found")}
	}
	result := analysis.AggregateResults(targetResults)
	if !analysis.HasMeaningfulChange(result) {
		return &ExitError{Code: 2, Err: errors.New("no supported npm or pnpm dependency change found")}
	}

	report := render.Report{
		Repo: repo.FullName(),
		PR: render.PullRequestMetadata{
			Number:      pr.Number,
			URL:         pr.URL,
			Title:       pr.Title,
			Draft:       pr.Draft,
			BaseSHA:     pr.BaseSHA,
			HeadSHA:     pr.HeadSHA,
			AuthorLogin: pr.AuthorLogin,
		},
		Analysis: result,
	}

	output, err := render.Render(report, opts.Format, opts.Lang)
	if err != nil {
		return &ExitError{Code: 1, Err: err}
	}
	if _, err := io.WriteString(deps.Stdout, output); err != nil {
		return &ExitError{Code: 1, Err: err}
	}

	if strings.TrimSpace(opts.BundleDir) != "" {
		if _, err := render.WriteBundle(report, opts.Lang, opts.BundleDir); err != nil {
			return &ExitError{Code: 1, Err: err}
		}
	}

	if opts.Comment {
		viewerLogin, err := deps.GitHub.ViewerLogin(ctx, repo)
		if err != nil {
			return wrapCommentModeError(repo, err)
		}
		commentBody, err := render.Render(report, "markdown", opts.Lang)
		if err != nil {
			return &ExitError{Code: 1, Err: err}
		}
		if err := ghclient.UpsertMarkerComment(ctx, deps.GitHub, repo, pr.Number, viewerLogin, commentBody, deps.Stderr); err != nil {
			return wrapCommentModeError(repo, err)
		}
	}

	if opts.FailLevel != analysis.RiskLevelNone && result.Score >= opts.FailLevel.Threshold() {
		return &ExitError{
			Code: 3,
			Err:  fmt.Errorf("risk score %d meets fail level %s", result.Score, opts.FailLevel),
		}
	}
	return nil
}

func resolveTarget(ctx context.Context, client ghclient.Client, opts RunPROptions) (ghclient.Repo, int, error) {
	repo, number, repoFromArg, err := parsePRArg(opts.PRArg)
	if err != nil {
		return ghclient.Repo{}, 0, err
	}
	if opts.Repo != "" {
		repo, err = client.ResolveRepo(ctx, opts.Repo)
		if err != nil {
			return ghclient.Repo{}, 0, err
		}
	} else if !repoFromArg {
		repo, err = client.ResolveRepo(ctx, "")
		if err != nil {
			return ghclient.Repo{}, 0, err
		}
	}
	if number == 0 {
		number, err = client.ResolveCurrentPR(ctx, repo)
		if err != nil {
			return ghclient.Repo{}, 0, fmt.Errorf("could not resolve a pull request for the current branch in %s: %w. Pass a PR number, a full PR URL, or --repo OWNER/REPO explicitly", repo.FullName(), err)
		}
	}
	return repo, number, nil
}

func parsePRArg(arg string) (ghclient.Repo, int, bool, error) {
	if strings.TrimSpace(arg) == "" {
		return ghclient.Repo{}, 0, false, nil
	}
	if number, err := strconv.Atoi(arg); err == nil {
		return ghclient.Repo{}, number, false, nil
	}
	parsed, err := url.Parse(arg)
	if err != nil {
		return ghclient.Repo{}, 0, false, fmt.Errorf("invalid PR argument %q", arg)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ghclient.Repo{}, 0, false, fmt.Errorf("unsupported PR URL %q", arg)
	}
	if parsed.Host == "" {
		return ghclient.Repo{}, 0, false, fmt.Errorf("unsupported PR URL %q", arg)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 4 || parts[2] != "pull" {
		return ghclient.Repo{}, 0, false, fmt.Errorf("unsupported PR URL %q", arg)
	}
	number, err := strconv.Atoi(parts[3])
	if err != nil {
		return ghclient.Repo{}, 0, false, fmt.Errorf("invalid PR number in URL %q", arg)
	}
	return ghclient.Repo{
		Host:  parsed.Host,
		Owner: parts[0],
		Name:  parts[1],
	}, number, true, nil
}

func toReviewChanges(changes []ghclient.DependencyReviewChange) []analysis.ReviewChange {
	result := make([]analysis.ReviewChange, 0, len(changes))
	for _, change := range changes {
		if !isSupportedJSEcosystem(change.Ecosystem) {
			continue
		}
		if !isSupportedManifestPath(change.Manifest) {
			continue
		}
		vulns := make([]analysis.Vulnerability, 0, len(change.Vulnerabilities))
		for _, vuln := range change.Vulnerabilities {
			vulns = append(vulns, analysis.Vulnerability{
				Severity: vuln.Severity,
				GHSAID:   vuln.GHSAID,
				Summary:  vuln.Summary,
				URL:      vuln.URL,
			})
		}
		result = append(result, analysis.ReviewChange{
			ChangeType:      analysis.ChangeType(change.ChangeType),
			Manifest:        change.Manifest,
			Name:            change.Name,
			Version:         change.Version,
			Vulnerabilities: vulns,
		})
	}
	return result
}

func compareTargetDependencies(ctx context.Context, client ghclient.Client, repo ghclient.Repo, baseSHA, headSHA string, target analysis.AnalysisTarget) ([]analysis.ReviewChange, bool, error) {
	reviewChanges, err := client.CompareDependenciesForManifest(ctx, repo, baseSHA, headSHA, target.ManifestPath)
	if err != nil {
		if ghclient.IsDependencyReviewUnavailable(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return toReviewChanges(reviewChanges), true, nil
}

func collectRegistryTargets(inputs []analysis.Input) []analysis.PackageVersion {
	seen := map[analysis.PackageVersion]struct{}{}
	for _, input := range inputs {
		for _, target := range analysis.CollectRegistryTargets(input) {
			seen[target] = struct{}{}
		}
	}
	targets := make([]analysis.PackageVersion, 0, len(seen))
	for target := range seen {
		targets = append(targets, target)
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Name == targets[j].Name {
			return targets[i].Version < targets[j].Version
		}
		return targets[i].Name < targets[j].Name
	})
	return targets
}

func wrapGitHubError(err error) error {
	if err == nil {
		return nil
	}
	if ghclient.IsPermissionError(err) || ghclient.IsAuthError(err) {
		return &ExitError{
			Code: 4,
			Err:  fmt.Errorf("%w. Run `gh auth login` or provide GH_TOKEN/GITHUB_TOKEN with repository access", err),
		}
	}
	return &ExitError{Code: 1, Err: err}
}

func wrapCommentModeError(repo ghclient.Repo, err error) error {
	if err == nil {
		return nil
	}
	if ghclient.IsPermissionError(err) || ghclient.IsAuthError(err) {
		return &ExitError{
			Code: 4,
			Err:  fmt.Errorf("comment mode requires permission to read the authenticated GitHub user and write PR issue comments in %s: %w. Check repo access, token scopes, and cross-repo workflow comment limits", repo.FullName(), err),
		}
	}
	return wrapGitHubError(err)
}

func isSupportedManifestPath(manifestPath string) bool {
	switch path.Base(strings.TrimSpace(manifestPath)) {
	case "package.json", "package-lock.json", "pnpm-lock.yaml":
		return true
	default:
		return false
	}
}

func isSupportedJSEcosystem(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "npm", "pnpm", "node", "javascript":
		return true
	default:
		return false
	}
}
