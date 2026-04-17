package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/parquet-go/parquet-go"

	gin "github.com/amikos-tech/ami-gin"
)

type cliTestRecord struct {
	ID         int64  `parquet:"id"`
	Attributes string `parquet:"attributes"`
}

func createCLIParquetFile(t *testing.T, path string, records []cliTestRecord) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create parquet file: %v", err)
	}
	defer f.Close()

	writer := parquet.NewGenericWriter[cliTestRecord](f,
		parquet.MaxRowsPerRowGroup(1),
	)

	for _, record := range records {
		if _, err := writer.Write([]cliTestRecord{record}); err != nil {
			t.Fatalf("write parquet record: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close parquet writer: %v", err)
	}
}

func TestRunParseErrorHelper(t *testing.T) {
	helper := os.Getenv("GIN_INDEX_PARSE_HELPER")
	if helper == "" {
		return
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	args := []string{"-definitely-invalid-flag"}

	var code int
	switch helper {
	case "build":
		code = runBuild(args, &stdout, &stderr)
	case "query":
		code = runQuery(args, &stdout, &stderr)
	case "info":
		code = runInfo(args, &stdout, &stderr)
	case "extract":
		code = runExtract(args, &stdout, &stderr)
	default:
		t.Fatalf("unknown helper command %q", helper)
	}

	_, _ = os.Stdout.Write(stdout.Bytes())
	_, _ = os.Stderr.Write(stderr.Bytes())
	os.Exit(code)
}

func buildAdaptiveCLIInfoIndex(t *testing.T) *gin.GINIndex {
	t.Helper()

	config := gin.DefaultConfig()
	config.CardinalityThreshold = 3
	config.AdaptiveMinRGCoverage = 2
	config.AdaptivePromotedTermCap = 8
	config.AdaptiveCoverageCeiling = 0.75
	config.AdaptiveBucketCount = 16

	builder, err := gin.NewBuilder(config, 6)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	docs := []struct {
		rgID int
		json string
	}{
		{rgID: 0, json: `{"field":"hot"}`},
		{rgID: 1, json: `{"field":"hot"}`},
		{rgID: 2, json: `{"field":"tail_2"}`},
		{rgID: 3, json: `{"field":"tail_3"}`},
		{rgID: 4, json: `{"field":"tail_4"}`},
		{rgID: 5, json: `{"field":"tail_5"}`},
	}
	for _, doc := range docs {
		if err := builder.AddDocument(gin.DocID(doc.rgID), []byte(doc.json)); err != nil {
			t.Fatalf("AddDocument(rg=%d): %v", doc.rgID, err)
		}
	}

	return builder.Finalize()
}

func TestParsePredicateSupportedOperators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		predicate gin.Predicate
	}{
		{
			name:      "regex",
			input:     `$.brand REGEX "Toyota|Tesla"`,
			predicate: gin.Predicate{Path: "$.brand", Operator: gin.OpRegex, Value: "Toyota|Tesla"},
		},
		{
			name:      "regex with equals",
			input:     `$.brand REGEX "a=b"`,
			predicate: gin.Predicate{Path: "$.brand", Operator: gin.OpRegex, Value: "a=b"},
		},
		{
			name:      "regex lowercase",
			input:     `$.brand regex "Toyota|Tesla"`,
			predicate: gin.Predicate{Path: "$.brand", Operator: gin.OpRegex, Value: "Toyota|Tesla"},
		},
		{
			name:      "contains",
			input:     `$.message CONTAINS "warn"`,
			predicate: gin.Predicate{Path: "$.message", Operator: gin.OpContains, Value: "warn"},
		},
		{
			name:      "contains with equals",
			input:     `$.message CONTAINS "a=b"`,
			predicate: gin.Predicate{Path: "$.message", Operator: gin.OpContains, Value: "a=b"},
		},
		{
			name:      "contains mixed case",
			input:     `$.message CoNtAiNs "warn"`,
			predicate: gin.Predicate{Path: "$.message", Operator: gin.OpContains, Value: "warn"},
		},
		{
			name:      "in list",
			input:     `$.count IN (1, 2, "three")`,
			predicate: gin.Predicate{Path: "$.count", Operator: gin.OpIN, Value: []any{float64(1), float64(2), "three"}},
		},
		{
			name:      "not in list",
			input:     `$.count NOT IN (1, 2, "three")`,
			predicate: gin.Predicate{Path: "$.count", Operator: gin.OpNIN, Value: []any{float64(1), float64(2), "three"}},
		},
		{
			name:      "not in lowercase",
			input:     `$.count not in (1, 2, "three")`,
			predicate: gin.Predicate{Path: "$.count", Operator: gin.OpNIN, Value: []any{float64(1), float64(2), "three"}},
		},
		{
			name:      "is null",
			input:     `$.deleted_at IS NULL`,
			predicate: gin.IsNull("$.deleted_at"),
		},
		{
			name:      "is not null",
			input:     `$.deleted_at IS NOT NULL`,
			predicate: gin.IsNotNull("$.deleted_at"),
		},
		{
			name:      "is null mixed case",
			input:     `$.deleted_at Is Null`,
			predicate: gin.IsNull("$.deleted_at"),
		},
		{
			name:      "is not null mixed case",
			input:     `$.deleted_at Is Not Null`,
			predicate: gin.IsNotNull("$.deleted_at"),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parsePredicate(tt.input)
			if err != nil {
				t.Fatalf("parsePredicate(%q) returned error: %v", tt.input, err)
			}

			if got.Path != tt.predicate.Path {
				t.Fatalf("parsePredicate(%q) path = %q, want %q", tt.input, got.Path, tt.predicate.Path)
			}
			if got.Operator != tt.predicate.Operator {
				t.Fatalf("parsePredicate(%q) operator = %v, want %v", tt.input, got.Operator, tt.predicate.Operator)
			}
			if !reflect.DeepEqual(got.Value, tt.predicate.Value) {
				t.Fatalf("parsePredicate(%q) value = %#v, want %#v", tt.input, got.Value, tt.predicate.Value)
			}
		})
	}
}

func TestParsePredicateRejectsMalformedInput(t *testing.T) {
	t.Parallel()

	tests := []string{
		``,
		`$.brand REGEX`,
		`$.message CONTAINS`,
		`$.count NOT IN`,
		`$.deleted_at IS`,
	}

	for _, input := range tests {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			_, err := parsePredicate(input)
			if err == nil {
				t.Fatalf("parsePredicate(%q) returned nil error", input)
			}
		})
	}
}

func TestParsePredicateRejectsUnsupportedJSONPath(t *testing.T) {
	t.Parallel()

	tests := []string{
		`$.items[0] = "x"`,
		`$..name = "alice"`,
	}

	for _, input := range tests {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			_, err := parsePredicate(input)
			if err == nil {
				t.Fatalf("parsePredicate(%q) returned nil error", input)
			}
		})
	}
}

func TestParsePredicateRegexRoundTrip(t *testing.T) {
	t.Parallel()

	builder, err := gin.NewBuilder(gin.DefaultConfig(), 2)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	if err := builder.AddDocument(0, []byte(`{"brand":"Toyota Corolla"}`)); err != nil {
		t.Fatalf("AddDocument(0): %v", err)
	}
	if err := builder.AddDocument(1, []byte(`{"brand":"Ford Mustang"}`)); err != nil {
		t.Fatalf("AddDocument(1): %v", err)
	}

	idx := builder.Finalize()

	pred, err := parsePredicate(`$.brand REGEX "Toy.*"`)
	if err != nil {
		t.Fatalf("parsePredicate: %v", err)
	}

	result := idx.Evaluate([]gin.Predicate{pred})
	if got := result.ToSlice(); !reflect.DeepEqual(got, []int{0}) {
		t.Fatalf("regex round trip = %v, want [0]", got)
	}

	pred, err = parsePredicate(`$.brand REGEX "Ford.*"`)
	if err != nil {
		t.Fatalf("parsePredicate: %v", err)
	}

	result = idx.Evaluate([]gin.Predicate{pred})
	if got := result.ToSlice(); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("regex round trip = %v, want [1]", got)
	}
}

func TestPathInfoReportsAdaptiveMode(t *testing.T) {
	t.Parallel()

	config := gin.DefaultConfig()
	config.CardinalityThreshold = 10
	config.AdaptivePromotedTermCap = 64
	config.AdaptiveBucketCount = 128

	idx := gin.NewGINIndex()
	idx.Config = &config
	idx.Header.NumRowGroups = 8
	idx.Header.NumDocs = 16
	idx.Header.NumPaths = 3
	idx.Header.CardinalityThresh = config.CardinalityThreshold
	idx.PathDirectory = []gin.PathEntry{
		{PathID: 0, PathName: "$.exact", ObservedTypes: gin.TypeString, Cardinality: 3, Mode: gin.PathModeClassic},
		{PathID: 1, PathName: "$.bloom", ObservedTypes: gin.TypeString, Cardinality: 120, Mode: gin.PathModeBloomOnly},
		{PathID: 2, PathName: "$.adaptive", ObservedTypes: gin.TypeString, Cardinality: 240, Mode: gin.PathModeAdaptiveHybrid, AdaptivePromotedTerms: 5, AdaptiveBucketCount: 128},
	}

	var buf bytes.Buffer
	writeIndexInfo(&buf, idx)
	out := buf.String()

	for _, want := range []string{
		"mode=exact",
		"mode=bloom-only",
		"mode=adaptive-hybrid",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("writeIndexInfo output missing %q:\n%s", want, out)
		}
	}
}

func TestCLIInfoShowsAdaptiveSummary(t *testing.T) {
	t.Parallel()

	idx := buildAdaptiveCLIInfoIndex(t)

	var buf bytes.Buffer
	writeIndexInfo(&buf, idx)
	out := buf.String()

	for _, want := range []string{
		"mode=adaptive-hybrid",
		"promoted=1",
		"buckets=16",
		"threshold=3",
		"cap=8",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("writeIndexInfo output missing %q:\n%s", want, out)
		}
	}
}

func TestPathInfoDerivesAdaptiveSummaryFromSection(t *testing.T) {
	t.Parallel()

	config := gin.DefaultConfig()
	config.CardinalityThreshold = 3
	config.AdaptivePromotedTermCap = 8

	adaptive, err := gin.NewAdaptiveStringIndex(
		[]string{"hot", "warm"},
		[]*gin.RGSet{gin.MustNewRGSet(4), gin.MustNewRGSet(4)},
		[]*gin.RGSet{gin.MustNewRGSet(4), gin.MustNewRGSet(4), gin.MustNewRGSet(4), gin.MustNewRGSet(4)},
	)
	if err != nil {
		t.Fatalf("NewAdaptiveStringIndex() error = %v", err)
	}

	idx := gin.NewGINIndex()
	idx.Config = &config
	idx.Header.CardinalityThresh = 3
	idx.AdaptiveStringIndexes[0] = adaptive

	info := formatPathInfo(idx, gin.PathEntry{
		PathID:        0,
		PathName:      "$.adaptive",
		ObservedTypes: gin.TypeString,
		Cardinality:   42,
		Mode:          gin.PathModeAdaptiveHybrid,
	})

	for _, want := range []string{"promoted=2", "buckets=4", "threshold=3", "cap=8"} {
		if !strings.Contains(info, want) {
			t.Fatalf("formatPathInfo() missing %q in %q", want, info)
		}
	}
}

func TestPathInfoOmitsAdaptiveSummaryOutsideAdaptiveMode(t *testing.T) {
	t.Parallel()

	idx := gin.NewGINIndex()
	idx.Header.NumRowGroups = 4
	idx.Header.NumDocs = 4
	idx.Header.NumPaths = 2
	idx.PathDirectory = []gin.PathEntry{
		{PathID: 0, PathName: "$.exact", ObservedTypes: gin.TypeString, Cardinality: 2, Mode: gin.PathModeClassic},
		{PathID: 1, PathName: "$.bloom", ObservedTypes: gin.TypeString, Cardinality: 20, Mode: gin.PathModeBloomOnly},
	}

	var buf bytes.Buffer
	writeIndexInfo(&buf, idx)
	out := buf.String()
	if strings.Contains(out, "promoted=") || strings.Contains(out, "buckets=") {
		t.Fatalf("writeIndexInfo output unexpectedly included adaptive summary:\n%s", out)
	}
}

func TestCLIInfoSuppressesInternalRepresentationPaths(t *testing.T) {
	t.Parallel()

	config, err := gin.NewConfig(
		gin.WithToLowerTransformer("$.email", "lower"),
	)
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}

	builder, err := gin.NewBuilder(config, 1)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	if err := builder.AddDocument(0, []byte(`{"email":"Alice@Example.COM"}`)); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	idx := builder.Finalize()

	var buf bytes.Buffer
	writeIndexInfo(&buf, idx)
	out := buf.String()

	if strings.Contains(out, "__derived:") {
		t.Fatalf("writeIndexInfo output leaked internal representation path:\n%s", out)
	}
	if !strings.Contains(out, "representations=lower:to_lower") {
		t.Fatalf("writeIndexInfo output missing rendered representation metadata:\n%s", out)
	}
}

func TestRunInfoReturnsNonZeroOnDecodeFailure(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "broken.gin")
	if err := os.WriteFile(indexPath, []byte("not-a-valid-index"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runInfo([]string{indexPath}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("runInfo() code = 0, want non-zero for decode failure")
	}
	if !strings.Contains(stderr.String(), "Failed to decode index") {
		t.Fatalf("stderr = %q, want decode failure", stderr.String())
	}
}

func TestRunCommandsReturnParseFailureCode(t *testing.T) {
	t.Parallel()

	for _, helper := range []string{"build", "query", "info", "extract"} {
		helper := helper
		t.Run(helper, func(t *testing.T) {
			t.Parallel()

			cmd := exec.Command(os.Args[0], "-test.run=TestRunParseErrorHelper$")
			cmd.Env = append(os.Environ(), "GIN_INDEX_PARSE_HELPER="+helper)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			if err == nil {
				t.Fatal("subprocess succeeded, want parse failure")
			}

			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("Run() error = %v, want *exec.ExitError", err)
			}
			if exitErr.ExitCode() != 1 {
				t.Fatalf("exit code = %d, want 1; stderr=%q", exitErr.ExitCode(), stderr.String())
			}
			if !strings.Contains(stderr.String(), "flag provided but not defined") {
				t.Fatalf("stderr = %q, want parse failure output", stderr.String())
			}
		})
	}
}

func TestRunBuildReturnsNonZeroOnBuildFailure(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	parquetPath := filepath.Join(tmpDir, "broken.parquet")
	if err := os.WriteFile(parquetPath, []byte("not-a-parquet-file"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBuild([]string{"-c", "attributes", parquetPath}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("runBuild() code = 0, want non-zero for build failure")
	}
	if !strings.Contains(stderr.String(), "Failed to build index") {
		t.Fatalf("stderr = %q, want build failure", stderr.String())
	}
}

func TestRunQueryReturnsNonZeroOnDecodeFailure(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "broken.gin")
	if err := os.WriteFile(indexPath, []byte("not-a-valid-index"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runQuery([]string{indexPath, `$.brand = "Toyota"`}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("runQuery() code = 0, want non-zero for decode failure")
	}
	if !strings.Contains(stderr.String(), "Failed to decode index") {
		t.Fatalf("stderr = %q, want decode failure", stderr.String())
	}
}

func TestRunExtractReturnsNonZeroOnExtractFailure(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	parquetPath := filepath.Join(tmpDir, "broken.parquet")
	outputPath := filepath.Join(tmpDir, "out.gin")
	if err := os.WriteFile(parquetPath, []byte("not-a-parquet-file"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExtract([]string{"-o", outputPath, parquetPath}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("runExtract() code = 0, want non-zero for extract failure")
	}
	if !strings.Contains(stderr.String(), "Failed to read embedded index") {
		t.Fatalf("stderr = %q, want extract failure", stderr.String())
	}
}

func TestRunBuildReportsPartialFailures(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	createCLIParquetFile(t, filepath.Join(tmpDir, "good.parquet"), []cliTestRecord{
		{ID: 1, Attributes: `{"brand":"Toyota"}`},
	})
	if err := os.WriteFile(filepath.Join(tmpDir, "bad.parquet"), []byte("not-a-parquet-file"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBuild([]string{"-c", "attributes", tmpDir}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("runBuild() code = 0, want non-zero for partial failure")
	}
	if !strings.Contains(stdout.String(), "Processed 1/2 file(s) (1 failed)") {
		t.Fatalf("stdout = %q, want partial-failure summary", stdout.String())
	}
}

func TestRunExtractReportsPartialFailures(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	goodParquet := filepath.Join(tmpDir, "good.parquet")
	createCLIParquetFile(t, goodParquet, []cliTestRecord{
		{ID: 1, Attributes: `{"status":"ok"}`},
	})
	idx, err := gin.BuildFromParquet(goodParquet, "attributes", gin.DefaultConfig())
	if err != nil {
		t.Fatalf("BuildFromParquet() error = %v", err)
	}
	if err := gin.RebuildWithIndex(goodParquet, idx, gin.DefaultParquetConfig()); err != nil {
		t.Fatalf("RebuildWithIndex() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "bad.parquet"), []byte("not-a-parquet-file"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExtract([]string{tmpDir}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("runExtract() code = 0, want non-zero for partial failure")
	}
	if !strings.Contains(stdout.String(), "Processed 1/2 file(s) (1 failed)") {
		t.Fatalf("stdout = %q, want partial-failure summary", stdout.String())
	}
}

func TestArtifactFileMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   os.FileMode
		want os.FileMode
	}{
		{name: "preserve rw bits", in: 0o640, want: 0o640},
		{name: "drop execute bits", in: 0o755, want: 0o644},
		{name: "preserve group write and world read", in: 0o664, want: 0o664},
		{name: "mask world writable and execute bits", in: 0o777, want: 0o666},
		{name: "high bits do not affect rw mask", in: 0o4755, want: 0o644},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := artifactFileMode(tt.in); got != tt.want {
				t.Fatalf("artifactFileMode(%o) = %o, want %o", tt.in, got, tt.want)
			}
		})
	}
}

func TestBuildSingleFileSidecarUsesSourcePermissions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		sourceMode os.FileMode
		wantMode   os.FileMode
	}{
		{name: "preserve rw bits", sourceMode: 0o640, wantMode: 0o640},
		{name: "drop execute bits", sourceMode: 0o755, wantMode: 0o644},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			parquetFile := filepath.Join(tmpDir, "data.parquet")
			createCLIParquetFile(t, parquetFile, []cliTestRecord{
				{ID: 1, Attributes: `{"brand":"Toyota"}`},
				{ID: 2, Attributes: `{"brand":"Tesla"}`},
			})

			if err := os.Chmod(parquetFile, tt.sourceMode); err != nil {
				t.Fatalf("chmod parquet file: %v", err)
			}

			buildSingleFile(parquetFile, "attributes", "", false, gin.DefaultConfig(), gin.DefaultParquetConfig())

			info, err := os.Stat(parquetFile + ".gin")
			if err != nil {
				t.Fatalf("stat sidecar: %v", err)
			}

			if got := info.Mode().Perm(); got != tt.wantMode {
				t.Fatalf("sidecar mode = %o, want %o", got, tt.wantMode)
			}
		})
	}
}

func TestExtractSingleFileUsesSourcePermissionsForNewOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		sourceMode os.FileMode
		wantMode   os.FileMode
	}{
		{name: "preserve rw bits", sourceMode: 0o640, wantMode: 0o640},
		{name: "drop execute bits", sourceMode: 0o755, wantMode: 0o644},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			parquetFile := filepath.Join(tmpDir, "embedded.parquet")
			createCLIParquetFile(t, parquetFile, []cliTestRecord{
				{ID: 1, Attributes: `{"status":"ok"}`},
				{ID: 2, Attributes: `{"status":"warn"}`},
			})

			if err := os.Chmod(parquetFile, tt.sourceMode); err != nil {
				t.Fatalf("chmod parquet file: %v", err)
			}

			idx, err := gin.BuildFromParquet(parquetFile, "attributes", gin.DefaultConfig())
			if err != nil {
				t.Fatalf("BuildFromParquet: %v", err)
			}
			if err := gin.RebuildWithIndex(parquetFile, idx, gin.DefaultParquetConfig()); err != nil {
				t.Fatalf("RebuildWithIndex: %v", err)
			}

			output := filepath.Join(tmpDir, "extracted.gin")

			extractSingleFile(parquetFile, output, gin.DefaultParquetConfig())

			info, err := os.Stat(output)
			if err != nil {
				t.Fatalf("stat extracted output: %v", err)
			}

			if got := info.Mode().Perm(); got != tt.wantMode {
				t.Fatalf("extracted output mode = %o, want %o", got, tt.wantMode)
			}
		})
	}
}

func TestLocalOutputMode(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	parquetFile := filepath.Join(tmpDir, "data.parquet")
	createCLIParquetFile(t, parquetFile, []cliTestRecord{
		{ID: 1, Attributes: `{"brand":"Toyota"}`},
	})

	if err := os.Chmod(parquetFile, 0o755); err != nil {
		t.Fatalf("chmod parquet file: %v", err)
	}

	mode, err := localOutputMode(parquetFile)
	if err != nil {
		t.Fatalf("localOutputMode(local): %v", err)
	}
	if mode != 0o644 {
		t.Fatalf("localOutputMode(local) = %o, want %o", mode, os.FileMode(0o644))
	}

	mode, err = localOutputMode("s3://bucket/data.parquet")
	if err != nil {
		t.Fatalf("localOutputMode(s3): %v", err)
	}
	if mode != 0o600 {
		t.Fatalf("localOutputMode(s3) = %o, want %o", mode, os.FileMode(0o600))
	}
}

func TestLocalOutputModeWrapsStatErrors(t *testing.T) {
	t.Parallel()

	_, err := localOutputMode(filepath.Join(t.TempDir(), "missing.parquet"))
	if err == nil {
		t.Fatal("localOutputMode(missing) returned nil error")
	}
	if !strings.Contains(err.Error(), "stat local file") {
		t.Fatalf("localOutputMode(missing) error = %q, want stat local file context", err)
	}
}
