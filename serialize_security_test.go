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

func buildRepresentationSerializationFixture(t *testing.T) *GINIndex {
	t.Helper()

	config, err := NewConfig(
		WithToLowerTransformer("$.email", "lower"),
		WithEmailDomainTransformer("$.email", "domain"),
	)
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	builder := mustNewBuilder(t, config, 3)
	docs := []string{
		`{"email":"Alice@Example.COM"}`,
		`{"email":"bob@other.dev"}`,
		`{"email":"CHARLIE@EXAMPLE.COM"}`,
	}
	for i, doc := range docs {
		if err := builder.AddDocument(DocID(i), []byte(doc)); err != nil {
			t.Fatalf("AddDocument(%d) error = %v", i, err)
		}
	}

	return builder.Finalize()
}

func locateRepresentationSection(t *testing.T, data []byte) ([]RepresentationSpec, int) {
	t.Helper()

	if len(data) < 4 {
		t.Fatalf("encoded data too short: %d", len(data))
	}

	payload := data[4:]
	buf := bytes.NewReader(payload)
	idx := NewGINIndex()

	if err := readHeader(buf, idx); err != nil {
		t.Fatalf("readHeader() error = %v", err)
	}
	if err := readPathDirectory(buf, idx); err != nil {
		t.Fatalf("readPathDirectory() error = %v", err)
	}
	if _, err := readBloomFilter(buf); err != nil {
		t.Fatalf("readBloomFilter() error = %v", err)
	}
	if err := readStringIndexes(buf, idx); err != nil {
		t.Fatalf("readStringIndexes() error = %v", err)
	}
	if err := readAdaptiveStringIndexes(buf, idx); err != nil {
		t.Fatalf("readAdaptiveStringIndexes() error = %v", err)
	}
	if err := readStringLengthIndexes(buf, idx, idx.Header.NumRowGroups); err != nil {
		t.Fatalf("readStringLengthIndexes() error = %v", err)
	}
	if err := readNumericIndexes(buf, idx, idx.Header.NumRowGroups); err != nil {
		t.Fatalf("readNumericIndexes() error = %v", err)
	}
	if err := readNullIndexes(buf, idx); err != nil {
		t.Fatalf("readNullIndexes() error = %v", err)
	}
	if err := readTrigramIndexes(buf, idx); err != nil {
		t.Fatalf("readTrigramIndexes() error = %v", err)
	}
	if err := readHyperLogLogs(buf, idx); err != nil {
		t.Fatalf("readHyperLogLogs() error = %v", err)
	}
	if idx.Header.Flags&FlagHasDocIDMap != 0 {
		if _, err := readDocIDMapping(buf, idx.Header.NumDocs); err != nil {
			t.Fatalf("readDocIDMapping() error = %v", err)
		}
	}
	if _, err := readConfig(buf); err != nil {
		t.Fatalf("readConfig() error = %v", err)
	}

	offset := len(data) - buf.Len()
	representations, err := readRepresentations(buf)
	if err != nil {
		t.Fatalf("readRepresentations() error = %v", err)
	}
	return representations, offset
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

func mustWriteOrderedStrings(t *testing.T, w io.Writer, values []string) {
	t.Helper()

	if err := writeOrderedStrings(w, values, defaultPrefixBlockSize); err != nil {
		t.Fatalf("writeOrderedStrings(%v) error = %v", values, err)
	}
}

func buildCompactPathFixture(t *testing.T) *GINIndex {
	t.Helper()

	builder := mustNewBuilder(t, DefaultConfig(), 1)
	doc := `{
		"alpha":"a",
		"alphabet":"b",
		"alphabetical":"c",
		"alphanumeric":"d",
		"alpha_nested":{"child":"e","child_two":"f"}
	}`
	if err := builder.AddDocument(0, []byte(doc)); err != nil {
		t.Fatalf("AddDocument() error = %v", err)
	}
	return builder.Finalize()
}

func buildCompactStringFixture(t *testing.T) *GINIndex {
	t.Helper()

	builder := mustNewBuilder(t, DefaultConfig(), 6)
	docs := []string{
		`{"code":"prefix-0001"}`,
		`{"code":"prefix-0002"}`,
		`{"code":"prefix-0003"}`,
		`{"code":"prefix-0100"}`,
		`{"code":"prefix-0101"}`,
		`{"code":"prefix-0102"}`,
	}
	for i, doc := range docs {
		if err := builder.AddDocument(DocID(i), []byte(doc)); err != nil {
			t.Fatalf("AddDocument(rg=%d) error = %v", i, err)
		}
	}
	return builder.Finalize()
}

func mustEncodeUncompressed(t *testing.T, idx *GINIndex) []byte {
	t.Helper()

	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel() error = %v", err)
	}
	return data
}

func locatePathOrderedStringsOffset(t *testing.T, data []byte) ([]byte, int) {
	t.Helper()

	body := data[len(uncompressedMagic):]
	reader := bytes.NewReader(body)
	idx := NewGINIndex()
	if err := readHeader(reader, idx); err != nil {
		t.Fatalf("readHeader() error = %v", err)
	}

	return body, len(body) - reader.Len()
}

func locateStringIndexOrderedStringsOffset(t *testing.T, data []byte, pathName string) ([]byte, int) {
	t.Helper()

	body := data[len(uncompressedMagic):]
	reader := bytes.NewReader(body)
	idx := NewGINIndex()
	if err := readHeader(reader, idx); err != nil {
		t.Fatalf("readHeader() error = %v", err)
	}
	if err := readPathDirectory(reader, idx); err != nil {
		t.Fatalf("readPathDirectory() error = %v", err)
	}
	if _, err := readBloomFilter(reader); err != nil {
		t.Fatalf("readBloomFilter() error = %v", err)
	}

	var numPaths uint32
	if err := binary.Read(reader, binary.LittleEndian, &numPaths); err != nil {
		t.Fatalf("binary.Read(numPaths) error = %v", err)
	}

	for i := uint32(0); i < numPaths; i++ {
		var pathID uint16
		if err := binary.Read(reader, binary.LittleEndian, &pathID); err != nil {
			t.Fatalf("binary.Read(pathID) error = %v", err)
		}
		var numTerms uint32
		if err := binary.Read(reader, binary.LittleEndian, &numTerms); err != nil {
			t.Fatalf("binary.Read(numTerms) error = %v", err)
		}

		offset := len(body) - reader.Len()
		if idx.PathDirectory[pathID].PathName == pathName {
			return body, offset
		}

		if _, err := readOrderedStrings(reader, numTerms); err != nil {
			t.Fatalf("readOrderedStrings(path=%s) error = %v", idx.PathDirectory[pathID].PathName, err)
		}
		for termIdx := uint32(0); termIdx < numTerms; termIdx++ {
			if _, err := readRGSet(reader, idx.Header.NumRowGroups); err != nil {
				t.Fatalf("readRGSet(path=%s term=%d) error = %v", idx.PathDirectory[pathID].PathName, termIdx, err)
			}
		}
	}

	t.Fatalf("path %q not found in string index section", pathName)
	return nil, 0
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

	tests := []struct {
		name    string
		version uint16
	}{
		{name: "future version", version: 99},
		{name: "zero version", version: 0},
		{name: "previous phase version", version: 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			binary.LittleEndian.PutUint16(data[8:10], tt.version)

			_, err = Decode(data)
			if err == nil {
				t.Fatalf("expected error for version %d, got nil", tt.version)
			}
			if !stderrors.Is(err, ErrVersionMismatch) {
				t.Fatalf("expected ErrVersionMismatch for version %d, got: %v", tt.version, err)
			}
		})
	}
}

func TestDecodeRejectsV6VersionMismatch(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 1)
	if err := builder.AddDocument(0, []byte(`{"name":"alice"}`)); err != nil {
		t.Fatalf("AddDocument() error = %v", err)
	}

	data, err := EncodeWithLevel(builder.Finalize(), CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel() error = %v", err)
	}

	data[8] = 6
	data[9] = 0

	_, err = Decode(data)
	if err == nil {
		t.Fatal("expected error for v6 payload, got nil")
	}
	if !stderrors.Is(err, ErrVersionMismatch) {
		t.Fatalf("expected ErrVersionMismatch for v6 payload, got: %v", err)
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

	body := data[len(uncompressedMagic):]
	reader := bytes.NewReader(body)
	headerOnly := NewGINIndex()
	if err := readHeader(reader, headerOnly); err != nil {
		t.Fatalf("readHeader() error = %v", err)
	}
	if _, err := readOrderedStrings(reader, headerOnly.Header.NumPaths); err != nil {
		t.Fatalf("readOrderedStrings() error = %v", err)
	}

	metadataStart := len(body) - reader.Len()
	modeOffset := metadataStart + 2 + 1 + 4
	body[modeOffset] = 99

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

func TestRepresentationMetadataRoundTrip(t *testing.T) {
	idx := buildRepresentationSerializationFixture(t)

	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel() error = %v", err)
	}

	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	want := []RepresentationInfo{
		{SourcePath: "$.email", Alias: "domain", Transformer: "email_domain"},
		{SourcePath: "$.email", Alias: "lower", Transformer: "to_lower"},
	}
	if got := decoded.Representations("$.email"); !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded.Representations($.email) = %#v, want %#v", got, want)
	}
}

func TestRepresentationFailureModeRoundTrip(t *testing.T) {
	config, err := NewConfig(
		WithToLowerTransformer("$.email", "lower", WithTransformerFailureMode(TransformerFailureSoft)),
	)
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	builder := mustNewBuilder(t, config, 1)
	if err := builder.AddDocument(0, []byte(`{"email":"Alice@Example.COM"}`)); err != nil {
		t.Fatalf("AddDocument() error = %v", err)
	}

	decoded := mustRoundTripIndex(t, builder.Finalize())

	specs := decoded.Config.representationSpecs["$.email"]
	if len(specs) != 1 {
		t.Fatalf("len(decoded.Config.representationSpecs[$.email]) = %d, want 1", len(specs))
	}
	if specs[0].Transformer.FailureMode != TransformerFailureSoft {
		t.Fatalf("decoded failure mode = %q, want %q", specs[0].Transformer.FailureMode, TransformerFailureSoft)
	}

	reloadedBuilder := mustNewBuilder(t, *decoded.Config, 1)
	if err := reloadedBuilder.AddDocument(0, []byte(`{"email":42}`)); err != nil {
		t.Fatalf("AddDocument() with decoded soft-fail config error = %v, want success", err)
	}
}

func TestDecodeRepresentationAliasParity(t *testing.T) {
	idx := buildRepresentationSerializationFixture(t)

	rawBefore := idx.Evaluate([]Predicate{EQ("$.email", "Alice@Example.COM")}).ToSlice()
	aliasBefore := idx.Evaluate([]Predicate{EQ("$.email", As("domain", "example.com"))}).ToSlice()

	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel() error = %v", err)
	}
	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	rawAfter := decoded.Evaluate([]Predicate{EQ("$.email", "Alice@Example.COM")}).ToSlice()
	if !reflect.DeepEqual(rawAfter, rawBefore) {
		t.Fatalf("decoded raw query = %v, want %v", rawAfter, rawBefore)
	}

	aliasAfter := decoded.Evaluate([]Predicate{EQ("$.email", As("domain", "example.com"))}).ToSlice()
	if !reflect.DeepEqual(aliasAfter, aliasBefore) {
		t.Fatalf("decoded alias query = %v, want %v", aliasAfter, aliasBefore)
	}
}

func TestDecodeRejectsCompactPathSectionCorruption(t *testing.T) {
	idx := buildCompactPathFixture(t)
	data := mustEncodeUncompressed(t, idx)

	body, offset := locatePathOrderedStringsOffset(t, data)
	if body[offset] != compactStringModeFrontCoded {
		t.Fatalf("path section mode = %d, want %d for compact fixture", body[offset], compactStringModeFrontCoded)
	}

	body[offset+1+4] = 0xff
	body[offset+1+4+1] = 0x7f

	if _, err := Decode(data); err == nil {
		t.Fatal("Decode() error = nil, want compact path corruption rejection")
	} else if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestDecodeRejectsCompactTermSectionCorruption(t *testing.T) {
	idx := buildCompactStringFixture(t)
	data := mustEncodeUncompressed(t, idx)

	body, offset := locateStringIndexOrderedStringsOffset(t, data, "$.code")
	if body[offset] != compactStringModeFrontCoded {
		t.Fatalf("term section mode = %d, want %d for compact fixture", body[offset], compactStringModeFrontCoded)
	}

	body[offset+1+4] = 0xff
	body[offset+1+4+1] = 0x7f

	if _, err := Decode(data); err == nil {
		t.Fatal("Decode() error = nil, want compact term corruption rejection")
	} else if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat, got %v", err)
	}
}

func TestDecodeRejectsOrderedStringModePayloadMismatch(t *testing.T) {
	values := []string{"prefix-0001", "prefix-0002"}

	frontPayload, err := encodeFrontCodedOrderedStrings(values, defaultPrefixBlockSize)
	if err != nil {
		t.Fatalf("encodeFrontCodedOrderedStrings() error = %v", err)
	}
	frontBytes := append([]byte(nil), frontPayload.Bytes()...)
	frontBytes[0] = compactStringModeRaw
	if _, err := readOrderedStrings(bytes.NewReader(frontBytes), uint32(len(values))); err == nil {
		t.Fatal("readOrderedStrings(front-coded as raw) error = nil, want mismatch rejection")
	} else if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat for front-coded/raw mismatch, got %v", err)
	}

	rawPayload, err := encodeRawOrderedStrings(values)
	if err != nil {
		t.Fatalf("encodeRawOrderedStrings() error = %v", err)
	}
	rawBytes := append([]byte(nil), rawPayload.Bytes()...)
	rawBytes[0] = compactStringModeFrontCoded
	if _, err := readOrderedStrings(bytes.NewReader(rawBytes), uint32(len(values))); err == nil {
		t.Fatal("readOrderedStrings(raw as front-coded) error = nil, want mismatch rejection")
	} else if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat for raw/front-coded mismatch, got %v", err)
	}
}

func TestWriteOrderedStringsPrefersRawOnTie(t *testing.T) {
	var buf bytes.Buffer
	if err := writeOrderedStrings(&buf, nil, defaultPrefixBlockSize); err != nil {
		t.Fatalf("writeOrderedStrings(nil) error = %v", err)
	}

	payload := buf.Bytes()
	if len(payload) == 0 {
		t.Fatal("writeOrderedStrings(nil) produced empty payload")
	}
	if payload[0] != compactStringModeRaw {
		t.Fatalf("mode = %d, want raw mode on tie", payload[0])
	}

	got, err := readOrderedStrings(bytes.NewReader(payload), 0)
	if err != nil {
		t.Fatalf("readOrderedStrings(raw tie payload) error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0", len(got))
	}
}

func TestRepresentationMetadataSectionFollowsConfig(t *testing.T) {
	idx := buildRepresentationSerializationFixture(t)

	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel() error = %v", err)
	}

	representations, offset := locateRepresentationSection(t, data)
	if len(representations) != 2 {
		t.Fatalf("representation count = %d, want 2", len(representations))
	}
	if offset <= 4 {
		t.Fatalf("representation section offset = %d, want > 4 and after config", offset)
	}
}

func TestEncodeRejectsNonSerializableRepresentation(t *testing.T) {
	config, err := NewConfig(
		WithCustomTransformer("$.email", "opaque", func(value any) (any, bool) {
			s, ok := value.(string)
			if !ok {
				return nil, false
			}
			return strings.ToLower(s), true
		}),
	)
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	builder := mustNewBuilder(t, config, 1)
	if err := builder.AddDocument(0, []byte(`{"email":"Alice@Example.COM"}`)); err != nil {
		t.Fatalf("AddDocument() error = %v", err)
	}

	if _, err := Encode(builder.Finalize()); err == nil {
		t.Fatal("Encode() error = nil, want non-serializable representation failure")
	} else if !strings.Contains(err.Error(), "not serializable") {
		t.Fatalf("Encode() error = %v, want non-serializable representation failure", err)
	}
}

func TestDecodeRejectsDuplicateRepresentationAlias(t *testing.T) {
	idx := buildRepresentationSerializationFixture(t)

	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel() error = %v", err)
	}

	representations, offset := locateRepresentationSection(t, data)
	representations = append(representations, representations[0])
	payload, err := json.Marshal(representations)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var mutated bytes.Buffer
	mutated.Write(data[:offset])
	if err := binary.Write(&mutated, binary.LittleEndian, uint32(len(payload))); err != nil {
		t.Fatalf("binary.Write() error = %v", err)
	}
	mutated.Write(payload)

	if _, err := Decode(mutated.Bytes()); err == nil {
		t.Fatal("Decode() error = nil, want duplicate representation alias rejection")
	} else if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("Decode() error = %v, want ErrInvalidFormat", err)
	}
}

func TestDecodeRejectsRepresentationTargetPathOutOfRange(t *testing.T) {
	idx := buildRepresentationSerializationFixture(t)

	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel() error = %v", err)
	}

	representations, offset := locateRepresentationSection(t, data)
	representations[0].TargetPath = "__derived:$.email#missing"
	payload, err := json.Marshal(representations)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var mutated bytes.Buffer
	mutated.Write(data[:offset])
	if err := binary.Write(&mutated, binary.LittleEndian, uint32(len(payload))); err != nil {
		t.Fatalf("binary.Write() error = %v", err)
	}
	mutated.Write(payload)

	if _, err := Decode(mutated.Bytes()); err == nil {
		t.Fatal("Decode() error = nil, want invalid target path rejection")
	} else if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("Decode() error = %v, want ErrInvalidFormat", err)
	}
}

func TestDecodeRejectsRepresentationMissingAlias(t *testing.T) {
	idx := buildRepresentationSerializationFixture(t)

	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel() error = %v", err)
	}

	representations, offset := locateRepresentationSection(t, data)
	representations[0].Alias = ""
	payload, err := json.Marshal(representations)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var mutated bytes.Buffer
	mutated.Write(data[:offset])
	if err := binary.Write(&mutated, binary.LittleEndian, uint32(len(payload))); err != nil {
		t.Fatalf("binary.Write() error = %v", err)
	}
	mutated.Write(payload)

	if _, err := Decode(mutated.Bytes()); err == nil {
		t.Fatal("Decode() error = nil, want missing alias rejection")
	} else if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("Decode() error = %v, want ErrInvalidFormat", err)
	}
}

func TestDecodeRejectsRepresentationMissingTransformerName(t *testing.T) {
	idx := buildRepresentationSerializationFixture(t)

	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel() error = %v", err)
	}

	representations, offset := locateRepresentationSection(t, data)
	representations[0].Transformer.Name = ""
	payload, err := json.Marshal(representations)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var mutated bytes.Buffer
	mutated.Write(data[:offset])
	if err := binary.Write(&mutated, binary.LittleEndian, uint32(len(payload))); err != nil {
		t.Fatalf("binary.Write() error = %v", err)
	}
	mutated.Write(payload)

	if _, err := Decode(mutated.Bytes()); err == nil {
		t.Fatal("Decode() error = nil, want missing transformer name rejection")
	} else if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("Decode() error = %v, want ErrInvalidFormat", err)
	}
}

func TestDecodeRejectsRepresentationTransformerPathMismatch(t *testing.T) {
	idx := buildRepresentationSerializationFixture(t)

	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel() error = %v", err)
	}

	representations, offset := locateRepresentationSection(t, data)
	representations[0].Transformer.Path = "$.other"
	payload, err := json.Marshal(representations)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var mutated bytes.Buffer
	mutated.Write(data[:offset])
	if err := binary.Write(&mutated, binary.LittleEndian, uint32(len(payload))); err != nil {
		t.Fatalf("binary.Write() error = %v", err)
	}
	mutated.Write(payload)

	if _, err := Decode(mutated.Bytes()); err == nil {
		t.Fatal("Decode() error = nil, want transformer path mismatch rejection")
	} else if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("Decode() error = %v, want ErrInvalidFormat", err)
	}
}

func TestDecodeRejectsRepresentationMarkedNonSerializable(t *testing.T) {
	idx := buildRepresentationSerializationFixture(t)

	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel() error = %v", err)
	}

	representations, offset := locateRepresentationSection(t, data)
	representations[0].Serializable = false
	payload, err := json.Marshal(representations)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var mutated bytes.Buffer
	mutated.Write(data[:offset])
	if err := binary.Write(&mutated, binary.LittleEndian, uint32(len(payload))); err != nil {
		t.Fatalf("binary.Write() error = %v", err)
	}
	mutated.Write(payload)

	if _, err := Decode(mutated.Bytes()); err == nil {
		t.Fatal("Decode() error = nil, want non-serializable metadata rejection")
	} else if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("Decode() error = %v, want ErrInvalidFormat", err)
	}
}

func TestDecodeRejectsOversizedRepresentationSection(t *testing.T) {
	idx := buildRepresentationSerializationFixture(t)

	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("EncodeWithLevel() error = %v", err)
	}

	_, offset := locateRepresentationSection(t, data)

	var mutated bytes.Buffer
	mutated.Write(data[:offset])
	if err := binary.Write(&mutated, binary.LittleEndian, uint32(maxRepresentationSectionSize+1)); err != nil {
		t.Fatalf("binary.Write() error = %v", err)
	}

	if _, err := Decode(mutated.Bytes()); err == nil {
		t.Fatal("Decode() error = nil, want oversized representation section rejection")
	} else if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("Decode() error = %v, want ErrInvalidFormat", err)
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
				mustWriteOrderedStrings(t, &buf, nil)
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
				mustWriteOrderedStrings(t, &buf, nil)
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
				mustWriteOrderedStrings(t, &buf, nil)
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
					mustWriteOrderedStrings(t, &buf, nil)
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
	mustWriteOrderedStrings(t, &buf, []string{"hot"})
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
	if err := buf.WriteByte(compactStringModeRaw); err != nil {
		t.Fatalf("WriteByte(compactStringModeRaw) error = %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(1)); err != nil {
		t.Fatalf("binary.Write(rawCount) error = %v", err)
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
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat, got %v", err)
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
					mustWriteOrderedStrings(t, &buf, nil)
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
			name: "adaptive path missing adaptive section",
			idx: func() *GINIndex {
				idx := NewGINIndex()
				idx.PathDirectory = []PathEntry{{PathID: 0, PathName: "$.field", Mode: PathModeAdaptiveHybrid}}
				return idx
			},
			want: "missing adaptive section",
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
	if _, err := readOrderedStrings(reader, headerOnly.Header.NumPaths); err != nil {
		t.Fatalf("readOrderedStrings() error = %v", err)
	}

	const pathMetadataSize = 2 + 1 + 4 + 1 + 1
	metadataStart := len(body) - reader.Len()
	for i := uint32(0); i < headerOnly.Header.NumPaths; i++ {
		pathIDOffsets = append(pathIDOffsets, metadataStart+int(i)*pathMetadataSize)
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

func TestReadConfigRejectsDuplicateTransformerAlias(t *testing.T) {
	lower := NewTransformerSpec("$.foo", TransformerToLower, nil)
	lower.Alias = "lower"
	lower.TargetPath = representationTargetPath("$.foo", "lower")

	duplicateAlias := NewTransformerSpec("$['foo']", TransformerEmailDomain, nil)
	duplicateAlias.Alias = "lower"
	duplicateAlias.TargetPath = representationTargetPath("$.foo", "lower")

	sc := SerializedConfig{
		Transformers: []TransformerSpec{
			lower,
			duplicateAlias,
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
		t.Fatal("expected duplicate transformer alias error, got nil")
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
