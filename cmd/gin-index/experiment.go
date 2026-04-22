package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"

	"github.com/pkg/errors"

	gin "github.com/amikos-tech/ami-gin"
	"github.com/amikos-tech/ami-gin/logging/slogadapter"
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
	testPredicate := fs.String("test", "", "Evaluate a predicate against the built index")
	outputPath := fs.String("o", "", "Write readable sidecar output to path")
	logLevel := fs.String("log-level", "off", "Log level: off|info|debug")
	sampleLimit := fs.Int("sample", 0, "Cap successful ingests at N documents")
	onError := fs.String("on-error", "abort", "Malformed-line handling: abort|continue")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *rgSize <= 0 {
		fmt.Fprintln(stderr, "Error: --rg-size must be greater than 0")
		fmt.Fprintln(stderr, "Usage: gin-index experiment [--rg-size N] [--sample N] [--on-error abort|continue] [--json] [--test '<predicate>'] [-o out.gin] [--log-level off|info|debug] <input-path|->")
		return 1
	}
	if *sampleLimit < 0 {
		fmt.Fprintln(stderr, "Error: --sample must be greater than or equal to 0")
		return 1
	}
	if *onError != "abort" && *onError != "continue" {
		fmt.Fprintf(stderr, "Error: invalid --on-error %q: want abort or continue\n", *onError)
		return 1
	}

	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "Error: exactly one input path is required")
		fmt.Fprintln(stderr, "Usage: gin-index experiment [--rg-size N] [--sample N] [--on-error abort|continue] [--json] [--test '<predicate>'] [-o out.gin] [--log-level off|info|debug] <input-path|->")
		return 1
	}

	inputArg := fs.Arg(0)
	config, err := experimentConfigForLogLevel(*logLevel, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	source, err := prepareExperimentSource(inputArg, stdin, *sampleLimit, config, *onError, stderr)
	if err != nil {
		if errors.Is(err, errExperimentAbort) {
			return 1
		}
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	defer func() {
		if cleanupErr := source.cleanup(); cleanupErr != nil {
			fmt.Fprintf(stderr, "Warning: %v\n", cleanupErr)
		}
	}()

	estimatedRGs := 1
	switch {
	case *sampleLimit > 0:
		estimatedRGs = max(1, ceilDiv(*sampleLimit, *rgSize))
	case source.candidateRecords > 0:
		estimatedRGs = ceilDiv(source.candidateRecords, *rgSize)
	}

	result, err := buildExperimentIndex(source.open, config, *rgSize, estimatedRGs, *sampleLimit, *onError, stderr)
	if err != nil {
		if errors.Is(err, errExperimentAbort) {
			return 1
		}
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	report := experimentReport{
		Source: experimentSource{
			Input: source.displayName,
			Stdin: source.isStdin(),
		},
		Summary: experimentSummary{
			Documents:      result.ingestedDocs,
			RowGroups:      result.rowGroups,
			RGSize:         *rgSize,
			SampleLimit:    *sampleLimit,
			ProcessedLines: result.processedLines,
			SkippedLines:   result.skippedLines,
			ErrorCount:     result.errorCount,
			Status:         experimentSummaryStatus(source, result),
			SidecarPath:    "",
		},
		Paths: collectExperimentPathRows(result.idx),
	}

	if *outputPath != "" {
		if err := writeExperimentSidecar(*outputPath, source, result.idx); err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
		report.Summary.SidecarPath = *outputPath
	}

	if *testPredicate != "" {
		pred, err := parsePredicate(*testPredicate)
		if err != nil {
			fmt.Fprintf(stderr, "Error: failed to parse predicate: %v\n", err)
			return 1
		}

		evalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()

		report.PredicateTest, err = evaluateExperimentPredicate(evalCtx, report.Summary.RowGroups, result.idx, *testPredicate, pred)
		if err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
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
	candidateRecords int
	openFn           func() (io.ReadCloser, error)
	cleanupFn        func() error
}

type experimentBuildResult struct {
	idx            *gin.GINIndex
	processedLines int
	ingestedDocs   int
	rowGroups      int
	skippedLines   int
	errorCount     int
	sampleCapped   bool
}

var errExperimentAbort = errors.New("experiment ingest aborted")

func newExperimentInputSource(displayName, inputPath string, candidateRecords int, openFn func() (io.ReadCloser, error), cleanupFn func() error) experimentInputSource {
	return experimentInputSource{
		inputPath:        inputPath,
		displayName:      displayName,
		candidateRecords: candidateRecords,
		openFn:           openFn,
		cleanupFn:        cleanupFn,
	}
}

func (s experimentInputSource) open() (io.ReadCloser, error) {
	if s.openFn == nil {
		return nil, errors.New("experiment input source is not initialized")
	}
	return s.openFn()
}

func (s experimentInputSource) cleanup() error {
	if s.cleanupFn == nil {
		return nil
	}
	return s.cleanupFn()
}

func (s experimentInputSource) isStdin() bool {
	return s.inputPath == ""
}

func prepareExperimentSource(inputArg string, stdin io.Reader, sampleLimit int, config gin.GINConfig, onError string, stderr io.Writer) (experimentInputSource, error) {
	if inputArg == "-" && sampleLimit > 0 {
		return newExperimentInputSource("-", "", 0, func() (io.ReadCloser, error) {
			return io.NopCloser(stdin), nil
		}, nil), nil
	}
	if inputArg == "-" {
		return prepareExperimentStdin(stdin, config, onError, stderr)
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

	return newExperimentInputSource(path, path, count, func() (io.ReadCloser, error) {
		f, err := os.Open(path)
		if err != nil {
			return nil, errors.Wrap(err, "open input")
		}
		return f, nil
	}, nil), nil
}

func prepareExperimentStdin(stdin io.Reader, config gin.GINConfig, onError string, stderr io.Writer) (experimentInputSource, error) {
	// Stdin may need a validation pass and then an ingest pass, so spool it to a
	// temp file when the original stream cannot be rewound.
	tmpFile, err := os.CreateTemp("", "gin-index-experiment-*.jsonl")
	if err != nil {
		return experimentInputSource{}, errors.Wrap(err, "create temp file for stdin")
	}

	var validator experimentRecordValidator
	if onError == "abort" {
		validator, err = newExperimentAbortValidator(config)
		if err != nil {
			_ = tmpFile.Close()
			if removeErr := removeExperimentTempFile(tmpFile.Name()); removeErr != nil {
				return experimentInputSource{}, errors.Wrapf(err, "cleanup failed: %v", removeErr)
			}
			return experimentInputSource{}, err
		}
	}

	count, countErr := countExperimentRecords(stdin, tmpFile, validator, stderr)
	closeErr := tmpFile.Close()
	if countErr != nil {
		if removeErr := removeExperimentTempFile(tmpFile.Name()); removeErr != nil {
			return experimentInputSource{}, errors.Wrapf(countErr, "cleanup failed: %v", removeErr)
		}
		return experimentInputSource{}, countErr
	}
	if closeErr != nil {
		if removeErr := removeExperimentTempFile(tmpFile.Name()); removeErr != nil {
			return experimentInputSource{}, errors.Wrapf(closeErr, "cleanup failed: %v", removeErr)
		}
		return experimentInputSource{}, errors.Wrap(closeErr, "close temp file for stdin")
	}

	tempPath := tmpFile.Name()
	return newExperimentInputSource("-", "", count, func() (io.ReadCloser, error) {
		f, err := os.Open(tempPath)
		if err != nil {
			return nil, errors.Wrap(err, "reopen temp file for stdin")
		}
		return f, nil
	}, func() error {
		return removeExperimentTempFile(tempPath)
	}), nil
}

func countExperimentFile(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, errors.Wrap(err, "open input for counting")
	}
	defer f.Close()

	return countExperimentRecords(f, nil, nil, nil)
}

func removeExperimentTempFile(path string) error {
	if err := os.Remove(path); err != nil {
		return errors.Wrap(err, "remove temp file for stdin")
	}
	return nil
}

type experimentRecordValidator func(record []byte) error

func newExperimentAbortValidator(config gin.GINConfig) (experimentRecordValidator, error) {
	builder, err := gin.NewBuilder(config, 1)
	if err != nil {
		return nil, errors.Wrap(err, "create stdin validator")
	}

	return func(record []byte) error {
		return validateExperimentRecord(builder, record, 0)
	}, nil
}

func countExperimentRecords(r io.Reader, spool io.Writer, validator experimentRecordValidator, stderr io.Writer) (int, error) {
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
			if validator != nil {
				record := trimExperimentLineEnding(line)
				if validateErr := validator(record); validateErr != nil {
					if stderr != nil {
						fmt.Fprintf(stderr, "line %d: %v\n", count, validateErr)
						return 0, errExperimentAbort
					}
					return 0, validateErr
				}
			}
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

func buildExperimentIndex(open func() (io.ReadCloser, error), config gin.GINConfig, rgSize, estimatedRGs, sampleLimit int, onError string, stderr io.Writer) (experimentBuildResult, error) {
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
		// Successful documents are packed densely into synthetic row groups, so
		// rgID is derived from accepted documents rather than source line numbers.
		lineErr := validateExperimentRecord(builder, record, result.ingestedDocs/rgSize)
		if lineErr != nil {
			fmt.Fprintf(stderr, "line %d: %v\n", lineNumber, lineErr)
			if onError == "abort" {
				return experimentBuildResult{}, errExperimentAbort
			}
			result.skippedLines++
			result.errorCount++
			if readErr == io.EOF {
				break
			}
			continue
		}

		result.ingestedDocs++
		if sampleLimit > 0 && result.ingestedDocs >= sampleLimit {
			result.sampleCapped = true
			break
		}

		if readErr == io.EOF {
			break
		}
	}

	result.idx = builder.Finalize()
	result.rowGroups = experimentUsedRowGroups(result.ingestedDocs, rgSize)
	trimExperimentIndexRowGroups(result.idx, result.rowGroups)
	return result, nil
}

func experimentUsedRowGroups(ingestedDocs, rgSize int) int {
	if ingestedDocs == 0 {
		return 0
	}
	return ceilDiv(ingestedDocs, rgSize)
}

func experimentSummaryStatus(source experimentInputSource, result experimentBuildResult) string {
	if result.sampleCapped && (source.candidateRecords == 0 || result.processedLines < source.candidateRecords) {
		return "truncated"
	}
	if result.skippedLines > 0 || result.errorCount > 0 {
		return "partial"
	}
	return "complete"
}

func evaluateExperimentPredicate(ctx context.Context, rowGroups int, idx *gin.GINIndex, predicateText string, pred gin.Predicate) (*experimentPredicateResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "predicate evaluation canceled")
	}

	matched := 0
	if rowGroups > 0 {
		matched = len(idx.EvaluateContext(ctx, []gin.Predicate{pred}).ToSlice())
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "predicate evaluation canceled")
	}

	pruned := 0
	if matched < rowGroups {
		pruned = rowGroups - matched
	}
	pruningRatio := 0.0
	if rowGroups > 0 {
		pruningRatio = float64(pruned) / float64(rowGroups)
	}

	return &experimentPredicateResult{
		Predicate:    predicateText,
		Matched:      matched,
		Pruned:       pruned,
		PruningRatio: pruningRatio,
	}, nil
}

// Builder sizing uses candidate line counts or sample limits as an upper bound,
// so trim unused row-group slots before reporting or writing sidecars.
func trimExperimentIndexRowGroups(idx *gin.GINIndex, rowGroups int) {
	if idx == nil {
		return
	}
	if rowGroups < 0 {
		rowGroups = 0
	}

	current := int(idx.Header.NumRowGroups)
	idx.Header.NumRowGroups = uint32(rowGroups)
	if rowGroups >= current {
		return
	}

	for _, si := range idx.StringIndexes {
		for _, rgSet := range si.RGBitmaps {
			trimExperimentRGSet(rgSet, rowGroups)
		}
	}

	for _, adaptive := range idx.AdaptiveStringIndexes {
		for _, rgSet := range adaptive.RGBitmaps {
			trimExperimentRGSet(rgSet, rowGroups)
		}
		for _, rgSet := range adaptive.BucketRGBitmaps {
			trimExperimentRGSet(rgSet, rowGroups)
		}
	}

	for _, ni := range idx.NumericIndexes {
		if len(ni.RGStats) > rowGroups {
			ni.RGStats = ni.RGStats[:rowGroups]
		}
	}

	for _, ni := range idx.NullIndexes {
		trimExperimentRGSet(ni.NullRGBitmap, rowGroups)
		trimExperimentRGSet(ni.PresentRGBitmap, rowGroups)
	}

	for _, ti := range idx.TrigramIndexes {
		ti.NumRGs = rowGroups
		for _, rgSet := range ti.Trigrams {
			trimExperimentRGSet(rgSet, rowGroups)
		}
	}

	for _, sli := range idx.StringLengthIndexes {
		if len(sli.RGStats) > rowGroups {
			sli.RGStats = sli.RGStats[:rowGroups]
		}
	}
}

func trimExperimentRGSet(rgSet *gin.RGSet, rowGroups int) {
	if rgSet == nil || rgSet.NumRGs <= rowGroups {
		return
	}
	rgSet.Roaring().RemoveRange(uint64(rowGroups), uint64(rgSet.NumRGs))
	rgSet.NumRGs = rowGroups
}

func validateExperimentRecord(builder *gin.GINBuilder, record []byte, rgID int) error {
	if len(record) == 0 {
		return errors.New("blank JSONL line")
	}
	if err := builder.AddDocument(gin.DocID(rgID), record); err != nil {
		return err
	}
	return nil
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

func experimentConfigForLogLevel(level string, stderr io.Writer) (gin.GINConfig, error) {
	config := gin.DefaultConfig()

	switch level {
	case "off":
		return config, nil
	case "info", "debug":
		slogLevel := slog.LevelInfo
		if level == "debug" {
			slogLevel = slog.LevelDebug
		}
		logger := slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: slogLevel}))
		config.Logger = slogadapter.New(logger)
		return config, nil
	default:
		return gin.GINConfig{}, errors.Errorf("invalid --log-level %q: want off, info, or debug", level)
	}
}

func writeExperimentSidecar(outputPath string, source experimentInputSource, idx *gin.GINIndex) error {
	if !strings.HasSuffix(outputPath, ".gin") {
		return errors.Errorf("output path %q must end with .gin", outputPath)
	}

	data, err := gin.Encode(idx)
	if err != nil {
		return errors.Wrap(err, "encode experiment index")
	}

	fileMode := defaultLocalArtifactMode
	if !source.isStdin() {
		fileMode, err = localOutputMode(source.inputPath)
		if err != nil {
			return errors.Wrap(err, "determine source file permissions")
		}
	}

	if err := writeLocalIndexFile(outputPath, data, fileMode); err != nil {
		return errors.Wrap(err, "write experiment sidecar")
	}
	return nil
}

func ceilDiv(value, divisor int) int {
	return (value + divisor - 1) / divisor
}
