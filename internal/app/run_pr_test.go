package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gh-dep-risk/internal/analysis"
	ghclient "gh-dep-risk/internal/github"
	"gh-dep-risk/internal/render"
	"github.com/cli/go-gh/v2/pkg/api"
)

func TestParsePRArg(t *testing.T) {
	t.Run("number", func(t *testing.T) {
		repo, number, repoFromArg, err := parsePRArg("123")
		if err != nil {
			t.Fatal(err)
		}
		if repo != (ghclient.Repo{}) {
			t.Fatalf("expected empty repo, got %#v", repo)
		}
		if number != 123 {
			t.Fatalf("expected PR 123, got %d", number)
		}
		if repoFromArg {
			t.Fatalf("expected repoFromArg=false")
		}
	})

	t.Run("url", func(t *testing.T) {
		repo, number, repoFromArg, err := parsePRArg("https://github.com/OWNER/REPO/pull/456")
		if err != nil {
			t.Fatal(err)
		}
		if number != 456 {
			t.Fatalf("expected PR 456, got %d", number)
		}
		if !repoFromArg {
			t.Fatalf("expected repoFromArg=true")
		}
		expected := ghclient.Repo{Host: "github.com", Owner: "OWNER", Name: "REPO"}
		if repo != expected {
			t.Fatalf("unexpected repo: %#v", repo)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		if _, _, _, err := parsePRArg("github.com/OWNER/REPO/pull/123"); err == nil {
			t.Fatalf("expected invalid URL error")
		}
	})
}

func TestResolveTargetUsesCurrentBranchPRWhenArgMissing(t *testing.T) {
	client := newFakeGitHubClient()
	client.repo = testRepo()
	client.resolveCurrentPRNumber = 77

	repo, number, err := resolveTarget(context.Background(), client, RunPROptions{})
	if err != nil {
		t.Fatal(err)
	}
	if repo != client.repo {
		t.Fatalf("unexpected repo: %#v", repo)
	}
	if number != 77 {
		t.Fatalf("expected PR 77, got %d", number)
	}
	if client.resolveCurrentPRCalls != 1 {
		t.Fatalf("expected ResolveCurrentPR to be called once, got %d", client.resolveCurrentPRCalls)
	}
}

func TestRunPRExitCodeNoSupportedChange(t *testing.T) {
	client := newConfiguredFakeGitHubClient(t)
	client.files = []ghclient.PullRequestFile{{Filename: "README.md", Status: "modified"}}
	client.compareChanges = nil

	_, _, err := runPRWithClient(t, client, RunPROptions{})
	assertExitCode(t, err, 2)
}

func TestRunPRExitCodeFailLevel(t *testing.T) {
	client := newConfiguredFakeGitHubClient(t)

	stdout, _, err := runPRWithClient(t, client, RunPROptions{FailLevel: analysis.RiskLevelHigh})
	assertExitCode(t, err, 3)
	if !strings.Contains(stdout, "owner/repo") {
		t.Fatalf("expected human output to include repo, got %q", stdout)
	}
}

func TestRunPRExitCodeAuth(t *testing.T) {
	client := newConfiguredFakeGitHubClient(t)
	client.getPullRequestErr = ghclient.AuthError{Op: "pull"}

	_, _, err := runPRWithClient(t, client, RunPROptions{})
	assertExitCode(t, err, 4)
}

func TestRunPRExitCodeGeneralError(t *testing.T) {
	client := newConfiguredFakeGitHubClient(t)
	client.getPullRequestErr = errors.New("boom")

	_, _, err := runPRWithClient(t, client, RunPROptions{})
	assertExitCode(t, err, 1)
}

func TestRunPRDependencyReviewFallback(t *testing.T) {
	client := newConfiguredFakeGitHubClient(t)
	client.compareErr = &api.HTTPError{StatusCode: 404, Message: "dependency review disabled"}

	stdout, _, err := runPRWithClient(t, client, RunPROptions{Format: "json"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"dependency_review_available": false`) {
		t.Fatalf("expected fallback JSON output, got %q", stdout)
	}
}

func TestRunPRDependencyReviewUnexpectedError(t *testing.T) {
	client := newConfiguredFakeGitHubClient(t)
	client.compareErr = errors.New("dependency review transport error")

	_, _, err := runPRWithClient(t, client, RunPROptions{})
	assertExitCode(t, err, 1)
}

func TestRunPRCommentUpsertCreatesComment(t *testing.T) {
	client := newConfiguredFakeGitHubClient(t)

	_, _, err := runPRWithClient(t, client, RunPROptions{Comment: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(client.createdComments) != 1 {
		t.Fatalf("expected one created comment, got %d", len(client.createdComments))
	}
	if !strings.HasPrefix(client.createdComments[0], ghclient.MarkerComment) {
		t.Fatalf("expected marker comment body, got %q", client.createdComments[0])
	}
	if len(client.updatedComments) != 0 || len(client.deletedComments) != 0 {
		t.Fatalf("expected no update/delete on create path")
	}
}

func TestRunPRCommentUpsertUpdatesNewestAndDeletesOlderDuplicates(t *testing.T) {
	client := newConfiguredFakeGitHubClient(t)
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	client.comments = []ghclient.IssueComment{
		{
			ID:        10,
			Body:      ghclient.MarkerComment + "\nold",
			UserLogin: "reviewer",
			CreatedAt: now.Add(-2 * time.Hour),
		},
		{
			ID:        11,
			Body:      ghclient.MarkerComment + "\nforeign",
			UserLogin: "teammate",
			CreatedAt: now.Add(-90 * time.Minute),
		},
		{
			ID:        12,
			Body:      ghclient.MarkerComment + "\nnewest",
			UserLogin: "reviewer",
			CreatedAt: now.Add(-30 * time.Minute),
		},
	}

	_, stderr, err := runPRWithClient(t, client, RunPROptions{Comment: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := client.updatedComments[12]; !ok {
		t.Fatalf("expected newest own marker comment to be updated")
	}
	if _, ok := client.updatedComments[11]; ok {
		t.Fatalf("expected foreign marker comment to remain untouched")
	}
	if !reflect.DeepEqual(client.deletedComments, []int64{10}) {
		t.Fatalf("expected only older own duplicate to be deleted, got %v", client.deletedComments)
	}
	if !strings.Contains(stderr, "warning: found marker comment owned by teammate") {
		t.Fatalf("expected foreign marker warning, got %q", stderr)
	}
}

func TestRunPRWritesBundleFromSingleAnalysisPass(t *testing.T) {
	client := newConfiguredFakeGitHubClient(t)
	bundleDir := t.TempDir()

	stdout, _, err := runPRWithClient(t, client, RunPROptions{
		Format:    "human",
		Lang:      "en",
		BundleDir: bundleDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "owner/repo") {
		t.Fatalf("expected stdout output, got %q", stdout)
	}
	for _, name := range []string{"dep-risk-human.txt", "dep-risk.json", "dep-risk.md", "metadata.json"} {
		if _, statErr := os.Stat(filepath.Join(bundleDir, name)); statErr != nil {
			t.Fatalf("expected bundle file %s: %v", name, statErr)
		}
	}
	if client.viewerLoginCalls != 0 || client.getPullRequestCalls != 1 || client.listPullRequestCalls != 1 || client.compareCalls != 1 || client.listRepositoryFileCalls != 2 {
		t.Fatalf("expected a single GitHub analysis pass, got viewer=%d pr=%d files=%d compare=%d trees=%d", client.viewerLoginCalls, client.getPullRequestCalls, client.listPullRequestCalls, client.compareCalls, client.listRepositoryFileCalls)
	}
	if client.getFileCalls != 4 {
		t.Fatalf("expected four manifest/lockfile fetches, got %d", client.getFileCalls)
	}
}

func TestRunPRWritesBundleBeforeFailLevelExit(t *testing.T) {
	client := newConfiguredFakeGitHubClient(t)
	bundleDir := t.TempDir()

	_, _, err := runPRWithClient(t, client, RunPROptions{
		Lang:      "en",
		BundleDir: bundleDir,
		FailLevel: analysis.RiskLevelHigh,
	})
	assertExitCode(t, err, 3)
	for _, name := range []string{"dep-risk-human.txt", "dep-risk.json", "dep-risk.md", "metadata.json"} {
		if _, statErr := os.Stat(filepath.Join(bundleDir, name)); statErr != nil {
			t.Fatalf("expected bundle file %s after fail-level exit: %v", name, statErr)
		}
	}
}

func TestRunPRListTargets(t *testing.T) {
	client := newWorkspaceFakeGitHubClient(t)

	stdout, _, err := runPRWithClient(t, client, RunPROptions{ListTargets: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"root\troot\tpackage.json\tpackage-lock.json",
		"workspace\tapps/web\tapps/web/package.json\tpackage-lock.json",
		"workspace\tpackages/ui\tpackages/ui/package.json\tpackage-lock.json",
		"standalone\tservices/api\tservices/api/package.json\tservices/api/package-lock.json",
	} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("expected target listing to contain %q, got %q", expected, stdout)
		}
	}
}

func TestRunPRPathFiltering(t *testing.T) {
	client := newWorkspaceFakeGitHubClient(t)
	client.files = []ghclient.PullRequestFile{
		{Filename: "apps/web/package.json", Status: "modified"},
		{Filename: "packages/ui/package.json", Status: "modified"},
		{Filename: "package-lock.json", Status: "modified"},
	}

	stdout, _, err := runPRWithClient(t, client, RunPROptions{
		Format: "json",
		Paths:  []string{"apps/web"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload render.JSONReport
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Targets) != 1 || payload.Targets[0].Target.DisplayName != "apps/web" {
		t.Fatalf("expected only apps/web target, got %#v", payload.Targets)
	}
	if client.compareManifestCalls["apps/web/package.json"] != 1 {
		t.Fatalf("expected apps/web manifest compare call")
	}
	if client.compareManifestCalls["packages/ui/package.json"] != 0 {
		t.Fatalf("expected packages/ui manifest compare to be skipped")
	}
}

func TestRunPRPathFilteringRejectsUnknownTarget(t *testing.T) {
	client := newWorkspaceFakeGitHubClient(t)

	_, _, err := runPRWithClient(t, client, RunPROptions{Paths: []string{"apps/unknown"}})
	assertExitCode(t, err, 1)
	if !strings.Contains(err.Error(), "unknown npm target path") {
		t.Fatalf("expected clear unknown target error, got %v", err)
	}
}

func TestRunPRAggregatesMultipleTargets(t *testing.T) {
	client := newWorkspaceFakeGitHubClient(t)
	client.files = []ghclient.PullRequestFile{
		{Filename: "apps/web/package.json", Status: "modified"},
		{Filename: "packages/ui/package.json", Status: "modified"},
		{Filename: "package-lock.json", Status: "modified"},
	}

	stdout, _, err := runPRWithClient(t, client, RunPROptions{Format: "json"})
	if err != nil {
		t.Fatal(err)
	}

	var payload render.JSONReport
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Targets) != 2 {
		t.Fatalf("expected two analyzed targets, got %#v", payload.Targets)
	}
	if payload.Targets[0].Target.DisplayName != "apps/web" {
		t.Fatalf("expected riskiest target first, got %#v", payload.Targets)
	}
	if payload.DependencyReviewAvailable != true {
		t.Fatalf("expected dependency review to be available")
	}
	if payload.Score <= payload.Targets[0].Score {
		t.Fatalf("expected aggregate score bonus above max target score, got %#v", payload)
	}
}

func TestRunPRDependencyReviewFallbackForSingleTargetMarksAggregate(t *testing.T) {
	client := newWorkspaceFakeGitHubClient(t)
	client.files = []ghclient.PullRequestFile{
		{Filename: "packages/ui/package.json", Status: "modified"},
		{Filename: "package-lock.json", Status: "modified"},
	}
	client.compareManifestErr["packages/ui/package.json"] = &api.HTTPError{StatusCode: 404, Message: "dependency review disabled"}

	stdout, _, err := runPRWithClient(t, client, RunPROptions{Format: "json", Paths: []string{"packages/ui"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, `"dependency_review_available": false`) {
		t.Fatalf("expected aggregate fallback output, got %q", stdout)
	}
}

func TestRunPRBundleIncludesPerTargetFiles(t *testing.T) {
	client := newWorkspaceFakeGitHubClient(t)
	client.files = []ghclient.PullRequestFile{
		{Filename: "apps/web/package.json", Status: "modified"},
		{Filename: "packages/ui/package.json", Status: "modified"},
		{Filename: "package-lock.json", Status: "modified"},
	}
	bundleDir := t.TempDir()

	_, _, err := runPRWithClient(t, client, RunPROptions{BundleDir: bundleDir})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		filepath.Join(bundleDir, "targets", "apps-web", "dep-risk.json"),
		filepath.Join(bundleDir, "targets", "apps-web", "dep-risk.md"),
		filepath.Join(bundleDir, "targets", "packages-ui", "dep-risk.json"),
		filepath.Join(bundleDir, "targets", "packages-ui", "dep-risk.md"),
	} {
		if _, err := os.Stat(name); err != nil {
			t.Fatalf("expected per-target bundle file %s: %v", name, err)
		}
	}
}

type fakeGitHubClient struct {
	repo                     ghclient.Repo
	viewerLogin              string
	viewerLoginErr           error
	resolveRepoErr           error
	resolveCurrentPRNumber   int
	resolveCurrentPRErr      error
	resolveCurrentPRCalls    int
	viewerLoginCalls         int
	pr                       ghclient.PullRequest
	getPullRequestErr        error
	getPullRequestCalls      int
	files                    []ghclient.PullRequestFile
	listPullRequestErr       error
	listPullRequestCalls     int
	repositoryFilesByRef     map[string][]string
	listRepositoryFileCalls  int
	compareChanges           []ghclient.DependencyReviewChange
	compareChangesByManifest map[string][]ghclient.DependencyReviewChange
	compareErr               error
	compareCalls             int
	compareManifestErr       map[string]error
	compareManifestCalls     map[string]int
	filesByKey               map[string][]byte
	getFileErr               map[string]error
	getFileCalls             int
	comments                 []ghclient.IssueComment
	listCommentsErr          error
	listCommentsCalls        int
	createCommentErr         error
	createCommentCalls       int
	updateCommentErr         error
	updateCommentCalls       int
	deleteCommentErr         error
	deleteCommentCalls       int
	createdComments          []string
	updatedComments          map[int64]string
	deletedComments          []int64
}

func newFakeGitHubClient() *fakeGitHubClient {
	return &fakeGitHubClient{
		repo:                     testRepo(),
		viewerLogin:              "reviewer",
		resolveCurrentPRNumber:   123,
		updatedComments:          map[int64]string{},
		repositoryFilesByRef:     map[string][]string{},
		compareChangesByManifest: map[string][]ghclient.DependencyReviewChange{},
		compareManifestErr:       map[string]error{},
		compareManifestCalls:     map[string]int{},
		filesByKey:               map[string][]byte{},
		getFileErr:               map[string]error{},
	}
}

func newConfiguredFakeGitHubClient(t *testing.T) *fakeGitHubClient {
	t.Helper()
	client := newFakeGitHubClient()
	client.pr = ghclient.PullRequest{
		Title:       "Update dependencies",
		Draft:       false,
		Number:      123,
		BaseSHA:     "base-sha",
		HeadSHA:     "head-sha",
		URL:         "https://github.com/owner/repo/pull/123",
		AuthorLogin: "octocat",
	}
	client.files = []ghclient.PullRequestFile{
		{Filename: "package.json", Status: "modified"},
		{Filename: "package-lock.json", Status: "modified"},
	}
	client.compareChanges = []ghclient.DependencyReviewChange{
		{Name: "left-pad", Manifest: "package.json", Ecosystem: "npm", ChangeType: "removed", Version: "1.0.0"},
		{Name: "left-pad", Manifest: "package.json", Ecosystem: "npm", ChangeType: "added", Version: "2.0.0"},
		{Name: "chalk", Manifest: "package.json", Ecosystem: "npm", ChangeType: "added", Version: "5.0.0"},
	}
	client.filesByKey[fileKey("package.json", "base-sha")] = readFixture(t, "base.package.json")
	client.filesByKey[fileKey("package.json", "head-sha")] = readFixture(t, "head.package.json")
	client.filesByKey[fileKey("package-lock.json", "base-sha")] = readFixture(t, "base.package-lock.json")
	client.filesByKey[fileKey("package-lock.json", "head-sha")] = readFixture(t, "head.package-lock.json")
	client.repositoryFilesByRef["base-sha"] = []string{"package.json", "package-lock.json"}
	client.repositoryFilesByRef["head-sha"] = []string{"package.json", "package-lock.json"}
	return client
}

func newWorkspaceFakeGitHubClient(t *testing.T) *fakeGitHubClient {
	t.Helper()
	client := newFakeGitHubClient()
	client.pr = ghclient.PullRequest{
		Title:       "Update workspace dependencies",
		Draft:       false,
		Number:      123,
		BaseSHA:     "base-sha",
		HeadSHA:     "head-sha",
		URL:         "https://github.com/owner/repo/pull/123",
		AuthorLogin: "octocat",
	}
	client.files = []ghclient.PullRequestFile{
		{Filename: "apps/web/package.json", Status: "modified"},
		{Filename: "packages/ui/package.json", Status: "modified"},
		{Filename: "package-lock.json", Status: "modified"},
	}
	client.repositoryFilesByRef["base-sha"] = []string{
		"package.json",
		"package-lock.json",
		"apps/web/package.json",
		"packages/ui/package.json",
		"services/api/package.json",
		"services/api/package-lock.json",
	}
	client.repositoryFilesByRef["head-sha"] = append([]string(nil), client.repositoryFilesByRef["base-sha"]...)
	client.filesByKey[fileKey("package.json", "base-sha")] = readFixture(t, "workspace.root.package.json")
	client.filesByKey[fileKey("package.json", "head-sha")] = readFixture(t, "workspace.root.package.json")
	client.filesByKey[fileKey("package-lock.json", "base-sha")] = readFixture(t, "workspace.root.base.package-lock.json")
	client.filesByKey[fileKey("package-lock.json", "head-sha")] = readFixture(t, "workspace.root.head.package-lock.json")
	client.filesByKey[fileKey("apps/web/package.json", "base-sha")] = readFixture(t, "workspace.apps-web.base.package.json")
	client.filesByKey[fileKey("apps/web/package.json", "head-sha")] = readFixture(t, "workspace.apps-web.head.package.json")
	client.filesByKey[fileKey("packages/ui/package.json", "base-sha")] = readFixture(t, "workspace.packages-ui.base.package.json")
	client.filesByKey[fileKey("packages/ui/package.json", "head-sha")] = readFixture(t, "workspace.packages-ui.head.package.json")
	client.filesByKey[fileKey("services/api/package.json", "base-sha")] = readFixture(t, "standalone.service.base.package.json")
	client.filesByKey[fileKey("services/api/package.json", "head-sha")] = readFixture(t, "standalone.service.head.package.json")
	client.filesByKey[fileKey("services/api/package-lock.json", "base-sha")] = readFixture(t, "standalone.service.base.package-lock.json")
	client.filesByKey[fileKey("services/api/package-lock.json", "head-sha")] = readFixture(t, "standalone.service.head.package-lock.json")
	client.compareChangesByManifest["apps/web/package.json"] = []ghclient.DependencyReviewChange{
		{Name: "axios", Manifest: "apps/web/package.json", Ecosystem: "npm", ChangeType: "added", Version: "1.7.0"},
	}
	client.compareChangesByManifest["packages/ui/package.json"] = []ghclient.DependencyReviewChange{
		{Name: "tailwind-merge", Manifest: "packages/ui/package.json", Ecosystem: "npm", ChangeType: "added", Version: "2.3.0"},
	}
	client.compareChangesByManifest["services/api/package.json"] = []ghclient.DependencyReviewChange{
		{Name: "lodash", Manifest: "services/api/package.json", Ecosystem: "npm", ChangeType: "removed", Version: "4.17.20"},
		{Name: "lodash", Manifest: "services/api/package.json", Ecosystem: "npm", ChangeType: "added", Version: "4.17.21"},
	}
	return client
}

func (f *fakeGitHubClient) ResolveRepo(_ context.Context, override string) (ghclient.Repo, error) {
	if f.resolveRepoErr != nil {
		return ghclient.Repo{}, f.resolveRepoErr
	}
	if override != "" {
		parts := strings.Split(override, "/")
		if len(parts) == 2 {
			return ghclient.Repo{Host: "github.com", Owner: parts[0], Name: parts[1]}, nil
		}
	}
	return f.repo, nil
}

func (f *fakeGitHubClient) ViewerLogin(context.Context, ghclient.Repo) (string, error) {
	f.viewerLoginCalls++
	return f.viewerLogin, f.viewerLoginErr
}

func (f *fakeGitHubClient) ResolveCurrentPR(context.Context, ghclient.Repo) (int, error) {
	f.resolveCurrentPRCalls++
	return f.resolveCurrentPRNumber, f.resolveCurrentPRErr
}

func (f *fakeGitHubClient) GetPullRequest(context.Context, ghclient.Repo, int) (ghclient.PullRequest, error) {
	f.getPullRequestCalls++
	return f.pr, f.getPullRequestErr
}

func (f *fakeGitHubClient) ListPullRequestFiles(context.Context, ghclient.Repo, int) ([]ghclient.PullRequestFile, error) {
	f.listPullRequestCalls++
	return append([]ghclient.PullRequestFile(nil), f.files...), f.listPullRequestErr
}

func (f *fakeGitHubClient) CompareDependencies(context.Context, ghclient.Repo, string, string) ([]ghclient.DependencyReviewChange, error) {
	f.compareCalls++
	return append([]ghclient.DependencyReviewChange(nil), f.compareChanges...), f.compareErr
}

func (f *fakeGitHubClient) CompareDependenciesForManifest(_ context.Context, _ ghclient.Repo, _, _ string, manifestPath string) ([]ghclient.DependencyReviewChange, error) {
	f.compareCalls++
	f.compareManifestCalls[manifestPath]++
	if err, ok := f.compareManifestErr[manifestPath]; ok {
		return nil, err
	}
	if changes, ok := f.compareChangesByManifest[manifestPath]; ok {
		return append([]ghclient.DependencyReviewChange(nil), changes...), nil
	}
	return append([]ghclient.DependencyReviewChange(nil), f.compareChanges...), f.compareErr
}

func (f *fakeGitHubClient) ListRepositoryFiles(_ context.Context, _ ghclient.Repo, ref string) ([]string, error) {
	f.listRepositoryFileCalls++
	return append([]string(nil), f.repositoryFilesByRef[ref]...), nil
}

func (f *fakeGitHubClient) GetRepositoryFile(_ context.Context, _ ghclient.Repo, path, ref string) ([]byte, error) {
	f.getFileCalls++
	if err, ok := f.getFileErr[fileKey(path, ref)]; ok {
		return nil, err
	}
	data, ok := f.filesByKey[fileKey(path, ref)]
	if !ok {
		return nil, ghclient.ErrNotFound
	}
	return append([]byte(nil), data...), nil
}

func (f *fakeGitHubClient) ListIssueComments(context.Context, ghclient.Repo, int) ([]ghclient.IssueComment, error) {
	f.listCommentsCalls++
	if f.listCommentsErr != nil {
		return nil, f.listCommentsErr
	}
	return append([]ghclient.IssueComment(nil), f.comments...), nil
}

func (f *fakeGitHubClient) CreateIssueComment(_ context.Context, _ ghclient.Repo, _ int, body string) (ghclient.IssueComment, error) {
	f.createCommentCalls++
	if f.createCommentErr != nil {
		return ghclient.IssueComment{}, f.createCommentErr
	}
	f.createdComments = append(f.createdComments, body)
	return ghclient.IssueComment{ID: int64(100 + len(f.createdComments)), Body: body, UserLogin: f.viewerLogin}, nil
}

func (f *fakeGitHubClient) UpdateIssueComment(_ context.Context, _ ghclient.Repo, commentID int64, body string) error {
	f.updateCommentCalls++
	if f.updateCommentErr != nil {
		return f.updateCommentErr
	}
	f.updatedComments[commentID] = body
	return nil
}

func (f *fakeGitHubClient) DeleteIssueComment(_ context.Context, _ ghclient.Repo, commentID int64) error {
	f.deleteCommentCalls++
	if f.deleteCommentErr != nil {
		return f.deleteCommentErr
	}
	f.deletedComments = append(f.deletedComments, commentID)
	return nil
}

func runPRWithClient(t *testing.T, client *fakeGitHubClient, opts RunPROptions) (string, string, error) {
	t.Helper()
	if opts.Format == "" {
		opts.Format = "human"
	}
	if opts.Lang == "" {
		opts.Lang = "en"
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := RunPR(context.Background(), RunPRDependencies{
		GitHub: client,
		Stdout: &stdout,
		Stderr: &stderr,
	}, opts)
	return stdout.String(), stderr.String(), err
}

func assertExitCode(t *testing.T, err error, expected int) {
	t.Helper()
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %v", err)
	}
	if exitErr.Code != expected {
		t.Fatalf("expected exit code %d, got %d", expected, exitErr.Code)
	}
}

func testRepo() ghclient.Repo {
	return ghclient.Repo{Host: "github.com", Owner: "owner", Name: "repo"}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func fileKey(path, ref string) string {
	return path + "@" + ref
}
