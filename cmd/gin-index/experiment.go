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
	jsonOutput := fs.Bool("json", false, "Emit experiment output as JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *rgSize <= 0 {
		fmt.Fprintln(stderr, "Error: --rg-size must be greater than 0")
		fmt.Fprintln(stderr, "Usage: gin-index experiment [--rg-size N] [--json] <input-path|->")
		return 1
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "Error: exactly one input path is required")
		fmt.Fprintln(stderr, "Usage: gin-index experiment [--rg-size N] [--json] <input-path|->")
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

	result, err := buildExperimentIndex(source.open, gin.DefaultConfig(), *rgSize, estimatedRGs)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	usedRowGroups := 0
	if result.ingestedDocs > 0 {
		usedRowGroups = ceilDiv(result.ingestedDocs, *rgSize)
	}

	report := experimentReport{
		Source: experimentSource{
			Input: source.displayName,
			Stdin: source.stdin,
		},
		Summary: experimentSummary{
			Documents:      result.ingestedDocs,
			RowGroups:      usedRowGroups,
			RGSize:         *rgSize,
			SampleLimit:    0,
			ProcessedLines: result.processedLines,
			SkippedLines:   0,
			ErrorCount:     0,
			SidecarPath:    "",
		},
		Paths: collectExperimentPathRows(result.idx),
	}

	if *jsonOutput {
		if err := writeExperimentJSON(stdout, report); err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
		return 0
	}

	writeExperimentText(stdout, report, result.idx)
	return 0
}

type experimentInputSource struct {
	inputPath        string
	displayName      string
	stdin            bool
	candidateRecords int
	open             func() (io.ReadCloser, error)
	cleanup          func()
}

type experimentBuildResult struct {
	idx            *gin.GINIndex
	processedLines int
	ingestedDocs   int
}

func prepareExperimentSource(inputArg string, stdin io.Reader) (experimentInputSource, error) {
	if inputArg == "-" {
		return prepareExperimentStdin(stdin)
	}
	return prepareExperimentFile(inputArg)
}

func prepareExperimentFile(path string) (experimentInputSource, error) {
	info, err := os.Stat(path)
	if err != nil {
		return experimentInputSource{}, errors.Wrap(err, "stat input")
	}
	if info.IsDir() {
		return experimentInputSource{}, errors.Errorf("input %q is a directory", path)
	}

	count, err := countExperimentFile(path)
	if err != nil {
		return experimentInputSource{}, err
	}

	return experimentInputSource{
		inputPath:        path,
		displayName:      path,
		stdin:            false,
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

func prepareExperimentStdin(stdin io.Reader) (experimentInputSource, error) {
	tmpFile, err := os.CreateTemp("", "gin-index-experiment-*.jsonl")
	if err != nil {
		return experimentInputSource{}, errors.Wrap(err, "create temp file for stdin")
	}

	count, countErr := countExperimentRecords(stdin, tmpFile)
	closeErr := tmpFile.Close()
	if countErr != nil {
		_ = os.Remove(tmpFile.Name())
		return experimentInputSource{}, countErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpFile.Name())
		return experimentInputSource{}, errors.Wrap(closeErr, "close temp file for stdin")
	}

	tempPath := tmpFile.Name()
	return experimentInputSource{
		inputPath:        "",
		displayName:      "-",
		stdin:            true,
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

func buildExperimentIndex(open func() (io.ReadCloser, error), config gin.GINConfig, rgSize, estimatedRGs int) (experimentBuildResult, error) {
	builder, err := gin.NewBuilder(config, estimatedRGs)
	if err != nil {
		return experimentBuildResult{}, errors.Wrap(err, "create builder")
	}

	r, err := open()
	if err != nil {
		return experimentBuildResult{}, err
	}
	defer r.Close()

	reader := bufio.NewReader(r)
	result := experimentBuildResult{}
	lineNumber := 0

	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) == 0 && readErr == io.EOF {
			break
		}
		if readErr != nil && readErr != io.EOF {
			return experimentBuildResult{}, errors.Wrap(readErr, "read input for ingest")
		}

		lineNumber++
		result.processedLines++

		record := trimExperimentLineEnding(line)
		if len(record) == 0 {
			return experimentBuildResult{}, errors.Errorf("line %d: blank JSONL line", lineNumber)
		}

		rgID := result.ingestedDocs / rgSize
		if err := builder.AddDocument(gin.DocID(rgID), record); err != nil {
			return experimentBuildResult{}, errors.Wrapf(err, "line %d", lineNumber)
		}
		result.ingestedDocs++

		if readErr == io.EOF {
			break
		}
	}

	result.idx = builder.Finalize()
	return result, nil
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

func ceilDiv(value, divisor int) int {
	return (value + divisor - 1) / divisor
}
