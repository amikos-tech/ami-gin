package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func cmdExperiment(args []string) {
	if code := runExperiment(args, os.Stdin, os.Stdout, os.Stderr); code != 0 {
		os.Exit(code)
	}
}

func runExperiment(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	_ = stdin
	_ = stdout

	fs := flag.NewFlagSet("experiment", flag.ContinueOnError)
	fs.SetOutput(stderr)
	rgSize := fs.Int("rg-size", 1, "Synthetic row-group size")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *rgSize == 0 {
		// Task 2 owns full validation and ingest behavior; Task 1 only establishes the surface.
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "Error: exactly one input path is required")
		fmt.Fprintln(stderr, "Usage: gin-index experiment [--rg-size N] <input-path|->")
		return 1
	}

	return 0
}
