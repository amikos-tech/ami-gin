package gin

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

var trigramUnicodeUnknownGolden = filepath.Join("testdata", "trigram-unicode", "unknown-version.gin")

// trigramUnicodeForeignToken stands in for a table version this reader does
// not know, as written by a future ami-gin.
const trigramUnicodeForeignToken = "17.0.0"

var trigramUnicodeVersionDocs = []string{
	`{"v":"hello world","k":"a"}`,
	`{"v":"other text","k":"b"}`,
}

func buildTrigramUnicodeVersionIndex(t *testing.T, cfg GINConfig) *GINIndex {
	t.Helper()
	builder, err := NewBuilder(cfg, len(trigramUnicodeVersionDocs))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	for i, doc := range trigramUnicodeVersionDocs {
		if err := builder.AddDocument(DocID(i), []byte(doc)); err != nil {
			t.Fatalf("AddDocument(%d): %v", i, err)
		}
	}
	return builder.Finalize()
}

// encodeWithTrigramToken builds the fixture, labels it with token as a
// foreign writer would, and encodes it.
func encodeWithTrigramToken(t *testing.T, token string) []byte {
	t.Helper()
	idx := buildTrigramUnicodeVersionIndex(t, DefaultConfig())
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

func trigramIndexFor(t *testing.T, idx *GINIndex, path string) *TrigramIndex {
	t.Helper()
	pathID, ok := idx.pathLookup[path]
	if !ok {
		t.Fatalf("path %s not in index", path)
	}
	ti, ok := idx.TrigramIndexes[pathID]
	if !ok {
		t.Fatalf("no trigram index for %s", path)
	}
	return ti
}

// trigramPruningQueries all prune on a supported index. Every regex literal
// is at least N runes long, so evaluateRegex reaches TrigramIndex.Search.
var trigramPruningQueries = []struct {
	name   string
	run    func(idx *GINIndex) []int
	pruned []int
}{
	{"contains hello", evalPreds(Contains("$.v", "hello")), []int{0}},
	{"regex hello|xyzzy", evalPreds(Regex("$.v", "hello|xyzzy")), []int{0}},
	{"regex hello.*world", evalPreds(Regex("$.v", "hello.*world")), []int{0}},
	{"contains zzzzzz", evalPreds(Contains("$.v", "zzzzzz")), nil},
	{"direct search zzzzzz", func(idx *GINIndex) []int {
		return idx.TrigramIndexes[idx.pathLookup["$.v"]].Search("zzzzzz").ToSlice()
	}, nil},
}

func evalPreds(preds ...Predicate) func(idx *GINIndex) []int {
	return func(idx *GINIndex) []int {
		return idx.Evaluate(preds).ToSlice()
	}
}

func assertRGs(t *testing.T, label string, got, want []int) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !slices.Equal(got, want) {
		t.Errorf("%s: got RGs %v, want %v", label, got, want)
	}
}

func TestTrigramUnicodeVersionFreshIndexWritesNoKey(t *testing.T) {
	if Version != 11 {
		t.Fatalf("wire Version = %d, want 11", Version)
	}
	cfg := DefaultConfig()
	if bytes.Contains(encodedConfigJSON(t, &GINIndex{Config: &cfg}), []byte("trigram_unicode_version")) {
		t.Errorf("writeConfig(DefaultConfig) wrote trigram_unicode_version")
	}
	idx := buildTrigramUnicodeVersionIndex(t, DefaultConfig())
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
		for _, q := range trigramPruningQueries {
			assertRGs(t, name+" control "+q.name, q.run(idx), q.pruned)
		}
	}

	unknown := decodeWithTrigramToken(t, trigramUnicodeForeignToken)
	if v, ok := unknown.TrigramUnicodeVersion(); v != trigramUnicodeForeignToken || ok {
		t.Errorf("TrigramUnicodeVersion() = (%q, %t), want (%q, false)", v, ok, trigramUnicodeForeignToken)
	}
	for _, q := range trigramPruningQueries {
		assertRGs(t, "unknown "+q.name, q.run(unknown), []int{0, 1})
	}

	// The fallback is per predicate: EQ keeps pruning on its own and when
	// intersected with a disabled Contains, in either order.
	assertRGs(t, "unknown EQ", evalPreds(EQ("$.k", "a"))(unknown), []int{0})
	assertRGs(t, "unknown EQ AND Contains", evalPreds(EQ("$.k", "a"), Contains("$.v", "hello"))(unknown), []int{0})
	assertRGs(t, "unknown Contains AND EQ", evalPreds(Contains("$.v", "hello"), EQ("$.k", "a"))(unknown), []int{0})
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
	assertRGs(t, "explicit 15.0.0", evalPreds(Contains("$.v", "hello"))(explicit), []int{0})
	if v, ok := explicit.TrigramUnicodeVersion(); v != trigramUnicodeVersion || !ok {
		t.Errorf("explicit TrigramUnicodeVersion() = (%q, %t)", v, ok)
	}

	// Explicit 15.0.0 normalises to absent: re-encoding gives the bytes of a
	// fresh build of the same docs.
	reencoded, err := Encode(explicit)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	fresh, err := Encode(buildTrigramUnicodeVersionIndex(t, DefaultConfig()))
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
	idx := buildTrigramUnicodeVersionIndex(t, DefaultConfig())
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
	assertRGs(t, "nil config", evalPreds(Contains("$.v", "hello"))(nilCfg), []int{0})
	if v, ok := nilCfg.TrigramUnicodeVersion(); v != trigramUnicodeVersion || !ok {
		t.Errorf("nil config TrigramUnicodeVersion() = (%q, %t)", v, ok)
	}
}

func TestTrigramUnicodeVersionFreshBuildFromForeignConfig(t *testing.T) {
	foreign := decodeWithTrigramToken(t, trigramUnicodeForeignToken)
	rebuilt := buildTrigramUnicodeVersionIndex(t, *foreign.Config)
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
	assertRGs(t, "rebuilt contains", evalPreds(Contains("$.v", "hello"))(rebuilt), []int{0})
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	assertRGs(t, "rebuilt decoded contains", evalPreds(Contains("$.v", "hello"))(decoded), []int{0})
	if v, ok := foreign.TrigramUnicodeVersion(); v != trigramUnicodeForeignToken || ok {
		t.Errorf("after NewBuilder the decoded index reports (%q, %t), want (%q, false)", v, ok, trigramUnicodeForeignToken)
	}
}

func TestTrigramUnicodeVersionUnknownGolden(t *testing.T) {
	data, err := os.ReadFile(trigramUnicodeUnknownGolden)
	if err != nil {
		t.Fatalf("read %s: %v", trigramUnicodeUnknownGolden, err)
	}
	idx, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode(%s): %v", trigramUnicodeUnknownGolden, err)
	}
	if v, ok := idx.TrigramUnicodeVersion(); v != trigramUnicodeForeignToken || ok {
		t.Errorf("TrigramUnicodeVersion() = (%q, %t), want (%q, false)", v, ok, trigramUnicodeForeignToken)
	}
	assertRGs(t, "golden contains", evalPreds(Contains("$.v", "hello"))(idx), []int{0, 1})
	assertRGs(t, "golden direct search", trigramIndexFor(t, idx, "$.v").Search("hello").ToSlice(), []int{0, 1})
	assertRGs(t, "golden EQ", evalPreds(EQ("$.k", "a"))(idx), []int{0})
}

func decodeTrigramUnicodeUnknownGolden(t *testing.T) *GINIndex {
	t.Helper()
	data, err := os.ReadFile(trigramUnicodeUnknownGolden)
	if err != nil {
		t.Fatalf("read %s: %v", trigramUnicodeUnknownGolden, err)
	}
	idx, err := Decode(data)
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
	assertRGs(t, "re-encoded contains", evalPreds(Contains("$.v", "hello"))(again), []int{0, 1})
	assertRGs(t, "re-encoded direct search", trigramIndexFor(t, again, "$.v").Search("hello").ToSlice(), []int{0, 1})
}

// An unknown token must survive Decode -> Encode -> Decode even when the
// index has no TrigramIndex to carry it.
func TestTrigramUnicodeVersionRoundTripWithoutTrigramPaths(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 1)
	if err := builder.AddDocument(0, []byte(`{"n":1}`)); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	idx := builder.Finalize()
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

func TestTrigramUnicodeVersionEncodeToken(t *testing.T) {
	t.Run("nil config rejects a foreign token", func(t *testing.T) {
		idx := decodeTrigramUnicodeUnknownGolden(t)
		idx.Config = nil
		if _, err := Encode(idx); err == nil {
			t.Fatal("Encode with nil config and a foreign trigram token succeeded")
		}
	})
	t.Run("explicit 15.0.0 matches absent", func(t *testing.T) {
		idx := buildTrigramUnicodeVersionIndex(t, DefaultConfig())
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
