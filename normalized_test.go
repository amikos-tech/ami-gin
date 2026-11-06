package gin

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestWireVersionUnchanged(t *testing.T) {
	if Version != 11 {
		t.Fatalf("Version = %d, want 11: the Normalized change must not bump the wire format (#102)", Version)
	}
}

func allRows(idx *GINIndex) []int {
	rows := make([]int, idx.Header.NumRowGroups)
	for i := range rows {
		rows[i] = i
	}
	return rows
}

func requireRowsSet(t *testing.T, idx *GINIndex, predicate Predicate, want []int) {
	t.Helper()
	got := idx.Evaluate([]Predicate{predicate}).ToSlice()
	if !reflect.DeepEqual(got, want) && (len(got) != 0 || len(want) != 0) {
		t.Fatalf("Evaluate(%+v) = %v, want %v", predicate, got, want)
	}
}

// normalizedVariants returns the same index as a Finalize-built index and as
// Decode(Encode(idx)), each with its Config intact, nil, and replaced.
func normalizedVariants(t *testing.T, build func() *GINIndex) map[string]*GINIndex {
	t.Helper()
	variants := map[string]*GINIndex{}
	for _, source := range []string{"built", "decoded"} {
		for _, config := range []string{"config", "nil-config", "replaced-config"} {
			idx := build()
			if source == "decoded" {
				idx = mustRoundTripIndex(t, idx)
			}
			switch config {
			case "nil-config":
				idx.Config = nil
			case "replaced-config":
				cfg := DefaultConfig()
				idx.Config = &cfg
			}
			variants[source+"/"+config] = idx
		}
	}
	return variants
}

func forEachVariant(t *testing.T, build func() *GINIndex, fn func(t *testing.T, idx *GINIndex)) {
	t.Helper()
	for name, idx := range normalizedVariants(t, build) {
		idx := idx
		t.Run(name, func(t *testing.T) { fn(t, idx) })
	}
}

func TestNormalizedEquivalentToAs(t *testing.T) {
	build := func() *GINIndex { return buildTransformerDriftIndex(t) }
	cases := []struct {
		name       string
		normalized Predicate
		as         Predicate
		want       []int
	}{
		{"lower EQ", EQ("$.name", Normalized("lower", "ALICE")), EQ("$.name", As("lower", "alice")), []int{5}},
		{"domain EQ", EQ("$.email", Normalized("domain", "Bob@EXAMPLE.com")), EQ("$.email", As("domain", "example.com")), []int{5}},
		{"host EQ", EQ("$.url", Normalized("host", "https://EXAMPLE.COM/z")), EQ("$.url", As("host", "example.com")), []int{5}},
		{"lower NE", NE("$.name", Normalized("lower", "ALICE")), NE("$.name", As("lower", "alice")), []int{0, 1, 2, 3, 4}},
		{"lower IN", IN("$.name", Normalized("lower", "ALICE"), Normalized("lower", "\uA7DCABC")), IN("$.name", As("lower", "alice"), As("lower", "\uA7DCabc")), []int{0, 5}},
		{"lower NIN", NIN("$.name", Normalized("lower", "ALICE"), Normalized("lower", "\uA7DCABC")), NIN("$.name", As("lower", "alice"), As("lower", "\uA7DCabc")), []int{1, 2, 3, 4}},
		{"lower GTE", GTE("$.name", Normalized("lower", "ALICE")), GTE("$.name", As("lower", "alice")), nil},
	}
	forEachVariant(t, build, func(t *testing.T, idx *GINIndex) {
		for _, tc := range cases {
			gotN := idx.Evaluate([]Predicate{tc.normalized}).ToSlice()
			gotA := idx.Evaluate([]Predicate{tc.as}).ToSlice()
			if !reflect.DeepEqual(gotN, gotA) {
				t.Errorf("%s: Normalized = %v, As = %v", tc.name, gotN, gotA)
			}
			if tc.want != nil && !reflect.DeepEqual(gotN, tc.want) {
				t.Errorf("%s: Normalized = %v, want %v", tc.name, gotN, tc.want)
			}
		}
		requireTransformerDriftQueries(t, idx, "drift")
	})
}

func buildNormalizedNumericIndex(t *testing.T) *GINIndex {
	t.Helper()
	cfg, err := NewConfig(
		WithISODateTransformer("$.ts", "epoch"),
		WithNumericBucketTransformer("$.n", "bucket", 100),
	)
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	builder := mustNewBuilder(t, cfg, 3)
	docs := []string{
		`{"ts":"2024-01-14T00:00:00Z","n":150}`,
		`{"ts":"2024-01-15T00:00:00Z","n":250}`,
		`{"ts":"2024-01-16T00:00:00Z","n":99}`,
	}
	for i, doc := range docs {
		if err := builder.AddDocument(DocID(i), []byte(doc)); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	return builder.Finalize()
}

func TestNormalizedNumericPreparation(t *testing.T) {
	forEachVariant(t, func() *GINIndex { return buildNormalizedNumericIndex(t) }, func(t *testing.T, idx *GINIndex) {
		gte := GTE("$.ts", Normalized("epoch", "2024-01-15T00:00:00Z"))
		gteAs := GTE("$.ts", As("epoch", float64(1705276800000)))
		requireRowsSet(t, idx, gte, []int{1, 2})
		requireRowsSet(t, idx, gteAs, []int{1, 2})

		eq := EQ("$.n", Normalized("bucket", json.Number("150")))
		eqAs := EQ("$.n", As("bucket", float64(100)))
		requireRowsSet(t, idx, eq, []int{0})
		requireRowsSet(t, idx, eqAs, []int{0})

		in := IN("$.n", Normalized("bucket", json.Number("150")), Normalized("bucket", json.Number("250")))
		requireRowsSet(t, idx, in, []int{0, 1})

		failOpen := []Predicate{
			GTE("$.ts", Normalized("epoch", "garbage")),
			EQ("$.n", Normalized("bucket", "not a number")),
		}
		for _, p := range failOpen {
			requireRowsSet(t, idx, p, allRows(idx))
		}
	})
}

func TestNormalizedFailOpen(t *testing.T) {
	build := func() *GINIndex { return buildTransformerDriftIndex(t) }
	ok := Normalized("domain", "u@Example.COM")
	cases := []struct {
		name      string
		predicate Predicate
	}{
		{"transformer fails", EQ("$.email", Normalized("domain", "nodomain"))},
		{"IN with one failing element", IN("$.email", Normalized("domain", "u@Example.COM"), Normalized("domain", "nodomain"))},
		{"IN mixes As and Normalized", IN("$.email", ok, As("domain", "example.com"))},
		{"IN mixes aliases", IN("$.name", Normalized("lower", "ALICE"), Normalized("other", "ALICE"))},
		{"IN mixes Normalized and plain value", IN("$.name", Normalized("lower", "ALICE"), "alice")},
		{"single Normalized value under IN", Predicate{Path: "$.name", Operator: OpIN, Value: Normalized("lower", "ALICE")}},
		{"single Normalized value under NIN", Predicate{Path: "$.name", Operator: OpNIN, Value: Normalized("lower", "ALICE")}},
		{"list under EQ", Predicate{Path: "$.name", Operator: OpEQ, Value: []any{Normalized("lower", "ALICE")}}},
		{"unknown alias", EQ("$.name", Normalized("missing", "ALICE"))},
		{"unknown path", EQ("$.missing", Normalized("lower", "ALICE"))},
		{"Contains", Predicate{Path: "$.name", Operator: OpContains, Value: Normalized("lower", "ALI")}},
		{"Regex", Predicate{Path: "$.name", Operator: OpRegex, Value: Normalized("lower", "ALI.*")}},
		{"IsNull", Predicate{Path: "$.name", Operator: OpIsNull, Value: Normalized("lower", "ALICE")}},
		{"IsNotNull", Predicate{Path: "$.name", Operator: OpIsNotNull, Value: Normalized("lower", "ALICE")}},
	}
	forEachVariant(t, build, func(t *testing.T, idx *GINIndex) {
		for _, tc := range cases {
			got := idx.Evaluate([]Predicate{tc.predicate}).ToSlice()
			if !reflect.DeepEqual(got, allRows(idx)) {
				t.Errorf("%s: Evaluate = %v, want every row group %v", tc.name, got, allRows(idx))
			}
		}
	})
}

func TestNormalizedCustomTransformer(t *testing.T) {
	custom := func(v any) (any, bool) {
		s, ok := v.(string)
		if !ok {
			return nil, false
		}
		return "x-" + s, true
	}
	cfg, err := NewConfig(WithCustomTransformer("$.name", "prefixed", custom))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	builder := mustNewBuilder(t, cfg, 2)
	for i, doc := range []string{`{"name":"abc"}`, `{"name":"def"}`} {
		if err := builder.AddDocument(DocID(i), []byte(doc)); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	idx := builder.Finalize()

	idx.Config = nil
	requireRowsSet(t, idx, EQ("$.name", Normalized("prefixed", "abc")), []int{0})
	requireRowsSet(t, idx, EQ("$.name", As("prefixed", "x-def")), []int{1})
	requireRowsSet(t, idx, IN("$.name", Normalized("prefixed", "abc"), Normalized("prefixed", "def")), []int{0, 1})

	other := DefaultConfig()
	idx.Config = &other
	requireRowsSet(t, idx, EQ("$.name", Normalized("prefixed", "def")), []int{1})

	if _, err := Encode(idx); err == nil {
		t.Fatal("Encode of an index with a custom transformer succeeded, want a not-serializable error")
	}
}

func TestNormalizedPanicsOnInvalidAlias(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Normalized with an empty alias did not panic")
		}
	}()
	_ = Normalized("", "x")
}

func TestNormalizedHandConstructedIndexFailsOpen(t *testing.T) {
	idx := buildTransformerDriftIndex(t)
	idx.companionTransformers = nil
	requireRowsSet(t, idx, EQ("$.name", Normalized("lower", "ALICE")), allRows(idx))
}
