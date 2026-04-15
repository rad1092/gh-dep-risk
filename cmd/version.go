package cmd

import (
	"flag"
	"fmt"
	"io"
)

func runVersion(stdout, stderr io.Writer, args []string) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var jsonOutput bool
	fs.BoolVar(&jsonOutput, "json", false, "print version metadata as JSON")
	fs.Usage = func() { printVersionUsage(stdout) }

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "version does not accept positional arguments")
		return 1
	}

	info := currentBuildInfo()
	if jsonOutput {
		payload, err := info.JSON()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprint(stdout, payload)
		return 0
	}
	fmt.Fprintln(stdout, info.String())
	return 0
}

func printVersionUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  gh dep-risk version [--json]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Print the gh-dep-risk build metadata.")
}
