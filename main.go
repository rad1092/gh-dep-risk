package main

import (
	"os"

	"gh-dep-risk/cmd"
)

func main() {
	os.Exit(cmd.Execute(os.Args[1:]))
}
