package gin

import (
	"testing"
)

func TestIdentityCodec(t *testing.T) {
	codec := NewIdentityCodec()

	if codec.Name() != "identity" {
		t.Errorf("expected name 'identity', got %q", codec.Name())
	}

	docID := codec.Encode(42)
	if docID != DocID(42) {
		t.Errorf("expected DocID 42, got %d", docID)
	}

	decoded := codec.Decode(docID)
	if len(decoded) != 1 || decoded[0] != 42 {
		t.Errorf("expected [42], got %v", decoded)
	}

	emptyID := codec.Encode()
	if emptyID != 0 {
		t.Errorf("expected DocID 0 for empty encode, got %d", emptyID)
	}
}

func TestRowGroupCodec(t *testing.T) {
	codec := NewRowGroupCodec(20)

	if codec.Name() != "rowgroup" {
		t.Errorf("expected name 'rowgroup', got %q", codec.Name())
	}

	if codec.RowGroupsPerFile() != 20 {
		t.Errorf("expected 20 row groups per file, got %d", codec.RowGroupsPerFile())
	}

	// Test encoding: file 3, row group 15 -> 3*20+15 = 75
	docID := codec.Encode(3, 15)
	if docID != DocID(75) {
		t.Errorf("expected DocID 75, got %d", docID)
	}

	// Test decoding
	decoded := codec.Decode(docID)
	if len(decoded) != 2 || decoded[0] != 3 || decoded[1] != 15 {
		t.Errorf("expected [3, 15], got %v", decoded)
	}

	// Test edge cases
	docID = codec.Encode(0, 0)
	if docID != DocID(0) {
		t.Errorf("expected DocID 0, got %d", docID)
	}
	decoded = codec.Decode(docID)
	if decoded[0] != 0 || decoded[1] != 0 {
		t.Errorf("expected [0, 0], got %v", decoded)
	}

	// Test with single argument
	docID = codec.Encode(5)
	if docID != DocID(5) {
		t.Errorf("expected DocID 5 for single arg, got %d", docID)
	}
}

func TestRowGroupCodecZeroRGs(t *testing.T) {
	codec := NewRowGroupCodec(0)
	if codec.RowGroupsPerFile() != 1 {
		t.Errorf("expected 1 row group per file for zero input, got %d", codec.RowGroupsPerFile())
	}
}

func TestBuilderWithCodec(t *testing.T) {
	codec := NewRowGroupCodec(10)
	builder, err := NewBuilder(DefaultConfig(), 30, WithCodec(codec))
	if err != nil {
		t.Fatalf("failed to create builder: %v", err)
	}

	// Add docs with composite DocIDs (file, rg)
	builder.AddDocument(codec.Encode(0, 0), []byte(`{"name": "alice"}`))
	builder.AddDocument(codec.Encode(0, 5), []byte(`{"name": "bob"}`))
	builder.AddDocument(codec.Encode(1, 0), []byte(`{"name": "alice"}`))
	builder.AddDocument(codec.Encode(2, 3), []byte(`{"name": "charlie"}`))

	idx := builder.Finalize()

	if idx.Header.NumDocs != 4 {
		t.Errorf("expected 4 docs, got %d", idx.Header.NumDocs)
	}

	// Query and get DocIDs
	result := idx.Evaluate([]Predicate{EQ("$.name", "alice")})
	docIDs := idx.MatchingDocIDs(result)

	if len(docIDs) != 2 {
		t.Errorf("expected 2 matching DocIDs, got %d", len(docIDs))
	}

	// Verify the DocIDs decode correctly
	for _, docID := range docIDs {
		decoded := codec.Decode(docID)
		t.Logf("DocID %d decodes to file=%d, rg=%d", docID, decoded[0], decoded[1])
	}
}

func TestDocIDMappingRoundTrip(t *testing.T) {
	codec := NewRowGroupCodec(5)
	builder, err := NewBuilder(DefaultConfig(), 15, WithCodec(codec))
	if err != nil {
		t.Fatalf("failed to create builder: %v", err)
	}

	// Add documents with specific DocIDs
	expectedDocIDs := []DocID{
		codec.Encode(0, 0), // 0
		codec.Encode(0, 3), // 3
		codec.Encode(1, 2), // 7
		codec.Encode(2, 4), // 14
	}

	for i, docID := range expectedDocIDs {
		builder.AddDocument(docID, []byte(`{"index": `+string(rune('0'+i))+`}`))
	}

	idx := builder.Finalize()

	// Verify DocIDMapping was stored
	if len(idx.DocIDMapping) != 4 {
		t.Fatalf("expected 4 DocIDs in mapping, got %d", len(idx.DocIDMapping))
	}

	// Serialize and deserialize
	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	// Verify DocIDMapping was restored
	if len(decoded.DocIDMapping) != len(idx.DocIDMapping) {
		t.Errorf("DocIDMapping length mismatch: %d vs %d", len(decoded.DocIDMapping), len(idx.DocIDMapping))
	}

	for i, expected := range idx.DocIDMapping {
		if decoded.DocIDMapping[i] != expected {
			t.Errorf("DocIDMapping[%d] mismatch: %d vs %d", i, decoded.DocIDMapping[i], expected)
		}
	}
}

func TestMatchingDocIDsWithIdentityCodec(t *testing.T) {
	builder, err := NewBuilder(DefaultConfig(), 3)
	if err != nil {
		t.Fatalf("failed to create builder: %v", err)
	}

	builder.AddDocument(0, []byte(`{"name": "alice"}`))
	builder.AddDocument(1, []byte(`{"name": "bob"}`))
	builder.AddDocument(2, []byte(`{"name": "alice"}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{EQ("$.name", "alice")})
	docIDs := idx.MatchingDocIDs(result)

	if len(docIDs) != 2 {
		t.Errorf("expected 2 matching DocIDs, got %d", len(docIDs))
	}

	// With identity codec, DocIDs should match positions
	if docIDs[0] != 0 || docIDs[1] != 2 {
		t.Errorf("expected DocIDs [0, 2], got %v", docIDs)
	}
}

func TestMatchingDocIDsFallback(t *testing.T) {
	idx := NewGINIndex()
	idx.Header.NumRowGroups = 3

	rgSet := MustNewRGSet(3)
	rgSet.Set(0)
	rgSet.Set(2)

	docIDs := idx.MatchingDocIDs(rgSet)

	if len(docIDs) != 2 {
		t.Errorf("expected 2 DocIDs, got %d", len(docIDs))
	}
	if docIDs[0] != 0 || docIDs[1] != 2 {
		t.Errorf("expected DocIDs [0, 2], got %v", docIDs)
	}
}

func TestBuilderWithNilCodec(t *testing.T) {
	// Note: WithCodec(nil) returns an error, so we expect the builder creation to fail
	_, err := NewBuilder(DefaultConfig(), 3, WithCodec(nil))
	if err == nil {
		t.Fatal("expected error when creating builder with nil codec")
	}

	// Without WithCodec, the builder uses the default identity codec
	builder, err := NewBuilder(DefaultConfig(), 3)
	if err != nil {
		t.Fatalf("failed to create builder: %v", err)
	}

	builder.AddDocument(0, []byte(`{"name": "test"}`))
	idx := builder.Finalize()

	if idx.Header.NumDocs != 1 {
		t.Errorf("expected 1 doc, got %d", idx.Header.NumDocs)
	}
}
