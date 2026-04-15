package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"gh-dep-risk/internal/analysis"
	"gh-dep-risk/internal/app"
	ghclient "gh-dep-risk/internal/github"
	"gh-dep-risk/internal/npm"
)

type multiStringFlag []string

func (f *multiStringFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *multiStringFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func runPR(stdout, stderr io.Writer, args []string) int {
	fs := flag.NewFlagSet("pr", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var opts app.RunPROptions
	var failLevel string
	var paths multiStringFlag

	fs.StringVar(&opts.Repo, "repo", "", "repository in OWNER/REPO form")
	fs.StringVar(&opts.Format, "format", "human", "output format: human|json|markdown")
	fs.StringVar(&opts.Lang, "lang", "ko", "output language: ko|en")
	fs.BoolVar(&opts.Comment, "comment", false, "upsert a PR timeline comment")
	fs.StringVar(&failLevel, "fail-level", string(analysis.RiskLevelNone), "fail threshold: low|medium|high|critical|none")
	fs.BoolVar(&opts.NoRegistry, "no-registry", false, "skip npm registry lookups")
	fs.StringVar(&opts.BundleDir, "bundle-dir", "", "write human/json/markdown bundle files to a directory")
	fs.Var(&paths, "path", "restrict analysis to a repo-relative directory or package.json path (repeatable)")
	fs.BoolVar(&opts.ListTargets, "list-targets", false, "print detected npm analysis targets and exit")
	fs.Usage = func() { printPRUsage(stderr) }

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
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
	opts.Paths = append([]string(nil), paths...)

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
	opts.Format = strings.ToLower(opts.Format)
	switch opts.Format {
	case "human", "json", "markdown":
	default:
		fmt.Fprintf(stderr, "unsupported format %q\n", opts.Format)
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

func printPRUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  gh dep-risk pr [<number>|<url>] [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  gh dep-risk pr 123")
	fmt.Fprintln(w, "  gh dep-risk pr https://github.com/OWNER/REPO/pull/123")
	fmt.Fprintln(w, "  gh dep-risk pr --format json")
	fmt.Fprintln(w, "  gh dep-risk pr 123 --list-targets")
	fmt.Fprintln(w, "  gh dep-risk pr 123 --path apps/web")
	fmt.Fprintln(w, "  gh dep-risk pr --bundle-dir ./dep-risk-bundle")
	fmt.Fprintln(w, "  gh dep-risk pr --comment")
	fmt.Fprintln(w, "  gh dep-risk pr --fail-level high")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  -repo string")
	fmt.Fprintln(w, "    \trepository in OWNER/REPO form")
	fmt.Fprintln(w, "  -format string")
	fmt.Fprintln(w, "    \toutput format: human|json|markdown (default \"human\")")
	fmt.Fprintln(w, "  -lang string")
	fmt.Fprintln(w, "    \toutput language: ko|en (default \"ko\")")
	fmt.Fprintln(w, "  -comment")
	fmt.Fprintln(w, "    \tupsert a PR timeline comment")
	fmt.Fprintln(w, "  -fail-level string")
	fmt.Fprintln(w, "    \tfail threshold: low|medium|high|critical|none (default \"none\")")
	fmt.Fprintln(w, "  -no-registry")
	fmt.Fprintln(w, "    \tskip npm registry lookups")
	fmt.Fprintln(w, "  -bundle-dir string")
	fmt.Fprintln(w, "    \twrite human/json/markdown bundle files to a directory")
	fmt.Fprintln(w, "  -path value")
	fmt.Fprintln(w, "    \trestrict analysis to a repo-relative directory or package.json path (repeatable)")
	fmt.Fprintln(w, "  -list-targets")
	fmt.Fprintln(w, "    \tprint detected npm analysis targets and exit")
}
