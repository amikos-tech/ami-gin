package gin

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var trigramUnicodeUnknownGolden = filepath.Join("testdata", "trigram-unicode", "unknown-version.gin")

// trigramUnicodeForeignToken stands in for a table version this reader does
// not know, as written by a future ami-gin.
const trigramUnicodeForeignToken = "17.0.0"

var trigramUnicodeVersionDocs = []map[string]string{
	{"v": "hello world", "k": "a"},
	{"v": "other text", "k": "b"},
}

// buildTrigramUnicodeIndex builds an index with one document per row group.
func buildTrigramUnicodeIndex(t *testing.T, cfg GINConfig, docs []map[string]string) *GINIndex {
	t.Helper()
	builder := mustNewBuilder(t, cfg, len(docs))
	for i, fields := range docs {
		doc, err := json.Marshal(fields)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		if err := builder.AddDocument(DocID(i), doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	return builder.Finalize()
}

// encodeWithTrigramToken builds the fixture, labels it with token as a
// foreign writer would, and encodes it.
func encodeWithTrigramToken(t *testing.T, token string) []byte {
	t.Helper()
	idx := buildTrigramUnicodeIndex(t, DefaultConfig(), trigramUnicodeVersionDocs)
	idx.trigramUnicodeToken = token
	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return encoded
}

func decodeWithTrigramToken(t *testing.T, token string) *GINIndex {
	t.Helper()
	idx, err := Decode(encodeWithTrigramToken(t, token))
	if err != nil {
		t.Fatalf("Decode with token %q: %v", token, err)
	}
	return idx
}

func encodedConfigJSON(t *testing.T, idx *GINIndex) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := writeConfig(&buf, idx.Config, idx.trigramUnicodeToken); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}
	return buf.Bytes()
}

// requireTrigramSearch checks a direct TrigramIndex.Search on $.v; an empty
// want matches an empty result.
func requireTrigramSearch(t *testing.T, idx *GINIndex, pattern string, want []int) {
	t.Helper()
	ti, ok := idx.TrigramIndexes[requirePathID(t, idx, "$.v")]
	if !ok {
		t.Fatal("no trigram index for $.v")
	}
	if got := ti.Search(pattern).ToSlice(); !slices.Equal(got, want) {
		t.Fatalf("Search(%q) = %v, want %v", pattern, got, want)
	}
}

// trigramPruningQueries all prune on a supported index. Every regex literal
// is at least N runes long, so evaluateRegex reaches TrigramIndex.Search.
var trigramPruningQueries = []struct {
	pred   Predicate
	pruned []int
}{
	{Contains("$.v", "hello"), []int{0}},
	{Regex("$.v", "hello|xyzzy"), []int{0}},
	{Regex("$.v", "hello.*world"), []int{0}},
	{Contains("$.v", "zzzzzz"), nil},
}

func TestTrigramUnicodeVersionFreshIndexWritesNoKey(t *testing.T) {
	if Version != 11 {
		t.Fatalf("wire Version = %d, want 11", Version)
	}
	cfg := DefaultConfig()
	if bytes.Contains(encodedConfigJSON(t, &GINIndex{Config: &cfg}), []byte("trigram_unicode_version")) {
		t.Errorf("writeConfig(DefaultConfig) wrote trigram_unicode_version")
	}
	idx := buildTrigramUnicodeIndex(t, DefaultConfig(), trigramUnicodeVersionDocs)
	if bytes.Contains(encodedConfigJSON(t, idx), []byte("trigram_unicode_version")) {
		t.Errorf("writeConfig of a freshly built index wrote trigram_unicode_version")
	}
	if v, ok := idx.TrigramUnicodeVersion(); v != trigramUnicodeVersion || !ok {
		t.Errorf("TrigramUnicodeVersion() = (%q, %t), want (%q, true)", v, ok, trigramUnicodeVersion)
	}
}

func TestTrigramUnicodeVersionReadConfigKeepsUnknownToken(t *testing.T) {
	buf := mustMarshalLengthPrefixedConfig(t, SerializedConfig{TrigramUnicodeVersion: trigramUnicodeForeignToken})
	_, token, err := readConfig(&buf)
	if err != nil {
		t.Fatalf("readConfig with unknown token: %v", err)
	}
	if token != trigramUnicodeForeignToken {
		t.Errorf("token = %q, want %q", token, trigramUnicodeForeignToken)
	}
}

func TestTrigramUnicodeVersionUnknownTokenDisablesTrigramPruning(t *testing.T) {
	controls := map[string]*GINIndex{
		"absent":   decodeWithTrigramToken(t, ""),
		"explicit": decodeWithTrigramToken(t, trigramUnicodeVersion),
	}
	for name, idx := range controls {
		t.Run(name, func(t *testing.T) {
			for _, q := range trigramPruningQueries {
				requireRows(t, idx, q.pred, q.pruned)
			}
			requireTrigramSearch(t, idx, "zzzzzz", nil)
		})
	}

	unknown := decodeWithTrigramToken(t, trigramUnicodeForeignToken)
	if v, ok := unknown.TrigramUnicodeVersion(); v != trigramUnicodeForeignToken || ok {
		t.Errorf("TrigramUnicodeVersion() = (%q, %t), want (%q, false)", v, ok, trigramUnicodeForeignToken)
	}
	for _, q := range trigramPruningQueries {
		requireRows(t, unknown, q.pred, []int{0, 1})
	}
	requireTrigramSearch(t, unknown, "zzzzzz", []int{0, 1})

	// The fallback is per predicate: EQ keeps pruning on its own and when
	// intersected with a disabled Contains, in either order.
	requireRows(t, unknown, EQ("$.k", "a"), []int{0})
	requirePredicateResult(t, unknown, []Predicate{EQ("$.k", "a"), Contains("$.v", "hello")}, []int{0}, "unknown EQ AND Contains")
	requirePredicateResult(t, unknown, []Predicate{Contains("$.v", "hello"), EQ("$.k", "a")}, []int{0}, "unknown Contains AND EQ")
}

func TestTrigramUnicodeVersionRoundTripKeepsUnknownToken(t *testing.T) {
	for _, token := range []string{trigramUnicodeForeignToken, "16.0.0-rc+foreign"} {
		first := encodeWithTrigramToken(t, token)
		decoded, err := Decode(first)
		if err != nil {
			t.Fatalf("Decode(%q): %v", token, err)
		}
		second, err := Encode(decoded)
		if err != nil {
			t.Fatalf("re-Encode(%q): %v", token, err)
		}
		if !bytes.Equal(first, second) {
			t.Errorf("token %q: re-encoded bytes differ from the first encode", token)
		}
		again, err := Decode(second)
		if err != nil {
			t.Fatalf("second Decode(%q): %v", token, err)
		}
		if v, ok := again.TrigramUnicodeVersion(); v != token || ok {
			t.Errorf("after round trip TrigramUnicodeVersion() = (%q, %t), want (%q, false)", v, ok, token)
		}
	}
}

func TestTrigramUnicodeVersionSupportedTokensKeepPruning(t *testing.T) {
	explicit := decodeWithTrigramToken(t, trigramUnicodeVersion)
	requireRows(t, explicit, Contains("$.v", "hello"), []int{0})
	if v, ok := explicit.TrigramUnicodeVersion(); v != trigramUnicodeVersion || !ok {
		t.Errorf("explicit TrigramUnicodeVersion() = (%q, %t)", v, ok)
	}

	// Explicit 15.0.0 normalises to absent: re-encoding gives the bytes of a
	// fresh build of the same docs.
	reencoded, err := Encode(explicit)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	fresh, err := Encode(buildTrigramUnicodeIndex(t, DefaultConfig(), trigramUnicodeVersionDocs))
	if err != nil {
		t.Fatalf("Encode fresh: %v", err)
	}
	if !bytes.Equal(reencoded, fresh) {
		t.Errorf("re-encoded explicit 15.0.0 index differs from a fresh encode")
	}
	if bytes.Contains(encodedConfigJSON(t, explicit), []byte("trigram_unicode_version")) {
		t.Errorf("re-encoded explicit 15.0.0 config wrote trigram_unicode_version")
	}

	// Nil config: Encode writes a zero-length config section.
	idx := buildTrigramUnicodeIndex(t, DefaultConfig(), trigramUnicodeVersionDocs)
	idx.Config = nil
	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("Encode with nil config: %v", err)
	}
	nilCfg, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode with nil config: %v", err)
	}
	if nilCfg.Config != nil {
		t.Fatalf("decoded Config = %+v, want nil", nilCfg.Config)
	}
	requireRows(t, nilCfg, Contains("$.v", "hello"), []int{0})
	if v, ok := nilCfg.TrigramUnicodeVersion(); v != trigramUnicodeVersion || !ok {
		t.Errorf("nil config TrigramUnicodeVersion() = (%q, %t)", v, ok)
	}
}

func TestTrigramUnicodeVersionFreshBuildFromForeignConfig(t *testing.T) {
	foreign := decodeWithTrigramToken(t, trigramUnicodeForeignToken)
	rebuilt := buildTrigramUnicodeIndex(t, *foreign.Config, trigramUnicodeVersionDocs)
	if bytes.Contains(encodedConfigJSON(t, rebuilt), []byte("trigram_unicode_version")) {
		t.Errorf("fresh build from a foreign config wrote trigram_unicode_version")
	}
	encoded, err := Encode(rebuilt)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if bytes.Contains(encoded, []byte("trigram_unicode_version")) {
		t.Errorf("encoded fresh build contains trigram_unicode_version")
	}
	if v, ok := rebuilt.TrigramUnicodeVersion(); v != trigramUnicodeVersion || !ok {
		t.Errorf("TrigramUnicodeVersion() = (%q, %t), want (%q, true)", v, ok, trigramUnicodeVersion)
	}
	requireRows(t, rebuilt, Contains("$.v", "hello"), []int{0})
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	requireRows(t, decoded, Contains("$.v", "hello"), []int{0})
	if v, ok := foreign.TrigramUnicodeVersion(); v != trigramUnicodeForeignToken || ok {
		t.Errorf("after NewBuilder the decoded index reports (%q, %t), want (%q, false)", v, ok, trigramUnicodeForeignToken)
	}
}

func TestTrigramUnicodeVersionUnknownGolden(t *testing.T) {
	idx := decodeTrigramUnicodeUnknownGolden(t)
	if v, ok := idx.TrigramUnicodeVersion(); v != trigramUnicodeForeignToken || ok {
		t.Errorf("TrigramUnicodeVersion() = (%q, %t), want (%q, false)", v, ok, trigramUnicodeForeignToken)
	}
	requireRows(t, idx, Contains("$.v", "hello"), []int{0, 1})
	requireTrigramSearch(t, idx, "hello", []int{0, 1})
	requireRows(t, idx, EQ("$.k", "a"), []int{0})
}

func decodeTrigramUnicodeUnknownGolden(t *testing.T) *GINIndex {
	t.Helper()
	idx, err := Decode(readTestFile(t, trigramUnicodeUnknownGolden))
	if err != nil {
		t.Fatalf("Decode(%s): %v", trigramUnicodeUnknownGolden, err)
	}
	return idx
}

// A caller may replace the exported Config of a decoded foreign index. The
// index, not the config, owns the token, so Encode must write it back rather
// than relabel the foreign grams as trigramUnicodeVersion.
func TestTrigramUnicodeVersionReplacedConfigKeepsToken(t *testing.T) {
	idx := decodeTrigramUnicodeUnknownGolden(t)
	cfg := DefaultConfig()
	idx.Config = &cfg
	if v, ok := idx.TrigramUnicodeVersion(); v != trigramUnicodeForeignToken || ok {
		t.Errorf("after Config replacement TrigramUnicodeVersion() = (%q, %t), want (%q, false)", v, ok, trigramUnicodeForeignToken)
	}
	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("Encode with replaced config: %v", err)
	}
	again, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if v, ok := again.TrigramUnicodeVersion(); v != trigramUnicodeForeignToken || ok {
		t.Errorf("TrigramUnicodeVersion() = (%q, %t), want (%q, false)", v, ok, trigramUnicodeForeignToken)
	}
	requireRows(t, again, Contains("$.v", "hello"), []int{0, 1})
	requireTrigramSearch(t, again, "hello", []int{0, 1})
}

// An unknown token must survive Decode -> Encode -> Decode even when the
// index has no TrigramIndex to carry it.
func TestTrigramUnicodeVersionRoundTripWithoutTrigramPaths(t *testing.T) {
	idx := buildTrigramUnicodeIndex(t, DefaultConfig(), []map[string]string{{"n": "ab"}})
	if len(idx.TrigramIndexes) != 0 {
		t.Fatalf("fixture has %d trigram indexes, want 0", len(idx.TrigramIndexes))
	}
	idx.trigramUnicodeToken = trigramUnicodeForeignToken
	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	reencoded, err := Encode(decoded)
	if err != nil {
		t.Fatalf("re-Encode: %v", err)
	}
	again, err := Decode(reencoded)
	if err != nil {
		t.Fatalf("second Decode: %v", err)
	}
	if v, ok := again.TrigramUnicodeVersion(); v != trigramUnicodeForeignToken || ok {
		t.Errorf("TrigramUnicodeVersion() = (%q, %t), want (%q, false)", v, ok, trigramUnicodeForeignToken)
	}
}

func TestTrigramUnicodeTokenSupported(t *testing.T) {
	cases := []struct {
		token     string
		supported bool
	}{
		{"", true},
		{"15.0.0", true},
		{"15.0", false},
		{"15.0.0 ", false},
		{" 15.0.0", false},
		{"v15.0.0", false},
		{"15.1.0", false},
		{"15.0.1", false},
		{"16.0.0", false},
		{"17.0.0", false},
	}
	for _, c := range cases {
		if got := trigramUnicodeTokenSupported(c.token); got != c.supported {
			t.Errorf("trigramUnicodeTokenSupported(%q) = %t, want %t", c.token, got, c.supported)
		}
		want := c.token
		if c.token == trigramUnicodeVersion {
			want = ""
		}
		if got := trigramUnicodeWireToken(c.token); got != want {
			t.Errorf("trigramUnicodeWireToken(%q) = %q, want %q", c.token, got, want)
		}
	}
}

func TestTrigramUnicodeVersionEncodeToken(t *testing.T) {
	t.Run("nil config rejects a foreign token", func(t *testing.T) {
		idx := decodeTrigramUnicodeUnknownGolden(t)
		idx.Config = nil
		_, err := Encode(idx)
		if err == nil {
			t.Fatal("Encode with nil config and a foreign trigram token succeeded")
		}
		for _, want := range []string{trigramUnicodeForeignToken, "needs a config to record it"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("Encode error %q does not contain %q", err, want)
			}
		}
	})
	t.Run("explicit 15.0.0 matches absent", func(t *testing.T) {
		idx := buildTrigramUnicodeIndex(t, DefaultConfig(), trigramUnicodeVersionDocs)
		fresh, err := Encode(idx)
		if err != nil {
			t.Fatalf("Encode fresh: %v", err)
		}
		idx.trigramUnicodeToken = trigramUnicodeVersion
		labelled, err := Encode(idx)
		if err != nil {
			t.Fatalf("Encode with explicit 15.0.0 token: %v", err)
		}
		if !bytes.Equal(fresh, labelled) {
			t.Error("explicit 15.0.0 token changed the encoded bytes")
		}
		idx.Config = nil
		if _, err := Encode(idx); err != nil {
			t.Errorf("Encode with nil config and 15.0.0 token: %v", err)
		}
	})
}

// A caller may move a TrigramIndex decoded from a foreign-version index into a
// native index through the exported map. Encode must refuse it: the native
// index writes no token, so the next Decode would relabel the foreign grams
// as trigramUnicodeVersion and prune matching row groups.
func TestTrigramUnicodeVersionEncodeRejectsSplicedForeignTrigramIndex(t *testing.T) {
	foreign := decodeTrigramUnicodeUnknownGolden(t)
	native := buildTrigramUnicodeIndex(t, DefaultConfig(), trigramUnicodeVersionDocs)
	pathID := requirePathID(t, native, "$.v")
	native.TrigramIndexes[pathID] = foreign.TrigramIndexes[requirePathID(t, foreign, "$.v")]

	_, err := Encode(native)
	if err == nil {
		t.Fatal("Encode of a native index with a spliced foreign TrigramIndex succeeded")
	}
	for _, want := range []string{trigramUnicodeForeignToken, trigramUnicodeVersion, "trigram index for path ID"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Encode error %q does not contain %q", err, want)
		}
	}
}
