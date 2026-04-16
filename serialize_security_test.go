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

	"github.com/klauspost/compress/zstd"
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

func legacyV5HeaderOnlyPayload(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	if _, err := buf.WriteString(uncompressedMagic); err != nil {
		t.Fatalf("WriteString(uncompressedMagic) error = %v", err)
	}
	if _, err := buf.WriteString(MagicBytes); err != nil {
		t.Fatalf("WriteString(MagicBytes) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint16(5)); err != nil {
		t.Fatalf("binary.Write(version) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint16(0)); err != nil {
		t.Fatalf("binary.Write(flags) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(0)); err != nil {
		t.Fatalf("binary.Write(numRowGroups) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint64(0)); err != nil {
		t.Fatalf("binary.Write(numDocs) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(0)); err != nil {
		t.Fatalf("binary.Write(numPaths) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(0)); err != nil {
		t.Fatalf("binary.Write(cardinalityThreshold) error = %v", err)
	}
	return buf.Bytes()
}

func mustAdaptiveIndex(t *testing.T, terms []string, rgBitmaps []*RGSet, bucketBitmaps []*RGSet) *AdaptiveStringIndex {
	t.Helper()

	adaptive, err := NewAdaptiveStringIndex(terms, rgBitmaps, bucketBitmaps)
	if err != nil {
		t.Fatalf("NewAdaptiveStringIndex() error = %v", err)
	}
	return adaptive
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

func TestDecodeRejectsUnknownPathMode(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 1)
	builder.AddDocument(0, []byte(`{"x": "alice"}`))
	idx := builder.Finalize()

	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	// Header layout: 4 (uncompressedMagic) + 4 (MagicBytes) + 2 (Version) + 2 (Flags) +
	// 4 (NumRowGroups) + 8 (NumDocs) + 4 (NumPaths) + 4 (CardinalityThreshold) = 32.
	// First PathEntry: 2 (PathID) + 2 (pathLen) + pathLen (pathBytes "$.x"=3) +
	// 1 (ObservedTypes) + 4 (Cardinality) = 12. Mode byte lives at 32+12 = 44.
	modeOffset := 32 + 2 + 2 + len("$.x") + 1 + 4
	data[modeOffset] = 99

	if _, err := Decode(data); err == nil {
		t.Fatal("Decode() error = nil, want ErrInvalidFormat for unknown path mode")
	} else if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestDecodeRejectsLegacyV5PayloadAfterWireFormatChange(t *testing.T) {
	data := legacyV5HeaderOnlyPayload(t)

	_, err := Decode(data)
	if err == nil {
		t.Fatal("Decode() error = nil, want ErrVersionMismatch for legacy v5 payload")
	}
	if !stderrors.Is(err, ErrVersionMismatch) {
		t.Fatalf("expected ErrVersionMismatch, got %v", err)
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
	if originalEntry.Mode != PathModeAdaptiveHybrid {
		t.Fatalf("Mode = %v, want adaptive hybrid", originalEntry.Mode)
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
	if decodedEntry.Mode != PathModeAdaptiveHybrid {
		t.Fatalf("decoded Mode = %v, want adaptive hybrid", decodedEntry.Mode)
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
	if len(decodedAdaptive.BucketRGBitmaps) != len(originalAdaptive.BucketRGBitmaps) {
		t.Fatalf("len(BucketRGBitmaps) = %d, want %d", len(decodedAdaptive.BucketRGBitmaps), len(originalAdaptive.BucketRGBitmaps))
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
	for i := range originalAdaptive.BucketRGBitmaps {
		if got, want := decodedAdaptive.BucketRGBitmaps[i].ToSlice(), originalAdaptive.BucketRGBitmaps[i].ToSlice(); !reflect.DeepEqual(got, want) {
			t.Fatalf("BucketRGBitmaps[%d] = %v, want %v", i, got, want)
		}
	}
}

func TestAdaptiveRoundTripPreservesQueryResults(t *testing.T) {
	config := DefaultConfig()
	config.CardinalityThreshold = 3
	config.AdaptiveMinRGCoverage = 2
	config.AdaptivePromotedTermCap = 8
	config.AdaptiveCoverageCeiling = 0.75
	config.AdaptiveBucketCount = 16

	original := buildAdaptiveSerializationFixture(t, config)
	data, err := EncodeWithLevel(original, CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel() error = %v", err)
	}

	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	tests := []struct {
		name string
		pred Predicate
	}{
		{name: "eq promoted", pred: EQ("$.field", "hot")},
		{name: "eq tail", pred: EQ("$.field", "tail_4")},
		{name: "ne promoted", pred: NE("$.field", "hot")},
		{name: "ne tail", pred: NE("$.field", "tail_4")},
		{name: "in mixed", pred: IN("$.field", "hot", "tail_4")},
		{name: "nin mixed", pred: NIN("$.field", "hot", "tail_4")},
		{name: "contains", pred: Contains("$.field", "tail")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := original.Evaluate([]Predicate{tt.pred})
			after := decoded.Evaluate([]Predicate{tt.pred})
			if got, want := after.ToSlice(), before.ToSlice(); !reflect.DeepEqual(got, want) {
				t.Fatalf("decoded Evaluate(%s) = %v, want %v", tt.name, got, want)
			}
		})
	}
}

func TestEncodeRejectsInvalidAdaptivePathReference(t *testing.T) {
	idx := NewGINIndex()
	idx.Header.NumRowGroups = 1
	idx.Header.NumPaths = 1
	idx.GlobalBloom = MustNewBloomFilter(64, 3)
	idx.PathDirectory = []PathEntry{{
		PathID:   0,
		PathName: "$.field",
		Mode:     PathModeAdaptiveHybrid,
	}}

	_, err := EncodeWithLevel(idx, CompressionNone)
	if err == nil {
		t.Fatal("EncodeWithLevel() error = nil, want invalid adaptive path reference rejection")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat, got %v", err)
	}
	if !strings.Contains(err.Error(), "adaptive path 0 missing adaptive section") {
		t.Fatalf("EncodeWithLevel() error = %v, want missing adaptive section context", err)
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
		Mode:   PathModeAdaptiveHybrid,
	}}

	err := readAdaptiveStringIndexes(&buf, idx)
	if err == nil {
		t.Fatal("expected error for oversized adaptive bucket section, got nil")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestDecodeRejectsInvalidAdaptiveSections(t *testing.T) {
	tests := []struct {
		name         string
		build        func(*testing.T) (*bytes.Buffer, *GINIndex)
		wantContains string
	}{
		{
			name: "bucket count zero",
			build: func(t *testing.T) (*bytes.Buffer, *GINIndex) {
				t.Helper()
				var buf bytes.Buffer
				if err := binary.Write(&buf, binary.LittleEndian, uint32(1)); err != nil {
					t.Fatalf("binary.Write(numPaths) error = %v", err)
				}
				if err := binary.Write(&buf, binary.LittleEndian, uint16(0)); err != nil {
					t.Fatalf("binary.Write(pathID) error = %v", err)
				}
				if err := binary.Write(&buf, binary.LittleEndian, uint32(0)); err != nil {
					t.Fatalf("binary.Write(numTerms) error = %v", err)
				}
				if err := binary.Write(&buf, binary.LittleEndian, uint32(0)); err != nil {
					t.Fatalf("binary.Write(bucketCount) error = %v", err)
				}
				idx := NewGINIndex()
				idx.Header.NumRowGroups = 10
				idx.PathDirectory = []PathEntry{{PathID: 0, Mode: PathModeAdaptiveHybrid}}
				return &buf, idx
			},
			wantContains: "must be greater than 0",
		},
		{
			name: "bucket count not power of two",
			build: func(t *testing.T) (*bytes.Buffer, *GINIndex) {
				t.Helper()
				var buf bytes.Buffer
				if err := binary.Write(&buf, binary.LittleEndian, uint32(1)); err != nil {
					t.Fatalf("binary.Write(numPaths) error = %v", err)
				}
				if err := binary.Write(&buf, binary.LittleEndian, uint16(0)); err != nil {
					t.Fatalf("binary.Write(pathID) error = %v", err)
				}
				if err := binary.Write(&buf, binary.LittleEndian, uint32(0)); err != nil {
					t.Fatalf("binary.Write(numTerms) error = %v", err)
				}
				if err := binary.Write(&buf, binary.LittleEndian, uint32(3)); err != nil {
					t.Fatalf("binary.Write(bucketCount) error = %v", err)
				}
				idx := NewGINIndex()
				idx.Header.NumRowGroups = 10
				idx.PathDirectory = []PathEntry{{PathID: 0, Mode: PathModeAdaptiveHybrid}}
				return &buf, idx
			},
			wantContains: "power of two",
		},
		{
			name: "adaptive section without adaptive mode",
			build: func(t *testing.T) (*bytes.Buffer, *GINIndex) {
				t.Helper()
				var buf bytes.Buffer
				if err := binary.Write(&buf, binary.LittleEndian, uint32(1)); err != nil {
					t.Fatalf("binary.Write(numPaths) error = %v", err)
				}
				if err := binary.Write(&buf, binary.LittleEndian, uint16(0)); err != nil {
					t.Fatalf("binary.Write(pathID) error = %v", err)
				}
				if err := binary.Write(&buf, binary.LittleEndian, uint32(0)); err != nil {
					t.Fatalf("binary.Write(numTerms) error = %v", err)
				}
				if err := binary.Write(&buf, binary.LittleEndian, uint32(2)); err != nil {
					t.Fatalf("binary.Write(bucketCount) error = %v", err)
				}
				for i := 0; i < 2; i++ {
					if err := writeRGSet(&buf, MustNewRGSet(4)); err != nil {
						t.Fatalf("writeRGSet(bucket=%d) error = %v", i, err)
					}
				}
				idx := NewGINIndex()
				idx.Header.NumRowGroups = 4
				idx.PathDirectory = []PathEntry{{PathID: 0, Mode: PathModeClassic}}
				return &buf, idx
			},
			wantContains: "missing adaptive mode",
		},
		{
			name: "duplicate path section",
			build: func(t *testing.T) (*bytes.Buffer, *GINIndex) {
				t.Helper()
				var buf bytes.Buffer
				if err := binary.Write(&buf, binary.LittleEndian, uint32(2)); err != nil {
					t.Fatalf("binary.Write(numPaths) error = %v", err)
				}
				for i := 0; i < 2; i++ {
					if err := binary.Write(&buf, binary.LittleEndian, uint16(0)); err != nil {
						t.Fatalf("binary.Write(pathID) error = %v", err)
					}
					if err := binary.Write(&buf, binary.LittleEndian, uint32(0)); err != nil {
						t.Fatalf("binary.Write(numTerms) error = %v", err)
					}
					if err := binary.Write(&buf, binary.LittleEndian, uint32(2)); err != nil {
						t.Fatalf("binary.Write(bucketCount) error = %v", err)
					}
					for bucketID := 0; bucketID < 2; bucketID++ {
						if err := writeRGSet(&buf, MustNewRGSet(4)); err != nil {
							t.Fatalf("writeRGSet(path=%d bucket=%d) error = %v", i, bucketID, err)
						}
					}
				}
				idx := NewGINIndex()
				idx.Header.NumRowGroups = 4
				idx.PathDirectory = []PathEntry{{PathID: 0, Mode: PathModeAdaptiveHybrid}}
				return &buf, idx
			},
			wantContains: "duplicate adaptive section",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, idx := tt.build(t)
			err := readAdaptiveStringIndexes(buf, idx)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !stderrors.Is(err, ErrInvalidFormat) {
				t.Fatalf("expected ErrInvalidFormat, got %v", err)
			}
			if !strings.Contains(err.Error(), tt.wantContains) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantContains)
			}
		})
	}
}

func TestDecodeAdaptiveSectionDerivesPathEntryCounts(t *testing.T) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, uint32(1)); err != nil {
		t.Fatalf("binary.Write(numPaths) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint16(0)); err != nil {
		t.Fatalf("binary.Write(pathID) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(1)); err != nil {
		t.Fatalf("binary.Write(numTerms) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(2)); err != nil {
		t.Fatalf("binary.Write(bucketCount) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint16(len("hot"))); err != nil {
		t.Fatalf("binary.Write(termLen) error = %v", err)
	}
	if _, err := buf.WriteString("hot"); err != nil {
		t.Fatalf("WriteString(term) error = %v", err)
	}
	if err := writeRGSet(&buf, MustNewRGSet(4)); err != nil {
		t.Fatalf("writeRGSet(promoted) error = %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := writeRGSet(&buf, MustNewRGSet(4)); err != nil {
			t.Fatalf("writeRGSet(bucket=%d) error = %v", i, err)
		}
	}

	idx := NewGINIndex()
	idx.Header.NumRowGroups = 4
	idx.PathDirectory = []PathEntry{{
		PathID:                0,
		PathName:              "$.field",
		Mode:                  PathModeAdaptiveHybrid,
		AdaptivePromotedTerms: 99,
		AdaptiveBucketCount:   99,
	}}

	err := readAdaptiveStringIndexes(&buf, idx)
	if err != nil {
		t.Fatalf("readAdaptiveStringIndexes() error = %v", err)
	}
	if idx.PathDirectory[0].AdaptivePromotedTerms != 1 {
		t.Fatalf("AdaptivePromotedTerms = %d, want 1", idx.PathDirectory[0].AdaptivePromotedTerms)
	}
	if idx.PathDirectory[0].AdaptiveBucketCount != 2 {
		t.Fatalf("AdaptiveBucketCount = %d, want 2", idx.PathDirectory[0].AdaptiveBucketCount)
	}
}

func TestDecodeRejectsOversizedAdaptiveTermSection(t *testing.T) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, uint32(1)); err != nil {
		t.Fatalf("binary.Write(numPaths) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint16(0)); err != nil {
		t.Fatalf("binary.Write(pathID) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(maxAdaptiveTermsPerPath+1)); err != nil {
		t.Fatalf("binary.Write(numTerms) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(2)); err != nil {
		t.Fatalf("binary.Write(bucketCount) error = %v", err)
	}

	idx := NewGINIndex()
	idx.Header.NumRowGroups = 1
	idx.PathDirectory = []PathEntry{{PathID: 0, Mode: PathModeAdaptiveHybrid}}

	err := readAdaptiveStringIndexes(&buf, idx)
	if err == nil {
		t.Fatal("expected error for oversized adaptive term section, got nil")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestDecodeRejectsAdaptivePathIDOutOfRange(t *testing.T) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, uint32(1)); err != nil {
		t.Fatalf("binary.Write(numPaths) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint16(1)); err != nil {
		t.Fatalf("binary.Write(pathID) error = %v", err)
	}

	idx := NewGINIndex()
	idx.Header.NumRowGroups = 1
	idx.PathDirectory = []PathEntry{{PathID: 0, Mode: PathModeAdaptiveHybrid}}

	err := readAdaptiveStringIndexes(&buf, idx)
	if err == nil {
		t.Fatal("expected out-of-range error, got nil")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestDecodeRejectsTruncatedAdaptiveTerm(t *testing.T) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, uint32(1)); err != nil {
		t.Fatalf("binary.Write(numPaths) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint16(0)); err != nil {
		t.Fatalf("binary.Write(pathID) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(1)); err != nil {
		t.Fatalf("binary.Write(numTerms) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(2)); err != nil {
		t.Fatalf("binary.Write(bucketCount) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint16(4)); err != nil {
		t.Fatalf("binary.Write(termLen) error = %v", err)
	}
	if _, err := buf.WriteString("abc"); err != nil {
		t.Fatalf("WriteString(term) error = %v", err)
	}

	idx := NewGINIndex()
	idx.Header.NumRowGroups = 1
	idx.PathDirectory = []PathEntry{{PathID: 0, Mode: PathModeAdaptiveHybrid}}

	err := readAdaptiveStringIndexes(&buf, idx)
	if err == nil {
		t.Fatal("expected truncation error, got nil")
	}
	if !stderrors.Is(err, io.ErrUnexpectedEOF) && !stderrors.Is(err, io.EOF) {
		t.Fatalf("expected EOF-class error, got %v", err)
	}
}

func TestDecodeRejectsDuplicatePathSectionsAcrossReaders(t *testing.T) {
	tests := []struct {
		name string
		run  func(*testing.T) error
	}{
		{
			name: "string indexes",
			run: func(t *testing.T) error {
				t.Helper()
				var buf bytes.Buffer
				binary.Write(&buf, binary.LittleEndian, uint32(2))
				for i := 0; i < 2; i++ {
					binary.Write(&buf, binary.LittleEndian, uint16(0))
					binary.Write(&buf, binary.LittleEndian, uint32(0))
				}
				return readStringIndexes(&buf, NewGINIndex())
			},
		},
		{
			name: "string length indexes",
			run: func(t *testing.T) error {
				t.Helper()
				var buf bytes.Buffer
				binary.Write(&buf, binary.LittleEndian, uint32(2))
				for i := 0; i < 2; i++ {
					binary.Write(&buf, binary.LittleEndian, uint16(0))
					binary.Write(&buf, binary.LittleEndian, uint32(0))
					binary.Write(&buf, binary.LittleEndian, uint32(0))
					binary.Write(&buf, binary.LittleEndian, uint32(0))
				}
				return readStringLengthIndexes(&buf, NewGINIndex(), 0)
			},
		},
		{
			name: "numeric indexes",
			run: func(t *testing.T) error {
				t.Helper()
				var buf bytes.Buffer
				binary.Write(&buf, binary.LittleEndian, uint32(2))
				for i := 0; i < 2; i++ {
					binary.Write(&buf, binary.LittleEndian, uint16(0))
					binary.Write(&buf, binary.LittleEndian, NumericValueTypeIntOnly)
					binary.Write(&buf, binary.LittleEndian, int64(0))
					binary.Write(&buf, binary.LittleEndian, int64(0))
					binary.Write(&buf, binary.LittleEndian, uint64(0))
					binary.Write(&buf, binary.LittleEndian, uint64(0))
					binary.Write(&buf, binary.LittleEndian, uint32(0))
				}
				return readNumericIndexes(&buf, NewGINIndex(), 0)
			},
		},
		{
			name: "null indexes",
			run: func(t *testing.T) error {
				t.Helper()
				var buf bytes.Buffer
				idx := NewGINIndex()
				idx.Header.NumRowGroups = 1
				binary.Write(&buf, binary.LittleEndian, uint32(2))
				for i := 0; i < 2; i++ {
					binary.Write(&buf, binary.LittleEndian, uint16(0))
					writeRGSet(&buf, MustNewRGSet(1))
					writeRGSet(&buf, MustNewRGSet(1))
				}
				return readNullIndexes(&buf, idx)
			},
		},
		{
			name: "trigram indexes",
			run: func(t *testing.T) error {
				t.Helper()
				var buf bytes.Buffer
				idx := NewGINIndex()
				idx.Header.NumRowGroups = 1
				binary.Write(&buf, binary.LittleEndian, uint32(2))
				for i := 0; i < 2; i++ {
					binary.Write(&buf, binary.LittleEndian, uint16(0))
					binary.Write(&buf, binary.LittleEndian, uint32(1))
					binary.Write(&buf, binary.LittleEndian, uint8(3))
					binary.Write(&buf, binary.LittleEndian, uint8(0))
					binary.Write(&buf, binary.LittleEndian, uint32(0))
				}
				return readTrigramIndexes(&buf, idx)
			},
		},
		{
			name: "hyperloglogs",
			run: func(t *testing.T) error {
				t.Helper()
				var buf bytes.Buffer
				binary.Write(&buf, binary.LittleEndian, uint32(2))
				for i := 0; i < 2; i++ {
					binary.Write(&buf, binary.LittleEndian, uint16(0))
					binary.Write(&buf, binary.LittleEndian, uint8(4))
					binary.Write(&buf, binary.LittleEndian, uint32(16))
					buf.Write(make([]byte, 16))
				}
				return readHyperLogLogs(&buf, NewGINIndex())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run(t)
			if err == nil {
				t.Fatal("expected duplicate-path rejection, got nil")
			}
			if !stderrors.Is(err, ErrInvalidFormat) {
				t.Fatalf("expected ErrInvalidFormat, got %v", err)
			}
			if !strings.Contains(err.Error(), "duplicate") {
				t.Fatalf("error = %v, want duplicate context", err)
			}
		})
	}
}

func TestValidatePathReferencesRejectsModeMismatches(t *testing.T) {
	tests := []struct {
		name string
		idx  func() *GINIndex
		want string
	}{
		{
			name: "classic path with adaptive section",
			idx: func() *GINIndex {
				idx := NewGINIndex()
				idx.PathDirectory = []PathEntry{{PathID: 0, PathName: "$.field", Mode: PathModeClassic}}
				adaptive, err := NewAdaptiveStringIndex([]string{"hot"}, []*RGSet{MustNewRGSet(1)}, []*RGSet{MustNewRGSet(1)})
				if err != nil {
					t.Fatalf("NewAdaptiveStringIndex() error = %v", err)
				}
				idx.AdaptiveStringIndexes[0] = adaptive
				return idx
			},
			want: "must not have adaptive section",
		},
		{
			name: "bloom-only path with exact string index",
			idx: func() *GINIndex {
				idx := NewGINIndex()
				idx.PathDirectory = []PathEntry{{PathID: 0, PathName: "$.field", Mode: PathModeBloomOnly}}
				idx.StringIndexes[0] = &StringIndex{}
				return idx
			},
			want: "must not have string index",
		},
		{
			name: "bloom-only path with adaptive section",
			idx: func() *GINIndex {
				idx := NewGINIndex()
				idx.PathDirectory = []PathEntry{{PathID: 0, PathName: "$.field", Mode: PathModeBloomOnly}}
				idx.AdaptiveStringIndexes[0] = mustAdaptiveIndex(t, []string{"hot"}, []*RGSet{MustNewRGSet(1)}, []*RGSet{MustNewRGSet(1)})
				return idx
			},
			want: "must not have adaptive section",
		},
		{
			name: "adaptive path with exact string index",
			idx: func() *GINIndex {
				idx := NewGINIndex()
				idx.PathDirectory = []PathEntry{{PathID: 0, PathName: "$.field", Mode: PathModeAdaptiveHybrid}}
				idx.StringIndexes[0] = &StringIndex{}
				idx.AdaptiveStringIndexes[0] = mustAdaptiveIndex(t, []string{"hot"}, []*RGSet{MustNewRGSet(1)}, []*RGSet{MustNewRGSet(1)})
				return idx
			},
			want: "must not have exact string index",
		},
		{
			name: "unknown mode",
			idx: func() *GINIndex {
				idx := NewGINIndex()
				idx.PathDirectory = []PathEntry{{PathID: 0, PathName: "$.field", Mode: PathMode(99)}}
				return idx
			},
			want: "unknown mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.idx().validatePathReferences()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !stderrors.Is(err, ErrInvalidFormat) {
				t.Fatalf("expected ErrInvalidFormat, got %v", err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestDecodeRejectsMissingConfigLengthTrailer(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 1)
	if err := builder.AddDocument(0, []byte(`{"name":"alice"}`)); err != nil {
		t.Fatalf("AddDocument() error = %v", err)
	}

	data, err := EncodeWithLevel(builder.Finalize(), CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel() error = %v", err)
	}
	if len(data) < 4 {
		t.Fatalf("encoded length = %d, want at least 4 bytes", len(data))
	}

	truncated := data[:len(data)-4]
	_, err = Decode(truncated)
	if err == nil {
		t.Fatal("Decode() error = nil, want ErrInvalidFormat for missing config length")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestDecodeRejectsOversizedCompressedPayload(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 1)
	if err := builder.AddDocument(0, []byte(`{"name":"alice"}`)); err != nil {
		t.Fatalf("AddDocument() error = %v", err)
	}

	uncompressed, err := EncodeWithLevel(builder.Finalize(), CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel() error = %v", err)
	}

	payload := append([]byte{}, uncompressed[4:]...)
	if len(payload) >= maxDecodedIndexSize {
		t.Fatalf("fixture payload length = %d, want less than cap %d", len(payload), maxDecodedIndexSize)
	}
	padding := bytes.Repeat([]byte{0}, maxDecodedIndexSize-len(payload)+1)
	payload = append(payload, padding...)

	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest))
	if err != nil {
		t.Fatalf("zstd.NewWriter() error = %v", err)
	}
	compressed := encoder.EncodeAll(payload, nil)
	encoder.Close()

	_, err = Decode(append([]byte(compressedMagic), compressed...))
	if err == nil {
		t.Fatal("Decode() error = nil, want oversized compressed payload rejection")
	}
	if !strings.Contains(err.Error(), "decompress data") {
		t.Fatalf("expected decompress data error, got %v", err)
	}
}

func TestDecodeRejectsOversizedHeaderRowGroups(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 1)
	if err := builder.AddDocument(0, []byte(`{"name":"alice"}`)); err != nil {
		t.Fatalf("AddDocument() error = %v", err)
	}

	data, err := EncodeWithLevel(builder.Finalize(), CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel() error = %v", err)
	}

	binary.LittleEndian.PutUint32(data[12:16], maxHeaderRowGroups+1)
	_, err = Decode(data)
	if err == nil {
		t.Fatal("Decode() error = nil, want oversized row-group count rejection")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestDecodeRejectsOversizedHeaderDocs(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 1)
	if err := builder.AddDocument(0, []byte(`{"name":"alice"}`)); err != nil {
		t.Fatalf("AddDocument() error = %v", err)
	}

	data, err := EncodeWithLevel(builder.Finalize(), CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel() error = %v", err)
	}

	binary.LittleEndian.PutUint64(data[16:24], maxHeaderDocs+1)
	_, err = Decode(data)
	if err == nil {
		t.Fatal("Decode() error = nil, want oversized doc count rejection")
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

	_, err := EncodeWithLevel(idx, CompressionNone)
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
