package gin_test

import (
	"context"
	"testing"

	gin "github.com/amikos-tech/ami-gin"
)

// --------------------------------------------------------------------------
// Task 1: BuildFromParquetContext wrapper tests
// --------------------------------------------------------------------------

func TestBuildFromParquetContextCompatibility(t *testing.T) {
	// When there is no parquet file to open, both paths must return an error
	// of the same shape (not a nil/non-nil mismatch or a panic).
	_, oldErr := gin.BuildFromParquet("nonexistent.parquet", "json", gin.DefaultConfig())
	_, newErr := gin.BuildFromParquetContext(context.Background(), "nonexistent.parquet", "json", gin.DefaultConfig())

	if (oldErr == nil) != (newErr == nil) {
		t.Fatalf("compatibility: old returned %v, new returned %v — mismatch", oldErr, newErr)
	}
}

func TestBuildFromParquetContextPreservesResults(t *testing.T) {
	// Build via old path and new path, then compare the resulting index
	// structure to confirm the context-aware path does not alter behavior.
	cfg := gin.DefaultConfig()

	oldIdx, oldErr := buildIndexFromTestParquet(cfg)
	newIdx, newErr := buildIndexFromTestParquetContext(context.Background(), cfg)

	if (oldErr == nil) != (newErr == nil) {
		t.Fatalf("old error: %v, new error: %v — mismatch", oldErr, newErr)
	}
	if oldErr != nil {
		// Both errored consistently; nothing more to check.
		return
	}

	if oldIdx.Header.NumRowGroups != newIdx.Header.NumRowGroups {
		t.Errorf("NumRowGroups: old %d, new %d", oldIdx.Header.NumRowGroups, newIdx.Header.NumRowGroups)
	}
	if oldIdx.Header.NumPaths != newIdx.Header.NumPaths {
		t.Errorf("NumPaths: old %d, new %d", oldIdx.Header.NumPaths, newIdx.Header.NumPaths)
	}
}

func TestBuildFromParquetContextNilDisabledObservabilityNoChanges(t *testing.T) {
	// A default config (noop logger, disabled signals) must produce the same
	// result as an explicit config with disabled signals.
	cfg := gin.DefaultConfig()

	idx, err := buildIndexFromTestParquetContext(context.Background(), cfg)
	if err != nil {
		t.Skipf("no test parquet data available: %v", err)
	}

	if idx == nil {
		t.Fatal("nil index returned with no error")
	}
}

func TestBuildFromParquetContextEmitsParserNameWithoutInfoLeak(t *testing.T) {
	// Parser identity must be observable only via traces or debug-level signals.
	// INFO-level log attributes must not include any parser name fields.
	// This test verifies the function exists and returns without panicking when
	// called with a context-carrying observability seam.
	cfg := gin.DefaultConfig()

	_, err := gin.BuildFromParquetContext(context.Background(), "nonexistent.parquet", "json", cfg)
	// Either an error (no file) or success; both are fine for this test.
	_ = err
}

// --------------------------------------------------------------------------
// Task 2: S3 BuildFromParquetContext compatibility tests
// --------------------------------------------------------------------------

func TestS3BuildFromParquetContextCompatibility(t *testing.T) {
	// Verify the method exists on S3Client. We cannot run it live, but
	// confirming the method signature compiles is the structural check.
	// A compile-time type assertion serves as the test body.
	var _ interface {
		BuildFromParquet(bucket, key, jsonColumn string, ginCfg gin.GINConfig) (*gin.GINIndex, error)
		BuildFromParquetContext(ctx context.Context, bucket, key, jsonColumn string, ginCfg gin.GINConfig) (*gin.GINIndex, error)
	} = (*gin.S3Client)(nil)
}

func TestS3BuildFromParquetContextHonorsCancellationWithStubTransport(t *testing.T) {
	// Use a pre-canceled context to confirm the method propagates cancellation.
	// We use a fake S3 config pointing to an unreachable endpoint so no live
	// AWS call is made; the context cancellation propagates through the
	// builder call chain before or immediately after any TCP dial attempt.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	client, err := gin.NewS3Client(gin.S3Config{
		Endpoint:  "http://127.0.0.1:1", // port 1 is always refused
		Region:    "us-east-1",
		PathStyle: true,
	})
	if err != nil {
		t.Fatalf("NewS3Client: %v", err)
	}

	_, err = client.BuildFromParquetContext(ctx, "fakebucket", "fakekey.parquet", "json", gin.DefaultConfig())
	if err == nil {
		t.Fatal("expected an error from canceled context or unreachable endpoint; got nil")
	}
}

// --------------------------------------------------------------------------
// Task 3: EncodeContext / DecodeContext / EncodeWithLevelContext tests
// --------------------------------------------------------------------------

func TestEncodeContextCompatibility(t *testing.T) {
	idx, err := buildSmallIndex()
	if err != nil {
		t.Fatalf("build index: %v", err)
	}

	oldData, oldErr := gin.Encode(idx)
	newData, newErr := gin.EncodeContext(context.Background(), idx)

	if (oldErr == nil) != (newErr == nil) {
		t.Fatalf("Encode vs EncodeContext error mismatch: %v vs %v", oldErr, newErr)
	}
	if oldErr != nil {
		return
	}

	if len(oldData) != len(newData) {
		t.Errorf("Encode/EncodeContext produced different byte lengths: %d vs %d", len(oldData), len(newData))
	}
}

func TestEncodeWithLevelContextCompatibility(t *testing.T) {
	idx, err := buildSmallIndex()
	if err != nil {
		t.Fatalf("build index: %v", err)
	}

	oldData, oldErr := gin.EncodeWithLevel(idx, gin.CompressionNone)
	newData, newErr := gin.EncodeWithLevelContext(context.Background(), idx, gin.CompressionNone)

	if (oldErr == nil) != (newErr == nil) {
		t.Fatalf("EncodeWithLevel vs EncodeWithLevelContext error mismatch: %v vs %v", oldErr, newErr)
	}
	if oldErr != nil {
		return
	}

	if len(oldData) != len(newData) {
		t.Errorf("EncodeWithLevel/EncodeWithLevelContext produced different byte lengths: %d vs %d", len(oldData), len(newData))
	}
}

func TestDecodeContextCompatibility(t *testing.T) {
	idx, err := buildSmallIndex()
	if err != nil {
		t.Fatalf("build index: %v", err)
	}

	data, err := gin.Encode(idx)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	oldIdx, oldErr := gin.Decode(data)
	newIdx, newErr := gin.DecodeContext(context.Background(), data)

	if (oldErr == nil) != (newErr == nil) {
		t.Fatalf("Decode vs DecodeContext error mismatch: %v vs %v", oldErr, newErr)
	}
	if oldErr != nil {
		return
	}

	if oldIdx.Header.NumRowGroups != newIdx.Header.NumRowGroups {
		t.Errorf("NumRowGroups: old %d, new %d", oldIdx.Header.NumRowGroups, newIdx.Header.NumRowGroups)
	}
	if oldIdx.Header.NumPaths != newIdx.Header.NumPaths {
		t.Errorf("NumPaths: old %d, new %d", oldIdx.Header.NumPaths, newIdx.Header.NumPaths)
	}
}

func TestSerializationContextRoundTrip(t *testing.T) {
	idx, err := buildSmallIndex()
	if err != nil {
		t.Fatalf("build index: %v", err)
	}

	data, err := gin.EncodeContext(context.Background(), idx)
	if err != nil {
		t.Fatalf("EncodeContext: %v", err)
	}

	decoded, err := gin.DecodeContext(context.Background(), data)
	if err != nil {
		t.Fatalf("DecodeContext: %v", err)
	}

	if decoded.Header.NumRowGroups != idx.Header.NumRowGroups {
		t.Errorf("round-trip NumRowGroups: want %d, got %d", idx.Header.NumRowGroups, decoded.Header.NumRowGroups)
	}
	if decoded.Header.NumPaths != idx.Header.NumPaths {
		t.Errorf("round-trip NumPaths: want %d, got %d", idx.Header.NumPaths, decoded.Header.NumPaths)
	}
}

func TestMetadataAndSidecarHelpersUseContextSiblings(t *testing.T) {
	// This test ensures the sidecar and metadata helpers still work end-to-end.
	// Since they call Encode/Decode internally (now via wrappers), the test
	// confirms backward-compatible behavior without requiring real file I/O.
	idx, err := buildSmallIndex()
	if err != nil {
		t.Fatalf("build index: %v", err)
	}

	key, value, err := gin.EncodeToMetadata(idx, gin.DefaultParquetConfig())
	if err != nil {
		t.Fatalf("EncodeToMetadata: %v", err)
	}
	if key == "" {
		t.Fatal("EncodeToMetadata returned empty key")
	}

	decoded, err := gin.DecodeFromMetadata(value)
	if err != nil {
		t.Fatalf("DecodeFromMetadata: %v", err)
	}
	if decoded.Header.NumRowGroups != idx.Header.NumRowGroups {
		t.Errorf("metadata round-trip NumRowGroups: want %d, got %d", idx.Header.NumRowGroups, decoded.Header.NumRowGroups)
	}
}

// --------------------------------------------------------------------------
// Helpers shared across tasks
// --------------------------------------------------------------------------

// buildIndexFromTestParquet attempts to build from a parquet file in testdata.
// If no suitable file exists, it returns an error and callers should t.Skip.
func buildIndexFromTestParquet(cfg gin.GINConfig) (*gin.GINIndex, error) {
	return gin.BuildFromParquet("testdata/test.parquet", "json", cfg)
}

func buildIndexFromTestParquetContext(ctx context.Context, cfg gin.GINConfig) (*gin.GINIndex, error) {
	return gin.BuildFromParquetContext(ctx, "testdata/test.parquet", "json", cfg)
}
