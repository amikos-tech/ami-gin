package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	gin "github.com/amikos-tech/ami-gin"
)

const defaultLocalArtifactMode os.FileMode = 0o600

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "build":
		cmdBuild(args)
	case "query":
		cmdQuery(args)
	case "info":
		cmdInfo(args)
	case "extract":
		cmdExtract(args)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`gin-index: CLI tool for GIN index operations on Parquet files

Usage:
  gin-index <command> [options]

Commands:
  build     Build GIN index from Parquet file(s)
  query     Query index with a predicate
  info      Show information about index(es)
  extract   Extract embedded index to sidecar file

Single File Examples:
  gin-index build -c attributes data.parquet
  gin-index build -c attributes -embed data.parquet
  gin-index query data.parquet.gin '$.status = "error"'
  gin-index info data.parquet.gin
  gin-index extract -o data.parquet.gin data.parquet

Batch Processing (Directory/S3 Prefix):
  # Build index for all .parquet files in directory
  gin-index build -c attributes ./data/
  gin-index build -c attributes -embed ./data/

  # Query all .gin files in directory
  gin-index query ./data/ '$.status = "error"'

  # Show info for all .gin files in directory
  gin-index info ./data/

  # S3 prefix (processes all .parquet files under prefix)
  gin-index build -c attributes s3://bucket/data/
  gin-index query s3://bucket/data/ '$.status = "error"'
  gin-index info s3://bucket/data/`)
}

func cmdBuild(args []string) {
	if code := runBuild(args, os.Stdout, os.Stderr); code != 0 {
		os.Exit(code)
	}
}

func runBuild(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	fs.SetOutput(stderr)
	column := fs.String("c", "", "JSON column name (required)")
	output := fs.String("o", "", "Output path (for single file only)")
	embed := fs.Bool("embed", false, "Embed index in Parquet file instead of sidecar")
	key := fs.String("key", gin.DefaultMetadataKey, "Metadata key for embedded index")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, "Error: input file or directory required")
		fs.Usage()
		return 1
	}
	if *column == "" {
		fmt.Fprintln(stderr, "Error: -c (column) is required")
		fs.Usage()
		return 1
	}

	input := fs.Arg(0)
	ginCfg := gin.DefaultConfig()
	pqCfg := gin.ParquetConfig{MetadataKey: *key}

	files, err := resolveParquetFiles(input)
	if err != nil {
		fmt.Fprintf(stderr, "Error: Failed to resolve files: %v\n", err)
		return 1
	}

	if len(files) == 0 {
		fmt.Fprintf(stderr, "Error: No .parquet files found in %s\n", input)
		return 1
	}

	if len(files) > 1 && *output != "" {
		fmt.Fprintln(stderr, "Error: -o cannot be used with multiple files (directory/prefix mode)")
		return 1
	}

	successes := 0
	for _, file := range files {
		fmt.Fprintf(stdout, "Processing: %s\n", file)
		if err := buildSingleFileWithIO(stdout, stderr, file, *column, *output, *embed, ginCfg, pqCfg); err != nil {
			continue
		}
		successes++
	}

	writeBatchSummary(stdout, successes, len(files))
	if successes != len(files) {
		return 1
	}
	return 0
}

// buildSingleFile/extractSingleFile are os.Stdout/os.Stderr wrappers kept for
// the historical API surface exercised by tests. The *WithIO variants are the
// canonical implementation and are what run* functions call so captured
// writers and exit codes are testable end-to-end.
func buildSingleFile(input, column, output string, embed bool, ginCfg gin.GINConfig, pqCfg gin.ParquetConfig) {
	_ = buildSingleFileWithIO(os.Stdout, os.Stderr, input, column, output, embed, ginCfg, pqCfg)
}

func buildSingleFileWithIO(stdout, stderr io.Writer, input, column, output string, embed bool, ginCfg gin.GINConfig, pqCfg gin.ParquetConfig) error {
	var idx *gin.GINIndex

	if gin.IsS3Path(input) {
		bucket, s3Key, err := gin.ParseS3Path(input)
		if err != nil {
			fmt.Fprintf(stderr, "  Error: Invalid S3 path: %v\n", err)
			return err
		}
		s3Client, err := gin.NewS3ClientFromEnv()
		if err != nil {
			fmt.Fprintf(stderr, "  Error: Failed to create S3 client: %v\n", err)
			return err
		}
		idx, err = s3Client.BuildFromParquet(bucket, s3Key, column, ginCfg)
		if err != nil {
			fmt.Fprintf(stderr, "  Error: Failed to build index: %v\n", err)
			return err
		}

		if embed {
			fmt.Fprintf(stderr, "  Warning: --embed not supported for S3 paths, using sidecar\n")
		}

		outPath := output
		if outPath == "" {
			outPath = "s3://" + bucket + "/" + s3Key + ".gin"
		}
		outBucket, outKey, err := gin.ParseS3Path(outPath)
		if err != nil {
			fmt.Fprintf(stderr, "  Error: Invalid S3 output path: %v\n", err)
			return err
		}
		if err := s3Client.WriteSidecar(outBucket, strings.TrimSuffix(outKey, ".gin"), idx); err != nil {
			fmt.Fprintf(stderr, "  Error: Failed to write sidecar: %v\n", err)
			return err
		}
		fmt.Fprintf(stdout, "  Index written to %s\n", outPath)
	} else {
		built, err := gin.BuildFromParquet(input, column, ginCfg)
		if err != nil {
			fmt.Fprintf(stderr, "  Error: Failed to build index: %v\n", err)
			return err
		}
		idx = built

		if embed {
			if err := gin.RebuildWithIndex(input, idx, pqCfg); err != nil {
				fmt.Fprintf(stderr, "  Error: Failed to embed index: %v\n", err)
				return err
			}
			fmt.Fprintf(stdout, "  Index embedded in %s\n", input)
		} else {
			outPath := output
			if outPath == "" {
				outPath = input + ".gin"
			}
			fileMode, err := localOutputMode(input)
			if err != nil {
				fmt.Fprintf(stderr, "  Error: Failed to determine source file permissions: %v\n", err)
				return err
			}
			data, err := gin.Encode(idx)
			if err != nil {
				fmt.Fprintf(stderr, "  Error: Failed to encode index: %v\n", err)
				return err
			}
			if err := writeLocalIndexFile(outPath, data, fileMode); err != nil {
				fmt.Fprintf(stderr, "  Error: Failed to write index: %v\n", err)
				return err
			}
			fmt.Fprintf(stdout, "  Index written to %s\n", outPath)
		}
	}

	fmt.Fprintf(stdout, "  Stats: %d row groups, %d paths, %d docs\n",
		idx.Header.NumRowGroups, idx.Header.NumPaths, idx.Header.NumDocs)
	return nil
}

func cmdQuery(args []string) {
	if code := runQuery(args, os.Stdout, os.Stderr); code != 0 {
		os.Exit(code)
	}
}

func runQuery(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	fs.SetOutput(stderr)
	key := fs.String("key", gin.DefaultMetadataKey, "Metadata key for embedded index")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 2 {
		fmt.Fprintln(stderr, "Error: index path and query required")
		fmt.Fprintln(stderr, "Usage: gin-index query <index-path> '<predicate>'")
		return 1
	}

	indexPath := fs.Arg(0)
	queryStr := fs.Arg(1)
	pqCfg := gin.ParquetConfig{MetadataKey: *key}

	pred, err := parsePredicate(queryStr)
	if err != nil {
		fmt.Fprintf(stderr, "Error: Failed to parse predicate: %v\n", err)
		return 1
	}

	files, err := resolveIndexFiles(indexPath)
	if err != nil {
		fmt.Fprintf(stderr, "Error: Failed to resolve files: %v\n", err)
		return 1
	}

	if len(files) == 0 {
		fmt.Fprintf(stderr, "Error: No .gin files found in %s\n", indexPath)
		return 1
	}

	exitCode := 0
	for _, file := range files {
		if len(files) > 1 {
			fmt.Fprintf(stdout, "=== %s ===\n", file)
		}
		if err := querySingleFileWithIO(stdout, stderr, file, pred, pqCfg); err != nil {
			exitCode = 1
		}
		if len(files) > 1 {
			fmt.Fprintln(stdout)
		}
	}
	return exitCode
}

func querySingleFileWithIO(stdout, stderr io.Writer, indexPath string, pred gin.Predicate, pqCfg gin.ParquetConfig) error {
	var idx *gin.GINIndex

	if gin.IsS3Path(indexPath) {
		bucket, s3Key, err := gin.ParseS3Path(indexPath)
		if err != nil {
			fmt.Fprintf(stderr, "Error: Invalid S3 path: %v\n", err)
			return err
		}
		s3Client, err := gin.NewS3ClientFromEnv()
		if err != nil {
			fmt.Fprintf(stderr, "Error: Failed to create S3 client: %v\n", err)
			return err
		}
		if strings.HasSuffix(s3Key, ".gin") {
			idx, err = s3Client.ReadSidecar(bucket, strings.TrimSuffix(s3Key, ".gin"))
		} else {
			idx, err = s3Client.LoadIndex(bucket, s3Key, pqCfg)
		}
		if err != nil {
			fmt.Fprintf(stderr, "Error: Failed to load index: %v\n", err)
			return err
		}
	} else {
		if strings.HasSuffix(indexPath, ".gin") {
			data, err := readLocalIndexFile(indexPath)
			if err != nil {
				fmt.Fprintf(stderr, "Error: Failed to read index: %v\n", err)
				return err
			}
			idx, err = gin.Decode(data)
			if err != nil {
				fmt.Fprintf(stderr, "Error: Failed to decode index: %v\n", err)
				return err
			}
		} else {
			loaded, err := gin.LoadIndex(indexPath, pqCfg)
			if err != nil {
				fmt.Fprintf(stderr, "Error: Failed to load index: %v\n", err)
				return err
			}
			idx = loaded
		}
	}

	result := idx.Evaluate([]gin.Predicate{pred})
	rgs := result.ToSlice()

	if len(rgs) == 0 {
		fmt.Fprintln(stdout, "No matching row groups")
	} else {
		fmt.Fprintf(stdout, "Matching row groups (%d/%d): %v\n", len(rgs), idx.Header.NumRowGroups, rgs)
	}
	return nil
}

func cmdInfo(args []string) {
	if code := runInfo(args, os.Stdout, os.Stderr); code != 0 {
		os.Exit(code)
	}
}

func runInfo(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	fs.SetOutput(stderr)
	key := fs.String("key", gin.DefaultMetadataKey, "Metadata key for embedded index")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, "Error: index path required")
		return 1
	}

	indexPath := fs.Arg(0)
	pqCfg := gin.ParquetConfig{MetadataKey: *key}

	files, err := resolveIndexFiles(indexPath)
	if err != nil {
		fmt.Fprintf(stderr, "Error: Failed to resolve files: %v\n", err)
		return 1
	}

	if len(files) == 0 {
		fmt.Fprintf(stderr, "Error: No .gin files found in %s\n", indexPath)
		return 1
	}

	exitCode := 0
	for _, file := range files {
		if len(files) > 1 {
			fmt.Fprintf(stdout, "=== %s ===\n", file)
		}
		if err := infoSingleFile(stdout, stderr, file, pqCfg); err != nil {
			exitCode = 1
		}
		if len(files) > 1 {
			fmt.Fprintln(stdout)
		}
	}
	return exitCode
}

func infoSingleFile(stdout, stderr io.Writer, indexPath string, pqCfg gin.ParquetConfig) error {
	var idx *gin.GINIndex

	if gin.IsS3Path(indexPath) {
		bucket, s3Key, err := gin.ParseS3Path(indexPath)
		if err != nil {
			fmt.Fprintf(stderr, "Error: Invalid S3 path: %v\n", err)
			return err
		}
		s3Client, err := gin.NewS3ClientFromEnv()
		if err != nil {
			fmt.Fprintf(stderr, "Error: Failed to create S3 client: %v\n", err)
			return err
		}
		if strings.HasSuffix(s3Key, ".gin") {
			idx, err = s3Client.ReadSidecar(bucket, strings.TrimSuffix(s3Key, ".gin"))
		} else {
			idx, err = s3Client.LoadIndex(bucket, s3Key, pqCfg)
		}
		if err != nil {
			fmt.Fprintf(stderr, "Error: Failed to load index: %v\n", err)
			return err
		}
	} else {
		if strings.HasSuffix(indexPath, ".gin") {
			data, err := readLocalIndexFile(indexPath)
			if err != nil {
				fmt.Fprintf(stderr, "Error: Failed to read index: %v\n", err)
				return err
			}
			idx, err = gin.Decode(data)
			if err != nil {
				fmt.Fprintf(stderr, "Error: Failed to decode index: %v\n", err)
				return err
			}
		} else {
			loaded, err := gin.LoadIndex(indexPath, pqCfg)
			if err != nil {
				fmt.Fprintf(stderr, "Error: Failed to load index: %v\n", err)
				return err
			}
			idx = loaded
		}
	}

	writeIndexInfo(stdout, idx)
	return nil
}

func writeIndexInfo(w io.Writer, idx *gin.GINIndex) {
	fmt.Fprintf(w, "GIN Index Info:\n")
	fmt.Fprintf(w, "  Version: %d\n", idx.Header.Version)
	fmt.Fprintf(w, "  Row Groups: %d\n", idx.Header.NumRowGroups)
	fmt.Fprintf(w, "  Documents: %d\n", idx.Header.NumDocs)
	fmt.Fprintf(w, "  Paths: %d\n", idx.Header.NumPaths)
	fmt.Fprintf(w, "  Cardinality Threshold: %d\n", idx.Header.CardinalityThresh)
	fmt.Fprintf(w, "\nPaths:\n")
	for _, pe := range idx.PathDirectory {
		if strings.HasPrefix(pe.PathName, "__derived:") {
			continue
		}
		fmt.Fprintln(w, formatPathInfo(idx, pe))
	}
}

func formatPathInfo(idx *gin.GINIndex, pe gin.PathEntry) string {
	info := fmt.Sprintf("  %s (id=%d, types=%s, cardinality=%d, mode=%s",
		pe.PathName, pe.PathID, describeTypes(pe.ObservedTypes), pe.Cardinality, pe.Mode.String())
	if pe.Mode == gin.PathModeAdaptiveHybrid {
		promotedTerms, bucketCount := adaptivePathSummary(idx, pe)
		info += fmt.Sprintf(", promoted=%d, buckets=%d, threshold=%d",
			promotedTerms, bucketCount, idx.Header.CardinalityThresh)
		if idx.Config != nil {
			info += fmt.Sprintf(", cap=%d", idx.Config.AdaptivePromotedTermCap)
		}
	}
	if representations := idx.Representations(pe.PathName); len(representations) > 0 {
		rendered := make([]string, 0, len(representations))
		for _, representation := range representations {
			rendered = append(rendered, representation.Alias+":"+representation.Transformer)
		}
		info += fmt.Sprintf(", representations=%s", strings.Join(rendered, ","))
	}
	info += ")"
	return info
}

// adaptivePathSummary returns the promoted-term and bucket counts for an
// adaptive-hybrid path. Both the builder and Decode enforce that every
// PathModeAdaptiveHybrid entry has a matching AdaptiveStringIndexes section,
// so a missing section indicates an invariant violation; we surface zeros so
// the CLI renders something coherent rather than panicking.
func adaptivePathSummary(idx *gin.GINIndex, pe gin.PathEntry) (int, int) {
	adaptive, ok := idx.AdaptiveStringIndexes[pe.PathID]
	if !ok || adaptive == nil {
		return 0, 0
	}
	return len(adaptive.Terms), len(adaptive.BucketRGBitmaps)
}

func cmdExtract(args []string) {
	if code := runExtract(args, os.Stdout, os.Stderr); code != 0 {
		os.Exit(code)
	}
}

func runExtract(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	fs.SetOutput(stderr)
	output := fs.String("o", "", "Output path (required for single file)")
	key := fs.String("key", gin.DefaultMetadataKey, "Metadata key for embedded index")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if fs.NArg() < 1 {
		fmt.Fprintln(stderr, "Error: parquet file or directory required")
		return 1
	}

	input := fs.Arg(0)
	pqCfg := gin.ParquetConfig{MetadataKey: *key}

	files, err := resolveParquetFiles(input)
	if err != nil {
		fmt.Fprintf(stderr, "Error: Failed to resolve files: %v\n", err)
		return 1
	}

	if len(files) == 0 {
		fmt.Fprintf(stderr, "Error: No .parquet files found in %s\n", input)
		return 1
	}

	if len(files) == 1 && *output == "" {
		fmt.Fprintln(stderr, "Error: -o (output) is required for single file")
		return 1
	}

	if len(files) > 1 && *output != "" {
		fmt.Fprintln(stderr, "Error: -o cannot be used with multiple files (directory/prefix mode)")
		return 1
	}

	successes := 0
	for _, file := range files {
		fmt.Fprintf(stdout, "Processing: %s\n", file)
		outPath := *output
		if outPath == "" {
			outPath = file + ".gin"
		}
		if err := extractSingleFileWithIO(stdout, stderr, file, outPath, pqCfg); err != nil {
			continue
		}
		successes++
	}

	writeBatchSummary(stdout, successes, len(files))
	if successes != len(files) {
		return 1
	}
	return 0
}

func extractSingleFile(parquetPath, output string, pqCfg gin.ParquetConfig) {
	_ = extractSingleFileWithIO(os.Stdout, os.Stderr, parquetPath, output, pqCfg)
}

func extractSingleFileWithIO(stdout, stderr io.Writer, parquetPath, output string, pqCfg gin.ParquetConfig) error {
	var idx *gin.GINIndex

	if gin.IsS3Path(parquetPath) {
		bucket, s3Key, err := gin.ParseS3Path(parquetPath)
		if err != nil {
			fmt.Fprintf(stderr, "  Error: Invalid S3 path: %v\n", err)
			return err
		}
		s3Client, err := gin.NewS3ClientFromEnv()
		if err != nil {
			fmt.Fprintf(stderr, "  Error: Failed to create S3 client: %v\n", err)
			return err
		}
		idx, err = s3Client.ReadFromParquetMetadata(bucket, s3Key, pqCfg)
		if err != nil {
			fmt.Fprintf(stderr, "  Error: Failed to read embedded index: %v\n", err)
			return err
		}
	} else {
		loaded, err := gin.ReadFromParquetMetadata(parquetPath, pqCfg)
		if err != nil {
			fmt.Fprintf(stderr, "  Error: Failed to read embedded index: %v\n", err)
			return err
		}
		idx = loaded
	}

	data, err := gin.Encode(idx)
	if err != nil {
		fmt.Fprintf(stderr, "  Error: Failed to encode index: %v\n", err)
		return err
	}
	if gin.IsS3Path(output) {
		bucket, s3Key, err := gin.ParseS3Path(output)
		if err != nil {
			fmt.Fprintf(stderr, "  Error: Invalid S3 output path: %v\n", err)
			return err
		}
		s3Client, err := gin.NewS3ClientFromEnv()
		if err != nil {
			fmt.Fprintf(stderr, "  Error: Failed to create S3 client: %v\n", err)
			return err
		}
		if err := s3Client.WriteFile(bucket, s3Key, data); err != nil {
			fmt.Fprintf(stderr, "  Error: Failed to write index: %v\n", err)
			return err
		}
	} else {
		fileMode, err := localOutputMode(parquetPath)
		if err != nil {
			fmt.Fprintf(stderr, "  Error: Failed to determine source file permissions: %v\n", err)
			return err
		}
		if err := writeLocalIndexFile(output, data, fileMode); err != nil {
			fmt.Fprintf(stderr, "  Error: Failed to write index: %v\n", err)
			return err
		}
	}

	fmt.Fprintf(stdout, "  Index extracted to %s\n", output)
	return nil
}

func readLocalIndexFile(path string) ([]byte, error) {
	cleanedPath := filepath.Clean(path)
	// #nosec G304 -- the CLI intentionally reads the user-selected local index path after cleaning it.
	data, err := os.ReadFile(cleanedPath)
	if err != nil {
		return nil, errors.Wrap(err, "read local index file")
	}
	return data, nil
}

// Keep in sync with parquet.go:artifactFileMode.
func artifactFileMode(mode os.FileMode) os.FileMode {
	return mode.Perm() & 0o666
}

func localFileMode(path string) (os.FileMode, error) {
	cleanedPath := filepath.Clean(path)
	info, err := os.Stat(cleanedPath)
	if err != nil {
		return 0, errors.Wrap(err, "stat local file")
	}

	return artifactFileMode(info.Mode()), nil
}

func localOutputMode(sourcePath string) (os.FileMode, error) {
	if gin.IsS3Path(sourcePath) {
		return defaultLocalArtifactMode, nil
	}

	return localFileMode(sourcePath)
}

func writeLocalIndexFile(path string, data []byte, mode os.FileMode) error {
	cleanedPath := filepath.Clean(path)
	if err := os.WriteFile(cleanedPath, data, mode); err != nil {
		return errors.Wrap(err, "write local index file")
	}
	if err := os.Chmod(cleanedPath, mode); err != nil {
		return errors.Wrap(err, "chmod local index file")
	}
	return nil
}

func resolveParquetFiles(path string) ([]string, error) {
	if gin.IsS3Path(path) {
		bucket, prefix, err := gin.ParseS3Path(path)
		if err != nil {
			return nil, err
		}
		s3Client, err := gin.NewS3ClientFromEnv()
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(path, ".parquet") {
			return []string{path}, nil
		}
		keys, err := s3Client.ListParquetFiles(bucket, prefix)
		if err != nil {
			return nil, err
		}
		var files []string
		for _, k := range keys {
			files = append(files, "s3://"+bucket+"/"+k)
		}
		return files, nil
	}

	if strings.HasSuffix(path, ".parquet") {
		return []string{path}, nil
	}

	if gin.IsDirectory(path) {
		return gin.ListParquetFiles(path)
	}

	matches, err := filepath.Glob(path)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, m := range matches {
		if strings.HasSuffix(m, ".parquet") {
			files = append(files, m)
		}
	}
	return files, nil
}

func resolveIndexFiles(path string) ([]string, error) {
	if gin.IsS3Path(path) {
		bucket, prefix, err := gin.ParseS3Path(path)
		if err != nil {
			return nil, err
		}
		s3Client, err := gin.NewS3ClientFromEnv()
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(path, ".gin") {
			return []string{path}, nil
		}
		keys, err := s3Client.ListGINFiles(bucket, prefix)
		if err != nil {
			return nil, err
		}
		var files []string
		for _, k := range keys {
			files = append(files, "s3://"+bucket+"/"+k)
		}
		return files, nil
	}

	if strings.HasSuffix(path, ".gin") {
		return []string{path}, nil
	}

	if gin.IsDirectory(path) {
		return gin.ListGINFiles(path)
	}

	matches, err := filepath.Glob(path)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, m := range matches {
		if strings.HasSuffix(m, ".gin") {
			files = append(files, m)
		}
	}
	return files, nil
}

func writeBatchSummary(stdout io.Writer, successes, total int) {
	failures := total - successes
	if failures == 0 {
		fmt.Fprintf(stdout, "\nProcessed %d file(s)\n", total)
		return
	}
	fmt.Fprintf(stdout, "\nProcessed %d/%d file(s) (%d failed)\n", successes, total, failures)
}

func parsePredicate(s string) (gin.Predicate, error) {
	s = strings.TrimSpace(s)
	upper := strings.ToUpper(s)

	if strings.HasSuffix(upper, " IS NULL") {
		path, err := normalizePredicatePath(strings.TrimSpace(s[:len(s)-len(" IS NULL")]))
		if err != nil {
			return gin.Predicate{}, err
		}
		return gin.IsNull(path), nil
	}

	if strings.HasSuffix(upper, " IS NOT NULL") {
		path, err := normalizePredicatePath(strings.TrimSpace(s[:len(s)-len(" IS NOT NULL")]))
		if err != nil {
			return gin.Predicate{}, err
		}
		return gin.IsNotNull(path), nil
	}

	patterns := []struct {
		regex *regexp.Regexp
		op    gin.Operator
	}{
		{regexp.MustCompile(`(?i)^(.+?)\s+NOT\s+IN\s+\((.+)\)$`), gin.OpNIN},
		{regexp.MustCompile(`(?i)^(.+?)\s+IN\s+\((.+)\)$`), gin.OpIN},
		{regexp.MustCompile(`(?i)^(.+?)\s+CONTAINS\s+(.+)$`), gin.OpContains},
		{regexp.MustCompile(`(?i)^(.+?)\s+REGEX\s+(.+)$`), gin.OpRegex},
		{regexp.MustCompile(`^(.+?)\s*!=\s*(.+)$`), gin.OpNE},
		{regexp.MustCompile(`^(.+?)\s*>=\s*(.+)$`), gin.OpGTE},
		{regexp.MustCompile(`^(.+?)\s*<=\s*(.+)$`), gin.OpLTE},
		{regexp.MustCompile(`^(.+?)\s*>\s*(.+)$`), gin.OpGT},
		{regexp.MustCompile(`^(.+?)\s*<\s*(.+)$`), gin.OpLT},
		{regexp.MustCompile(`^(.+?)\s*=\s*(.+)$`), gin.OpEQ},
	}

	for _, p := range patterns {
		if matches := p.regex.FindStringSubmatch(s); matches != nil {
			path, err := normalizePredicatePath(strings.TrimSpace(matches[1]))
			if err != nil {
				return gin.Predicate{}, err
			}
			valueStr := strings.TrimSpace(matches[2])

			if p.op == gin.OpIN || p.op == gin.OpNIN {
				return gin.Predicate{Path: path, Operator: p.op, Value: parseValueList(valueStr)}, nil
			}

			return gin.Predicate{Path: path, Operator: p.op, Value: parseValue(valueStr)}, nil
		}
	}

	return gin.Predicate{}, errors.Errorf("cannot parse predicate: %s", s)
}

func normalizePredicatePath(path string) (string, error) {
	if err := gin.ValidateJSONPath(path); err != nil {
		return "", errors.Wrap(err, "invalid JSONPath")
	}
	return gin.NormalizePath(path), nil
}

func parseValue(s string) any {
	s = strings.TrimSpace(s)

	if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) ||
		(strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) {
		return s[1 : len(s)-1]
	}

	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	if s == "null" {
		return nil
	}

	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return float64(i)
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}

	return s
}

func parseValueList(s string) []any {
	var values []any
	err := json.Unmarshal([]byte("["+s+"]"), &values)
	if err != nil {
		parts := strings.Split(s, ",")
		for _, p := range parts {
			values = append(values, parseValue(strings.TrimSpace(p)))
		}
	}
	return values
}

func describeTypes(types uint8) string {
	var parts []string
	if types&gin.TypeString != 0 {
		parts = append(parts, "string")
	}
	if types&gin.TypeInt != 0 {
		parts = append(parts, "int")
	}
	if types&gin.TypeFloat != 0 {
		parts = append(parts, "float")
	}
	if types&gin.TypeBool != 0 {
		parts = append(parts, "bool")
	}
	if types&gin.TypeNull != 0 {
		parts = append(parts, "null")
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ",")
}
