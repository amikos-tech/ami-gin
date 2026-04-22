package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
