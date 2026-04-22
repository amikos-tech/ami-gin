package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gin "github.com/amikos-tech/ami-gin"
)

func writeJSONLFixture(t *testing.T, dir, name string, lines []string, trailingNewline bool) string {
	t.Helper()

	content := strings.Join(lines, "\n")
	if trailingNewline {
		content += "\n"
	}

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}

	return path
}

func decodeJSONMap(t *testing.T, raw []byte) map[string]json.RawMessage {
	t.Helper()

	var out map[string]json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("json.Unmarshal(map): %v\n%s", err, string(raw))
	}
	return out
}

func decodeJSONArray(t *testing.T, raw json.RawMessage) []json.RawMessage {
	t.Helper()

	var out []json.RawMessage
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("json.Unmarshal(array): %v\n%s", err, string(raw))
	}
	return out
}

func decodeJSONString(t *testing.T, raw json.RawMessage) string {
	t.Helper()

	var out string
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("json.Unmarshal(string): %v\n%s", err, string(raw))
	}
	return out
}

func decodeJSONBool(t *testing.T, raw json.RawMessage) bool {
	t.Helper()

	var out bool
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("json.Unmarshal(bool): %v\n%s", err, string(raw))
	}
	return out
}

func decodeJSONInt(t *testing.T, raw json.RawMessage) int {
	t.Helper()

	var out int
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("json.Unmarshal(int): %v\n%s", err, string(raw))
	}
	return out
}

func decodeJSONFloat(t *testing.T, raw json.RawMessage) float64 {
	t.Helper()

	var out float64
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("json.Unmarshal(float64): %v\n%s", err, string(raw))
	}
	return out
}

func TestRunExperimentJSONGolden(t *testing.T) {
	t.Parallel()

	stdin := strings.NewReader("{\"status\":\"ok\"}\n{\"status\":\"error\"}\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{"--json", "-"}, stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
	}

	root := decodeJSONMap(t, stdout.Bytes())
	if len(root) != 3 {
		t.Fatalf("top-level key count = %d, want 3", len(root))
	}
	for _, key := range []string{"source", "summary", "paths"} {
		if _, ok := root[key]; !ok {
			t.Fatalf("top-level key %q missing from %s", key, stdout.String())
		}
	}
	if _, ok := root["predicate_test"]; ok {
		t.Fatalf("predicate_test unexpectedly present in %s", stdout.String())
	}

	source := decodeJSONMap(t, root["source"])
	if got := decodeJSONString(t, source["input"]); got != "-" {
		t.Fatalf("source.input = %q, want \"-\"", got)
	}
	if got := decodeJSONBool(t, source["stdin"]); !got {
		t.Fatalf("source.stdin = %v, want true", got)
	}

	summary := decodeJSONMap(t, root["summary"])
	if got := decodeJSONInt(t, summary["documents"]); got != 2 {
		t.Fatalf("summary.documents = %d, want 2", got)
	}
	if got := decodeJSONInt(t, summary["row_groups"]); got != 2 {
		t.Fatalf("summary.row_groups = %d, want 2", got)
	}
	if got := decodeJSONInt(t, summary["rg_size"]); got != 1 {
		t.Fatalf("summary.rg_size = %d, want 1", got)
	}

	paths := decodeJSONArray(t, root["paths"])
	if len(paths) != 2 {
		t.Fatalf("len(paths) = %d, want 2", len(paths))
	}

	statusPath := decodeJSONMap(t, paths[1])
	if got := decodeJSONString(t, statusPath["path"]); got != "$.status" {
		t.Fatalf("path[1].path = %q, want $.status", got)
	}
	representations, ok := statusPath["representations"]
	if !ok {
		t.Fatalf("path[1].representations missing from %s", stdout.String())
	}
	if bytes.TrimSpace(representations)[0] != '[' {
		t.Fatalf("path[1].representations = %s, want [] not null", string(statusPath["representations"]))
	}
	if trimmed := string(bytes.TrimSpace(representations)); trimmed != "[]" {
		t.Fatalf("path[1].representations = %s, want []", trimmed)
	}
}

func TestRunExperimentPredicateReportText(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inputPath := writeJSONLFixture(t, tmpDir, "docs.jsonl", []string{
		`{"status":"ok","user":"alice"}`,
		`{"status":"ok","user":"bob"}`,
		`{"status":"error","user":"cora"}`,
	}, true)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{"--rg-size", "2", "--test", `$.status = "error"`, inputPath}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{
		"Predicate Test:",
		`Predicate: $.status = "error"`,
		"Matched: 1",
		"Pruned: 1",
		"Pruning Ratio: 0.5000",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Matching row groups") {
		t.Fatalf("stdout unexpectedly leaked row-group list:\n%s", out)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty output with default log level", stderr.String())
	}
}

func TestRunExperimentPredicateReportJSON(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inputPath := writeJSONLFixture(t, tmpDir, "docs.jsonl", []string{
		`{"status":"ok","user":"alice"}`,
		`{"status":"ok","user":"bob"}`,
		`{"status":"error","user":"cora"}`,
	}, true)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{"--json", "--rg-size", "2", "--test", `$.status = "error"`, inputPath}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
	}

	root := decodeJSONMap(t, stdout.Bytes())
	predicate, ok := root["predicate_test"]
	if !ok {
		t.Fatalf("predicate_test missing from %s", stdout.String())
	}
	if len(root) != 4 {
		t.Fatalf("top-level key count = %d, want 4", len(root))
	}

	predicateMap := decodeJSONMap(t, predicate)
	if len(predicateMap) != 4 {
		t.Fatalf("predicate_test key count = %d, want 4", len(predicateMap))
	}
	if got := decodeJSONString(t, predicateMap["predicate"]); got != `$.status = "error"` {
		t.Fatalf("predicate_test.predicate = %q, want canonical predicate", got)
	}
	if got := decodeJSONInt(t, predicateMap["matched"]); got != 1 {
		t.Fatalf("predicate_test.matched = %d, want 1", got)
	}
	if got := decodeJSONInt(t, predicateMap["pruned"]); got != 1 {
		t.Fatalf("predicate_test.pruned = %d, want 1", got)
	}
	if got := decodeJSONFloat(t, predicateMap["pruning_ratio"]); math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("predicate_test.pruning_ratio = %f, want 0.5", got)
	}
}

func TestRunExperimentWritesSidecarRoundTrip(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inputPath := writeJSONLFixture(t, tmpDir, "docs.jsonl", []string{
		`{"status":"ok","user":"alice"}`,
		`{"status":"ok","user":"bob"}`,
		`{"status":"error","user":"cora"}`,
	}, true)
	outputPath := filepath.Join(tmpDir, "experiment-artifact.gin")
	predicate := `$.status = "error"`

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{"--json", "--rg-size", "2", "--test", predicate, "-o", outputPath, inputPath}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
	}

	root := decodeJSONMap(t, stdout.Bytes())
	summary := decodeJSONMap(t, root["summary"])
	if got := decodeJSONString(t, summary["sidecar_path"]); got != outputPath {
		t.Fatalf("summary.sidecar_path = %q, want %q", got, outputPath)
	}

	idx, err := gin.ReadSidecar(strings.TrimSuffix(outputPath, ".gin"))
	if err != nil {
		t.Fatalf("ReadSidecar(%q): %v", outputPath, err)
	}
	pred, err := parsePredicate(predicate)
	if err != nil {
		t.Fatalf("parsePredicate(%q): %v", predicate, err)
	}

	matched := len(idx.EvaluateContext(context.Background(), []gin.Predicate{pred}).ToSlice())
	usedRowGroups := 2
	wantRatio := float64(usedRowGroups-matched) / float64(usedRowGroups)

	predicateMap := decodeJSONMap(t, root["predicate_test"])
	if got := decodeJSONFloat(t, predicateMap["pruning_ratio"]); math.Abs(got-wantRatio) > 1e-9 {
		t.Fatalf("predicate_test.pruning_ratio = %f, want %f", got, wantRatio)
	}
}

func TestRunExperimentRejectsNonGinOutput(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inputPath := writeJSONLFixture(t, tmpDir, "docs.jsonl", []string{
		`{"status":"ok"}`,
	}, true)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{"-o", filepath.Join(tmpDir, "artifact.txt"), inputPath}, bytes.NewReader(nil), &stdout, &stderr)
	if code == 0 {
		t.Fatal("runExperiment() code = 0, want non-zero for non-.gin output")
	}
	if !strings.Contains(stderr.String(), ".gin") {
		t.Fatalf("stderr = %q, want .gin validation error", stderr.String())
	}
}

func TestRunExperimentLogLevelWritesOnlyToStderr(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inputPath := writeJSONLFixture(t, tmpDir, "docs.jsonl", []string{
		`{"status":"ok","user":"alice"}`,
		`{"status":"error","user":"bob"}`,
	}, true)
	predicate := `$.status = "error"`

	t.Run("info", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := runExperiment([]string{"--log-level", "info", "--test", predicate, inputPath}, bytes.NewReader(nil), &stdout, &stderr)
		if code != 0 {
			t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
		}
		if strings.Contains(stdout.String(), "evaluate completed") {
			t.Fatalf("stdout leaked log output:\n%s", stdout.String())
		}
		if !strings.Contains(stderr.String(), "evaluate completed") {
			t.Fatalf("stderr = %q, want evaluate log line", stderr.String())
		}
	})

	t.Run("off", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := runExperiment([]string{"--log-level", "off", "--test", predicate, inputPath}, bytes.NewReader(nil), &stdout, &stderr)
		if code != 0 {
			t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
		}
		if strings.Contains(stderr.String(), "evaluate completed") {
			t.Fatalf("stderr = %q, want no library log output when off", stderr.String())
		}
	})
}

func TestRunExperimentFromFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inputPath := writeJSONLFixture(t, tmpDir, "docs.jsonl", []string{
		`{"status":"ok","user":"alice"}`,
		`{"status":"ok","user":"bob"}`,
		`{"status":"error","user":"cora"}`,
	}, true)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{"--rg-size", "2", inputPath}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{
		"Experiment Summary:",
		fmt.Sprintf("Input: %s", inputPath),
		"Documents: 3",
		"Row Groups: 2",
		"RG Size: 2",
		"GIN Index Info:",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout missing %q:\n%s", want, out)
		}
	}
}

func TestRunExperimentFromStdin(t *testing.T) {
	t.Parallel()

	stdin := strings.NewReader("{\"status\":\"ok\"}\n{\"status\":\"error\"}\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{"-"}, stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{
		"Experiment Summary:",
		"Input: -",
		"Documents: 2",
		"Row Groups: 2",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout missing %q:\n%s", want, out)
		}
	}
}

func TestRunExperimentFinalLineWithoutTrailingNewline(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inputPath := writeJSONLFixture(t, tmpDir, "docs.jsonl", []string{
		`{"status":"ok"}`,
		`{"status":"tail"}`,
	}, false)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{inputPath}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Documents: 2") {
		t.Fatalf("stdout = %q, want final line counted", stdout.String())
	}
}

func TestRunExperimentLargeLineNoTruncation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	largeValue := strings.Repeat("x", 70*1024)
	inputPath := writeJSONLFixture(t, tmpDir, "large.jsonl", []string{
		fmt.Sprintf(`{"message":"%s"}`, largeValue),
	}, true)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{inputPath}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Documents: 1") {
		t.Fatalf("stdout = %q, want one ingested large document", stdout.String())
	}
}

func TestRunExperimentTextOutputOrder(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inputPath := writeJSONLFixture(t, tmpDir, "docs.jsonl", []string{
		`{"status":"ok"}`,
	}, true)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{inputPath}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
	}

	out := stdout.String()
	summaryPos := strings.Index(out, "Experiment Summary:")
	infoPos := strings.Index(out, "GIN Index Info:")
	if summaryPos < 0 || infoPos < 0 || summaryPos >= infoPos {
		t.Fatalf("stdout order invalid:\n%s", out)
	}
}

func TestRunExperimentRejectsInvalidRGSize(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{"--rg-size", "0", "-"}, bytes.NewReader(nil), &stdout, &stderr)
	if code == 0 {
		t.Fatal("runExperiment() code = 0, want non-zero for invalid rg-size")
	}
	if !strings.Contains(stderr.String(), "--rg-size") {
		t.Fatalf("stderr = %q, want rg-size validation error", stderr.String())
	}
}

func TestRunExperimentRejectsDirectoryInput(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{tmpDir}, bytes.NewReader(nil), &stdout, &stderr)
	if code == 0 {
		t.Fatal("runExperiment() code = 0, want non-zero for directory input")
	}
	if !strings.Contains(strings.ToLower(stderr.String()), "directory") {
		t.Fatalf("stderr = %q, want directory error", stderr.String())
	}
}

func TestRunExperimentEmptyInput(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inputPath := writeJSONLFixture(t, tmpDir, "empty.jsonl", nil, false)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{inputPath}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{
		"Experiment Summary:",
		"Documents: 0",
		"Row Groups: 0",
		"GIN Index Info:",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout missing %q:\n%s", want, out)
		}
	}
}
