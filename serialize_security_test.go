package gin

import (
	"bytes"
	"encoding/binary"
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

func TestDecodeBoundsRGSet(t *testing.T) {
	var buf bytes.Buffer
	// numRGs = 1
	binary.Write(&buf, binary.LittleEndian, uint32(1))
	// dataLen = maxRGSetSize + 1 (exceeds limit)
	binary.Write(&buf, binary.LittleEndian, uint32(maxRGSetSize+1))

	_, err := readRGSet(&buf)
	if err == nil {
		t.Fatal("expected error for oversized RGSet, got nil")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Errorf("expected ErrInvalidFormat, got: %v", err)
	}
}

func TestDecodeBoundsPathDirectory(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)
	builder.AddDocument(0, []byte(`{"name": "alice"}`))
	idx := builder.Finalize()

	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	// Header layout in uncompressed format:
	// [0:4] "GINu" outer magic
	// [4:8] "GIN\x01" inner magic
	// [8:10] version uint16
	// [10:12] flags uint16
	// [12:16] NumRowGroups uint32
	// [16:24] NumDocs uint64
	// [24:28] NumPaths uint32
	// Set NumPaths to 0xFFFFFFFF (>65535)
	binary.LittleEndian.PutUint32(data[24:28], 0xFFFFFFFF)

	_, err = Decode(data)
	if err == nil {
		t.Fatal("expected error for oversized NumPaths, got nil")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Errorf("expected ErrInvalidFormat, got: %v", err)
	}
}

func TestDecodeBoundsStringIndexes(t *testing.T) {
	var buf bytes.Buffer
	// numPaths = 1
	binary.Write(&buf, binary.LittleEndian, uint32(1))
	// pathID = 0
	binary.Write(&buf, binary.LittleEndian, uint16(0))
	// numTerms = maxTermsPerPath + 1
	binary.Write(&buf, binary.LittleEndian, uint32(maxTermsPerPath+1))

	idx := NewGINIndex()
	err := readStringIndexes(&buf, idx)
	if err == nil {
		t.Fatal("expected error for oversized numTerms, got nil")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Errorf("expected ErrInvalidFormat, got: %v", err)
	}
}

func TestDecodeBoundsTrigramIndexes(t *testing.T) {
	var buf bytes.Buffer
	// numPaths = 1
	binary.Write(&buf, binary.LittleEndian, uint32(1))
	// pathID = 0
	binary.Write(&buf, binary.LittleEndian, uint16(0))
	// numRGs = 5
	binary.Write(&buf, binary.LittleEndian, uint32(5))
	// n = 3
	binary.Write(&buf, binary.LittleEndian, uint8(3))
	// padLen = 0
	binary.Write(&buf, binary.LittleEndian, uint8(0))
	// numTrigrams = maxTrigramsPerPath + 1
	binary.Write(&buf, binary.LittleEndian, uint32(maxTrigramsPerPath+1))

	idx := NewGINIndex()
	err := readTrigramIndexes(&buf, idx)
	if err == nil {
		t.Fatal("expected error for oversized numTrigrams, got nil")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Errorf("expected ErrInvalidFormat, got: %v", err)
	}
}

func TestDecodeBoundsDocIDMapping(t *testing.T) {
	var buf bytes.Buffer
	// numDocs = maxDocs + 1 where maxDocs = 10
	var maxDocs uint64 = 10
	binary.Write(&buf, binary.LittleEndian, maxDocs+1)

	_, err := readDocIDMapping(&buf, maxDocs)
	if err == nil {
		t.Fatal("expected error for oversized docid mapping, got nil")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Errorf("expected ErrInvalidFormat, got: %v", err)
	}
}

func TestDecodeBoundsBloomFilter(t *testing.T) {
	var buf bytes.Buffer
	// numBits = 1024
	binary.Write(&buf, binary.LittleEndian, uint32(1024))
	// numHashes = 5
	binary.Write(&buf, binary.LittleEndian, uint8(5))
	// numWords = maxBloomWords + 1
	binary.Write(&buf, binary.LittleEndian, uint32(maxBloomWords+1))

	_, err := readBloomFilter(&buf)
	if err == nil {
		t.Fatal("expected error for oversized bloom filter, got nil")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Errorf("expected ErrInvalidFormat, got: %v", err)
	}
}

func TestDecodeBoundsHLLRegisters(t *testing.T) {
	var buf bytes.Buffer
	// numPaths = 1
	binary.Write(&buf, binary.LittleEndian, uint32(1))
	// pathID = 0
	binary.Write(&buf, binary.LittleEndian, uint16(0))
	// precision = 12
	binary.Write(&buf, binary.LittleEndian, uint8(12))
	// numRegisters = maxHLLRegisters + 1
	binary.Write(&buf, binary.LittleEndian, uint32(maxHLLRegisters+1))

	idx := NewGINIndex()
	err := readHyperLogLogs(&buf, idx)
	if err == nil {
		t.Fatal("expected error for oversized HLL registers, got nil")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Errorf("expected ErrInvalidFormat, got: %v", err)
	}
}

func TestDecodeBoundsNumericRGs(t *testing.T) {
	var buf bytes.Buffer
	// numPaths = 1
	binary.Write(&buf, binary.LittleEndian, uint32(1))
	// pathID = 0
	binary.Write(&buf, binary.LittleEndian, uint16(0))
	// valueType = TypeInt
	binary.Write(&buf, binary.LittleEndian, uint8(TypeInt))
	// globalMin
	binary.Write(&buf, binary.LittleEndian, uint64(0))
	// globalMax
	binary.Write(&buf, binary.LittleEndian, uint64(0))
	// numRGs = maxRGs + 1 where maxRGs = 10
	binary.Write(&buf, binary.LittleEndian, uint32(11))

	idx := NewGINIndex()
	err := readNumericIndexes(&buf, idx, 10)
	if err == nil {
		t.Fatal("expected error for oversized numeric RGs, got nil")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Errorf("expected ErrInvalidFormat, got: %v", err)
	}
}

func TestDecodeBoundsStringLengthRGs(t *testing.T) {
	var buf bytes.Buffer
	// numPaths = 1
	binary.Write(&buf, binary.LittleEndian, uint32(1))
	// pathID = 0
	binary.Write(&buf, binary.LittleEndian, uint16(0))
	// globalMin
	binary.Write(&buf, binary.LittleEndian, uint32(0))
	// globalMax
	binary.Write(&buf, binary.LittleEndian, uint32(100))
	// numRGs = maxRGs + 1 where maxRGs = 10
	binary.Write(&buf, binary.LittleEndian, uint32(11))

	idx := NewGINIndex()
	err := readStringLengthIndexes(&buf, idx, 10)
	if err == nil {
		t.Fatal("expected error for oversized string length RGs, got nil")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Errorf("expected ErrInvalidFormat, got: %v", err)
	}
}

func TestDecodeCraftedPayload(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)
	builder.AddDocument(0, []byte(`{"name": "alice"}`))
	idx := builder.Finalize()

	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	// Set NumPaths to max uint32 in header
	binary.LittleEndian.PutUint32(data[24:28], 0xFFFFFFFF)

	_, err = Decode(data)
	if err == nil {
		t.Fatal("expected error for crafted payload, got nil")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Errorf("expected ErrInvalidFormat, got: %v", err)
	}
}
