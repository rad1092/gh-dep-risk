package cmd

import (
	"fmt"
	"io"
	"os"
)

var Version = "dev"

func Execute(args []string) int {
	return execute(os.Stdout, os.Stderr, args)
}

func execute(stdout, stderr io.Writer, args []string) int {
	if len(args) == 0 {
		printRootUsage(stderr)
		return 1
	}

	switch args[0] {
	case "pr":
		return runPR(stdout, stderr, args[1:])
	case "version":
		return runVersion(stdout, stderr, args[1:])
	case "-h", "--help":
		printRootUsage(stdout)
		return 0
	case "help":
		if len(args) == 1 {
			printRootUsage(stdout)
			return 0
		}
		switch args[1] {
		case "pr":
			printPRUsage(stdout)
			return 0
		case "version":
			printVersionUsage(stdout)
			return 0
		default:
			fmt.Fprintf(stderr, "unknown help topic %q\n\n", args[1])
			printRootUsage(stderr)
			return 1
		}
	default:
		fmt.Fprintf(stderr, "unknown subcommand %q\n\n", args[0])
		printRootUsage(stderr)
		return 1
	}
}

func printRootUsage(w io.Writer) {
	fmt.Fprintln(w, "gh dep-risk")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  gh dep-risk pr [<number>|<url>] [flags]")
	fmt.Fprintln(w, "  gh dep-risk version")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Examples:")
	fmt.Fprintln(w, "  gh dep-risk pr 123")
	fmt.Fprintln(w, "  gh dep-risk pr https://github.com/OWNER/REPO/pull/123")
	fmt.Fprintln(w, "  gh dep-risk pr --format json --fail-level high")
	fmt.Fprintln(w, "  gh dep-risk version")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run `gh dep-risk pr --help` for flags.")
}
