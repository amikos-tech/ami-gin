package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	gin "github.com/amikos-tech/ami-gin"
)

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
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	column := fs.String("c", "", "JSON column name (required)")
	output := fs.String("o", "", "Output path (for single file only)")
	embed := fs.Bool("embed", false, "Embed index in Parquet file instead of sidecar")
	key := fs.String("key", gin.DefaultMetadataKey, "Metadata key for embedded index")
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: input file or directory required")
		fs.Usage()
		os.Exit(1)
	}
	if *column == "" {
		fmt.Fprintln(os.Stderr, "Error: -c (column) is required")
		fs.Usage()
		os.Exit(1)
	}

	input := fs.Arg(0)
	ginCfg := gin.DefaultConfig()
	pqCfg := gin.ParquetConfig{MetadataKey: *key}

	files, err := resolveParquetFiles(input)
	if err != nil {
		fatal("Failed to resolve files: %v", err)
	}

	if len(files) == 0 {
		fatal("No .parquet files found in %s", input)
	}

	if len(files) > 1 && *output != "" {
		fatal("-o cannot be used with multiple files (directory/prefix mode)")
	}

	for _, file := range files {
		fmt.Printf("Processing: %s\n", file)
		buildSingleFile(file, *column, *output, *embed, ginCfg, pqCfg)
	}

	fmt.Printf("\nProcessed %d file(s)\n", len(files))
}

func buildSingleFile(input, column, output string, embed bool, ginCfg gin.GINConfig, pqCfg gin.ParquetConfig) {
	var idx *gin.GINIndex
	var err error

	if gin.IsS3Path(input) {
		bucket, s3Key, err := gin.ParseS3Path(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error: Invalid S3 path: %v\n", err)
			return
		}
		s3Client, err := gin.NewS3ClientFromEnv()
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error: Failed to create S3 client: %v\n", err)
			return
		}
		idx, err = s3Client.BuildFromParquet(bucket, s3Key, column, ginCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error: Failed to build index: %v\n", err)
			return
		}

		if embed {
			fmt.Fprintf(os.Stderr, "  Warning: --embed not supported for S3 paths, using sidecar\n")
		}

		outPath := output
		if outPath == "" {
			outPath = "s3://" + bucket + "/" + s3Key + ".gin"
		}
		outBucket, outKey, err := gin.ParseS3Path(outPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error: Invalid S3 output path: %v\n", err)
			return
		}
		if err := s3Client.WriteSidecar(outBucket, strings.TrimSuffix(outKey, ".gin"), idx); err != nil {
			fmt.Fprintf(os.Stderr, "  Error: Failed to write sidecar: %v\n", err)
			return
		}
		fmt.Printf("  Index written to %s\n", outPath)
	} else {
		idx, err = gin.BuildFromParquet(input, column, ginCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error: Failed to build index: %v\n", err)
			return
		}

		if embed {
			if err := gin.RebuildWithIndex(input, idx, pqCfg); err != nil {
				fmt.Fprintf(os.Stderr, "  Error: Failed to embed index: %v\n", err)
				return
			}
			fmt.Printf("  Index embedded in %s\n", input)
		} else {
			outPath := output
			if outPath == "" {
				outPath = input + ".gin"
			}
			data, err := gin.Encode(idx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Error: Failed to encode index: %v\n", err)
				return
			}
			if err := writeLocalIndexFile(outPath, data); err != nil {
				fmt.Fprintf(os.Stderr, "  Error: Failed to write index: %v\n", err)
				return
			}
			fmt.Printf("  Index written to %s\n", outPath)
		}
	}

	fmt.Printf("  Stats: %d row groups, %d paths, %d docs\n",
		idx.Header.NumRowGroups, idx.Header.NumPaths, idx.Header.NumDocs)
}

func cmdQuery(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	key := fs.String("key", gin.DefaultMetadataKey, "Metadata key for embedded index")
	_ = fs.Parse(args)

	if fs.NArg() < 2 {
		fmt.Fprintln(os.Stderr, "Error: index path and query required")
		fmt.Fprintln(os.Stderr, "Usage: gin-index query <index-path> '<predicate>'")
		os.Exit(1)
	}

	indexPath := fs.Arg(0)
	queryStr := fs.Arg(1)
	pqCfg := gin.ParquetConfig{MetadataKey: *key}

	pred, err := parsePredicate(queryStr)
	if err != nil {
		fatal("Failed to parse predicate: %v", err)
	}

	files, err := resolveIndexFiles(indexPath)
	if err != nil {
		fatal("Failed to resolve files: %v", err)
	}

	if len(files) == 0 {
		fatal("No .gin files found in %s", indexPath)
	}

	for _, file := range files {
		if len(files) > 1 {
			fmt.Printf("=== %s ===\n", file)
		}
		querySingleFile(file, pred, pqCfg)
		if len(files) > 1 {
			fmt.Println()
		}
	}
}

func querySingleFile(indexPath string, pred gin.Predicate, pqCfg gin.ParquetConfig) {
	var idx *gin.GINIndex
	var err error

	if gin.IsS3Path(indexPath) {
		bucket, s3Key, err := gin.ParseS3Path(indexPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid S3 path: %v\n", err)
			return
		}
		s3Client, err := gin.NewS3ClientFromEnv()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to create S3 client: %v\n", err)
			return
		}
		if strings.HasSuffix(s3Key, ".gin") {
			idx, err = s3Client.ReadSidecar(bucket, strings.TrimSuffix(s3Key, ".gin"))
		} else {
			idx, err = s3Client.LoadIndex(bucket, s3Key, pqCfg)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to load index: %v\n", err)
			return
		}
	} else {
		if strings.HasSuffix(indexPath, ".gin") {
			data, err := readLocalIndexFile(indexPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed to read index: %v\n", err)
				return
			}
			idx, err = gin.Decode(data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed to decode index: %v\n", err)
				return
			}
		} else {
			idx, err = gin.LoadIndex(indexPath, pqCfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed to load index: %v\n", err)
				return
			}
		}
	}

	result := idx.Evaluate([]gin.Predicate{pred})
	rgs := result.ToSlice()

	if len(rgs) == 0 {
		fmt.Println("No matching row groups")
	} else {
		fmt.Printf("Matching row groups (%d/%d): %v\n", len(rgs), idx.Header.NumRowGroups, rgs)
	}
}

func cmdInfo(args []string) {
	fs := flag.NewFlagSet("info", flag.ExitOnError)
	key := fs.String("key", gin.DefaultMetadataKey, "Metadata key for embedded index")
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: index path required")
		os.Exit(1)
	}

	indexPath := fs.Arg(0)
	pqCfg := gin.ParquetConfig{MetadataKey: *key}

	files, err := resolveIndexFiles(indexPath)
	if err != nil {
		fatal("Failed to resolve files: %v", err)
	}

	if len(files) == 0 {
		fatal("No .gin files found in %s", indexPath)
	}

	for _, file := range files {
		if len(files) > 1 {
			fmt.Printf("=== %s ===\n", file)
		}
		infoSingleFile(file, pqCfg)
		if len(files) > 1 {
			fmt.Println()
		}
	}
}

func infoSingleFile(indexPath string, pqCfg gin.ParquetConfig) {
	var idx *gin.GINIndex
	var err error

	if gin.IsS3Path(indexPath) {
		bucket, s3Key, err := gin.ParseS3Path(indexPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Invalid S3 path: %v\n", err)
			return
		}
		s3Client, err := gin.NewS3ClientFromEnv()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to create S3 client: %v\n", err)
			return
		}
		if strings.HasSuffix(s3Key, ".gin") {
			idx, err = s3Client.ReadSidecar(bucket, strings.TrimSuffix(s3Key, ".gin"))
		} else {
			idx, err = s3Client.LoadIndex(bucket, s3Key, pqCfg)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Failed to load index: %v\n", err)
			return
		}
	} else {
		if strings.HasSuffix(indexPath, ".gin") {
			data, err := readLocalIndexFile(indexPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed to read index: %v\n", err)
				return
			}
			idx, err = gin.Decode(data)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed to decode index: %v\n", err)
				return
			}
		} else {
			idx, err = gin.LoadIndex(indexPath, pqCfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: Failed to load index: %v\n", err)
				return
			}
		}
	}

	fmt.Printf("GIN Index Info:\n")
	fmt.Printf("  Version: %d\n", idx.Header.Version)
	fmt.Printf("  Row Groups: %d\n", idx.Header.NumRowGroups)
	fmt.Printf("  Documents: %d\n", idx.Header.NumDocs)
	fmt.Printf("  Paths: %d\n", idx.Header.NumPaths)
	fmt.Printf("  Cardinality Threshold: %d\n", idx.Header.CardinalityThresh)
	fmt.Printf("\nPaths:\n")
	for _, pe := range idx.PathDirectory {
		types := describeTypes(pe.ObservedTypes)
		fmt.Printf("  %s (id=%d, types=%s, cardinality=%d)\n", pe.PathName, pe.PathID, types, pe.Cardinality)
	}
}

func cmdExtract(args []string) {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	output := fs.String("o", "", "Output path (required for single file)")
	key := fs.String("key", gin.DefaultMetadataKey, "Metadata key for embedded index")
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: parquet file or directory required")
		os.Exit(1)
	}

	input := fs.Arg(0)
	pqCfg := gin.ParquetConfig{MetadataKey: *key}

	files, err := resolveParquetFiles(input)
	if err != nil {
		fatal("Failed to resolve files: %v", err)
	}

	if len(files) == 0 {
		fatal("No .parquet files found in %s", input)
	}

	if len(files) == 1 && *output == "" {
		fmt.Fprintln(os.Stderr, "Error: -o (output) is required for single file")
		os.Exit(1)
	}

	if len(files) > 1 && *output != "" {
		fatal("-o cannot be used with multiple files (directory/prefix mode)")
	}

	for _, file := range files {
		fmt.Printf("Processing: %s\n", file)
		outPath := *output
		if outPath == "" {
			outPath = file + ".gin"
		}
		extractSingleFile(file, outPath, pqCfg)
	}

	fmt.Printf("\nProcessed %d file(s)\n", len(files))
}

func extractSingleFile(parquetPath, output string, pqCfg gin.ParquetConfig) {
	var idx *gin.GINIndex
	var err error

	if gin.IsS3Path(parquetPath) {
		bucket, s3Key, err := gin.ParseS3Path(parquetPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error: Invalid S3 path: %v\n", err)
			return
		}
		s3Client, err := gin.NewS3ClientFromEnv()
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error: Failed to create S3 client: %v\n", err)
			return
		}
		idx, err = s3Client.ReadFromParquetMetadata(bucket, s3Key, pqCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error: Failed to read embedded index: %v\n", err)
			return
		}
	} else {
		idx, err = gin.ReadFromParquetMetadata(parquetPath, pqCfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error: Failed to read embedded index: %v\n", err)
			return
		}
	}

	data, err := gin.Encode(idx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Error: Failed to encode index: %v\n", err)
		return
	}

	if gin.IsS3Path(output) {
		bucket, s3Key, err := gin.ParseS3Path(output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error: Invalid S3 output path: %v\n", err)
			return
		}
		s3Client, err := gin.NewS3ClientFromEnv()
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error: Failed to create S3 client: %v\n", err)
			return
		}
		if err := s3Client.WriteFile(bucket, s3Key, data); err != nil {
			fmt.Fprintf(os.Stderr, "  Error: Failed to write index: %v\n", err)
			return
		}
	} else {
		if err := writeLocalIndexFile(output, data); err != nil {
			fmt.Fprintf(os.Stderr, "  Error: Failed to write index: %v\n", err)
			return
		}
	}

	fmt.Printf("  Index extracted to %s\n", output)
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

func writeLocalIndexFile(path string, data []byte) error {
	cleanedPath := filepath.Clean(path)
	if err := os.WriteFile(cleanedPath, data, 0600); err != nil {
		return errors.Wrap(err, "write local index file")
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

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}

func parsePredicate(s string) (gin.Predicate, error) {
	s = strings.TrimSpace(s)

	if strings.HasSuffix(strings.ToUpper(s), " IS NULL") {
		path := strings.TrimSuffix(s, " IS NULL")
		path = strings.TrimSuffix(path, " is null")
		path = strings.TrimSpace(path)
		return gin.IsNull(path), nil
	}

	if strings.HasSuffix(strings.ToUpper(s), " IS NOT NULL") {
		path := strings.TrimSuffix(s, " IS NOT NULL")
		path = strings.TrimSuffix(path, " is not null")
		path = strings.TrimSpace(path)
		return gin.IsNotNull(path), nil
	}

	patterns := []struct {
		regex *regexp.Regexp
		op    gin.Operator
	}{
		{regexp.MustCompile(`^(.+?)\s*!=\s*(.+)$`), gin.OpNE},
		{regexp.MustCompile(`^(.+?)\s*>=\s*(.+)$`), gin.OpGTE},
		{regexp.MustCompile(`^(.+?)\s*<=\s*(.+)$`), gin.OpLTE},
		{regexp.MustCompile(`^(.+?)\s*>\s*(.+)$`), gin.OpGT},
		{regexp.MustCompile(`^(.+?)\s*<\s*(.+)$`), gin.OpLT},
		{regexp.MustCompile(`^(.+?)\s*=\s*(.+)$`), gin.OpEQ},
		{regexp.MustCompile(`(?i)^(.+?)\s+CONTAINS\s+(.+)$`), gin.OpContains},
		{regexp.MustCompile(`(?i)^(.+?)\s+IN\s+\((.+)\)$`), gin.OpIN},
		{regexp.MustCompile(`(?i)^(.+?)\s+NOT\s+IN\s+\((.+)\)$`), gin.OpNIN},
	}

	for _, p := range patterns {
		if matches := p.regex.FindStringSubmatch(s); matches != nil {
			path := strings.TrimSpace(matches[1])
			valueStr := strings.TrimSpace(matches[2])

			if p.op == gin.OpIN || p.op == gin.OpNIN {
				return gin.Predicate{Path: path, Operator: p.op, Value: parseValueList(valueStr)}, nil
			}

			return gin.Predicate{Path: path, Operator: p.op, Value: parseValue(valueStr)}, nil
		}
	}

	return gin.Predicate{}, errors.Errorf("cannot parse predicate: %s", s)
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
