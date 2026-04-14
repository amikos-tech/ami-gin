package gin

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	stderrors "errors"
	"io"
	"strings"
	"testing"

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

	_, err := readRGSet(&buf, 100)
	if err == nil {
		t.Fatal("expected error for oversized RGSet, got nil")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Errorf("expected ErrInvalidFormat, got: %v", err)
	}
}

func TestDecodeBoundsRGSetNumRGs(t *testing.T) {
	var buf bytes.Buffer
	// numRGs = maxRGs + 1 (exceeds header-derived limit)
	binary.Write(&buf, binary.LittleEndian, uint32(101))

	_, err := readRGSet(&buf, 100)
	if err == nil {
		t.Fatal("expected error for oversized numRGs, got nil")
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

func TestDecodeRejectsOutOfOrderPathDirectoryIDs(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)
	builder.AddDocument(0, []byte(`{"foo": "x", "bar": "y"}`))
	builder.AddDocument(1, []byte(`{"foo": "z", "bar": "w"}`))
	idx := builder.Finalize()

	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	body := data[4:]
	reader := bytes.NewReader(body)
	headerOnly := NewGINIndex()
	if err := readHeader(reader, headerOnly); err != nil {
		t.Fatalf("readHeader() error = %v", err)
	}

	pathIDOffsets := make([]int, 0, headerOnly.Header.NumPaths)
	for i := uint32(0); i < headerOnly.Header.NumPaths; i++ {
		pathIDOffsets = append(pathIDOffsets, len(body)-reader.Len())

		var pathID uint16
		if err := binary.Read(reader, binary.LittleEndian, &pathID); err != nil {
			t.Fatalf("read pathID %d: %v", i, err)
		}

		var pathLen uint16
		if err := binary.Read(reader, binary.LittleEndian, &pathLen); err != nil {
			t.Fatalf("read pathLen %d: %v", i, err)
		}
		if _, err := reader.Seek(int64(pathLen)+1+4+1, io.SeekCurrent); err != nil {
			t.Fatalf("seek path entry %d: %v", i, err)
		}
	}

	if len(pathIDOffsets) < 2 {
		t.Fatalf("need at least 2 path entries, got %d", len(pathIDOffsets))
	}

	firstID := binary.LittleEndian.Uint16(body[pathIDOffsets[0] : pathIDOffsets[0]+2])
	secondID := binary.LittleEndian.Uint16(body[pathIDOffsets[1] : pathIDOffsets[1]+2])
	binary.LittleEndian.PutUint16(body[pathIDOffsets[0]:pathIDOffsets[0]+2], secondID)
	binary.LittleEndian.PutUint16(body[pathIDOffsets[1]:pathIDOffsets[1]+2], firstID)

	_, err = Decode(data)
	if err == nil {
		t.Fatal("expected error for out-of-order path ids, got nil")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestDecodeRejectsOutOfRangeNumericIndexPathID(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 1)
	builder.AddDocument(0, []byte(`{"age": 30}`))
	idx := builder.Finalize()

	var numeric *NumericIndex
	for pathID, ni := range idx.NumericIndexes {
		numeric = ni
		delete(idx.NumericIndexes, pathID)
		break
	}
	if numeric == nil {
		t.Fatal("expected numeric index")
	}
	idx.NumericIndexes[outOfRangePathID(t, idx)] = numeric

	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	_, err = Decode(data)
	if err == nil {
		t.Fatal("expected error for out-of-range numeric index path id, got nil")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestValidatePathReferencesRejectsOutOfRangeIDsForAllIndexKinds(t *testing.T) {
	t.Run("all index maps", func(t *testing.T) {
		numRGs := 1
		cases := []struct {
			name string
			kind string
			add  func(t *testing.T, idx *GINIndex)
		}{
			{
				name: "string index",
				kind: "string index",
				add: func(t *testing.T, idx *GINIndex) {
					idx.StringIndexes[outOfRangePathID(t, idx)] = &StringIndex{}
				},
			},
			{
				name: "string length index",
				kind: "string length index",
				add: func(t *testing.T, idx *GINIndex) {
					idx.StringLengthIndexes[outOfRangePathID(t, idx)] = &StringLengthIndex{}
				},
			},
			{
				name: "numeric index",
				kind: "numeric index",
				add: func(t *testing.T, idx *GINIndex) {
					idx.NumericIndexes[outOfRangePathID(t, idx)] = &NumericIndex{}
				},
			},
			{
				name: "null index",
				kind: "null index",
				add: func(t *testing.T, idx *GINIndex) {
					idx.NullIndexes[outOfRangePathID(t, idx)] = &NullIndex{
						NullRGBitmap:    MustNewRGSet(numRGs),
						PresentRGBitmap: MustNewRGSet(numRGs),
					}
				},
			},
			{
				name: "trigram index",
				kind: "trigram index",
				add: func(t *testing.T, idx *GINIndex) {
					idx.TrigramIndexes[outOfRangePathID(t, idx)] = &TrigramIndex{
						Trigrams:  make(map[string]*RGSet),
						NumRGs:    numRGs,
						N:         3,
						MinLength: 3,
					}
				},
			},
			{
				name: "path cardinality",
				kind: "path cardinality",
				add: func(t *testing.T, idx *GINIndex) {
					idx.PathCardinality[outOfRangePathID(t, idx)] = MustNewHyperLogLog(4)
				},
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				idx := NewGINIndex()
				idx.PathDirectory = []PathEntry{{PathID: 0, PathName: "$"}}
				tc.add(t, idx)

				err := idx.validatePathReferences()
				if err == nil {
					t.Fatalf("validatePathReferences() error = nil, want ErrInvalidFormat for %s", tc.kind)
				}
				if !stderrors.Is(err, ErrInvalidFormat) {
					t.Fatalf("validatePathReferences() error = %v, want ErrInvalidFormat for %s", err, tc.kind)
				}
				if !strings.Contains(err.Error(), tc.kind) {
					t.Fatalf("validatePathReferences() error = %v, want kind %q in error", err, tc.kind)
				}
			})
		}
	})
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

func TestReadConfigRejectsCanonicalFTSPathCollision(t *testing.T) {
	sc := SerializedConfig{
		FTSPaths: []string{"$.foo", "$['foo']"},
	}

	data, err := json.Marshal(sc)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(data))); err != nil {
		t.Fatalf("binary.Write() error = %v", err)
	}
	if _, err := buf.Write(data); err != nil {
		t.Fatalf("buf.Write() error = %v", err)
	}

	_, err = readConfig(&buf)
	if err == nil {
		t.Fatal("expected canonical FTS collision error, got nil")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestReadConfigRejectsCanonicalTransformerPathCollision(t *testing.T) {
	sc := SerializedConfig{
		Transformers: []TransformerSpec{
			NewTransformerSpec("$.foo", TransformerToLower, nil),
			NewTransformerSpec("$['foo']", TransformerEmailDomain, nil),
		},
	}

	data, err := json.Marshal(sc)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(data))); err != nil {
		t.Fatalf("binary.Write() error = %v", err)
	}
	if _, err := buf.Write(data); err != nil {
		t.Fatalf("buf.Write() error = %v", err)
	}

	_, err = readConfig(&buf)
	if err == nil {
		t.Fatal("expected canonical transformer collision error, got nil")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat, got %v", err)
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
	binary.Write(&buf, binary.LittleEndian, TypeInt)
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

func TestDecodeCraftedInnerMagic(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)
	builder.AddDocument(0, []byte(`{"name": "alice"}`))
	idx := builder.Finalize()

	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	// Header layout: [4:8] is inner magic "GIN\x01"
	// Corrupt inner magic to trigger ErrInvalidFormat in readHeader
	copy(data[4:8], []byte("XXXX"))

	_, err = Decode(data)
	if err == nil {
		t.Fatal("expected error for corrupted inner magic, got nil")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Errorf("expected ErrInvalidFormat, got: %v", err)
	}
}
