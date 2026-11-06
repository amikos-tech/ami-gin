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
		requireRows(t, idx, gte, []int{1, 2})
		requireRows(t, idx, gteAs, []int{1, 2})

		eq := EQ("$.n", Normalized("bucket", json.Number("150")))
		eqAs := EQ("$.n", As("bucket", float64(100)))
		requireRows(t, idx, eq, []int{0})
		requireRows(t, idx, eqAs, []int{0})

		in := IN("$.n", Normalized("bucket", json.Number("150")), Normalized("bucket", json.Number("250")))
		requireRows(t, idx, in, []int{0, 1})

		failOpen := []Predicate{
			GTE("$.ts", Normalized("epoch", "garbage")),
			EQ("$.n", Normalized("bucket", "not a number")),
		}
		for _, p := range failOpen {
			requireRows(t, idx, p, allRows(idx))
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
		{"NIN with one failing element", NIN("$.email", Normalized("domain", "u@Example.COM"), Normalized("domain", "nodomain"))},
		{"IN mixes As first and Normalized", IN("$.name", As("lower", "alice"), Normalized("lower", "KELVIN"))},
		{"NIN mixes As first and Normalized", NIN("$.name", As("lower", "alice"), Normalized("lower", "KELVIN"))},
		{"literal with empty alias", EQ("$.name", NormalizedValue{Alias: "", Value: "ALICE"})},
		{"literal with alias containing #", EQ("$.name", NormalizedValue{Alias: "lower#x", Value: "ALICE"})},
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
	requireRows(t, idx, EQ("$.name", Normalized("prefixed", "abc")), []int{0})
	requireRows(t, idx, EQ("$.name", As("prefixed", "x-def")), []int{1})
	requireRows(t, idx, IN("$.name", Normalized("prefixed", "abc"), Normalized("prefixed", "def")), []int{0, 1})

	other := DefaultConfig()
	idx.Config = &other
	requireRows(t, idx, EQ("$.name", Normalized("prefixed", "def")), []int{1})

	if _, err := Encode(idx); err == nil {
		t.Fatal("Encode of an index with a custom transformer succeeded, want a not-serializable error")
	}
}

// TestNormalizedRecoversTransformerPanic: ingest passes JSON float64, so this
// transformer never panics there. A Go int query value makes it panic; the
// predicate must return every row group, not crash Evaluate.
func TestNormalizedRecoversTransformerPanic(t *testing.T) {
	toMs := func(v any) (any, bool) {
		return v.(float64) * 1000, true
	}
	cfg, err := NewConfig(WithCustomTransformer("$.secs", "ms", toMs))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	builder := mustNewBuilder(t, cfg, 3)
	for i, doc := range []string{`{"secs":1}`, `{"secs":2}`, `{"secs":3}`} {
		if err := builder.AddDocument(DocID(i), []byte(doc)); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	idx := builder.Finalize()

	requireRows(t, idx, EQ("$.secs", As("ms", float64(2000))), []int{1})
	requireRows(t, idx, EQ("$.secs", Normalized("ms", float64(2))), []int{1})
	for name, predicate := range map[string]Predicate{
		"EQ int":       EQ("$.secs", Normalized("ms", 2)),
		"GT int":       GT("$.secs", Normalized("ms", 2)),
		"IN int last":  IN("$.secs", Normalized("ms", float64(1)), Normalized("ms", 2)),
		"NIN int only": NIN("$.secs", Normalized("ms", 2)),
	} {
		t.Run(name, func(t *testing.T) {
			requireRows(t, idx, predicate, allRows(idx))
		})
	}
}

// TestNormalizedNilTransformedValueFailsOpen: a custom transformer may return
// (nil, true). The pruning code has no rule for a nil value, so every operator
// must return every row group; nothing may prune.
func TestNormalizedNilTransformedValueFailsOpen(t *testing.T) {
	toNil := func(v any) (any, bool) {
		s, ok := v.(string)
		if !ok {
			return nil, false
		}
		if s == "N" {
			return nil, true
		}
		return s, true
	}
	cfg, err := NewConfig(WithCustomTransformer("$.k", "c", toNil))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	builder := mustNewBuilder(t, cfg, 3)
	for i, doc := range []string{`{"k":"N"}`, `{"k":"x"}`, `{"k":"y"}`} {
		if err := builder.AddDocument(DocID(i), []byte(doc)); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	idx := builder.Finalize()

	requireRows(t, idx, EQ("$.k", As("c", "x")), []int{1})
	requireRows(t, idx, EQ("$.k", Normalized("c", "x")), []int{1})
	for name, predicate := range map[string]Predicate{
		"EQ":  EQ("$.k", Normalized("c", "N")),
		"NE":  NE("$.k", Normalized("c", "N")),
		"GT":  GT("$.k", Normalized("c", "N")),
		"IN":  IN("$.k", Normalized("c", "N"), Normalized("c", "x")),
		"NIN": NIN("$.k", Normalized("c", "N")),
	} {
		t.Run(name, func(t *testing.T) {
			requireRows(t, idx, predicate, allRows(idx))
		})
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
	requireRows(t, idx, EQ("$.name", Normalized("lower", "ALICE")), allRows(idx))
}

func buildNormalizedRegistryIndex(t *testing.T) *GINIndex {
	t.Helper()
	cfg, err := NewConfig(
		WithDateTransformer("$.date", "day"),
		WithCustomDateTransformer("$.stamp", "stamp", "2006/01/02 15:04"),
		WithIPv4Transformer("$.ip", "ip"),
		WithSemVerTransformer("$.ver", "ver"),
		WithRegexExtractTransformer("$.log", "code", `ERROR\[(\w+)\]:`, 1),
		WithRegexExtractIntTransformer("$.order", "num", `order-(\d+)`, 1),
		WithDurationTransformer("$.dur", "ms"),
		WithBoolNormalizeTransformer("$.flag", "flag"),
	)
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	docs := []string{
		`{"date":"2024-01-14","stamp":"2024/01/14 10:00","ip":"10.0.0.1","ver":"v1.2.3","log":"ERROR[db]: x","order":"order-100","dur":"1h","flag":"yes"}`,
		`{"date":"2024-01-15","stamp":"2024/01/15 10:00","ip":"10.0.0.200","ver":"v2.0.0","log":"ERROR[net]: y","order":"order-200","dur":"30m","flag":"no"}`,
		`{"date":"2024-01-16","stamp":"2024/01/16 10:00","ip":"192.168.1.1","ver":"v3.1.0","log":"ERROR[db]: z","order":"order-300","dur":"2h","flag":"on"}`,
		`{"other":1}`,
	}
	builder := mustNewBuilder(t, cfg, len(docs))
	for i, doc := range docs {
		if err := builder.AddDocument(DocID(i), []byte(doc)); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	return builder.Finalize()
}

// TestNormalizedEqualsAsForRegistryTransformers covers the registry
// transformers that the drift and numeric fixtures do not. Each row compares
// Normalized(alias, raw) with As(alias, T(raw)), where T is the exported
// transformer, on built and decoded indexes, and requires an exact pruned set.
func TestNormalizedEqualsAsForRegistryTransformers(t *testing.T) {
	transformed := func(fn FieldTransformer, raw any) any {
		t.Helper()
		v, ok := fn(raw)
		if !ok {
			t.Fatalf("transformer failed on %v", raw)
		}
		return v
	}
	cases := []struct {
		name       string
		normalized Predicate
		as         Predicate
		want       []int
	}{
		{"DateToEpochMs EQ", EQ("$.date", Normalized("day", "2024-01-15")),
			EQ("$.date", As("day", transformed(DateToEpochMs, "2024-01-15"))), []int{1}},
		{"CustomDateToEpochMs GTE", GTE("$.stamp", Normalized("stamp", "2024/01/15 10:00")),
			GTE("$.stamp", As("stamp", transformed(CustomDateToEpochMs("2006/01/02 15:04"), "2024/01/15 10:00"))), []int{1, 2}},
		{"IPv4ToInt GTE", GTE("$.ip", Normalized("ip", "10.0.0.100")),
			GTE("$.ip", As("ip", transformed(IPv4ToInt, "10.0.0.100"))), []int{1, 2}},
		{"IPv4ToInt LTE", LTE("$.ip", Normalized("ip", "10.0.0.255")),
			LTE("$.ip", As("ip", transformed(IPv4ToInt, "10.0.0.255"))), []int{0, 1}},
		{"SemVerToInt EQ", EQ("$.ver", Normalized("ver", "v2.0.0")),
			EQ("$.ver", As("ver", transformed(SemVerToInt, "v2.0.0"))), []int{1}},
		{"RegexExtract EQ", EQ("$.log", Normalized("code", "ERROR[db]: other")),
			EQ("$.log", As("code", transformed(RegexExtract(`ERROR\[(\w+)\]:`, 1), "ERROR[db]: other"))), []int{0, 2}},
		{"RegexExtractInt EQ", EQ("$.order", Normalized("num", "order-200")),
			EQ("$.order", As("num", transformed(RegexExtractInt(`order-(\d+)`, 1), "order-200"))), []int{1}},
		{"DurationToMs GT", GT("$.dur", Normalized("ms", "45m")),
			GT("$.dur", As("ms", transformed(DurationToMs, "45m"))), []int{0, 2}},
		{"BoolNormalize EQ", EQ("$.flag", Normalized("flag", "on")),
			EQ("$.flag", As("flag", transformed(BoolNormalize, "on"))), []int{0, 2}},
	}
	forEachVariant(t, func() *GINIndex { return buildNormalizedRegistryIndex(t) }, func(t *testing.T, idx *GINIndex) {
		for _, tc := range cases {
			gotN := idx.Evaluate([]Predicate{tc.normalized}).ToSlice()
			gotA := idx.Evaluate([]Predicate{tc.as}).ToSlice()
			if !reflect.DeepEqual(gotN, gotA) {
				t.Errorf("%s: Normalized = %v, As = %v", tc.name, gotN, gotA)
			}
			if !reflect.DeepEqual(gotN, tc.want) {
				t.Errorf("%s: Normalized = %v, want %v", tc.name, gotN, tc.want)
			}
			if len(gotN) >= len(allRows(idx)) {
				t.Errorf("%s: Normalized = %v does not prune", tc.name, gotN)
			}
		}
	})
}
