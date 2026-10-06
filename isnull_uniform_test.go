package gin

import (
	"reflect"
	"testing"
)

// issue89Index is the four-row-group index from issue #89: RG0 holds two
// documents with env, RG1 one document with env, RG2 two documents without
// env, RG3 one document without env.
func issue89Index(t *testing.T) *GINIndex {
	t.Helper()
	return buildAggregated(t, 4, []aggregatedDoc{
		{0, map[string]any{"env": "prod"}},
		{0, map[string]any{"env": "prod"}},
		{1, map[string]any{"env": "prod"}},
		{2, map[string]any{"app": "x"}},
		{2, map[string]any{"app": "y"}},
		{3, map[string]any{"app": "z"}},
	})
}

func TestIsNullUniformIssue89Repro(t *testing.T) {
	idx := issue89Index(t)

	requirePredicateResult(t, idx, []Predicate{IsNull("$.env")}, []int{2, 3}, "IsNull($.env)")
	requirePredicateResult(t, idx, []Predicate{NE("$.env", "prod")}, []int{}, "NE($.env, prod)")
	union := idx.Evaluate([]Predicate{NE("$.env", "prod")}).Union(idx.Evaluate([]Predicate{IsNull("$.env")}))
	if got := union.ToSlice(); !reflect.DeepEqual(got, []int{2, 3}) {
		t.Fatalf("NE union IsNull = %v, want [2 3]", got)
	}
	requirePredicateResult(t, idx, []Predicate{IsNull("$.app")}, []int{0, 1}, "IsNull($.app)")

	// Other operators keep their answers.
	requirePredicateResult(t, idx, []Predicate{EQ("$.env", "prod")}, []int{0, 1}, "EQ($.env, prod)")
	requirePredicateResult(t, idx, []Predicate{IsNotNull("$.env")}, []int{0, 1}, "IsNotNull($.env)")
	requirePredicateResult(t, idx, []Predicate{NIN("$.env", "prod")}, []int{}, "NIN($.env, prod)")
}

func TestIsNullUniformOneDocumentPerRG(t *testing.T) {
	idx := buildAggregated(t, 3, []aggregatedDoc{
		{0, map[string]any{"env": "prod"}},
		{1, map[string]any{"env": nil}},
		{2, map[string]any{"app": "x"}},
	})
	requirePredicateResult(t, idx, []Predicate{IsNull("$.env")}, []int{1, 2}, "IsNull($.env)")
}

func TestIsNullUniformPathAbsentFromEveryDocument(t *testing.T) {
	idx := buildAggregated(t, 3, []aggregatedDoc{
		{0, map[string]any{"x": 1.0}},
		{0, map[string]any{"x": 1.0}},
		{1, map[string]any{"x": 1.0}},
		{2, map[string]any{"env": "p"}},
	})
	requirePredicateResult(t, idx, []Predicate{IsNull("$.env")}, []int{0, 1}, "IsNull($.env)")
}

func TestIsNullUniformMixedRowGroup(t *testing.T) {
	idx := buildAggregated(t, 2, []aggregatedDoc{
		{0, map[string]any{"env": "prod"}},
		{0, map[string]any{"app": "x"}},
		{1, map[string]any{"env": "p"}},
		{1, map[string]any{"env": "p"}},
	})
	// The mixed RG is selected; the RG where every document has env is not.
	requirePredicateResult(t, idx, []Predicate{IsNull("$.env")}, []int{0}, "IsNull($.env)")
}

func TestIsNullUniformEmptyRowGroupsNeverSelected(t *testing.T) {
	// RGs 4 and 5 receive no document.
	idx := buildAggregated(t, 6, []aggregatedDoc{
		{0, map[string]any{"env": "p"}},
		{1, map[string]any{"env": "p"}},
		{1, map[string]any{"env": "p"}},
		{2, map[string]any{"env": "p"}},
		{3, map[string]any{"app": "x"}},
	})
	requirePredicateResult(t, idx, []Predicate{IsNull("$.env")}, []int{3}, "IsNull($.env)")
}

func TestIsNullUniformSparseDocIDs(t *testing.T) {
	idx := buildAggregated(t, 8, []aggregatedDoc{
		{5, map[string]any{"env": "prod"}},
		{5, map[string]any{"env": "prod"}},
		{2, map[string]any{"app": "x"}},
	})
	if got := indexDocIDs(idx, IsNull("$.env")).sorted(); !reflect.DeepEqual(got, []int{2}) {
		t.Errorf("IsNull($.env) DocIDs = %v, want [2]", got)
	}
}

func TestIsNullUniformNestedAndArrayPaths(t *testing.T) {
	idx := buildAggregated(t, 3, []aggregatedDoc{
		{0, map[string]any{"a": nil}},
		{1, map[string]any{"tags": []any{}}},
		{2, map[string]any{"a": map[string]any{"b": 1.0}, "tags": []any{"x"}}},
	})
	requirePredicateResult(t, idx, []Predicate{IsNull("$.a.b")}, []int{0, 1}, "IsNull($.a.b)")
	requirePredicateResult(t, idx, []Predicate{IsNull("$.tags[*]")}, []int{0, 1}, "IsNull($.tags[*])")
}

func TestIsNullUniformUnknownPathReturnsAllRGs(t *testing.T) {
	idx := issue89Index(t)
	requirePredicateResult(t, idx, []Predicate{IsNull("$.nope")}, []int{0, 1, 2, 3}, "IsNull($.nope)")
}

func TestIsNullUniformMissingRootNullIndexFailsOpen(t *testing.T) {
	idx := issue89Index(t)
	delete(idx.NullIndexes, idx.pathLookup["$"])
	requirePredicateResult(t, idx, []Predicate{IsNull("$.env")}, []int{0, 1, 2, 3}, "IsNull($.env)")
}

func TestIsNullUniformMissingPathNullIndexFailsOpen(t *testing.T) {
	idx := issue89Index(t)
	delete(idx.NullIndexes, idx.pathLookup["$.env"])
	requirePredicateResult(t, idx, []Predicate{IsNull("$.env")}, []int{0, 1, 2, 3}, "IsNull($.env)")
}

func TestIsNullUniformMissingRootPathFailsOpen(t *testing.T) {
	idx := issue89Index(t)
	delete(idx.pathLookup, "$")
	requirePredicateResult(t, idx, []Predicate{IsNull("$.env")}, []int{0, 1, 2, 3}, "IsNull($.env)")
}

// A decoded path bitmap may carry fewer row groups than the header; the
// complement must still cover the high row groups.
func TestIsNullUniformShortPathBitmapKeepsHighRowGroups(t *testing.T) {
	idx := issue89Index(t)
	short := MustNewRGSet(2)
	short.Set(0)
	short.Set(1)
	idx.NullIndexes[idx.pathLookup["$.env"]].PresentRGBitmap = short
	requirePredicateResult(t, idx, []Predicate{IsNull("$.env")}, []int{2, 3}, "IsNull($.env)")
}

// assertRootPresenceAllDocShapes checks that every committed document marks
// the root path present, whatever its JSON type, for the given parser.
func assertRootPresenceAllDocShapes(t *testing.T, parser Parser) {
	t.Helper()
	builder, err := NewBuilder(DefaultConfig(), 6, WithParser(parser))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	docs := []string{`null`, `5`, `[]`, `"s"`, `{}`, `{"env":"p"}`}
	for rg, doc := range docs {
		if err := builder.AddDocument(DocID(rg), []byte(doc)); err != nil {
			t.Fatalf("AddDocument(%d, %s): %v", rg, doc, err)
		}
	}
	idx := builder.Finalize()

	rootID, ok := idx.pathLookup["$"]
	if !ok {
		t.Fatal(`root path "$" missing from pathLookup`)
	}
	root, ok := idx.NullIndexes[rootID]
	if !ok {
		t.Fatal(`root path "$" has no NullIndex`)
	}
	if got := root.PresentRGBitmap.ToSlice(); !reflect.DeepEqual(got, []int{0, 1, 2, 3, 4, 5}) {
		t.Fatalf("root PresentRGBitmap = %v, want [0 1 2 3 4 5]", got)
	}
	requirePredicateResult(t, idx, []Predicate{IsNull("$.env")}, []int{0, 1, 2, 3, 4}, "IsNull($.env)")
}

func TestIsNullUniformRootPresenceAllDocShapes(t *testing.T) {
	t.Run("stdlib", func(t *testing.T) {
		assertRootPresenceAllDocShapes(t, stdlibParser{})
	})
	t.Run("materializing", func(t *testing.T) {
		assertRootPresenceAllDocShapes(t, materializingParser{})
	})
}

// TestIsNullUniformOldGoldenFixedOnRead decodes a v11 index written before the
// fix: the one-document row group without the path is now selected on read.
func TestIsNullUniformOldGoldenFixedOnRead(t *testing.T) {
	idx, err := Decode(loadGolden(t, "nulls-and-missing"))
	if err != nil {
		t.Fatalf("Decode golden: %v", err)
	}
	if idx.Header.Version != 11 {
		t.Fatalf("golden Header.Version = %d, want 11", idx.Header.Version)
	}
	requirePredicateResult(t, idx, []Predicate{IsNull("$.b")}, []int{2, 3}, "IsNull($.b)")
	requirePredicateResult(t, idx, []Predicate{IsNull("$.a")}, []int{0, 1, 2}, "IsNull($.a)")
	requirePredicateResult(t, idx, []Predicate{IsNotNull("$.a")}, []int{0, 2, 3}, "IsNotNull($.a)")
}

func TestIsNullUniformRoundTrip(t *testing.T) {
	idx := issue89Index(t)
	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	for _, path := range []string{"$.env", "$.app", "$.nope"} {
		want := idx.Evaluate([]Predicate{IsNull(path)}).ToSlice()
		requirePredicateResult(t, decoded, []Predicate{IsNull(path)}, want, "decoded IsNull("+path+")")
	}
}
