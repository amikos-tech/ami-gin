package gin

import (
	stderrors "errors"
	"fmt"
	"testing"
)

func mustNewBuilder(t *testing.T, config GINConfig, numRGs int) *GINBuilder {
	t.Helper()
	builder, err := NewBuilder(config, numRGs)
	if err != nil {
		t.Fatalf("failed to create builder: %v", err)
	}
	return builder
}

func TestBloomFilter(t *testing.T) {
	bf := MustNewBloomFilter(1024, 3)
	bf.AddString("hello")
	bf.AddString("world")

	if !bf.MayContainString("hello") {
		t.Error("bloom filter should contain 'hello'")
	}
	if !bf.MayContainString("world") {
		t.Error("bloom filter should contain 'world'")
	}
	if bf.MayContainString("notpresent") {
		t.Log("bloom filter false positive (expected occasionally)")
	}
}

func TestRGSet(t *testing.T) {
	rs := MustNewRGSet(100)
	rs.Set(5)
	rs.Set(10)
	rs.Set(99)

	if !rs.IsSet(5) {
		t.Error("bit 5 should be set")
	}
	if !rs.IsSet(10) {
		t.Error("bit 10 should be set")
	}
	if !rs.IsSet(99) {
		t.Error("bit 99 should be set")
	}
	if rs.IsSet(0) {
		t.Error("bit 0 should not be set")
	}
	if rs.Count() != 3 {
		t.Errorf("expected count 3, got %d", rs.Count())
	}
}

func TestRGSetIntersect(t *testing.T) {
	a := MustNewRGSet(64)
	a.Set(1)
	a.Set(2)
	a.Set(3)

	b := MustNewRGSet(64)
	b.Set(2)
	b.Set(3)
	b.Set(4)

	result := a.Intersect(b)
	if result.Count() != 2 {
		t.Errorf("expected 2 bits set, got %d", result.Count())
	}
	if !result.IsSet(2) || !result.IsSet(3) {
		t.Error("bits 2 and 3 should be set")
	}
}

func TestRGSetUnion(t *testing.T) {
	a := MustNewRGSet(64)
	a.Set(1)
	a.Set(2)

	b := MustNewRGSet(64)
	b.Set(3)
	b.Set(4)

	result := a.Union(b)
	if result.Count() != 4 {
		t.Errorf("expected 4 bits set, got %d", result.Count())
	}
}

func TestBuilderSimple(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)

	docs := []struct {
		docID DocID
		json  string
	}{
		{0, `{"name": "alice", "age": 30}`},
		{0, `{"name": "bob", "age": 25}`},
		{1, `{"name": "charlie", "age": 35}`},
		{2, `{"name": "alice", "age": 40}`},
	}

	for _, d := range docs {
		if err := builder.AddDocument(d.docID, []byte(d.json)); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	idx := builder.Finalize()

	if idx.Header.NumDocs != 4 {
		t.Errorf("expected 4 docs, got %d", idx.Header.NumDocs)
	}
	if idx.Header.NumRowGroups != 3 {
		t.Errorf("expected 3 row groups, got %d", idx.Header.NumRowGroups)
	}
	if len(idx.PathDirectory) == 0 {
		t.Error("path directory should not be empty")
	}
}

func TestQueryEQ(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)

	builder.AddDocument(0, []byte(`{"name": "alice"}`))
	builder.AddDocument(1, []byte(`{"name": "bob"}`))
	builder.AddDocument(2, []byte(`{"name": "alice"}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{EQ("$.name", "alice")})
	rgs := result.ToSlice()

	if len(rgs) != 2 {
		t.Errorf("expected 2 matching RGs, got %d", len(rgs))
	}
	if !result.IsSet(0) || !result.IsSet(2) {
		t.Error("RG 0 and 2 should match")
	}
}

func TestQueryNumericRange(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 4)

	builder.AddDocument(0, []byte(`{"score": 10}`))
	builder.AddDocument(1, []byte(`{"score": 20}`))
	builder.AddDocument(2, []byte(`{"score": 30}`))
	builder.AddDocument(3, []byte(`{"score": 40}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{GT("$.score", 15)})
	if result.Count() != 3 {
		t.Errorf("expected 3 matching RGs for score > 15, got %d", result.Count())
	}

	result = idx.Evaluate([]Predicate{LTE("$.score", 20)})
	if result.Count() != 2 {
		t.Errorf("expected 2 matching RGs for score <= 20, got %d", result.Count())
	}
}

func TestQueryIN(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)

	builder.AddDocument(0, []byte(`{"status": "active"}`))
	builder.AddDocument(1, []byte(`{"status": "pending"}`))
	builder.AddDocument(2, []byte(`{"status": "inactive"}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{IN("$.status", "active", "pending")})
	if result.Count() != 2 {
		t.Errorf("expected 2 matching RGs, got %d", result.Count())
	}
}

func TestQueryNIN(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 4)

	builder.AddDocument(0, []byte(`{"status": "active"}`))
	builder.AddDocument(1, []byte(`{"status": "pending"}`))
	builder.AddDocument(2, []byte(`{"status": "inactive"}`))
	builder.AddDocument(3, []byte(`{"status": "archived"}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{NIN("$.status", "active", "pending")})
	if result.Count() != 2 {
		t.Errorf("expected 2 matching RGs for NOT IN, got %d", result.Count())
	}
	if !result.IsSet(2) || !result.IsSet(3) {
		t.Error("RG 2 and 3 should match NOT IN")
	}
	if result.IsSet(0) || result.IsSet(1) {
		t.Error("RG 0 and 1 should not match NOT IN")
	}
}

func TestQueryNull(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)

	builder.AddDocument(0, []byte(`{"value": null}`))
	builder.AddDocument(1, []byte(`{"value": 42}`))
	builder.AddDocument(2, []byte(`{"other": "field"}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{IsNull("$.value")})
	if !result.IsSet(0) {
		t.Error("RG 0 should match IS NULL")
	}

	result = idx.Evaluate([]Predicate{IsNotNull("$.value")})
	if !result.IsSet(0) || !result.IsSet(1) {
		t.Error("RG 0 and 1 should match IS NOT NULL")
	}
}

func TestSerializeRoundTrip(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)

	builder.AddDocument(0, []byte(`{"name": "alice", "age": 30, "active": true}`))
	builder.AddDocument(1, []byte(`{"name": "bob", "age": 25, "active": false}`))
	builder.AddDocument(2, []byte(`{"name": "charlie", "age": null}`))

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
		t.Errorf("NumRowGroups mismatch")
	}
	if len(decoded.PathDirectory) != len(idx.PathDirectory) {
		t.Errorf("PathDirectory length mismatch")
	}

	result := decoded.Evaluate([]Predicate{EQ("$.name", "alice")})
	if !result.IsSet(0) {
		t.Error("query on decoded index failed")
	}
}

func TestCompressionLevels(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)
	builder.AddDocument(0, []byte(`{"name": "alice", "age": 30, "active": true}`))
	builder.AddDocument(1, []byte(`{"name": "bob", "age": 25, "active": false}`))
	builder.AddDocument(2, []byte(`{"name": "charlie", "age": 35}`))
	idx := builder.Finalize()

	levels := []struct {
		name  string
		level CompressionLevel
	}{
		{"None", CompressionNone},
		{"Fastest", CompressionFastest},
		{"Balanced", CompressionBalanced},
		{"Better", CompressionBetter},
		{"Best", CompressionBest},
		{"Max", CompressionMax},
	}

	for _, tc := range levels {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := EncodeWithLevel(idx, tc.level)
			if err != nil {
				t.Fatalf("encode with level %d failed: %v", tc.level, err)
			}

			decoded, err := Decode(encoded)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			if decoded.Header.NumDocs != idx.Header.NumDocs {
				t.Errorf("NumDocs mismatch: %d vs %d", decoded.Header.NumDocs, idx.Header.NumDocs)
			}
			if decoded.Header.NumRowGroups != idx.Header.NumRowGroups {
				t.Errorf("NumRowGroups mismatch")
			}

			result := decoded.Evaluate([]Predicate{EQ("$.name", "bob")})
			if !result.IsSet(1) {
				t.Error("query on decoded index failed")
			}
		})
	}
}

func TestCompressionUncompressedMagic(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)
	builder.AddDocument(0, []byte(`{"value": 1}`))
	builder.AddDocument(1, []byte(`{"value": 2}`))
	idx := builder.Finalize()

	encoded, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("encode uncompressed failed: %v", err)
	}

	if len(encoded) < 4 {
		t.Fatal("encoded data too short")
	}
	if string(encoded[:4]) != "GINu" {
		t.Errorf("expected uncompressed magic 'GINu', got %q", string(encoded[:4]))
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("decode uncompressed failed: %v", err)
	}

	result := decoded.Evaluate([]Predicate{EQ("$.value", float64(2))})
	if !result.IsSet(1) {
		t.Error("query on decoded uncompressed index failed")
	}
}

func TestCompressionCompressedMagic(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)
	builder.AddDocument(0, []byte(`{"value": 1}`))
	builder.AddDocument(1, []byte(`{"value": 2}`))
	idx := builder.Finalize()

	encoded, err := EncodeWithLevel(idx, CompressionFastest)
	if err != nil {
		t.Fatalf("encode compressed failed: %v", err)
	}

	if len(encoded) < 4 {
		t.Fatal("encoded data too short")
	}
	if string(encoded[:4]) != "GINc" {
		t.Errorf("expected compressed magic 'GINc', got %q", string(encoded[:4]))
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("decode compressed failed: %v", err)
	}

	result := decoded.Evaluate([]Predicate{EQ("$.value", float64(2))})
	if !result.IsSet(1) {
		t.Error("query on decoded compressed index failed")
	}
}

func TestCompressionSizeReduction(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 100)
	for i := 0; i < 100; i++ {
		builder.AddDocument(DocID(i), []byte(fmt.Sprintf(`{"id": %d, "name": "user_%d", "value": %d}`, i, i%10, i*100)))
	}
	idx := builder.Finalize()

	uncompressed, _ := EncodeWithLevel(idx, CompressionNone)
	fastest, _ := EncodeWithLevel(idx, CompressionFastest)

	if len(fastest) >= len(uncompressed) {
		t.Errorf("zstd-1 should be smaller than uncompressed: %d >= %d", len(fastest), len(uncompressed))
	}

	// Note: For small datasets, higher compression levels may not always produce smaller output
	// due to compression dictionary overhead. The benefit is more pronounced with larger data.
	t.Logf("Sizes - Uncompressed: %d, Zstd-1: %d (%.1f%% of original)",
		len(uncompressed), len(fastest), float64(len(fastest))/float64(len(uncompressed))*100)
}

func TestCompressionInvalidLevel(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)
	builder.AddDocument(0, []byte(`{"value": 1}`))
	idx := builder.Finalize()

	tests := []struct {
		name  string
		level CompressionLevel
	}{
		{"negative", CompressionLevel(-1)},
		{"too_high", CompressionLevel(20)},
		{"way_too_high", CompressionLevel(100)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := EncodeWithLevel(idx, tc.level)
			if err == nil {
				t.Errorf("expected error for compression level %d", tc.level)
			}
		})
	}
}

func TestNestedJSON(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)

	builder.AddDocument(0, []byte(`{"user": {"name": "alice", "address": {"city": "NYC"}}}`))
	builder.AddDocument(1, []byte(`{"user": {"name": "bob", "address": {"city": "LA"}}}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{EQ("$.user.name", "alice")})
	if !result.IsSet(0) || result.IsSet(1) {
		t.Error("nested query failed")
	}

	result = idx.Evaluate([]Predicate{EQ("$.user.address.city", "LA")})
	if result.IsSet(0) || !result.IsSet(1) {
		t.Error("deep nested query failed")
	}
}

func TestArrayJSON(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)

	builder.AddDocument(0, []byte(`{"tags": ["go", "rust"]}`))
	builder.AddDocument(1, []byte(`{"tags": ["python", "java"]}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{EQ("$.tags[*]", "go")})
	if !result.IsSet(0) || result.IsSet(1) {
		t.Error("array wildcard query failed")
	}
}

func TestMultiplePredicates(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 4)

	builder.AddDocument(0, []byte(`{"name": "alice", "age": 30}`))
	builder.AddDocument(1, []byte(`{"name": "alice", "age": 25}`))
	builder.AddDocument(2, []byte(`{"name": "bob", "age": 30}`))
	builder.AddDocument(3, []byte(`{"name": "bob", "age": 25}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{
		EQ("$.name", "alice"),
		GTE("$.age", 28),
	})
	if result.Count() != 1 || !result.IsSet(0) {
		t.Errorf("expected only RG 0, got %v", result.ToSlice())
	}
}

func TestEmptyIndex(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 5)
	idx := builder.Finalize()

	if idx.Header.NumDocs != 0 {
		t.Error("empty index should have 0 docs")
	}

	result := idx.Evaluate([]Predicate{EQ("$.foo", "bar")})
	if result.Count() != 5 {
		t.Error("query on empty index should return all RGs")
	}
}

func TestBloomFastPath(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)

	builder.AddDocument(0, []byte(`{"status": "active"}`))
	builder.AddDocument(1, []byte(`{"status": "inactive"}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{EQ("$.status", "nonexistent")})
	if result.Count() != 0 {
		t.Error("bloom filter should eliminate all RGs for nonexistent value")
	}
}

func TestBooleanValues(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)

	builder.AddDocument(0, []byte(`{"active": true}`))
	builder.AddDocument(1, []byte(`{"active": false}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{EQ("$.active", true)})
	if !result.IsSet(0) || result.IsSet(1) {
		t.Error("boolean true query failed")
	}

	result = idx.Evaluate([]Predicate{EQ("$.active", false)})
	if result.IsSet(0) || !result.IsSet(1) {
		t.Error("boolean false query failed")
	}
}

func TestLargeRGCount(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 100)

	for i := 0; i < 100; i++ {
		builder.AddDocument(DocID(i), []byte(`{"value": "test"}`))
	}

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{EQ("$.value", "test")})
	if result.Count() != 100 {
		t.Errorf("expected 100 matching RGs, got %d", result.Count())
	}
}

func TestTrigramIndex(t *testing.T) {
	ti, err := NewTrigramIndex(3)
	if err != nil {
		t.Fatalf("failed to create trigram index: %v", err)
	}
	ti.Add("hello world", 0)
	ti.Add("hello there", 1)
	ti.Add("goodbye world", 2)

	result := ti.Search("hello")
	if !result.IsSet(0) || !result.IsSet(1) || result.IsSet(2) {
		t.Error("trigram search for 'hello' failed")
	}

	result = ti.Search("world")
	if !result.IsSet(0) || result.IsSet(1) || !result.IsSet(2) {
		t.Error("trigram search for 'world' failed")
	}

	result = ti.Search("xyz")
	if !result.IsEmpty() {
		t.Error("trigram search for non-existent pattern should return empty")
	}
}

func TestTrigramExtraction(t *testing.T) {
	trigrams := ExtractTrigrams("hello")
	if len(trigrams) == 0 {
		t.Error("should extract trigrams from 'hello'")
	}

	hasHel := false
	for _, tg := range trigrams {
		if tg == " he" || tg == "hel" || tg == "ell" || tg == "llo" {
			hasHel = true
			break
		}
	}
	if !hasHel {
		t.Error("expected to find trigrams from 'hello'")
	}
}

func TestHyperLogLog(t *testing.T) {
	hll := MustNewHyperLogLog(12)

	for i := 0; i < 10000; i++ {
		hll.AddString(string(rune('a' + i%26)))
	}

	estimate := hll.Estimate()
	if estimate < 20 || estimate > 35 {
		t.Errorf("HLL estimate for 26 unique values is off: got %d", estimate)
	}
}

func TestHyperLogLogLargeCardinality(t *testing.T) {
	hll := MustNewHyperLogLog(14)

	for i := 0; i < 100000; i++ {
		hll.AddString(fmt.Sprintf("value_%d", i))
	}

	estimate := hll.Estimate()
	diff := float64(estimate) - 100000.0
	errorRate := diff / 100000.0
	if errorRate < -0.05 || errorRate > 0.05 {
		t.Errorf("HLL estimate error too high: got %d, expected ~100000, error rate: %.2f%%", estimate, errorRate*100)
	}
}

func TestHyperLogLogMerge(t *testing.T) {
	hll1 := MustNewHyperLogLog(12)
	hll2 := MustNewHyperLogLog(12)

	for i := 0; i < 1000; i++ {
		hll1.AddString(fmt.Sprintf("set1_%d", i))
	}
	for i := 0; i < 1000; i++ {
		hll2.AddString(fmt.Sprintf("set2_%d", i))
	}

	hll1.Merge(hll2)
	estimate := hll1.Estimate()
	if estimate < 1800 || estimate > 2200 {
		t.Errorf("merged HLL estimate is off: got %d, expected ~2000", estimate)
	}
}

func TestPrefixCompression(t *testing.T) {
	terms := []string{
		"application",
		"application_config",
		"application_data",
		"application_log",
		"database",
		"database_config",
		"database_pool",
	}

	pc := MustNewPrefixCompressor(4)
	blocks := pc.Compress(terms)

	if len(blocks) == 0 {
		t.Error("expected at least one block")
	}

	decompressed := pc.Decompress(blocks)
	if len(decompressed) != len(terms) {
		t.Errorf("decompressed length mismatch: got %d, expected %d", len(decompressed), len(terms))
	}

	for i, term := range decompressed {
		found := false
		for _, orig := range terms {
			if term == orig {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("decompressed term %d '%s' not in original terms", i, term)
		}
	}
}

func TestPrefixCompressionStats(t *testing.T) {
	terms := []string{
		"prefix_a",
		"prefix_ab",
		"prefix_abc",
		"prefix_abcd",
	}

	compressed, original, ratio := CompressionStats(terms)
	if ratio >= 1.0 {
		t.Errorf("compression ratio should be < 1.0 for terms with shared prefixes, got %.2f", ratio)
	}
	t.Logf("Compression: %d bytes -> %d bytes (ratio: %.2f)", original, compressed, ratio)
}

func TestContainsQuery(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)

	builder.AddDocument(0, []byte(`{"description": "hello world application"}`))
	builder.AddDocument(1, []byte(`{"description": "goodbye world service"}`))
	builder.AddDocument(2, []byte(`{"description": "hello universe"}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{Contains("$.description", "hello")})
	if !result.IsSet(0) || result.IsSet(1) || !result.IsSet(2) {
		t.Errorf("CONTAINS 'hello' failed: got RGs %v", result.ToSlice())
	}

	result = idx.Evaluate([]Predicate{Contains("$.description", "world")})
	if !result.IsSet(0) || !result.IsSet(1) || result.IsSet(2) {
		t.Errorf("CONTAINS 'world' failed: got RGs %v", result.ToSlice())
	}
}

func TestContainsWithSerialize(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)

	builder.AddDocument(0, []byte(`{"text": "the quick brown fox"}`))
	builder.AddDocument(1, []byte(`{"text": "lazy dog sleeps"}`))

	idx := builder.Finalize()

	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	result := decoded.Evaluate([]Predicate{Contains("$.text", "quick")})
	if !result.IsSet(0) || result.IsSet(1) {
		t.Error("CONTAINS query on decoded index failed")
	}
}

func TestHLLCardinality(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)

	builder.AddDocument(0, []byte(`{"status": "active"}`))
	builder.AddDocument(0, []byte(`{"status": "inactive"}`))
	builder.AddDocument(0, []byte(`{"status": "pending"}`))
	builder.AddDocument(1, []byte(`{"status": "active"}`))

	idx := builder.Finalize()

	var statusPath *PathEntry
	for i := range idx.PathDirectory {
		if idx.PathDirectory[i].PathName == "$.status" {
			statusPath = &idx.PathDirectory[i]
			break
		}
	}

	if statusPath == nil {
		t.Fatal("$.status path not found")
	}

	if statusPath.Cardinality < 2 || statusPath.Cardinality > 5 {
		t.Errorf("cardinality estimate is off: got %d, expected ~3", statusPath.Cardinality)
	}
}

func TestJSONPathValidation(t *testing.T) {
	validPaths := []string{
		"$",
		"$.foo",
		"$.foo.bar",
		"$.foo.bar.baz",
		"$['foo']",
		`$["foo"]`,
		"$.foo['bar']",
		"$.items[*]",
		"$.foo_bar",
		"$['foo-bar']['baz']",
	}

	for _, path := range validPaths {
		if err := ValidateJSONPath(path); err != nil {
			t.Errorf("expected valid path %q, got error: %v", path, err)
		}
	}

	// Invalid syntax (caught by ojg parser)
	syntaxErrors := []struct {
		path string
		desc string
	}{
		{"", "empty path"},
		{"foo", "missing $ prefix"},
		{"$[", "unclosed bracket"},
		{"$['foo", "unclosed string"},
	}

	for _, tc := range syntaxErrors {
		if err := ValidateJSONPath(tc.path); err == nil {
			t.Errorf("expected invalid path %q (%s) to fail validation", tc.path, tc.desc)
		}
	}

	// Unsupported features (valid JSONPath but not supported by GIN)
	unsupportedPaths := []struct {
		path string
		desc string
	}{
		{"$.items[0]", "array index"},
		{"$.items[123]", "array index"},
		{"$.data[0].name", "array index in path"},
		{"$..foo", "recursive descent"},
		{"$.items[0:5]", "slice notation"},
		{"$.items[?(@.price > 10)]", "filter expression"},
	}

	for _, tc := range unsupportedPaths {
		err := ValidateJSONPath(tc.path)
		if err == nil {
			t.Errorf("expected unsupported path %q (%s) to fail validation", tc.path, tc.desc)
		}
	}
}

func TestJSONPathNormalize(t *testing.T) {
	// Just verify normalization doesn't panic and produces consistent output
	paths := []string{
		"$",
		"$.foo",
		"$.foo.bar",
		"$['foo']",
		"$.items[*]",
	}

	for _, path := range paths {
		result := NormalizePath(path)
		if result == "" {
			t.Errorf("NormalizePath(%q) returned empty string", path)
		}
		// Normalized path should be valid
		if !IsValidJSONPath(result) {
			t.Errorf("NormalizePath(%q) = %q is not valid", path, result)
		}
	}
}

func TestJSONPathCanonicalizeSupportedPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "$.foo", want: "$.foo"},
		{path: "$['foo']", want: "$.foo"},
		{path: `$["foo"]`, want: "$.foo"},
	}

	for _, tc := range tests {
		got, err := canonicalizeSupportedPath(tc.path)
		if err != nil {
			t.Fatalf("canonicalizeSupportedPath(%q) error = %v", tc.path, err)
		}
		if got != tc.want {
			t.Errorf("canonicalizeSupportedPath(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestJSONPathCanonicalizeUnsupportedPath(t *testing.T) {
	unsupported := []string{
		"$.items[0]",
		"$..foo",
		"$.items[0:5]",
		"$.items[?(@.price > 10)]",
	}

	for _, path := range unsupported {
		got, err := canonicalizeSupportedPath(path)
		if err == nil {
			t.Fatalf("canonicalizeSupportedPath(%q) = %q, want error", path, got)
		}
	}
}

func TestIsValidJSONPath(t *testing.T) {
	if !IsValidJSONPath("$.foo.bar") {
		t.Error("expected $.foo.bar to be valid")
	}
	if IsValidJSONPath("invalid") {
		t.Error("expected 'invalid' to be invalid")
	}
}

func TestMustValidateJSONPath(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid path")
		}
	}()
	MustValidateJSONPath("invalid")
}

func TestWithFTSPathsExact(t *testing.T) {
	config, err := NewConfig(WithFTSPaths("$.description"))
	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}
	builder := mustNewBuilder(t, config, 3)

	builder.AddDocument(0, []byte(`{"description": "hello world", "title": "test title"}`))
	builder.AddDocument(1, []byte(`{"description": "goodbye world", "title": "another title"}`))
	builder.AddDocument(2, []byte(`{"description": "hello universe", "title": "hello title"}`))

	idx := builder.Finalize()

	// CONTAINS on $.description should prune (trigrams enabled)
	result := idx.Evaluate([]Predicate{Contains("$.description", "hello")})
	if result.Count() != 2 || !result.IsSet(0) || !result.IsSet(2) {
		t.Errorf("CONTAINS on $.description should prune, got RGs %v", result.ToSlice())
	}

	// CONTAINS on $.title should return all RGs (no trigrams)
	result = idx.Evaluate([]Predicate{Contains("$.title", "hello")})
	if result.Count() != 3 {
		t.Errorf("CONTAINS on $.title should return all RGs (graceful degradation), got %d", result.Count())
	}
}

func TestWithFTSPathsPrefix(t *testing.T) {
	config, err := NewConfig(WithFTSPaths("$.content.*"))
	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}
	builder := mustNewBuilder(t, config, 3)

	builder.AddDocument(0, []byte(`{"content": {"body": "hello world", "summary": "test summary"}, "meta": "hello meta"}`))
	builder.AddDocument(1, []byte(`{"content": {"body": "goodbye world", "summary": "another summary"}, "meta": "world meta"}`))
	builder.AddDocument(2, []byte(`{"content": {"body": "hello universe", "summary": "hello summary"}, "meta": "something"}`))

	idx := builder.Finalize()

	// CONTAINS on $.content.body should prune
	result := idx.Evaluate([]Predicate{Contains("$.content.body", "hello")})
	if result.Count() != 2 || !result.IsSet(0) || !result.IsSet(2) {
		t.Errorf("CONTAINS on $.content.body should prune, got RGs %v", result.ToSlice())
	}

	// CONTAINS on $.content.summary should also prune
	result = idx.Evaluate([]Predicate{Contains("$.content.summary", "hello")})
	if result.Count() != 1 || !result.IsSet(2) {
		t.Errorf("CONTAINS on $.content.summary should prune, got RGs %v", result.ToSlice())
	}

	// CONTAINS on $.meta should return all RGs (no trigrams)
	result = idx.Evaluate([]Predicate{Contains("$.meta", "hello")})
	if result.Count() != 3 {
		t.Errorf("CONTAINS on $.meta should return all RGs, got %d", result.Count())
	}
}

func TestWithFTSPathsCanonicalizesEquivalentSupportedPaths(t *testing.T) {
	config, err := NewConfig(WithFTSPaths("$['description']", `$["content"].*`))
	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}
	builder := mustNewBuilder(t, config, 3)

	builder.AddDocument(0, []byte(`{"description": "hello world", "content": {"body": "hello body"}, "meta": "alpha"}`))
	builder.AddDocument(1, []byte(`{"description": "goodbye world", "content": {"body": "goodbye body"}, "meta": "beta"}`))
	builder.AddDocument(2, []byte(`{"description": "hello universe", "content": {"body": "hello again"}, "meta": "gamma"}`))

	idx := builder.Finalize()

	if got, want := idx.Config.ftsPaths, []string{"$.description", "$.content.*"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("fresh ftsPaths = %v, want %v", got, want)
	}

	assertContainsRGs := func(t *testing.T, queryPath string, value string, want []int) {
		t.Helper()
		result := idx.Evaluate([]Predicate{Contains(queryPath, value)})
		if got := result.ToSlice(); len(got) != len(want) {
			t.Fatalf("Contains(%q, %q) RGs = %v, want %v", queryPath, value, got, want)
		} else {
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("Contains(%q, %q) RGs = %v, want %v", queryPath, value, got, want)
				}
			}
		}
	}

	assertContainsRGs(t, "$.description", "hello", []int{0, 2})
	assertContainsRGs(t, "$.content.body", "hello", []int{0, 2})

	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if got, want := decoded.Config.ftsPaths, []string{"$.description", "$.content.*"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("decoded ftsPaths = %v, want %v", got, want)
	}

	idx = decoded
	assertContainsRGs(t, "$.description", "hello", []int{0, 2})
	assertContainsRGs(t, "$.content.body", "hello", []int{0, 2})
}

func TestWithFTSPathsBackwardCompatible(t *testing.T) {
	// Without WithFTSPaths, all string paths should have trigrams
	config := DefaultConfig()
	builder := mustNewBuilder(t, config, 2)

	builder.AddDocument(0, []byte(`{"description": "hello world", "title": "test title"}`))
	builder.AddDocument(1, []byte(`{"description": "goodbye world", "title": "hello title"}`))

	idx := builder.Finalize()

	// Both paths should support CONTAINS with pruning
	result := idx.Evaluate([]Predicate{Contains("$.description", "hello")})
	if result.Count() != 1 || !result.IsSet(0) {
		t.Errorf("CONTAINS on $.description should prune, got RGs %v", result.ToSlice())
	}

	result = idx.Evaluate([]Predicate{Contains("$.title", "hello")})
	if result.Count() != 1 || !result.IsSet(1) {
		t.Errorf("CONTAINS on $.title should prune, got RGs %v", result.ToSlice())
	}
}

func TestWithFTSPathsOnNumericField(t *testing.T) {
	// Configuring FTS path on a field that has numeric values should gracefully degrade
	config, err := NewConfig(WithFTSPaths("$.score"))
	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}
	builder := mustNewBuilder(t, config, 2)

	builder.AddDocument(0, []byte(`{"score": 100, "name": "alice"}`))
	builder.AddDocument(1, []byte(`{"score": 200, "name": "bob"}`))

	idx := builder.Finalize()

	// CONTAINS on numeric field should return all RGs (graceful degradation)
	result := idx.Evaluate([]Predicate{Contains("$.score", "100")})
	if result.Count() != 2 {
		t.Errorf("CONTAINS on numeric field should return all RGs, got %d", result.Count())
	}
}

func TestBuilderCanonicalizesSupportedPathVariants(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)

	builder.walkJSON("$['foo']", "alpha", 0)
	builder.walkJSON(`$["foo"]`, "beta", 1)

	idx := builder.Finalize()

	if len(idx.PathDirectory) != 1 {
		t.Fatalf("PathDirectory len = %d, want 1", len(idx.PathDirectory))
	}

	entry := idx.PathDirectory[0]
	if entry.PathName != "$.foo" {
		t.Fatalf("PathDirectory[0].PathName = %q, want $.foo", entry.PathName)
	}

	if len(idx.pathLookup) != 1 {
		t.Fatalf("pathLookup len = %d, want 1", len(idx.pathLookup))
	}

	if got, ok := idx.pathLookup["$.foo"]; !ok || got != entry.PathID {
		t.Fatalf("pathLookup[$.foo] = (%d, %v), want (%d, true)", got, ok, entry.PathID)
	}
}

func TestRebuildPathLookupRejectsDuplicateCanonicalPaths(t *testing.T) {
	idx := NewGINIndex()
	idx.PathDirectory = []PathEntry{
		{PathID: 0, PathName: "$.foo"},
		{PathID: 1, PathName: "$['foo']"},
	}

	err := idx.rebuildPathLookup()
	if err == nil {
		t.Fatal("rebuildPathLookup() error = nil, want ErrInvalidFormat")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("rebuildPathLookup() error = %v, want ErrInvalidFormat", err)
	}
}

func TestRebuildPathLookupEmptyDirectory(t *testing.T) {
	idx := NewGINIndex()
	idx.pathLookup = nil

	if err := idx.rebuildPathLookup(); err != nil {
		t.Fatalf("rebuildPathLookup() error = %v", err)
	}
	if idx.pathLookup == nil {
		t.Fatal("pathLookup = nil, want empty map")
	}
	if len(idx.pathLookup) != 0 {
		t.Fatalf("len(pathLookup) = %d, want 0", len(idx.pathLookup))
	}
}

func TestRebuildPathLookupMidDirectoryTruncationPreservesExistingLookupOnError(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)
	builder.AddDocument(0, []byte(`{"foo": "bar", "bar": "baz"}`))
	builder.AddDocument(1, []byte(`{"foo": "qux", "bar": "zap"}`))

	idx := builder.Finalize()

	originalLookup := make(map[string]uint16, len(idx.pathLookup))
	for path, pathID := range idx.pathLookup {
		originalLookup[path] = pathID
	}

	idx.PathDirectory = append(idx.PathDirectory[:1], idx.PathDirectory[2:]...)

	err := idx.rebuildPathLookup()
	if err == nil {
		t.Fatal("rebuildPathLookup() error = nil, want ErrInvalidFormat")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("rebuildPathLookup() error = %v, want ErrInvalidFormat", err)
	}
	if len(idx.pathLookup) != len(originalLookup) {
		t.Fatalf("pathLookup len = %d, want %d", len(idx.pathLookup), len(originalLookup))
	}
	for path, wantPathID := range originalLookup {
		gotPathID, ok := idx.pathLookup[path]
		if !ok || gotPathID != wantPathID {
			t.Fatalf("pathLookup[%q] = (%d, %v), want (%d, true)", path, gotPathID, ok, wantPathID)
		}
	}
}

func TestFindPathCanonicalLookupAndFallback(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)
	builder.AddDocument(0, []byte(`{"foo": "bar"}`))
	builder.AddDocument(1, []byte(`{"foo": "baz"}`))

	idx := builder.Finalize()

	for _, path := range []string{"$.foo", "$['foo']", `$["foo"]`} {
		pathID, entry := idx.findPath(path)
		if pathID != 1 {
			t.Fatalf("findPath(%q) pathID = %d, want 1", path, pathID)
		}
		if entry == nil || entry.PathName != "$.foo" {
			t.Fatalf("findPath(%q) entry = %#v, want canonical $.foo entry", path, entry)
		}
	}

	for _, path := range []string{"$.missing"} {
		pathID, entry := idx.findPath(path)
		if pathID != -1 || entry != nil {
			t.Fatalf("findPath(%q) = (%d, %#v), want (-1, nil)", path, pathID, entry)
		}

		result := idx.Evaluate([]Predicate{EQ(path, "bar")})
		if result.Count() != int(idx.Header.NumRowGroups) {
			t.Fatalf("Evaluate(EQ(%q, bar)) count = %d, want %d", path, result.Count(), idx.Header.NumRowGroups)
		}
	}
}

func TestFindPathPanicsOnInvalidPath(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)
	builder.AddDocument(0, []byte(`{"foo": "bar"}`))
	builder.AddDocument(1, []byte(`{"foo": "baz"}`))

	idx := builder.Finalize()

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("findPath() did not panic for invalid path")
		}

		err, ok := r.(error)
		if !ok {
			t.Fatalf("findPath() panic = %T, want error", r)
		}

		var pathErr *JSONPathError
		if !stderrors.As(err, &pathErr) {
			t.Fatalf("findPath() panic = %v, want *JSONPathError", err)
		}
	}()

	idx.findPath("$.items[0]")
}

func TestQueryEQCanonicalPathDecodeParity(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)
	builder.AddDocument(0, []byte(`{"foo": "x"}`))
	builder.AddDocument(1, []byte(`{"foo": "y"}`))
	builder.AddDocument(2, []byte(`{"foo": "x"}`))

	idx := builder.Finalize()

	assertMatches := func(t *testing.T, current *GINIndex, path string, want []int) {
		t.Helper()
		result := current.Evaluate([]Predicate{EQ(path, "x")})
		got := result.ToSlice()
		if len(got) != len(want) {
			t.Fatalf("Evaluate(EQ(%q, x)) = %v, want %v", path, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("Evaluate(EQ(%q, x)) = %v, want %v", path, got, want)
			}
		}
	}

	for _, path := range []string{"$.foo", "$['foo']", `$["foo"]`} {
		assertMatches(t, idx, path, []int{0, 2})
	}

	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	for _, path := range []string{"$.foo", "$['foo']", `$["foo"]`} {
		assertMatches(t, decoded, path, []int{0, 2})
	}
}

func TestEvaluateUnsupportedPathsPanic(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)
	builder.AddDocument(0, []byte(`{"items": [{"foo": "x"}]}`))
	builder.AddDocument(1, []byte(`{"items": [{"foo": "y"}]}`))

	idx := builder.Finalize()

	for _, path := range []string{"$.items[0]", "$..foo", "$.items[0:5]", "$.items[?(@.price > 10)]"} {
		func() {
			defer func() {
				r := recover()
				if r == nil {
					t.Fatalf("Evaluate(EQ(%q, x)) did not panic", path)
				}

				err, ok := r.(error)
				if !ok {
					t.Fatalf("Evaluate(EQ(%q, x)) panic = %T, want error", path, r)
				}

				var pathErr *JSONPathError
				if !stderrors.As(err, &pathErr) {
					t.Fatalf("Evaluate(EQ(%q, x)) panic = %v, want *JSONPathError", path, err)
				}
			}()

			_ = idx.Evaluate([]Predicate{EQ(path, "x")})
		}()
	}
}

func TestStringLengthIndex(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)
	builder.AddDocument(0, []byte(`{"name": "ab"}`))
	builder.AddDocument(1, []byte(`{"name": "abcdef"}`))
	builder.AddDocument(2, []byte(`{"name": "abcd"}`))
	idx := builder.Finalize()

	sli, ok := idx.StringLengthIndexes[1]
	if !ok {
		t.Fatal("StringLengthIndex should exist for $.name")
	}
	if sli.GlobalMin != 2 {
		t.Errorf("expected GlobalMin=2, got %d", sli.GlobalMin)
	}
	if sli.GlobalMax != 6 {
		t.Errorf("expected GlobalMax=6, got %d", sli.GlobalMax)
	}

	result := idx.Evaluate([]Predicate{EQ("$.name", "a")})
	if !result.IsEmpty() {
		t.Error("len=1 query should return empty (min=2)")
	}

	result = idx.Evaluate([]Predicate{EQ("$.name", "abcdefghij")})
	if !result.IsEmpty() {
		t.Error("len=10 query should return empty (max=6)")
	}

	result = idx.Evaluate([]Predicate{EQ("$.name", "abcd")})
	if !result.IsSet(2) {
		t.Error("len=4 query should match RG 2")
	}
}

func TestStringLengthIndexSerialization(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)
	builder.AddDocument(0, []byte(`{"name": "short"}`))
	builder.AddDocument(1, []byte(`{"name": "mediumname"}`))
	builder.AddDocument(2, []byte(`{"name": "verylongname"}`))
	idx := builder.Finalize()

	data, err := Encode(idx)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	idx2, err := Decode(data)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	sli, ok := idx2.StringLengthIndexes[1]
	if !ok {
		t.Fatal("StringLengthIndex should exist after round-trip")
	}
	if sli.GlobalMin != 5 {
		t.Errorf("expected GlobalMin=5, got %d", sli.GlobalMin)
	}
	if sli.GlobalMax != 12 {
		t.Errorf("expected GlobalMax=12, got %d", sli.GlobalMax)
	}

	result := idx2.Evaluate([]Predicate{EQ("$.name", "ab")})
	if !result.IsEmpty() {
		t.Error("len=2 query should return empty after round-trip (min=5)")
	}
}
