package gin

import (
	"testing"

	stderrors "errors"

	"github.com/pkg/errors"
)

func TestDecodeVersionMismatch(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)
	builder.AddDocument(0, []byte(`{"name": "alice", "age": 30}`))
	builder.AddDocument(1, []byte(`{"name": "bob", "age": 25}`))
	idx := builder.Finalize()

	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	// Uncompressed layout: [0:4] "GINu" + [4:8] "GIN\x01" + [8:10] version uint16 LE
	// Set version to 99
	data[8] = 99
	data[9] = 0

	_, err = Decode(data)
	if err == nil {
		t.Fatal("expected error for version mismatch, got nil")
	}
	if !stderrors.Is(err, ErrVersionMismatch) {
		t.Errorf("expected ErrVersionMismatch, got: %v", err)
	}

	// Also test version 0
	data[8] = 0
	data[9] = 0
	_, err = Decode(data)
	if err == nil {
		t.Fatal("expected error for version 0, got nil")
	}
	if !stderrors.Is(err, ErrVersionMismatch) {
		t.Errorf("expected ErrVersionMismatch for version 0, got: %v", err)
	}
}

func TestDecodeLegacyRejected(t *testing.T) {
	// Create data that doesn't start with recognized magic bytes.
	// This would have previously been handled by the legacy zstd fallback.
	badMagic := []byte("XXXX" + "some payload data here")

	_, err := Decode(badMagic)
	if err == nil {
		t.Fatal("expected error for unrecognized magic, got nil")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Errorf("expected ErrInvalidFormat, got: %v", err)
	}
}

func TestSentinelErrors(t *testing.T) {
	// Verify errors.Is works through errors.Wrapf wrapping
	wrapped := errors.Wrapf(ErrVersionMismatch, "got version %d, expected %d", 99, 3)
	if !stderrors.Is(wrapped, ErrVersionMismatch) {
		t.Error("errors.Is failed for wrapped ErrVersionMismatch")
	}

	wrapped = errors.Wrapf(ErrInvalidFormat, "unrecognized magic bytes: %q", "XXXX")
	if !stderrors.Is(wrapped, ErrInvalidFormat) {
		t.Error("errors.Is failed for wrapped ErrInvalidFormat")
	}

	// Verify double wrapping still works
	doubleWrapped := errors.Wrap(wrapped, "decode")
	if !stderrors.Is(doubleWrapped, ErrInvalidFormat) {
		t.Error("errors.Is failed for double-wrapped ErrInvalidFormat")
	}
}

func TestDecodeRoundTripRegression(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)
	builder.AddDocument(0, []byte(`{"name": "alice", "age": 30, "active": true}`))
	builder.AddDocument(1, []byte(`{"name": "bob", "age": 25, "active": false}`))
	builder.AddDocument(2, []byte(`{"name": "charlie", "age": 35}`))
	idx := builder.Finalize()

	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if decoded.Header.NumDocs != idx.Header.NumDocs {
		t.Errorf("NumDocs mismatch: %d vs %d", decoded.Header.NumDocs, idx.Header.NumDocs)
	}
	if decoded.Header.NumRowGroups != idx.Header.NumRowGroups {
		t.Errorf("NumRowGroups mismatch: %d vs %d", decoded.Header.NumRowGroups, idx.Header.NumRowGroups)
	}

	result := decoded.Evaluate([]Predicate{EQ("$.name", "alice")})
	if !result.IsSet(0) {
		t.Error("query on decoded index should find alice in RG 0")
	}
}
