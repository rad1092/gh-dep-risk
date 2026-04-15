package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gh-dep-risk/internal/github"
)

func TestRunPRPipeline(t *testing.T) {
	basePackageJSON, _ := os.ReadFile(filepath.Join("..", "..", "testdata", "base.package.json"))
	headPackageJSON, _ := os.ReadFile(filepath.Join("..", "..", "testdata", "head.package.json"))
	basePackageLock, _ := os.ReadFile(filepath.Join("..", "..", "testdata", "base.package-lock.json"))
	headPackageLock, _ := os.ReadFile(filepath.Join("..", "..", "testdata", "head.package-lock.json"))

	t.Run("normal dependency review path", func(t *testing.T) {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		client := &fakeGitHub{
			repo:   github.Repo{Host: "github.com", Owner: "owner", Name: "repo"},
			viewer: "reviewer",
			pr: github.PullRequest{
				Title:       "Update deps",
				Number:      12,
				BaseSHA:     "base",
				HeadSHA:     "head",
				URL:         "https://github.com/owner/repo/pull/12",
				AuthorLogin: "author",
			},
			reviewChanges: []github.DependencyReviewChange{
				{Ecosystem: "npm", Manifest: "package.json", Name: "left-pad", ChangeType: "removed", Version: "1.0.0"},
				{Ecosystem: "npm", Manifest: "package.json", Name: "left-pad", ChangeType: "added", Version: "2.0.0"},
			},
			files: map[string][]byte{
				"base:package.json":      basePackageJSON,
				"head:package.json":      headPackageJSON,
				"base:package-lock.json": basePackageLock,
				"head:package-lock.json": headPackageLock,
			},
		}
		registry := &fakeRegistry{times: map[string]time.Time{
			"left-pad@2.0.0": time.Now().UTC().Add(-48 * time.Hour),
		}}

		err := RunPR(context.Background(), RunPRDependencies{
			GitHub:   client,
			Registry: registry,
			Stdout:   stdout,
			Stderr:   stderr,
		}, RunPROptions{Format: "json", Lang: "en"})
		if err != nil {
			t.Fatal(err)
		}

		var payload struct {
			Analysis struct {
				Score                     int  `json:"score"`
				DependencyReviewAvailable bool `json:"dependency_review_available"`
			} `json:"analysis"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Analysis.Score == 0 {
			t.Fatalf("expected non-zero score")
		}
		if !payload.Analysis.DependencyReviewAvailable {
			t.Fatalf("expected dependency review to be available")
		}
	})

	t.Run("dependency review fallback", func(t *testing.T) {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		client := &fakeGitHub{
			repo:   github.Repo{Host: "github.com", Owner: "owner", Name: "repo"},
			viewer: "reviewer",
			pr: github.PullRequest{
				Title:       "Update deps",
				Number:      12,
				BaseSHA:     "base",
				HeadSHA:     "head",
				URL:         "https://github.com/owner/repo/pull/12",
				AuthorLogin: "author",
			},
			compareErr: errors.New("dependency review unavailable"),
			files: map[string][]byte{
				"base:package.json":      basePackageJSON,
				"head:package.json":      headPackageJSON,
				"base:package-lock.json": basePackageLock,
				"head:package-lock.json": headPackageLock,
			},
		}

		err := RunPR(context.Background(), RunPRDependencies{
			GitHub:   client,
			Registry: &fakeRegistry{},
			Stdout:   stdout,
			Stderr:   stderr,
		}, RunPROptions{Format: "json", Lang: "en"})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(stdout.String(), `"dependency_review_available": false`) {
			t.Fatalf("expected fallback flag in output")
		}
	})

	t.Run("comment upsert rules", func(t *testing.T) {
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		client := &fakeGitHub{
			repo:   github.Repo{Host: "github.com", Owner: "owner", Name: "repo"},
			viewer: "reviewer",
			pr: github.PullRequest{
				Title:       "Update deps",
				Number:      12,
				BaseSHA:     "base",
				HeadSHA:     "head",
				URL:         "https://github.com/owner/repo/pull/12",
				AuthorLogin: "author",
			},
			reviewChanges: []github.DependencyReviewChange{
				{Ecosystem: "npm", Manifest: "package.json", Name: "left-pad", ChangeType: "removed", Version: "1.0.0"},
				{Ecosystem: "npm", Manifest: "package.json", Name: "left-pad", ChangeType: "added", Version: "2.0.0"},
			},
			files: map[string][]byte{
				"base:package.json":      basePackageJSON,
				"head:package.json":      headPackageJSON,
				"base:package-lock.json": basePackageLock,
				"head:package-lock.json": headPackageLock,
			},
			comments: []github.IssueComment{
				{ID: 1, Body: "<!-- gh-dep-risk -->\nold", UserLogin: "reviewer", CreatedAt: time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)},
				{ID: 2, Body: "<!-- gh-dep-risk -->\nnewer", UserLogin: "reviewer", CreatedAt: time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC)},
				{ID: 3, Body: "<!-- gh-dep-risk -->\nforeign", UserLogin: "another-user", CreatedAt: time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC)},
			},
		}

		err := RunPR(context.Background(), RunPRDependencies{
			GitHub:   client,
			Registry: &fakeRegistry{},
			Stdout:   stdout,
			Stderr:   stderr,
		}, RunPROptions{Format: "markdown", Lang: "en", Comment: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(client.deletedComments) != 1 || client.deletedComments[0] != 1 {
			t.Fatalf("expected older duplicate to be deleted, got %+v", client.deletedComments)
		}
		if client.updatedCommentID != 2 {
			t.Fatalf("expected newest own marker comment to be updated, got %d", client.updatedCommentID)
		}
		if !strings.Contains(stderr.String(), "another-user") {
			t.Fatalf("expected warning for foreign marker comment")
		}
	})
}

type fakeGitHub struct {
	repo               github.Repo
	viewer             string
	pr                 github.PullRequest
	reviewChanges      []github.DependencyReviewChange
	compareErr         error
	files              map[string][]byte
	comments           []github.IssueComment
	updatedCommentID   int64
	updatedCommentBody string
	deletedComments    []int64
}

func (f *fakeGitHub) ResolveRepo(context.Context, string) (github.Repo, error) {
	return f.repo, nil
}

func (f *fakeGitHub) ViewerLogin(context.Context, github.Repo) (string, error) {
	return f.viewer, nil
}

func (f *fakeGitHub) ResolveCurrentPR(context.Context, github.Repo) (int, error) {
	return f.pr.Number, nil
}

func (f *fakeGitHub) GetPullRequest(context.Context, github.Repo, int) (github.PullRequest, error) {
	return f.pr, nil
}

func (f *fakeGitHub) ListPullRequestFiles(context.Context, github.Repo, int) ([]github.PullRequestFile, error) {
	return []github.PullRequestFile{{Filename: "package.json", Status: "modified"}}, nil
}

func (f *fakeGitHub) CompareDependencies(context.Context, github.Repo, string, string) ([]github.DependencyReviewChange, error) {
	if f.compareErr != nil {
		return nil, f.compareErr
	}
	return f.reviewChanges, nil
}

func (f *fakeGitHub) GetRepositoryFile(_ context.Context, _ github.Repo, path, ref string) ([]byte, error) {
	key := ref + ":" + path
	data, ok := f.files[key]
	if !ok {
		return nil, github.ErrNotFound
	}
	return data, nil
}

func (f *fakeGitHub) ListIssueComments(context.Context, github.Repo, int) ([]github.IssueComment, error) {
	return append([]github.IssueComment(nil), f.comments...), nil
}

func (f *fakeGitHub) CreateIssueComment(context.Context, github.Repo, int, string) (github.IssueComment, error) {
	return github.IssueComment{}, errors.New("not used")
}

func (f *fakeGitHub) UpdateIssueComment(_ context.Context, _ github.Repo, commentID int64, body string) error {
	f.updatedCommentID = commentID
	f.updatedCommentBody = body
	return nil
}

func (f *fakeGitHub) DeleteIssueComment(_ context.Context, _ github.Repo, commentID int64) error {
	f.deletedComments = append(f.deletedComments, commentID)
	return nil
}

type fakeRegistry struct {
	times map[string]time.Time
}

func (f *fakeRegistry) PublishedAt(_ context.Context, packageName, version string) (time.Time, error) {
	if f.times == nil {
		return time.Time{}, errors.New("not found")
	}
	value, ok := f.times[packageName+"@"+version]
	if !ok {
		return time.Time{}, errors.New("not found")
	}
	return value, nil
}
