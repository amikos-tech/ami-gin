package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gin "github.com/amikos-tech/ami-gin"
)

const experimentStatusErrorPredicate = `$.status = "error"`

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

func requireJSONKeys(t *testing.T, raw map[string]json.RawMessage, required, forbidden []string) {
	t.Helper()

	for _, key := range required {
		if _, ok := raw[key]; !ok {
			t.Fatalf("missing JSON key %q in %v", key, mapsKeys(raw))
		}
	}
	for _, key := range forbidden {
		if _, ok := raw[key]; ok {
			t.Fatalf("unexpected JSON key %q in %v", key, mapsKeys(raw))
		}
	}
}

func mapsKeys(raw map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	return keys
}

type failingReader struct {
	err error
}

func (r failingReader) Read(_ []byte) (int, error) {
	return 0, r.err
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
	requireJSONKeys(t, root, []string{"source", "summary", "paths"}, []string{"predicate_test"})

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
	if got := decodeJSONString(t, summary["status"]); got != experimentStatusComplete {
		t.Fatalf("summary.status = %q, want %s", got, experimentStatusComplete)
	}

	paths := decodeJSONArray(t, root["paths"])
	if len(paths) != 2 {
		t.Fatalf("len(paths) = %d, want 2", len(paths))
	}

	var statusPath map[string]json.RawMessage
	for _, raw := range paths {
		candidate := decodeJSONMap(t, raw)
		if decodeJSONString(t, candidate["path"]) == "$.status" {
			statusPath = candidate
			break
		}
	}
	if statusPath == nil {
		t.Fatalf("path $.status not found in %s", stdout.String())
	}
	representations, ok := statusPath["representations"]
	if !ok {
		t.Fatalf("$.status representations missing from %s", stdout.String())
	}
	if bytes.TrimSpace(representations)[0] != '[' {
		t.Fatalf("$.status representations = %s, want [] not null", string(statusPath["representations"]))
	}
	if trimmed := string(bytes.TrimSpace(representations)); trimmed != "[]" {
		t.Fatalf("$.status representations = %s, want []", trimmed)
	}
}

func TestExperimentIngestFailureGroupsDeterministic(t *testing.T) {
	t.Parallel()

	result := experimentBuildResult{}
	recordExperimentIngestFailure(&result, 8, &gin.IngestError{
		Path:  "$.score",
		Layer: gin.IngestLayerNumeric,
		Value: "1.5",
		Err:   errors.New("mixed numeric promotion"),
	})
	recordExperimentIngestFailure(&result, 2, &gin.IngestError{
		Layer: gin.IngestLayerParser,
		Value: "not-json",
		Err:   errors.New("read JSON token"),
	})
	recordExperimentIngestFailure(&result, 6, &gin.IngestError{
		Path:  "$.email",
		Layer: gin.IngestLayerTransformer,
		Value: "42",
		Err:   errors.New("transformer failed"),
	})

	failures := experimentIngestFailureGroups(result.ingestFailures)
	if len(failures) != 3 {
		t.Fatalf("len(failures) = %d, want 3", len(failures))
	}
	wantLayers := []string{"parser", "transformer", "numeric"}
	for i, want := range wantLayers {
		if failures[i].Layer != want {
			t.Fatalf("failures[%d].Layer = %q, want %q", i, failures[i].Layer, want)
		}
	}

	parser := failures[0]
	if parser.Count != 1 {
		t.Fatalf("parser.Count = %d, want 1", parser.Count)
	}
	if len(parser.Samples) != 1 {
		t.Fatalf("len(parser.Samples) = %d, want 1", len(parser.Samples))
	}
	sample := parser.Samples[0]
	if sample.Line != 2 || sample.InputIndex != 1 || sample.Path != "" || sample.Value != "not-json" || sample.Message == "" {
		t.Fatalf("parser sample = %+v, want structured parser sample", sample)
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
	code := runExperiment([]string{"--rg-size", "2", "--test", experimentStatusErrorPredicate, inputPath}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{
		"Predicate Test:",
		`Predicate: ` + experimentStatusErrorPredicate,
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
	code := runExperiment([]string{"--json", "--rg-size", "2", "--test", experimentStatusErrorPredicate, inputPath}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
	}

	root := decodeJSONMap(t, stdout.Bytes())
	requireJSONKeys(t, root, []string{"source", "summary", "paths", "predicate_test"}, nil)
	predicate := root["predicate_test"]

	predicateMap := decodeJSONMap(t, predicate)
	requireJSONKeys(t, predicateMap, []string{"predicate", "matched", "pruned", "pruning_ratio"}, nil)
	if got := decodeJSONString(t, predicateMap["predicate"]); got != experimentStatusErrorPredicate {
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

	summary := decodeJSONMap(t, root["summary"])
	if got := decodeJSONString(t, summary["status"]); got != experimentStatusComplete {
		t.Fatalf("summary.status = %q, want %s", got, experimentStatusComplete)
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
	predicate := experimentStatusErrorPredicate

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

func TestRunExperimentTrimsOverestimatedRowGroups(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inputPath := writeJSONLFixture(t, tmpDir, "docs.jsonl", []string{
		`{"status":"ok","user":"alice"}`,
		`{"status":"ok","user":"bob"}`,
		`{"status":"error","user":"cora"}`,
	}, true)
	outputPath := filepath.Join(tmpDir, "sample-trim.gin")
	predicate := experimentStatusErrorPredicate

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{"--json", "--rg-size", "2", "--sample", "10", "--test", predicate, "-o", outputPath, inputPath}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
	}

	root := decodeJSONMap(t, stdout.Bytes())
	summary := decodeJSONMap(t, root["summary"])
	if got := decodeJSONInt(t, summary["row_groups"]); got != 2 {
		t.Fatalf("summary.row_groups = %d, want 2", got)
	}

	predicateMap := decodeJSONMap(t, root["predicate_test"])
	if got := decodeJSONInt(t, predicateMap["matched"]); got != 1 {
		t.Fatalf("predicate_test.matched = %d, want 1", got)
	}
	if got := decodeJSONInt(t, predicateMap["pruned"]); got != 1 {
		t.Fatalf("predicate_test.pruned = %d, want 1", got)
	}

	idx, err := gin.ReadSidecar(strings.TrimSuffix(outputPath, ".gin"))
	if err != nil {
		t.Fatalf("ReadSidecar(%q): %v", outputPath, err)
	}
	if got := idx.Header.NumRowGroups; got != 2 {
		t.Fatalf("sidecar NumRowGroups = %d, want 2", got)
	}

	pred, err := parsePredicate(predicate)
	if err != nil {
		t.Fatalf("parsePredicate(%q): %v", predicate, err)
	}
	if got := len(idx.EvaluateContext(context.Background(), []gin.Predicate{pred}).ToSlice()); got != 1 {
		t.Fatalf("sidecar predicate matched = %d, want 1", got)
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

	for _, tc := range []struct {
		name    string
		level   string
		wantLog bool
	}{
		{name: experimentLogLevelInfo, level: experimentLogLevelInfo, wantLog: true},
		{name: experimentLogLevelDebug, level: experimentLogLevelDebug, wantLog: true},
		{name: experimentLogLevelOff, level: experimentLogLevelOff, wantLog: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := runExperiment([]string{"--log-level", tc.level, "--test", predicate, inputPath}, bytes.NewReader(nil), &stdout, &stderr)
			if code != 0 {
				t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
			}
			if strings.Contains(stdout.String(), "operation=query.evaluate") {
				t.Fatalf("stdout leaked query log output:\n%s", stdout.String())
			}
			if tc.wantLog {
				for _, want := range []string{"operation=query.evaluate", "status=ok"} {
					if !strings.Contains(stderr.String(), want) {
						t.Fatalf("stderr missing %q:\n%s", want, stderr.String())
					}
				}
				return
			}
			if strings.Contains(stderr.String(), "operation=query.evaluate") {
				t.Fatalf("stderr = %q, want no query log output when off", stderr.String())
			}
		})
	}
}

func TestRunExperimentOnErrorAbort(t *testing.T) {
	t.Parallel()

	stdin := strings.NewReader("{\"status\":\"ok\"}\n\n{\"status\":\"error\"}\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{"--on-error", experimentOnErrorAbort, "-"}, stdin, &stdout, &stderr)
	if code == 0 {
		t.Fatal("runExperiment() code = 0, want non-zero for abort mode")
	}
	if !strings.Contains(stderr.String(), "line 2:") {
		t.Fatalf("stderr = %q, want line-numbered abort error", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want no summary after abort", stdout.String())
	}
}

func TestRunExperimentOnErrorAbortFromStdinFailsFastBeforeDrain(t *testing.T) {
	t.Parallel()

	stdin := io.MultiReader(
		strings.NewReader("not-json\n"),
		failingReader{err: errors.New("stdin was drained after abort")},
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{"--on-error", experimentOnErrorAbort, "-"}, stdin, &stdout, &stderr)
	if code == 0 {
		t.Fatal("runExperiment() code = 0, want non-zero for abort mode")
	}
	if !strings.Contains(stderr.String(), "line 1:") {
		t.Fatalf("stderr = %q, want line-numbered abort error", stderr.String())
	}
	if strings.Contains(stderr.String(), "stdin was drained after abort") {
		t.Fatalf("stderr = %q, want abort before stdin is fully drained", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want no summary after abort", stdout.String())
	}
}

func TestRunExperimentOnErrorAbortIngestErrorDoesNotEmitGroupedSummary(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inputPath := writeJSONLFixture(t, tmpDir, "malformed.jsonl", []string{"not-json"}, true)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{"--on-error", experimentOnErrorAbort, inputPath}, bytes.NewReader(nil), &stdout, &stderr)
	if code == 0 {
		t.Fatal("runExperiment() code = 0, want non-zero for abort mode")
	}
	for _, want := range []string{"line 1:", "ingest parser failure"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
	if strings.Contains(stdout.String(), "Failures:") {
		t.Fatalf("stdout = %q, want no grouped summary after abort", stdout.String())
	}
}

func TestRunExperimentOnErrorContinue(t *testing.T) {
	t.Parallel()

	input := "{\"status\":\"ok\"}\n\n{\"status\":\"error\"}\n"

	t.Run("text", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := runExperiment([]string{"--on-error", experimentOnErrorContinue, "-"}, strings.NewReader(input), &stdout, &stderr)
		if code != 0 {
			t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "line 2:") {
			t.Fatalf("stderr = %q, want line-numbered continue error", stderr.String())
		}

		out := stdout.String()
		for _, want := range []string{
			"Documents: 2",
			"Status: " + experimentStatusPartial,
			"Processed Lines: 3",
			"Skipped Lines: 1",
			"Error Count: 1",
		} {
			if !strings.Contains(out, want) {
				t.Fatalf("stdout missing %q:\n%s", want, out)
			}
		}
	})

	t.Run("json", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := runExperiment([]string{"--json", "--on-error", experimentOnErrorContinue, "-"}, strings.NewReader(input), &stdout, &stderr)
		if code != 0 {
			t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "line 2:") {
			t.Fatalf("stderr = %q, want line-numbered continue error", stderr.String())
		}

		root := decodeJSONMap(t, stdout.Bytes())
		summary := decodeJSONMap(t, root["summary"])
		if got := decodeJSONInt(t, summary["documents"]); got != 2 {
			t.Fatalf("summary.documents = %d, want 2", got)
		}
		if got := decodeJSONInt(t, summary["processed_lines"]); got != 3 {
			t.Fatalf("summary.processed_lines = %d, want 3", got)
		}
		if got := decodeJSONInt(t, summary["skipped_lines"]); got != 1 {
			t.Fatalf("summary.skipped_lines = %d, want 1", got)
		}
		if got := decodeJSONInt(t, summary["error_count"]); got != 1 {
			t.Fatalf("summary.error_count = %d, want 1", got)
		}
		if got := decodeJSONString(t, summary["status"]); got != experimentStatusPartial {
			t.Fatalf("summary.status = %q, want %s", got, experimentStatusPartial)
		}
	})
}

func TestRunExperimentOnErrorContinueIngestFailuresText(t *testing.T) {
	t.Parallel()

	input := "{\"status\":\"ok\"}\nnot-json\n{\"status\":\"error\"}\n"

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{"--on-error", experimentOnErrorContinue, "-"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "line 2:") {
		t.Fatalf("stderr = %q, want line-numbered continue error", stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{
		"Documents: 2",
		"Skipped Lines: 1",
		"Error Count: 1",
		"Failures:",
		"parser: 1",
		"line 2",
		"input_index 1",
		`path ""`,
		`value "not-json"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout missing %q:\n%s", want, out)
		}
	}
}

func TestRunExperimentOnErrorContinueMalformedJSONFromFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inputPath := writeJSONLFixture(t, tmpDir, "malformed.jsonl", []string{
		`{"status":"ok"}`,
		`not-json`,
		`{"status":"error"}`,
	}, true)

	t.Run("text", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := runExperiment([]string{"--on-error", experimentOnErrorContinue, inputPath}, bytes.NewReader(nil), &stdout, &stderr)
		if code != 0 {
			t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "line 2:") {
			t.Fatalf("stderr = %q, want line-numbered continue error", stderr.String())
		}

		for _, want := range []string{
			"Documents: 2",
			"Status: " + experimentStatusPartial,
			"Processed Lines: 3",
			"Skipped Lines: 1",
			"Error Count: 1",
		} {
			if !strings.Contains(stdout.String(), want) {
				t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
			}
		}
	})

	t.Run("json", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		code := runExperiment([]string{"--json", "--on-error", experimentOnErrorContinue, inputPath}, bytes.NewReader(nil), &stdout, &stderr)
		if code != 0 {
			t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "line 2:") {
			t.Fatalf("stderr = %q, want line-numbered continue error", stderr.String())
		}

		root := decodeJSONMap(t, stdout.Bytes())
		summary := decodeJSONMap(t, root["summary"])
		if got := decodeJSONInt(t, summary["documents"]); got != 2 {
			t.Fatalf("summary.documents = %d, want 2", got)
		}
		if got := decodeJSONInt(t, summary["processed_lines"]); got != 3 {
			t.Fatalf("summary.processed_lines = %d, want 3", got)
		}
		if got := decodeJSONInt(t, summary["skipped_lines"]); got != 1 {
			t.Fatalf("summary.skipped_lines = %d, want 1", got)
		}
		if got := decodeJSONInt(t, summary["error_count"]); got != 1 {
			t.Fatalf("summary.error_count = %d, want 1", got)
		}
		if got := decodeJSONString(t, summary["status"]); got != experimentStatusPartial {
			t.Fatalf("summary.status = %q, want %s", got, experimentStatusPartial)
		}
	})
}

func TestRunExperimentSampleLimit(t *testing.T) {
	t.Parallel()

	stdin := io.MultiReader(
		strings.NewReader("{\"status\":\"ok\"}\n{\"status\":\"error\"}\n"),
		failingReader{err: errors.New("sample over-read")},
	)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{"--sample", "2", "--rg-size", "2", "-"}, stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
	}

	out := stdout.String()
	for _, want := range []string{
		"Documents: 2",
		"Row Groups: 1",
		"Sample Limit: 2",
		"Status: truncated",
		"Processed Lines: 2",
		"Skipped Lines: 0",
		"Error Count: 0",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout missing %q:\n%s", want, out)
		}
	}
}

func TestRunExperimentSampleLimitUsesHeadTake(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inputPath := writeJSONLFixture(t, tmpDir, "sample.jsonl", []string{
		`{"id":"first"}`,
		`{"id":"second"}`,
		`{"id":"third"}`,
	}, true)
	outputPath := filepath.Join(tmpDir, "sample.gin")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{"--json", "--sample", "2", "--rg-size", "1", "-o", outputPath, inputPath}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
	}

	root := decodeJSONMap(t, stdout.Bytes())
	summary := decodeJSONMap(t, root["summary"])
	if got := decodeJSONString(t, summary["status"]); got != "truncated" {
		t.Fatalf("summary.status = %q, want truncated", got)
	}

	idx, err := gin.ReadSidecar(strings.TrimSuffix(outputPath, ".gin"))
	if err != nil {
		t.Fatalf("ReadSidecar(%q): %v", outputPath, err)
	}

	for _, tc := range []struct {
		value string
		want  int
	}{
		{value: "first", want: 1},
		{value: "second", want: 1},
		{value: "third", want: 0},
	} {
		got := len(idx.EvaluateContext(context.Background(), []gin.Predicate{gin.EQ("$.id", tc.value)}).ToSlice())
		if got != tc.want {
			t.Fatalf("sampled index match count for %q = %d, want %d", tc.value, got, tc.want)
		}
	}
}

func TestRunExperimentRejectsInvalidOnErrorValue(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{"--on-error", "skip", "-"}, bytes.NewReader(nil), &stdout, &stderr)
	if code == 0 {
		t.Fatal("runExperiment() code = 0, want non-zero for invalid on-error")
	}
	if !strings.Contains(stderr.String(), "--on-error") {
		t.Fatalf("stderr = %q, want on-error validation error", stderr.String())
	}
}

func TestRunExperimentRejectsInvalidLogLevel(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{"--log-level", "trace", "-"}, bytes.NewReader(nil), &stdout, &stderr)
	if code == 0 {
		t.Fatal("runExperiment() code = 0, want non-zero for invalid log level")
	}
	if !strings.Contains(stderr.String(), "--log-level") {
		t.Fatalf("stderr = %q, want log-level validation error", stderr.String())
	}
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

func TestRunExperimentEmptyInputSidecarHasZeroRowGroups(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	inputPath := writeJSONLFixture(t, tmpDir, "empty.jsonl", nil, false)
	outputPath := filepath.Join(tmpDir, "empty.gin")
	predicate := experimentStatusErrorPredicate

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runExperiment([]string{"--json", "--test", predicate, "-o", outputPath, inputPath}, bytes.NewReader(nil), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExperiment() code = %d, want 0; stderr=%q", code, stderr.String())
	}

	root := decodeJSONMap(t, stdout.Bytes())
	summary := decodeJSONMap(t, root["summary"])
	if got := decodeJSONInt(t, summary["row_groups"]); got != 0 {
		t.Fatalf("summary.row_groups = %d, want 0", got)
	}

	predicateMap := decodeJSONMap(t, root["predicate_test"])
	if got := decodeJSONInt(t, predicateMap["matched"]); got != 0 {
		t.Fatalf("predicate_test.matched = %d, want 0", got)
	}
	if got := decodeJSONInt(t, predicateMap["pruned"]); got != 0 {
		t.Fatalf("predicate_test.pruned = %d, want 0", got)
	}

	idx, err := gin.ReadSidecar(strings.TrimSuffix(outputPath, ".gin"))
	if err != nil {
		t.Fatalf("ReadSidecar(%q): %v", outputPath, err)
	}
	if got := idx.Header.NumRowGroups; got != 0 {
		t.Fatalf("sidecar NumRowGroups = %d, want 0", got)
	}

	pred, err := parsePredicate(predicate)
	if err != nil {
		t.Fatalf("parsePredicate(%q): %v", predicate, err)
	}
	if got := len(idx.EvaluateContext(context.Background(), []gin.Predicate{pred}).ToSlice()); got != 0 {
		t.Fatalf("sidecar predicate matched = %d, want 0", got)
	}
}

func TestEvaluateExperimentPredicateCanceled(t *testing.T) {
	t.Parallel()

	idx := gin.NewGINIndex()
	idx.Header.NumRowGroups = 3

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := evaluateExperimentPredicate(ctx, 3, idx, `$.status = "ok"`, gin.EQ("$.status", "ok"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("evaluateExperimentPredicate() error = %v, want context.Canceled", err)
	}
	if result != nil {
		t.Fatalf("evaluateExperimentPredicate() result = %#v, want nil on cancellation", result)
	}
}

func TestRunExperimentCanceledPredicateExitsCleanly(t *testing.T) {
	// Cannot run in parallel: swaps the package-level newExperimentInterruptContext.
	orig := newExperimentInterruptContext
	t.Cleanup(func() { newExperimentInterruptContext = orig })
	newExperimentInterruptContext = func(parent context.Context) (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(parent)
		cancel()
		return ctx, func() {}
	}

	tmpDir := t.TempDir()
	inputPath := writeJSONLFixture(t, tmpDir, "docs.jsonl", []string{
		`{"status":"ok"}`,
		`{"status":"error"}`,
	}, true)

	var stdout, stderr bytes.Buffer
	code := runExperiment([]string{"--test", `$.status = "ok"`, inputPath}, bytes.NewReader(nil), &stdout, &stderr)
	if code == 0 {
		t.Fatalf("runExperiment() code = 0, want non-zero on canceled predicate eval; stderr=%q", stderr.String())
	}
	errOut := stderr.String()
	if !strings.Contains(errOut, "Error:") || !strings.Contains(errOut, "canceled") {
		t.Fatalf("stderr = %q, want 'Error: ... canceled ...'", errOut)
	}
}
