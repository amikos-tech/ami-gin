package main

import (
	"os"
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

func createCLIParquetFile(t *testing.T, path string, records []cliTestRecord, rowsPerRG int64) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create parquet file: %v", err)
	}
	defer f.Close()

	writer := parquet.NewGenericWriter[cliTestRecord](f,
		parquet.MaxRowsPerRowGroup(rowsPerRG),
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
			}, 1)

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
			}, 1)

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
	}, 1)

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
