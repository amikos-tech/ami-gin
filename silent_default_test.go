package gin_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	gin "github.com/amikos-tech/ami-gin"
)

func TestLibraryIsSilentWithDefaultConfig(t *testing.T) {
	idx, err := buildSmallIndex()
	if err != nil {
		t.Fatalf("build index: %v", err)
	}

	dir := t.TempDir()
	parquetPath := filepath.Join(dir, "silent.parquet")
	writeCtxTestParquet(t, parquetPath)

	stdout, stderr := captureStandardPipes(t, func() {
		data, err := gin.EncodeContext(context.Background(), idx)
		if err != nil {
			t.Fatalf("EncodeContext: %v", err)
		}

		decoded, err := gin.DecodeContext(context.Background(), data)
		if err != nil {
			t.Fatalf("DecodeContext: %v", err)
		}
		if got := decoded.EvaluateContext(context.Background(), []gin.Predicate{gin.EQ("$.name", "alice")}); got == nil {
			t.Fatal("EvaluateContext returned nil")
		}

		built, err := gin.BuildFromParquetContext(context.Background(), parquetPath, "json", gin.DefaultConfig())
		if err != nil {
			t.Fatalf("BuildFromParquetContext: %v", err)
		}
		if built == nil {
			t.Fatal("BuildFromParquetContext returned nil")
		}
	})

	if stdout != "" {
		t.Fatalf("stdout = %q; want empty", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q; want empty", stderr)
	}
}

func captureStandardPipes(t *testing.T, fn func()) (string, string) {
	t.Helper()

	origStdout := os.Stdout
	origStderr := os.Stderr

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		_ = stdoutR.Close()
		_ = stdoutW.Close()
		t.Fatalf("stderr pipe: %v", err)
	}

	stdoutCh := make(chan string, 1)
	stderrCh := make(chan string, 1)
	go func() {
		defer func() { _ = stdoutR.Close() }()
		data, _ := io.ReadAll(stdoutR)
		stdoutCh <- string(data)
	}()
	go func() {
		defer func() { _ = stderrR.Close() }()
		data, _ := io.ReadAll(stderrR)
		stderrCh <- string(data)
	}()

	os.Stdout = stdoutW
	os.Stderr = stderrW

	cleaned := false
	cleanup := func() {
		if cleaned {
			return
		}
		cleaned = true
		os.Stdout = origStdout
		os.Stderr = origStderr
		_ = stdoutW.Close()
		_ = stderrW.Close()
	}
	defer cleanup()

	fn()
	cleanup()

	return <-stdoutCh, <-stderrCh
}
