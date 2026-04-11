package main

import (
	"os"
	"path/filepath"
	"reflect"
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
			name:      "contains",
			input:     `$.message CONTAINS "warn"`,
			predicate: gin.Predicate{Path: "$.message", Operator: gin.OpContains, Value: "warn"},
		},
		{
			name:      "in list",
			input:     `$.count IN (1, 2, "three")`,
			predicate: gin.Predicate{Path: "$.count", Operator: gin.OpIN, Value: []any{float64(1), float64(2), "three"}},
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

func TestBuildSingleFileSidecarUsesSourcePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	parquetFile := filepath.Join(tmpDir, "data.parquet")
	createCLIParquetFile(t, parquetFile, []cliTestRecord{
		{ID: 1, Attributes: `{"brand":"Toyota"}`},
		{ID: 2, Attributes: `{"brand":"Tesla"}`},
	}, 1)

	if err := os.Chmod(parquetFile, 0o640); err != nil {
		t.Fatalf("chmod parquet file: %v", err)
	}

	buildSingleFile(parquetFile, "attributes", "", false, gin.DefaultConfig(), gin.DefaultParquetConfig())

	info, err := os.Stat(parquetFile + ".gin")
	if err != nil {
		t.Fatalf("stat sidecar: %v", err)
	}

	if got, want := info.Mode().Perm(), os.FileMode(0o640); got != want {
		t.Fatalf("sidecar mode = %o, want %o", got, want)
	}
}

func TestExtractSingleFileUsesSourcePermissionsForNewOutput(t *testing.T) {
	tmpDir := t.TempDir()
	parquetFile := filepath.Join(tmpDir, "embedded.parquet")
	createCLIParquetFile(t, parquetFile, []cliTestRecord{
		{ID: 1, Attributes: `{"status":"ok"}`},
		{ID: 2, Attributes: `{"status":"warn"}`},
	}, 1)

	if err := os.Chmod(parquetFile, 0o640); err != nil {
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

	if got, want := info.Mode().Perm(), os.FileMode(0o640); got != want {
		t.Fatalf("extracted output mode = %o, want %o", got, want)
	}
}
