package gin_test

import (
	"context"
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/parquet-go/parquet-go"

	gin "github.com/amikos-tech/ami-gin"
	"github.com/amikos-tech/ami-gin/telemetry"
)

type ctxTestRecord struct {
	ID   int64  `parquet:"id"`
	JSON string `parquet:"json"`
}

func writeCtxTestParquet(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	defer func() { _ = f.Close() }()

	writer := parquet.NewGenericWriter[ctxTestRecord](f,
		parquet.MaxRowsPerRowGroup(2),
	)
	records := []ctxTestRecord{
		{ID: 1, JSON: `{"a":1}`},
		{ID: 2, JSON: `{"a":2}`},
		{ID: 3, JSON: `{"a":3}`},
		{ID: 4, JSON: `{"a":4}`},
	}
	if _, err := writer.Write(records); err != nil {
		t.Fatalf("write records: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
}

// TestBuildFromParquetContextHonorsPreCanceledContext proves P1: a pre-canceled
// ctx must stop the local build instead of returning a populated index.
func TestBuildFromParquetContextHonorsPreCanceledContext(t *testing.T) {
	dir := t.TempDir()
	pq := filepath.Join(dir, "cancel.parquet")
	writeCtxTestParquet(t, pq)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	idx, err := gin.BuildFromParquetContext(ctx, pq, "json", gin.DefaultConfig())
	if err == nil {
		t.Fatalf("expected canceled context to surface an error; got idx=%v", idx)
	}
	if !errorIsCanceledOrDeadline(err) {
		t.Fatalf("expected context.Canceled or DeadlineExceeded, got %v", err)
	}
	if idx != nil {
		t.Fatalf("expected nil index on canceled context; got non-nil")
	}
}

// TestEncodeContextAcceptsWithEncodeSignals proves P2: external callers can
// construct an EncodeOption via the exported helper.
func TestEncodeContextAcceptsWithEncodeSignals(t *testing.T) {
	idx, err := buildSmallIndex()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	opt := gin.WithEncodeSignals(telemetry.Disabled())
	data, err := gin.EncodeContext(context.Background(), idx, opt)
	if err != nil {
		t.Fatalf("EncodeContext with signals option: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("EncodeContext returned empty bytes")
	}
}

// TestDecodeContextAcceptsWithDecodeSignals proves P2 for Decode.
func TestDecodeContextAcceptsWithDecodeSignals(t *testing.T) {
	idx, err := buildSmallIndex()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	data, err := gin.Encode(idx)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	opt := gin.WithDecodeSignals(telemetry.Disabled())
	decoded, err := gin.DecodeContext(context.Background(), data, opt)
	if err != nil {
		t.Fatalf("DecodeContext with signals option: %v", err)
	}
	if decoded == nil {
		t.Fatal("DecodeContext returned nil index")
	}
}

func errorIsCanceledOrDeadline(err error) bool {
	if err == nil {
		return false
	}
	if stderrors.Is(err, context.Canceled) || stderrors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// Some SDK paths (notably AWS) wrap with an OperationError that drops
	// the sentinel from the chain but keeps the text.
	msg := err.Error()
	return strings.Contains(msg, "context canceled") ||
		strings.Contains(msg, "context deadline exceeded")
}
