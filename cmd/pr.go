package cmd

import (
	"context"
	"flag"
	"fmt"
	"io"

	"gh-dep-risk/internal/analysis"
	"gh-dep-risk/internal/app"
	ghclient "gh-dep-risk/internal/github"
	"gh-dep-risk/internal/npm"
)

func runPR(stdout, stderr io.Writer, args []string) int {
	fs := flag.NewFlagSet("pr", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var opts app.RunPROptions
	var failLevel string

	fs.StringVar(&opts.Repo, "repo", "", "repository in OWNER/REPO form")
	fs.StringVar(&opts.Format, "format", "human", "output format: human|json|markdown")
	fs.StringVar(&opts.Lang, "lang", "ko", "output language: ko|en")
	fs.BoolVar(&opts.Comment, "comment", false, "upsert a PR timeline comment")
	fs.StringVar(&failLevel, "fail-level", string(analysis.RiskLevelNone), "fail threshold: low|medium|high|critical|none")
	fs.BoolVar(&opts.NoRegistry, "no-registry", false, "skip npm registry lookups")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage:")
		fmt.Fprintln(stderr, "  gh dep-risk pr [<number>|<url>] [flags]")
		fmt.Fprintln(stderr)
		fmt.Fprintln(stderr, "Flags:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(stderr, "expected at most one PR argument")
		fs.Usage()
		return 1
	}
	if fs.NArg() == 1 {
		opts.PRArg = fs.Arg(0)
	}

	level, err := analysis.ParseRiskLevel(failLevel)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	opts.FailLevel = level
	if opts.Lang != "ko" && opts.Lang != "en" {
		fmt.Fprintf(stderr, "unsupported lang %q\n", opts.Lang)
		return 1
	}

	runErr := app.RunPR(context.Background(), app.RunPRDependencies{
		GitHub:   ghclient.NewClient(),
		Registry: npm.NewRegistryClient(),
		Stdout:   stdout,
		Stderr:   stderr,
	}, opts)
	if runErr == nil {
		return 0
	}

	exitErr, ok := runErr.(*app.ExitError)
	if !ok {
		fmt.Fprintln(stderr, runErr)
		return 1
	}
	if exitErr.Err != nil {
		fmt.Fprintln(stderr, exitErr.Err)
	}
	return exitErr.Code
}
