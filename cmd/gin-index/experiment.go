package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/pkg/errors"

	gin "github.com/amikos-tech/ami-gin"
)

func cmdExperiment(args []string) {
	if code := runExperiment(args, os.Stdin, os.Stdout, os.Stderr); code != 0 {
		os.Exit(code)
	}
}

func runExperiment(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("experiment", flag.ContinueOnError)
	fs.SetOutput(stderr)
	rgSize := fs.Int("rg-size", 1, "Synthetic row-group size")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *rgSize <= 0 {
		fmt.Fprintln(stderr, "Error: --rg-size must be greater than 0")
		fmt.Fprintln(stderr, "Usage: gin-index experiment [--rg-size N] <input-path|->")
		return 1
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "Error: exactly one input path is required")
		fmt.Fprintln(stderr, "Usage: gin-index experiment [--rg-size N] <input-path|->")
		return 1
	}

	inputArg := fs.Arg(0)
	source, err := prepareExperimentSource(inputArg, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	defer source.cleanup()

	estimatedRGs := 1
	if source.candidateRecords > 0 {
		estimatedRGs = ceilDiv(source.candidateRecords, *rgSize)
	}

	idx, ingestedDocs, err := buildExperimentIndex(source.open, *rgSize, estimatedRGs)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	usedRowGroups := 0
	if ingestedDocs > 0 {
		usedRowGroups = ceilDiv(ingestedDocs, *rgSize)
	}

	writeExperimentSummary(stdout, source.displayName, ingestedDocs, usedRowGroups, *rgSize)
	writeExperimentIndexInfo(stdout, idx, usedRowGroups)
	return 0
}

type experimentSource struct {
	displayName      string
	candidateRecords int
	open             func() (io.ReadCloser, error)
	cleanup          func()
}

func prepareExperimentSource(inputArg string, stdin io.Reader) (experimentSource, error) {
	if inputArg == "-" {
		return prepareExperimentStdin(stdin)
	}
	return prepareExperimentFile(inputArg)
}

func prepareExperimentFile(path string) (experimentSource, error) {
	info, err := os.Stat(path)
	if err != nil {
		return experimentSource{}, errors.Wrap(err, "stat input")
	}
	if info.IsDir() {
		return experimentSource{}, errors.Errorf("input %q is a directory", path)
	}

	count, err := countExperimentFile(path)
	if err != nil {
		return experimentSource{}, err
	}

	return experimentSource{
		displayName:      path,
		candidateRecords: count,
		open: func() (io.ReadCloser, error) {
			f, err := os.Open(path)
			if err != nil {
				return nil, errors.Wrap(err, "open input")
			}
			return f, nil
		},
		cleanup: func() {},
	}, nil
}

func prepareExperimentStdin(stdin io.Reader) (experimentSource, error) {
	tmpFile, err := os.CreateTemp("", "gin-index-experiment-*.jsonl")
	if err != nil {
		return experimentSource{}, errors.Wrap(err, "create temp file for stdin")
	}

	count, countErr := countExperimentRecords(stdin, tmpFile)
	closeErr := tmpFile.Close()
	if countErr != nil {
		_ = os.Remove(tmpFile.Name())
		return experimentSource{}, countErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpFile.Name())
		return experimentSource{}, errors.Wrap(closeErr, "close temp file for stdin")
	}

	tempPath := tmpFile.Name()
	return experimentSource{
		displayName:      "-",
		candidateRecords: count,
		open: func() (io.ReadCloser, error) {
			f, err := os.Open(tempPath)
			if err != nil {
				return nil, errors.Wrap(err, "reopen temp file for stdin")
			}
			return f, nil
		},
		cleanup: func() {
			_ = os.Remove(tempPath)
		},
	}, nil
}

func countExperimentFile(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, errors.Wrap(err, "open input for counting")
	}
	defer f.Close()

	return countExperimentRecords(f, nil)
}

func countExperimentRecords(r io.Reader, spool io.Writer) (int, error) {
	reader := bufio.NewReader(r)
	count := 0

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if spool != nil {
				if _, writeErr := spool.Write(line); writeErr != nil {
					return 0, errors.Wrap(writeErr, "spool stdin to temp file")
				}
			}
			count++
		}

		if err == nil {
			continue
		}
		if err == io.EOF {
			return count, nil
		}
		return 0, errors.Wrap(err, "read input for counting")
	}
}

func buildExperimentIndex(open func() (io.ReadCloser, error), rgSize, estimatedRGs int) (*gin.GINIndex, int, error) {
	builder, err := gin.NewBuilder(gin.DefaultConfig(), estimatedRGs)
	if err != nil {
		return nil, 0, errors.Wrap(err, "create builder")
	}

	r, err := open()
	if err != nil {
		return nil, 0, err
	}
	defer r.Close()

	reader := bufio.NewReader(r)
	ingestedDocs := 0
	lineNumber := 0

	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) == 0 && readErr == io.EOF {
			break
		}
		if readErr != nil && readErr != io.EOF {
			return nil, 0, errors.Wrap(readErr, "read input for ingest")
		}

		lineNumber++
		record := trimExperimentLineEnding(line)
		if len(record) == 0 {
			return nil, 0, errors.Errorf("line %d: blank JSONL line", lineNumber)
		}

		rgID := ingestedDocs / rgSize
		if err := builder.AddDocument(gin.DocID(rgID), record); err != nil {
			return nil, 0, errors.Wrapf(err, "line %d", lineNumber)
		}
		ingestedDocs++

		if readErr == io.EOF {
			break
		}
	}

	return builder.Finalize(), ingestedDocs, nil
}

func trimExperimentLineEnding(line []byte) []byte {
	if n := len(line); n > 0 && line[n-1] == '\n' {
		line = line[:n-1]
	}
	if n := len(line); n > 0 && line[n-1] == '\r' {
		line = line[:n-1]
	}
	return line
}

func writeExperimentSummary(w io.Writer, input string, documents, rowGroups, rgSize int) {
	fmt.Fprintln(w, "Experiment Summary:")
	fmt.Fprintf(w, "  Input: %s\n", input)
	fmt.Fprintf(w, "  Documents: %d\n", documents)
	fmt.Fprintf(w, "  Row Groups: %d\n", rowGroups)
	fmt.Fprintf(w, "  RG Size: %d\n", rgSize)
	fmt.Fprintln(w)
}

func writeExperimentIndexInfo(w io.Writer, idx *gin.GINIndex, usedRowGroups int) {
	idxCopy := *idx
	idxCopy.Header.NumRowGroups = uint32(usedRowGroups)
	writeIndexInfo(w, &idxCopy)
}

func ceilDiv(value, divisor int) int {
	return (value + divisor - 1) / divisor
}
