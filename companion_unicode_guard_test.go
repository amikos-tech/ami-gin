package gin

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// companionGuardDocs is one document per row group. RG3 has no email and url.
var companionGuardDocs = []map[string]any{
	{"name": "Alice", "email": "a@Example.com", "url": "https://Example.com/", "n": 150},
	{"name": "Bob", "email": "b@Other.org", "url": "https://Other.org/", "n": 250},
	{"name": "Carol", "email": "c@Third.net", "url": "https://Third.net/", "n": 350},
	{"name": "Dave", "n": 450},
}

func companionGuardConfig(t *testing.T) GINConfig {
	t.Helper()
	cfg, err := NewConfig(
		WithToLowerTransformer("$.name", "lower"),
		WithToLowerTransformer("$.name", "folded"),
		WithEmailDomainTransformer("$.email", "domain"),
		WithURLHostTransformer("$.url", "host"),
		// A numeric companion that reuses the alias "lower": the guard must
		// key on the transformer spec, not on the alias name.
		WithNumericBucketTransformer("$.n", "lower", 100),
	)
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	return cfg
}

func buildCompanionGuardIndex(t *testing.T, cfg GINConfig) *GINIndex {
	t.Helper()
	builder := mustNewBuilder(t, cfg, len(companionGuardDocs))
	for i, doc := range companionGuardDocs {
		data, err := json.Marshal(doc)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if err := builder.AddDocument(DocID(i), data); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	return builder.Finalize()
}

// companionGuardIndex builds the fixture, labels it with token, encodes and
// decodes it.
func companionGuardIndex(t *testing.T, cfg GINConfig, token string) *GINIndex {
	t.Helper()
	idx := buildCompanionGuardIndex(t, cfg)
	idx.trigramUnicodeToken = token
	return mustRoundTripIndex(t, idx)
}

// companionGuardCase is one lowering-companion predicate for the guard tests.
// knownPrune says whether the same predicate prunes on an index with a
// supported token; it gets the config's EnableTrigrams setting.
type companionGuardCase struct {
	predicate  Predicate
	knownPrune func(trigrams bool) bool
}

// companionGuardLoweringPredicates covers EQ, NE, IN, NIN and GT through As
// and through Normalized, plus IsNull, Contains and the bloom, string-length
// and empty-string paths through As. The guard must answer each with every row
// group. GT on a string companion never prunes, and Contains prunes only with
// trigrams; those rows are in the expected-no-pruning set of the known index.
func companionGuardLoweringPredicates() map[string]companionGuardCase {
	long := strings.Repeat("x", 600)
	always := func(bool) bool { return true }
	never := func(bool) bool { return false }
	withTrigrams := func(trigrams bool) bool { return trigrams }
	cases := map[string]companionGuardCase{}
	add := func(name string, p Predicate, knownPrune func(bool) bool) {
		cases[name] = companionGuardCase{predicate: p, knownPrune: knownPrune}
	}
	for _, c := range []struct{ path, alias, raw, lowered string }{
		{"$.name", "lower", "ALICE", "alice"},
		{"$.name", "folded", "BOB", "bob"},
		{"$.email", "domain", "x@EXAMPLE.com", "example.com"},
		{"$.url", "host", "https://EXAMPLE.com/", "example.com"},
	} {
		key := c.path + "#" + c.alias
		add("EQ As "+key, EQ(c.path, As(c.alias, c.lowered)), always)
		add("EQ Normalized "+key, EQ(c.path, Normalized(c.alias, c.raw)), always)
		add("NE As "+key, NE(c.path, As(c.alias, c.lowered)), always)
		add("NE Normalized "+key, NE(c.path, Normalized(c.alias, c.raw)), always)
		add("IN As "+key, IN(c.path, As(c.alias, c.lowered)), always)
		add("IN Normalized "+key, IN(c.path, Normalized(c.alias, c.raw)), always)
		add("NIN As "+key, NIN(c.path, As(c.alias, c.lowered)), always)
		add("NIN Normalized "+key, NIN(c.path, Normalized(c.alias, c.raw)), always)
		add("GT As "+key, GT(c.path, As(c.alias, c.lowered)), never)
		add("GT Normalized "+key, GT(c.path, Normalized(c.alias, c.raw)), never)
		add("IsNull As "+key, Predicate{Path: c.path, Operator: OpIsNull, Value: As(c.alias, nil)}, always)
		add("absent from bloom As "+key, EQ(c.path, As(c.alias, "zzz-not-present")), always)
		add("longer than bounds As "+key, EQ(c.path, As(c.alias, long)), always)
		add("empty string As "+key, EQ(c.path, As(c.alias, "")), always)
		add("Contains As "+key, Predicate{Path: c.path, Operator: OpContains, Value: As(c.alias, "zzz")}, withTrigrams)
	}
	return cases
}

func TestCompanionUnicodeGuardLoweringReturnsAllRGs(t *testing.T) {
	noTrigrams := companionGuardConfig(t)
	noTrigrams.EnableTrigrams = false
	for name, cfg := range map[string]GINConfig{
		"trigrams":    companionGuardConfig(t),
		"no-trigrams": noTrigrams,
	} {
		t.Run(name, func(t *testing.T) {
			unknown := companionGuardIndex(t, cfg, trigramUnicodeForeignToken)
			known := companionGuardIndex(t, cfg, "")
			for label, c := range companionGuardLoweringPredicates() {
				if got := unknown.Evaluate([]Predicate{c.predicate}).ToSlice(); !reflect.DeepEqual(got, allRows(unknown)) {
					t.Errorf("%s: unknown token = %v, want every row group", label, got)
				}
				got := known.Evaluate([]Predicate{c.predicate}).ToSlice()
				if c.knownPrune(cfg.EnableTrigrams) {
					if len(got) >= len(allRows(known)) {
						t.Errorf("%s: supported token = %v, want pruning: the guard row proves nothing", label, got)
					}
				} else if !reflect.DeepEqual(got, allRows(known)) {
					t.Errorf("%s: supported token = %v, want every row group (expected no pruning)", label, got)
				}
			}
		})
	}
}

func TestCompanionUnicodeGuardSupportedTokensPruneExactly(t *testing.T) {
	for _, token := range []string{"", trigramUnicodeVersion} {
		idx := companionGuardIndex(t, companionGuardConfig(t), token)
		requireRows(t, idx, EQ("$.name", As("lower", "alice")), []int{0})
		requireRows(t, idx, EQ("$.name", Normalized("lower", "BOB")), []int{1})
		requireRows(t, idx, EQ("$.name", As("lower", "zzz-not-present")), []int{})
		requireRows(t, idx, EQ("$.email", As("domain", "other.org")), []int{1})
		requireRows(t, idx, EQ("$.url", As("host", "third.net")), []int{2})
		requireRows(t, idx, NE("$.name", As("lower", "alice")), []int{1, 2, 3})
	}
}

func TestCompanionUnicodeGuardKeysOnSpecNotAlias(t *testing.T) {
	idx := companionGuardIndex(t, companionGuardConfig(t), trigramUnicodeForeignToken)

	// NumericBucket companion that carries the alias "lower".
	requireRows(t, idx, EQ("$.n", As("lower", float64(100))), []int{0})
	requireRows(t, idx, EQ("$.n", As("lower", float64(900))), []int{})
	requireRows(t, idx, EQ("$.n", Normalized("lower", json.Number("250"))), []int{1})

	// Raw paths keep pruning.
	requireRows(t, idx, EQ("$.name", "Alice"), []int{0})
	requireRows(t, idx, EQ("$.name", "nobody"), []int{})
	requireRows(t, idx, GT("$.n", float64(300)), []int{2, 3})

	// Conjunction: the raw predicate prunes, the lowering one adds nothing.
	got := idx.Evaluate([]Predicate{EQ("$.name", "Bob"), EQ("$.name", As("lower", "alice"))}).ToSlice()
	if !reflect.DeepEqual(got, []int{1}) {
		t.Errorf("conjunction = %v, want [1]", got)
	}

	// Contains and Regex on a raw path stay behind the #99 trigram guard.
	for _, p := range []Predicate{Contains("$.name", "lic"), Regex("$.name", "Ali.*")} {
		requireRows(t, idx, p, allRows(idx))
	}
}

func TestCompanionUnicodeGuardCustomTransformerAliasDomainStillPrunes(t *testing.T) {
	cfg, err := NewConfig(
		WithEmailDomainTransformer("$.email", "domain"),
		WithCustomTransformer("$.name", "domain", func(v any) (any, bool) {
			s, ok := v.(string)
			return "c-" + s, ok
		}),
	)
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	idx := buildCompanionGuardIndex(t, cfg)
	idx.trigramUnicodeToken = trigramUnicodeForeignToken // custom transformers cannot be encoded
	requireRows(t, idx, EQ("$.name", As("domain", "c-Alice")), []int{0})
	requireRows(t, idx, EQ("$.name", As("domain", "c-nobody")), []int{})
	requireRows(t, idx, EQ("$.email", As("domain", "example.com")), allRows(idx))
}

func TestCompanionUnicodeGuardTokenRoundTrip(t *testing.T) {
	idx := companionGuardIndex(t, companionGuardConfig(t), trigramUnicodeForeignToken)
	again := mustRoundTripIndex(t, idx)
	if again.trigramUnicodeToken != trigramUnicodeForeignToken {
		t.Fatalf("token after round trip = %q, want %q", again.trigramUnicodeToken, trigramUnicodeForeignToken)
	}
	requireRows(t, again, EQ("$.name", As("lower", "alice")), allRows(again))
	requireRows(t, again, EQ("$.name", Normalized("lower", "ALICE")), allRows(again))
}

func TestIsLoweringSpec(t *testing.T) {
	tests := []struct {
		name string
		spec TransformerSpec
		want bool
	}{
		{"id only to_lower, name empty", TransformerSpec{ID: TransformerToLower}, true},
		{"id only email_domain, name foreign", TransformerSpec{ID: TransformerEmailDomain, Name: "something"}, true},
		{"id only url_host", TransformerSpec{ID: TransformerURLHost, Name: "numeric_bucket"}, true},
		{"name only to_lower", TransformerSpec{Name: "to_lower"}, true},
		{"name only email_domain", TransformerSpec{Name: "email_domain"}, true},
		{"name only url_host", TransformerSpec{Name: "url_host"}, true},
		{"both", TransformerSpec{ID: TransformerToLower, Name: "to_lower"}, true},
		{"numeric bucket", TransformerSpec{ID: TransformerNumericBucket, Name: "numeric_bucket"}, false},
		{"custom with another name", TransformerSpec{ID: TransformerUnknown, Name: "custom"}, false},
		{"zero spec", TransformerSpec{}, false},
	}
	for _, tc := range tests {
		if got := isLoweringSpec(tc.spec); got != tc.want {
			t.Errorf("%s: isLoweringSpec = %t, want %t", tc.name, got, tc.want)
		}
	}
}

// encodeTamperedCompanionGuardIndex builds the fixture with the foreign token
// and lets mutate change the index before it is encoded.
func encodeTamperedCompanionGuardIndex(t *testing.T, mutate func(idx *GINIndex)) []byte {
	t.Helper()
	idx := buildCompanionGuardIndex(t, companionGuardConfig(t))
	idx.trigramUnicodeToken = trigramUnicodeForeignToken
	mutate(idx)
	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return encoded
}

func lowerRepresentation(t *testing.T, idx *GINIndex) *RepresentationSpec {
	t.Helper()
	for i := range idx.representations {
		if idx.representations[i].SourcePath == "$.name" && idx.representations[i].Alias == lowerAlias {
			return &idx.representations[i]
		}
	}
	t.Fatal("representation $.name#lower missing")
	return nil
}

func TestCompanionUnicodeGuardTamperedRepresentationRejected(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(rep *RepresentationSpec)
	}{
		{"zero ID", func(rep *RepresentationSpec) { rep.Transformer.ID = TransformerUnknown }},
		{"different registry ID", func(rep *RepresentationSpec) { rep.Transformer.ID = TransformerNumericBucket }},
		{"different name", func(rep *RepresentationSpec) { rep.Transformer.Name = "numeric_bucket" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encoded := encodeTamperedCompanionGuardIndex(t, func(idx *GINIndex) { tc.mutate(lowerRepresentation(t, idx)) })
			_, err := Decode(encoded)
			if err == nil {
				t.Fatal("Decode accepted a representation that disagrees with the config section")
			}
			if !errors.Is(err, ErrInvalidFormat) {
				t.Errorf("Decode error = %v, want ErrInvalidFormat", err)
			}
			if !strings.Contains(err.Error(), "disagrees") || !strings.Contains(err.Error(), "$.name") || !strings.Contains(err.Error(), "lower") {
				t.Errorf("Decode error %q should name the source path and alias", err)
			}
		})
	}
}

func TestCompanionUnicodeGuardRepresentationOnlyStillGuards(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(rep *RepresentationSpec)
	}{
		{"spec intact", func(rep *RepresentationSpec) {}},
		{"ID zeroed, name intact", func(rep *RepresentationSpec) { rep.Transformer.ID = TransformerUnknown }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The config section does not list the companion; only the
			// representation section does.
			encoded := encodeTamperedCompanionGuardIndex(t, func(idx *GINIndex) {
				tc.mutate(lowerRepresentation(t, idx))
				idx.Config.representationSpecs["$.name"] = nil
				idx.Config.representationTransformers["$.name"] = nil
			})
			idx, err := Decode(encoded)
			if err != nil {
				t.Fatalf("Decode: %v (an absent config entry must not fail Decode)", err)
			}
			requireRows(t, idx, EQ("$.name", As("lower", "alice")), allRows(idx))
			requireRows(t, idx, EQ("$.name", Normalized("lower", "ALICE")), allRows(idx))
			requireRows(t, idx, EQ("$.name", "nobody"), []int{})
		})
	}
}

// TestCompanionTransformerRepresentationOnlySupportedToken covers the
// registry fallback under a supported token. The config section does not
// list the companion, so the transformer comes from the representation
// section alone.
func TestCompanionTransformerRepresentationOnlySupportedToken(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(rep *RepresentationSpec)
		want   []int
	}{
		{"spec intact reconstructs from the registry", func(rep *RepresentationSpec) {}, []int{0}},
		{"ID zeroed leaves no function and fails open", func(rep *RepresentationSpec) { rep.Transformer.ID = TransformerUnknown }, []int{0, 1, 2, 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			idx := buildCompanionGuardIndex(t, companionGuardConfig(t))
			tc.mutate(lowerRepresentation(t, idx))
			idx.Config.representationSpecs["$.name"] = nil
			idx.Config.representationTransformers["$.name"] = nil
			encoded, err := Encode(idx)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			decoded, err := Decode(encoded)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			requireRows(t, decoded, EQ("$.name", Normalized("lower", "ALICE")), tc.want)
			requireRows(t, decoded, EQ("$.name", As("lower", "alice")), []int{0})
		})
	}
}

func TestCompanionTransformerParamsMismatchRejected(t *testing.T) {
	idx := buildCompanionGuardIndex(t, companionGuardConfig(t))
	found := false
	for i := range idx.representations {
		rep := &idx.representations[i]
		if rep.SourcePath == "$.n" && rep.Alias == lowerAlias {
			if rep.Transformer.ID != TransformerNumericBucket {
				t.Fatalf("$.n#lower transformer = %v, want NumericBucket", rep.Transformer.ID)
			}
			rep.Transformer.Params = json.RawMessage(`{"size":200}`)
			found = true
		}
	}
	if !found {
		t.Fatal("representation $.n#lower missing")
	}
	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	_, err = Decode(encoded)
	if err == nil {
		t.Fatal("Decode accepted representation params that disagree with the config section")
	}
	if !errors.Is(err, ErrInvalidFormat) {
		t.Errorf("Decode error = %v, want ErrInvalidFormat", err)
	}
	if !strings.Contains(err.Error(), "params") || !strings.Contains(err.Error(), "$.n") {
		t.Errorf("Decode error %q should name the params and the source path", err)
	}
}

func TestTransformerParamsEqual(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"", "", true},
		{"", "null", true},
		{`{"size":100}`, `{"size":100}`, true},
		{`{"size":100}`, `{ "size" : 100.0 }`, true},
		{`{"pattern":"a","group":1}`, `{"group":1,"pattern":"a"}`, true},
		{`{"size":100}`, `{"size":200}`, false},
		{`{"size":100}`, "", false},
		{"not json", "not json", true},
		{"not json", `{"size":100}`, false},
	}
	for _, tc := range tests {
		if got := transformerParamsEqual(json.RawMessage(tc.a), json.RawMessage(tc.b)); got != tc.want {
			t.Errorf("transformerParamsEqual(%q, %q) = %t, want %t", tc.a, tc.b, got, tc.want)
		}
	}
}
