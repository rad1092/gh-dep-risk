package cmd

import (
	"fmt"
	"io"
)

func runVersion(stdout, stderr io.Writer, args []string) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "version does not accept arguments")
		return 1
	}
	fmt.Fprintln(stdout, Version)
	return 0
}
