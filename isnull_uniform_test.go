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

func evalSlice(idx *GINIndex, p Predicate) []int {
	return idx.Evaluate([]Predicate{p}).ToSlice()
}

func assertSlice(t *testing.T, label string, got, want []int) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

func TestIsNullUniformIssue89Repro(t *testing.T) {
	idx := issue89Index(t)

	assertSlice(t, "IsNull($.env)", evalSlice(idx, IsNull("$.env")), []int{2, 3})
	assertSlice(t, "NE($.env, prod)", evalSlice(idx, NE("$.env", "prod")), []int{})
	union := idx.Evaluate([]Predicate{NE("$.env", "prod")}).Union(idx.Evaluate([]Predicate{IsNull("$.env")}))
	assertSlice(t, "NE union IsNull", union.ToSlice(), []int{2, 3})
	assertSlice(t, "IsNull($.app)", evalSlice(idx, IsNull("$.app")), []int{0, 1})

	// Other operators keep their answers.
	assertSlice(t, "EQ($.env, prod)", evalSlice(idx, EQ("$.env", "prod")), []int{0, 1})
	assertSlice(t, "IsNotNull($.env)", evalSlice(idx, IsNotNull("$.env")), []int{0, 1})
	assertSlice(t, "NIN($.env, prod)", evalSlice(idx, NIN("$.env", "prod")), []int{})
}

func TestIsNullUniformOneDocumentPerRG(t *testing.T) {
	idx := buildAggregated(t, 3, []aggregatedDoc{
		{0, map[string]any{"env": "prod"}},
		{1, map[string]any{"env": nil}},
		{2, map[string]any{"app": "x"}},
	})
	assertSlice(t, "IsNull($.env)", evalSlice(idx, IsNull("$.env")), []int{1, 2})
}

func TestIsNullUniformPathAbsentFromEveryDocument(t *testing.T) {
	idx := buildAggregated(t, 3, []aggregatedDoc{
		{0, map[string]any{"x": 1.0}},
		{0, map[string]any{"x": 1.0}},
		{1, map[string]any{"x": 1.0}},
		{2, map[string]any{"env": "p"}},
	})
	assertSlice(t, "IsNull($.env)", evalSlice(idx, IsNull("$.env")), []int{0, 1})
}

func TestIsNullUniformMixedRowGroup(t *testing.T) {
	idx := buildAggregated(t, 2, []aggregatedDoc{
		{0, map[string]any{"env": "prod"}},
		{0, map[string]any{"app": "x"}},
		{1, map[string]any{"env": "p"}},
		{1, map[string]any{"env": "p"}},
	})
	// The mixed RG is selected; the RG where every document has env is not.
	assertSlice(t, "IsNull($.env)", evalSlice(idx, IsNull("$.env")), []int{0})
}

func TestIsNullUniformEmptyRowGroupsNeverSelected(t *testing.T) {
	idx := buildAggregated(t, 6, []aggregatedDoc{
		{0, map[string]any{"env": "p"}},
		{1, map[string]any{"env": "p"}},
		{1, map[string]any{"env": "p"}},
		{2, map[string]any{"env": "p"}},
		{3, map[string]any{"app": "x"}},
	})
	got := evalSlice(idx, IsNull("$.env"))
	assertSlice(t, "IsNull($.env)", got, []int{3})
	for _, rg := range got {
		if rg == 4 || rg == 5 {
			t.Errorf("IsNull($.env) selected RG %d that received no document", rg)
		}
	}
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
	assertSlice(t, "IsNull($.a.b)", evalSlice(idx, IsNull("$.a.b")), []int{0, 1})
	assertSlice(t, "IsNull($.tags[*])", evalSlice(idx, IsNull("$.tags[*]")), []int{0, 1})
}

func TestIsNullUniformUnknownPathReturnsAllRGs(t *testing.T) {
	idx := issue89Index(t)
	assertSlice(t, "IsNull($.nope)", evalSlice(idx, IsNull("$.nope")), []int{0, 1, 2, 3})
}

func TestIsNullUniformMissingRootNullIndexFailsOpen(t *testing.T) {
	idx := issue89Index(t)
	delete(idx.NullIndexes, idx.pathLookup["$"])
	assertSlice(t, "IsNull($.env)", evalSlice(idx, IsNull("$.env")), []int{0, 1, 2, 3})
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
	assertSlice(t, "root PresentRGBitmap", root.PresentRGBitmap.ToSlice(), []int{0, 1, 2, 3, 4, 5})
	assertSlice(t, "IsNull($.env)", evalSlice(idx, IsNull("$.env")), []int{0, 1, 2, 3, 4})
}

func TestIsNullUniformRootPresenceAllDocShapes(t *testing.T) {
	t.Run("stdlib", func(t *testing.T) {
		assertRootPresenceAllDocShapes(t, stdlibParser{})
	})
	t.Run("materializing", func(t *testing.T) {
		assertRootPresenceAllDocShapes(t, materializingParser{})
	})
}

func TestIsNullUniformFormatUnchanged(t *testing.T) {
	if Version != 11 {
		t.Fatalf("Version = %d, want 11: the IsNull fix changes no wire format", Version)
	}
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
	assertSlice(t, "IsNull($.b)", evalSlice(idx, IsNull("$.b")), []int{2, 3})
	assertSlice(t, "IsNull($.a)", evalSlice(idx, IsNull("$.a")), []int{0, 1, 2})
	assertSlice(t, "IsNotNull($.a)", evalSlice(idx, IsNotNull("$.a")), []int{0, 2, 3})
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
		want := evalSlice(idx, IsNull(path))
		got := evalSlice(decoded, IsNull(path))
		assertSlice(t, "decoded IsNull("+path+")", got, want)
	}
}
