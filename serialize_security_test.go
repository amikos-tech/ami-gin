package gin

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	stderrors "errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/pkg/errors"
)

func buildAdaptiveSerializationFixture(t *testing.T, config GINConfig) *GINIndex {
	t.Helper()

	builder := mustNewBuilder(t, config, 6)
	docs := []struct {
		rgID int
		json string
	}{
		{rgID: 0, json: `{"field":"hot","other":"tail_0"}`},
		{rgID: 1, json: `{"field":"hot","other":"tail_1"}`},
		{rgID: 2, json: `{"field":"hot","other":"tail_2"}`},
		{rgID: 3, json: `{"field":"tail_3"}`},
		{rgID: 4, json: `{"field":"tail_4"}`},
		{rgID: 5, json: `{"field":"tail_5"}`},
	}

	for _, doc := range docs {
		if err := builder.AddDocument(DocID(doc.rgID), []byte(doc.json)); err != nil {
			t.Fatalf("AddDocument(rg=%d) failed: %v", doc.rgID, err)
		}
	}

	return builder.Finalize()
}

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

func TestAdaptiveConfigRoundTrip(t *testing.T) {
	config := DefaultConfig()
	config.CardinalityThreshold = 3
	config.AdaptiveMinRGCoverage = 3
	config.AdaptivePromotedTermCap = 11
	config.AdaptiveCoverageCeiling = 0.75
	config.AdaptiveBucketCount = 16

	idx := buildAdaptiveSerializationFixture(t, config)

	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel() error = %v", err)
	}

	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if decoded.Config == nil {
		t.Fatal("decoded Config = nil, want adaptive config")
	}

	if decoded.Config.AdaptiveMinRGCoverage != config.AdaptiveMinRGCoverage {
		t.Fatalf("AdaptiveMinRGCoverage = %d, want %d", decoded.Config.AdaptiveMinRGCoverage, config.AdaptiveMinRGCoverage)
	}
	if decoded.Config.AdaptivePromotedTermCap != config.AdaptivePromotedTermCap {
		t.Fatalf("AdaptivePromotedTermCap = %d, want %d", decoded.Config.AdaptivePromotedTermCap, config.AdaptivePromotedTermCap)
	}
	if decoded.Config.AdaptiveCoverageCeiling != config.AdaptiveCoverageCeiling {
		t.Fatalf("AdaptiveCoverageCeiling = %v, want %v", decoded.Config.AdaptiveCoverageCeiling, config.AdaptiveCoverageCeiling)
	}
	if decoded.Config.AdaptiveBucketCount != config.AdaptiveBucketCount {
		t.Fatalf("AdaptiveBucketCount = %d, want %d", decoded.Config.AdaptiveBucketCount, config.AdaptiveBucketCount)
	}
}

func TestAdaptivePathMetadataRoundTrip(t *testing.T) {
	config := DefaultConfig()
	config.CardinalityThreshold = 3
	config.AdaptiveMinRGCoverage = 2
	config.AdaptivePromotedTermCap = 8
	config.AdaptiveCoverageCeiling = 0.75
	config.AdaptiveBucketCount = 16

	idx := buildAdaptiveSerializationFixture(t, config)
	originalEntry := findPathEntry(idx, "$.field")
	if originalEntry == nil {
		t.Fatal("adaptive path entry not found")
	}
	if originalEntry.Flags&FlagAdaptiveHybrid == 0 {
		t.Fatalf("path flags = %08b, want adaptive hybrid", originalEntry.Flags)
	}
	originalAdaptive := idx.AdaptiveStringIndexes[originalEntry.PathID]
	if originalAdaptive == nil {
		t.Fatal("adaptive string index missing before encode")
	}

	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel() error = %v", err)
	}

	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	decodedEntry := findPathEntry(decoded, "$.field")
	if decodedEntry == nil {
		t.Fatal("decoded adaptive path entry not found")
	}
	if decodedEntry.Flags&FlagAdaptiveHybrid == 0 {
		t.Fatalf("decoded path flags = %08b, want adaptive hybrid", decodedEntry.Flags)
	}
	if decodedEntry.AdaptivePromotedTerms != originalEntry.AdaptivePromotedTerms {
		t.Fatalf("AdaptivePromotedTerms = %d, want %d", decodedEntry.AdaptivePromotedTerms, originalEntry.AdaptivePromotedTerms)
	}
	if decodedEntry.AdaptiveBucketCount != originalEntry.AdaptiveBucketCount {
		t.Fatalf("AdaptiveBucketCount = %d, want %d", decodedEntry.AdaptiveBucketCount, originalEntry.AdaptiveBucketCount)
	}

	decodedAdaptive := decoded.AdaptiveStringIndexes[decodedEntry.PathID]
	if decodedAdaptive == nil {
		t.Fatal("adaptive string index missing after decode")
	}
	if decodedAdaptive.BucketCount != originalAdaptive.BucketCount {
		t.Fatalf("BucketCount = %d, want %d", decodedAdaptive.BucketCount, originalAdaptive.BucketCount)
	}
	if !reflect.DeepEqual(decodedAdaptive.Terms, originalAdaptive.Terms) {
		t.Fatalf("Terms = %v, want %v", decodedAdaptive.Terms, originalAdaptive.Terms)
	}
	if len(decodedAdaptive.RGBitmaps) != len(originalAdaptive.RGBitmaps) {
		t.Fatalf("RGBitmaps len = %d, want %d", len(decodedAdaptive.RGBitmaps), len(originalAdaptive.RGBitmaps))
	}
	for i := range originalAdaptive.RGBitmaps {
		if got, want := decodedAdaptive.RGBitmaps[i].ToSlice(), originalAdaptive.RGBitmaps[i].ToSlice(); !reflect.DeepEqual(got, want) {
			t.Fatalf("RGBitmaps[%d] = %v, want %v", i, got, want)
		}
	}
	if len(decodedAdaptive.BucketRGBitmaps) != len(originalAdaptive.BucketRGBitmaps) {
		t.Fatalf("BucketRGBitmaps len = %d, want %d", len(decodedAdaptive.BucketRGBitmaps), len(originalAdaptive.BucketRGBitmaps))
	}
	for i := range originalAdaptive.BucketRGBitmaps {
		if got, want := decodedAdaptive.BucketRGBitmaps[i].ToSlice(), originalAdaptive.BucketRGBitmaps[i].ToSlice(); !reflect.DeepEqual(got, want) {
			t.Fatalf("BucketRGBitmaps[%d] = %v, want %v", i, got, want)
		}
	}
}

func TestDecodeRejectsOversizedAdaptiveBucketSection(t *testing.T) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, uint32(1)); err != nil {
		t.Fatalf("binary.Write(numPaths) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint16(0)); err != nil {
		t.Fatalf("binary.Write(pathID) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(0)); err != nil {
		t.Fatalf("binary.Write(numPromotedTerms) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(maxAdaptiveBucketsPerPath+1)); err != nil {
		t.Fatalf("binary.Write(bucketCount) error = %v", err)
	}

	idx := NewGINIndex()
	idx.Header.NumRowGroups = 10
	idx.PathDirectory = []PathEntry{{
		PathID: 0,
		Flags:  FlagAdaptiveHybrid,
	}}

	err := readAdaptiveStringIndexes(&buf, idx)
	if err == nil {
		t.Fatal("expected error for oversized adaptive bucket section, got nil")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat, got %v", err)
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
	// valueType = int-only
	binary.Write(&buf, binary.LittleEndian, uint8(0))
	// intGlobalMin
	binary.Write(&buf, binary.LittleEndian, int64(0))
	// intGlobalMax
	binary.Write(&buf, binary.LittleEndian, int64(0))
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
