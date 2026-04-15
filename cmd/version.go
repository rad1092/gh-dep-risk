package cmd

import (
	"fmt"
	"io"
)

func runVersion(stdout, stderr io.Writer, args []string) int {
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help" || args[0] == "help") {
		printVersionUsage(stdout)
		return 0
	}
	if len(args) != 0 {
		fmt.Fprintln(stderr, "version does not accept arguments")
		return 1
	}
	fmt.Fprintln(stdout, Version)
	return 0
}

func printVersionUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  gh dep-risk version")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Print the gh-dep-risk build version.")
}
