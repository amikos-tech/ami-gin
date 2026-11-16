package gin

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

// Regression tests for #111: trigram gram-eligibility checks count characters,
// and GINConfig.TrigramMinLength above the gram size must never prune a row
// group that holds a match.

var trigramMinLengthLegacyGolden = filepath.Join("testdata", "trigram-min-length", "legacy-min5.gin")

// tmlDocs holds one document per row group. Every fixture keeps a longer term
// on $.s so the trigram index exists (builder.go only keeps indexes with
// grams); without it queries return all row groups and hide the bug.
var tmlDocs = []map[string]any{
	{"s": "abcd", "n": 10, "z": nil},
	{"s": "zzzzzz", "n": 20, "z": "q"},
	{"s": "日本語", "n": 30, "z": "q"},
	{"s": "éééé", "n": 40, "z": "q"},
	{"s": "abcdefgh", "n": 50, "z": "q"},
}

var tmlAllRGs = []int{0, 1, 2, 3, 4}

// buildLegacyTrigramMinLengthEncoded builds and encodes docs with minLen set
// on the builder's own config after NewBuilder. Code without the NewBuilder
// clamp keeps the value as it is, which reproduces what older writers stored;
// code with the clamp is bypassed on purpose.
func buildLegacyTrigramMinLengthEncoded(t *testing.T, cfg GINConfig, minLen int, docs []map[string]any) []byte {
	t.Helper()
	builder := mustNewBuilder(t, cfg, len(docs))
	builder.config.TrigramMinLength = minLen
	for i, fields := range docs {
		raw, err := json.Marshal(fields)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if err := builder.AddDocument(DocID(i), raw); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	encoded, err := Encode(builder.Finalize())
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return encoded
}

func tmlBuild(t *testing.T, cfg GINConfig, docs []map[string]any) *GINIndex {
	t.Helper()
	builder := mustNewBuilder(t, cfg, len(docs))
	for i, fields := range docs {
		raw, err := json.Marshal(fields)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if err := builder.AddDocument(DocID(i), raw); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	return builder.Finalize()
}

func tmlStrings(t *testing.T, values []string) *GINIndex {
	t.Helper()
	docs := make([]map[string]any, len(values))
	for i, v := range values {
		docs[i] = map[string]any{"s": v}
	}
	return tmlBuild(t, DefaultConfig(), docs)
}

func tmlLoadLegacy(t *testing.T) *GINIndex {
	t.Helper()
	data, err := os.ReadFile(trigramMinLengthLegacyGolden)
	if err != nil {
		t.Fatalf("read golden %s: %v (regenerate on pre-fix code via `go test -tags regenerate_goldens -run TestRegenerateTrigramMinLengthGolden .`)", trigramMinLengthLegacyGolden, err)
	}
	idx, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode golden: %v", err)
	}
	return idx
}

func tmlRoundTrip(t *testing.T, idx *GINIndex) *GINIndex {
	t.Helper()
	return mustRoundTripIndex(t, idx)
}

func tmlRows(t *testing.T, idx *GINIndex, pred Predicate, want []int, label string) {
	t.Helper()
	got := idx.Evaluate([]Predicate{pred}).ToSlice()
	if !slices.Equal(got, want) {
		t.Errorf("%s: Evaluate(%v %q) = %v, want %v", label, pred.Operator, pred.Value, got, want)
	}
}

func TestTrigramMinLengthGoldenSanity(t *testing.T) {
	idx := tmlLoadLegacy(t)
	if idx.Config == nil || idx.Config.TrigramMinLength != 5 {
		t.Fatalf("golden Config = %+v, want TrigramMinLength 5", idx.Config)
	}
}

func TestTrigramMinLengthClampNewBuild(t *testing.T) {
	for _, minLen := range []int{5, 4} {
		cfg := DefaultConfig()
		cfg.TrigramMinLength = minLen
		built := tmlBuild(t, cfg, tmlDocs)
		if cfg.TrigramMinLength != minLen {
			t.Errorf("caller cfg.TrigramMinLength = %d after NewBuilder, want %d", cfg.TrigramMinLength, minLen)
		}
		for name, idx := range map[string]*GINIndex{"fresh": built, "decoded": tmlRoundTrip(t, built)} {
			label := name
			tmlRows(t, idx, Contains("$.s", "abc"), []int{0, 4}, label+" Contains abc")
			tmlRows(t, idx, Regex("$.s", "abc"), []int{0, 4}, label+" Regex abc")
			tmlRows(t, idx, Contains("$.s", "ééé"), []int{3}, label+" Contains ééé")
			tmlRows(t, idx, Contains("$.s", "日本語"), []int{2}, label+" Contains 日本語")
		}
		decoded := tmlRoundTrip(t, built)
		if decoded.Config == nil || decoded.Config.TrigramMinLength != 3 {
			t.Errorf("minLen %d: decoded Config = %+v, want TrigramMinLength 3", minLen, decoded.Config)
		}
	}
}

func TestTrigramMinLengthLegacyIndexGuard(t *testing.T) {
	idx := tmlLoadLegacy(t)
	for _, pat := range []string{"abc", "ééé", "日本語", "zzzyyy"} {
		tmlRows(t, idx, Contains("$.s", pat), tmlAllRGs, "Contains "+pat)
		tmlRows(t, idx, Regex("$.s", pat), tmlAllRGs, "Regex "+pat)
	}
	ti := idx.TrigramIndexes[requirePathID(t, idx, "$.s")]
	if ti == nil {
		t.Fatal("no trigram index for $.s")
	}
	if got := ti.Search("abc").ToSlice(); !slices.Equal(got, tmlAllRGs) {
		t.Errorf("direct Search(abc) = %v, want all row groups", got)
	}

	// Controls: other operators keep pruning.
	tmlRows(t, idx, GT("$.n", 35), []int{3, 4}, "GT n")
	tmlRows(t, idx, IsNull("$.z"), []int{0}, "IsNull z")
	tmlRows(t, idx, EQ("$.s", "abcd"), []int{0}, "EQ s")
	requirePredicateResult(t, idx, []Predicate{EQ("$.s", "abcd"), Contains("$.s", "abc")}, []int{0}, "EQ AND Contains")
	requirePredicateResult(t, idx, []Predicate{Contains("$.s", "abc"), EQ("$.s", "abcd")}, []int{0}, "Contains AND EQ")

	// An absent path answers as it does on a default-built decoded index.
	control := tmlRoundTrip(t, tmlBuild(t, DefaultConfig(), tmlDocs))
	want := control.Evaluate([]Predicate{Contains("$.nope", "abc")}).ToSlice()
	tmlRows(t, idx, Contains("$.nope", "abc"), want, "absent path")
}

func TestTrigramMinLengthPerIndexGuard(t *testing.T) {
	docs := []map[string]any{
		{"a": "abc", "b": "abcd"},
		{"a": "zzzzzz", "b": "zzzzzz"},
	}
	idx := tmlBuild(t, DefaultConfig(), docs)
	bID := requirePathID(t, idx, "$.b")
	n4 := MustNewTrigramIndex(len(docs), WithN(4))
	n4.Add("abcd", 0)
	n4.Add("zzzzzz", 1)
	idx.TrigramIndexes[bID] = n4
	idx.Config.TrigramMinLength = 4
	decoded := tmlRoundTrip(t, idx)

	tmlRows(t, decoded, Contains("$.a", "qqq"), []int{0, 1}, "N=3 path guarded")
	tmlRows(t, decoded, Contains("$.b", "qqqq"), nil, "N=4 path keeps pruning")
	tmlRows(t, decoded, Contains("$.b", "abcd"), []int{0}, "N=4 path hit")
}

func TestTrigramMinLengthCompanionGuard(t *testing.T) {
	cfg, err := NewConfig(WithToLowerTransformer("$.name", "lower"))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	docs := []map[string]any{{"name": "Abcd"}, {"name": "Zzzzzz"}, {"name": "Abcdefgh"}}
	idx, err := Decode(buildLegacyTrigramMinLengthEncoded(t, cfg, 5, docs))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	all := []int{0, 1, 2}
	tmlRows(t, idx, Predicate{Path: "$.name", Operator: OpContains, Value: As("lower", "abc")}, all, "companion As")
	tmlRows(t, idx, Contains("$.name", "abc"), all, "raw path")
}

func TestTrigramMinLengthEncodeKeepsGuard(t *testing.T) {
	// (a) Encode, Decode, Encode keeps the stored value and the guard, and
	// the bytes are stable across the round trip.
	legacy := tmlLoadLegacy(t)
	first, err := Encode(legacy)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	again, err := Decode(first)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if again.Config == nil || again.Config.TrigramMinLength != 5 {
		t.Errorf("re-decoded Config = %+v, want TrigramMinLength 5", again.Config)
	}
	tmlRows(t, again, Contains("$.s", "abc"), tmlAllRGs, "re-decoded Contains")
	second, err := Encode(again)
	if err != nil {
		t.Fatalf("second Encode: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Error("bytes differ after an Encode, Decode, Encode round trip")
	}

	// (b) A nil config cannot carry the guard.
	nilCfg := tmlLoadLegacy(t)
	nilCfg.Config = nil
	if _, err := Encode(nilCfg); err == nil {
		t.Error("Encode with nil config succeeded, want error")
	}

	// (c) A config storing a value at or below N cannot carry the guard.
	lowCfg := tmlLoadLegacy(t)
	c := *lowCfg.Config
	c.TrigramMinLength = 3
	lowCfg.Config = &c
	if _, err := Encode(lowCfg); err == nil {
		t.Error("Encode with TrigramMinLength 3 succeeded, want error")
	}

	// (d) Move the guarded trigram index into a fresh default-built index.
	for _, tc := range []struct {
		minLen  int
		wantErr bool
	}{{3, true}, {5, false}} {
		src := tmlLoadLegacy(t)
		fresh := tmlBuild(t, DefaultConfig(), tmlDocs)
		fresh.TrigramIndexes[requirePathID(t, fresh, "$.s")] = src.TrigramIndexes[requirePathID(t, src, "$.s")]
		fresh.Config.TrigramMinLength = tc.minLen
		encoded, err := Encode(fresh)
		if tc.wantErr {
			if err == nil {
				t.Errorf("moved guard with TrigramMinLength %d: Encode succeeded, want error", tc.minLen)
			}
			continue
		}
		if err != nil {
			t.Fatalf("moved guard with TrigramMinLength %d: Encode: %v", tc.minLen, err)
		}
		decoded, err := Decode(encoded)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		tmlRows(t, decoded, Contains("$.s", "abc"), tmlAllRGs, "moved guard Contains")
	}
}

func TestTrigramRuneLengthGate(t *testing.T) {
	values := []string{"éé", "日本", "ab", "zzzzzz", "ééé", "日本語", "Kab"}
	idx := tmlStrings(t, values)
	ti := idx.TrigramIndexes[requirePathID(t, idx, "$.s")]
	if ti == nil {
		t.Fatal("no trigram index for $.s")
	}
	for gram, rgs := range ti.Trigrams {
		for _, rg := range []int{0, 1, 2} {
			if rgs.IsSet(rg) {
				t.Errorf("gram %q has short-term row group %d", gram, rg)
			}
		}
	}
	all := []int{0, 1, 2, 3, 4, 5, 6}
	tmlRows(t, idx, Contains("$.s", "ééé"), []int{4}, "Contains ééé")
	tmlRows(t, idx, Regex("$.s", "ééé"), []int{4}, "Regex ééé")
	tmlRows(t, idx, Contains("$.s", "日本語"), []int{5}, "Contains 日本語")
	tmlRows(t, idx, Regex("$.s", "日本語"), []int{5}, "Regex 日本語")
	got := idx.Evaluate([]Predicate{Contains("$.s", "kab")}).ToSlice()
	if !slices.Contains(got, 6) {
		t.Errorf("Contains kab = %v, want it to include row group 6", got)
	}
	tmlRows(t, idx, Contains("$.s", "éé"), all, "Contains éé")
	tmlRows(t, idx, Regex("$.s", "éé"), all, "Regex éé")
}

func TestTrigramRegexRuneLiterals(t *testing.T) {
	idx := tmlStrings(t, []string{"éé", "日本", "ab", "zzzzzz", "ééé", "日本語", "Kab"})
	all := []int{0, 1, 2, 3, 4, 5, 6}
	tmlRows(t, idx, Regex("$.s", "éé|abcdef"), all, "short alternative")
	tmlRows(t, idx, Regex("$.s", "ééé|zzzzzz"), []int{3, 4}, "long alternatives")
	tmlRows(t, idx, Regex("$.s", "日本|abcdef"), all, "mixed 日本|abcdef")
	// 4 bytes, 2 characters.
	tmlRows(t, idx, Regex("$.s", "éé"), all, "long in bytes, short in characters")
}

func TestAnalyzeRegexMinLengthStaysBytes(t *testing.T) {
	info, err := AnalyzeRegex("éé|abcd")
	if err != nil {
		t.Fatalf("AnalyzeRegex: %v", err)
	}
	if info.MinLength != 4 {
		t.Errorf("AnalyzeRegex(éé|abcd).MinLength = %d, want 4 (bytes)", info.MinLength)
	}
}

func TestTrigramMinLengthAtOrBelowNUnchanged(t *testing.T) {
	queries := []Predicate{
		Contains("$.s", "abc"), Regex("$.s", "abc"), Contains("$.s", "ééé"),
		Contains("$.s", "日本語"), Contains("$.s", "zzzyyy"), Regex("$.s", "abcd|zzzzzz"),
	}
	baseline := tmlBuild(t, DefaultConfig(), tmlDocs)
	baselineBytes, err := Encode(baseline)
	if err != nil {
		t.Fatalf("Encode baseline: %v", err)
	}
	for _, minLen := range []int{3, 0, -7} {
		cfg := DefaultConfig()
		cfg.TrigramMinLength = minLen
		idx := tmlBuild(t, cfg, tmlDocs)
		for _, q := range queries {
			want := baseline.Evaluate([]Predicate{q}).ToSlice()
			got := idx.Evaluate([]Predicate{q}).ToSlice()
			if !reflect.DeepEqual(got, want) {
				t.Errorf("minLen %d: %v %q = %v, want %v", minLen, q.Operator, q.Value, got, want)
			}
		}
		encoded, err := Encode(idx)
		if err != nil {
			t.Fatalf("Encode: %v", err)
		}
		decoded, err := Decode(encoded)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if decoded.Config == nil || decoded.Config.TrigramMinLength != minLen {
			t.Errorf("minLen %d: decoded Config = %+v, want value kept", minLen, decoded.Config)
		}
		if minLen == 3 && !bytes.Equal(encoded, baselineBytes) {
			t.Error("minLen 3: encoded bytes differ from the default config")
		}
	}
}

func TestTrigramMinLengthNilConfigDecode(t *testing.T) {
	idx := tmlBuild(t, DefaultConfig(), tmlDocs)
	idx.Config = nil
	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.Config != nil {
		t.Errorf("decoded Config = %+v, want nil", decoded.Config)
	}
	tmlRows(t, decoded, Contains("$.s", "abc"), []int{0, 4}, "nil config Contains")
}
