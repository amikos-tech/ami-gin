package gin

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"unicode"
)

// Shared #115 fixture: one document per row group. U+A7DC is an uppercase
// letter in Unicode 17.0.0 and unassigned in 15.0.0.
//
// Measured with the stdlib on go1.26.9 (15.0.0) and go1.27.3 (17.0.0):
//   - (\p{Lu}+) group 1: "ABCꟜ" -> "ABC" (15) / "ABCꟜ" (17).
//   - (\d+)\p{Lu} group 1: "1Ƛ2A" -> "2" (15) / "1" (17).
//
// Pre-fix negative control (documented, not run): on the code before #115 a
// 17.0.0 builder and a 15.0.0 reader return {1} for
// EQ $.name Normalized("upper", "ABCꟜ"); the reader extracts "ABC",
// which RG1 holds, and wrongly prunes RG0. The fixed result is {0}.
var regexUnicodeDocs = []string{
	`{"name":"ABC` + "Ƛ" + `","ref":"1` + "Ƛ" + `2A","code":"order-7","status":"error"}`,
	`{"name":"ABC","ref":"2A","code":"order-8","status":"ok"}`,
	`{"name":"xyz","ref":"1B","code":"x","status":"error"}`,
	`{"name":"DEF","ref":"none","code":"order-7","status":"ok"}`,
}

// Unicode versions of the two toolchains the #115 tests know.
const (
	regexUnicodeV15 = "15.0.0" // go1.26.9
	regexUnicodeV17 = "17.0.0" // go1.27.3
)

const (
	regexUnicodeUpperPattern = `(\p{Lu}+)`
	regexUnicodeRefPattern   = `(\d+)\p{Lu}`
	regexUnicodeCodePattern  = `order-(\d+)`
)

// regexUnicodeConfig is the fixture config through the convenience options.
func regexUnicodeConfig(t *testing.T) GINConfig {
	t.Helper()
	soft := WithTransformerFailureMode(IngestFailureSoft)
	cfg, err := NewConfig(
		WithRegexExtractTransformer("$.name", "upper", regexUnicodeUpperPattern, 1, soft),
		WithRegexExtractIntTransformer("$.ref", "n2", regexUnicodeRefPattern, 1, soft),
		WithRegexExtractIntTransformer("$.code", "num", regexUnicodeCodePattern, 1, soft),
	)
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	return cfg
}

// regexUnicodeRegisteredConfig is the same config through
// WithRegisteredTransformer.
func regexUnicodeRegisteredConfig(t *testing.T) GINConfig {
	t.Helper()
	soft := WithTransformerFailureMode(IngestFailureSoft)
	params := func(pattern string) []byte {
		data, err := json.Marshal(RegexParams{Pattern: pattern, Group: 1})
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	cfg, err := NewConfig(
		WithRegisteredTransformer("$.name", "upper", TransformerRegexExtract, params(regexUnicodeUpperPattern), soft),
		WithRegisteredTransformer("$.ref", "n2", TransformerRegexExtractInt, params(regexUnicodeRefPattern), soft),
		WithRegisteredTransformer("$.code", "num", TransformerRegexExtractInt, params(regexUnicodeCodePattern), soft),
	)
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	return cfg
}

// buildRegexUnicodeIndex builds the fixture docs with cfg. Every document
// must be accepted (soft failure mode, R2).
func buildRegexUnicodeIndex(t *testing.T, cfg GINConfig) *GINIndex {
	t.Helper()
	builder := mustNewBuilder(t, cfg, len(regexUnicodeDocs))
	for rg, doc := range regexUnicodeDocs {
		if err := builder.AddDocument(DocID(rg), []byte(doc)); err != nil {
			t.Fatalf("AddDocument(%d): %v", rg, err)
		}
	}
	idx := builder.Finalize()
	if idx.Header.NumDocs != 4 {
		t.Fatalf("NumDocs = %d, want 4", idx.Header.NumDocs)
	}
	return idx
}

// regexUnicodeQuery is one baseline row.
type regexUnicodeQuery struct {
	name       string
	predicates []Predicate
}

func regexUnicodeQueries() []regexUnicodeQuery {
	return []regexUnicodeQuery{
		{"upper Normalized ABC+A7DC", []Predicate{EQ("$.name", Normalized("upper", "ABCꟜ"))}},
		{"upper Normalized ABC", []Predicate{EQ("$.name", Normalized("upper", "ABC"))}},
		{"upper As ABC+A7DC", []Predicate{EQ("$.name", As("upper", "ABCꟜ"))}},
		{"upper As DEF", []Predicate{EQ("$.name", As("upper", "DEF"))}},
		{"n2 Normalized 1+A7DC+2A", []Predicate{EQ("$.ref", Normalized("n2", "1Ƛ2A"))}},
		{"n2 As 1", []Predicate{EQ("$.ref", As("n2", float64(1)))}},
		{"n2 As 2", []Predicate{EQ("$.ref", As("n2", float64(2)))}},
		{"num As 7", []Predicate{EQ("$.code", As("num", float64(7)))}},
		{"upper Normalized ABC+A7DC AND status error", []Predicate{
			EQ("$.name", Normalized("upper", "ABCꟜ")),
			EQ("$.status", "error"),
		}},
	}
}

// regexUnicodeBaselines holds the exact expected sets per builder Unicode
// version, in the order of regexUnicodeQueries.
var regexUnicodeBaselines = map[string][][]int{
	regexUnicodeV15: {{0, 1}, {0, 1}, {}, {3}, {0, 1}, {2}, {0, 1}, {0, 3}, {0}},
	regexUnicodeV17: {{0}, {1}, {0}, {3}, {0, 2}, {0, 2}, {1}, {0, 3}, {0}},
}

// requireRegexUnicodeBaseline asserts the exact baseline of a builder with
// builderUnicode tables, whatever toolchain runs the query.
func requireRegexUnicodeBaseline(t *testing.T, idx *GINIndex, builderUnicode, label string) {
	t.Helper()
	want, ok := regexUnicodeBaselines[builderUnicode]
	if !ok {
		t.Fatalf("%s: no baseline for builder unicode %s", label, builderUnicode)
	}
	for i, query := range regexUnicodeQueries() {
		got := idx.Evaluate(query.predicates).ToSlice()
		if len(got) != len(want[i]) {
			t.Errorf("%s: %s = %v, want %v (builder %s, reader %s)", label, query.name, got, want[i], builderUnicode, unicode.Version)
			continue
		}
		for j := range got {
			if got[j] != want[i][j] {
				t.Errorf("%s: %s = %v, want %v (builder %s, reader %s)", label, query.name, got, want[i], builderUnicode, unicode.Version)
				break
			}
		}
	}
}

// regexUnicodeStoredParams returns the config params of alias on path.
func regexUnicodeStoredParams(t *testing.T, cfg *GINConfig, path, alias string) json.RawMessage {
	t.Helper()
	for _, representation := range cfg.representations(path) {
		if representation.Alias == alias {
			return representation.Transformer.Params
		}
	}
	t.Fatalf("no representation %s on %s", alias, path)
	return nil
}

func requireRegexUnicodeKnownVersion(t *testing.T) {
	t.Helper()
	if _, ok := regexUnicodeBaselines[unicode.Version]; !ok {
		t.Fatalf("unicode.Version %s has no literal baseline", unicode.Version)
	}
}

func TestRegexUnicodeFreezeAtRegistration(t *testing.T) {
	requireRegexUnicodeKnownVersion(t)
	for name, cfg := range map[string]GINConfig{
		"convenience": regexUnicodeConfig(t),
		"registered":  regexUnicodeRegisteredConfig(t),
	} {
		for _, tc := range []struct{ path, alias, pattern string }{
			{"$.name", "upper", regexUnicodeUpperPattern},
			{"$.ref", "n2", regexUnicodeRefPattern},
		} {
			raw := regexUnicodeStoredParams(t, &cfg, tc.path, tc.alias)
			wire, marked, err := decodeRegexWireParams(raw)
			if err != nil {
				t.Fatalf("%s %s: decode %s: %v", name, tc.alias, raw, err)
			}
			frozen, err := freezeRegexPattern(tc.pattern, maxConfigSize)
			if err != nil {
				t.Fatal(err)
			}
			if !marked || wire.Pattern != frozen || wire.SourcePattern != tc.pattern || wire.FrozenUnicode != unicode.Version || wire.Group != 1 {
				t.Fatalf("%s %s: stored %+v marked %v, want frozen %q source %q unicode %s", name, tc.alias, wire, marked, frozen, tc.pattern, unicode.Version)
			}
			if err := verifyFrozenRegexText(wire.Pattern); err != nil {
				t.Fatalf("%s %s: stored pattern fails the verifier: %v", name, tc.alias, err)
			}
		}
	}
}

func TestRegexUnicodeSafePatternKeepsBytes(t *testing.T) {
	cfg, err := NewConfig(WithRegexExtractIntTransformer("$.code", "num", regexUnicodeCodePattern, 1))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(regexUnicodeStoredParams(t, &cfg, "$.code", "num")); got != `{"pattern":"order-(\\d+)","group":1}` {
		t.Fatalf("stored params = %s", got)
	}
	callerBytes := []byte(`{ "pattern" : "order-(\\d+)",  "group": 1 }`)
	cfg, err = NewConfig(WithRegisteredTransformer("$.code", "num", TransformerRegexExtractInt, callerBytes))
	if err != nil {
		t.Fatal(err)
	}
	if got := regexUnicodeStoredParams(t, &cfg, "$.code", "num"); !bytes.Equal(got, callerBytes) {
		t.Fatalf("stored params = %s, want caller bytes %s", got, callerBytes)
	}
}

func TestRegexUnicodeRejectsReservedKeys(t *testing.T) {
	for _, id := range []TransformerID{TransformerRegexExtract, TransformerRegexExtractInt} {
		for _, params := range []string{
			`{"pattern":"x","group":0,"source_pattern":"x"}`,
			`{"pattern":"x","group":0,"frozen_unicode":"17.0.0"}`,
			`{"pattern":"x","group":0,"source_pattern":""}`,
			`{"pattern":"x","group":0,"frozen_unicode":null}`,
			`{"pattern":"x","group":0,"source_pattern":"x","frozen_unicode":"17.0.0"}`,
		} {
			_, err := NewConfig(WithRegisteredTransformer("$.name", "a", id, []byte(params)))
			if err == nil || !strings.Contains(err.Error(), "builder owns") {
				t.Errorf("id %d params %s: err = %v, want reserved key error", id, params, err)
			}
		}
	}
	params := []byte(`{"layout":"2006-01-02","source_pattern":"x","frozen_unicode":"17.0.0"}`)
	if _, err := NewConfig(WithRegisteredTransformer("$.date", "d", TransformerCustomDateToEpochMs, params)); err != nil {
		t.Fatalf("non-regex transformer with the same keys: %v", err)
	}
}

func TestRegexUnicodeGarayRegistration(t *testing.T) {
	cfg, err := NewConfig(WithRegexExtractTransformer("$.name", "garay", `(\p{Garay}+)`, 1))
	switch unicode.Version {
	case regexUnicodeV15:
		if err == nil {
			t.Fatal("register \\p{Garay} on 15.0.0: want error")
		}
	case regexUnicodeV17:
		if err != nil {
			t.Fatalf("register \\p{Garay} on 17.0.0: %v", err)
		}
		wire, marked, err := decodeRegexWireParams(regexUnicodeStoredParams(t, &cfg, "$.name", "garay"))
		if err != nil || !marked || strings.Contains(wire.Pattern, `\p`) {
			t.Fatalf("stored %+v marked %v err %v", wire, marked, err)
		}
	default:
		t.Fatalf("unicode.Version %s has no literal expectation", unicode.Version)
	}
}

func TestRegexUnicodeFixtureBaseline(t *testing.T) {
	requireRegexUnicodeKnownVersion(t)
	for name, cfg := range map[string]GINConfig{
		"convenience": regexUnicodeConfig(t),
		"registered":  regexUnicodeRegisteredConfig(t),
	} {
		idx := buildRegexUnicodeIndex(t, cfg)
		requireRegexUnicodeBaseline(t, idx, unicode.Version, name+" in memory")
		decoded, err := Decode(mustEncodeUncompressed(t, idx))
		if err != nil {
			t.Fatalf("%s: Decode: %v", name, err)
		}
		requireRegexUnicodeBaseline(t, decoded, unicode.Version, name+" encoded")
	}
}

// regexUnicodeLargeSafePattern is a safe (never frozen) pattern of about
// 100 KB, so a few companions pass the 1 MiB section limits.
var regexUnicodeLargeSafePattern = "(" + strings.Repeat("a", 100_000) + ")"

func regexUnicodeLargeOptions(prefix string, n int) []ConfigOption {
	opts := make([]ConfigOption, n)
	for i := range opts {
		opts[i] = WithRegexExtractTransformer("$.big", prefix+string(rune('a'+i)), regexUnicodeLargeSafePattern, 1)
	}
	return opts
}

func TestRegexUnicodeEarlySizeLimits(t *testing.T) {
	if _, err := NewConfig(WithRegexExtractTransformer("$.name", "huge", strings.Repeat(`\pL`, 300), 0)); err == nil ||
		!strings.Contains(err.Error(), "freeze regex pattern") {
		t.Fatalf("NewConfig with 300 x \\pL: err = %v, want freezer size error", err)
	}

	if _, err := NewConfig(regexUnicodeLargeOptions("c", 11)...); err == nil || !strings.Contains(err.Error(), "config section size") {
		t.Fatalf("NewConfig with 11 large companions: err = %v, want config size error", err)
	}

	cfg := DefaultConfig()
	for _, opt := range regexUnicodeLargeOptions("c", 11) {
		if err := opt(&cfg); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := NewBuilder(cfg, 1); err == nil || !strings.Contains(err.Error(), "config section size") {
		t.Fatalf("NewBuilder with 11 large companions: err = %v, want config size error", err)
	}
}

// TestRegexUnicodeWriteConfigSizeLimit: the config section alone exceeds the
// limit; the representation section stays under its own.
func TestRegexUnicodeWriteConfigSizeLimit(t *testing.T) {
	small, err := NewConfig(WithRegexExtractIntTransformer("$.code", "num", regexUnicodeCodePattern, 1, WithTransformerFailureMode(IngestFailureSoft)))
	if err != nil {
		t.Fatal(err)
	}
	idx := buildRegexUnicodeIndex(t, small)

	big := DefaultConfig()
	for _, opt := range regexUnicodeLargeOptions("c", 11) {
		if err := opt(&big); err != nil {
			t.Fatal(err)
		}
	}
	configData, err := marshalSerializedConfig(&big, "")
	if err != nil {
		t.Fatal(err)
	}
	repData, err := marshalRepresentationSection(idx.representations)
	if err != nil {
		t.Fatal(err)
	}
	if len(configData) <= maxConfigSize || len(repData) > maxRepresentationSectionSize {
		t.Fatalf("setup: config %d bytes, representations %d bytes", len(configData), len(repData))
	}
	var buf bytes.Buffer
	if err := writeConfig(&buf, &big, ""); err == nil || !strings.Contains(err.Error(), "config size") {
		t.Fatalf("writeConfig: err = %v, want config size error", err)
	}
	idx.Config = &big
	if _, err := EncodeWithLevel(idx, CompressionNone); err == nil || !strings.Contains(err.Error(), "config size") {
		t.Fatalf("Encode: err = %v, want config size error", err)
	}
}

// TestRegexUnicodeWriteRepresentationSizeLimit: the representation section
// alone exceeds the limit; the config section stays under its own.
func TestRegexUnicodeWriteRepresentationSizeLimit(t *testing.T) {
	small, err := NewConfig(WithRegexExtractIntTransformer("$.code", "num", regexUnicodeCodePattern, 1, WithTransformerFailureMode(IngestFailureSoft)))
	if err != nil {
		t.Fatal(err)
	}
	idx := buildRegexUnicodeIndex(t, small)
	params, err := json.Marshal(RegexParams{Pattern: regexUnicodeLargeSafePattern, Group: 1})
	if err != nil {
		t.Fatal(err)
	}
	var reps []RepresentationSpec
	for i := 0; i < 11; i++ {
		alias := "r" + string(rune('a'+i))
		spec := NewTransformerSpec("$.big", TransformerRegexExtract, params)
		spec.Alias = alias
		spec.TargetPath = representationTargetPath("$.big", alias)
		reps = append(reps, RepresentationSpec{SourcePath: "$.big", Alias: alias, TargetPath: spec.TargetPath, Transformer: spec, Serializable: true})
	}
	configData, err := marshalSerializedConfig(idx.Config, "")
	if err != nil {
		t.Fatal(err)
	}
	repData, err := marshalRepresentationSection(reps)
	if err != nil {
		t.Fatal(err)
	}
	if len(configData) > maxConfigSize || len(repData) <= maxRepresentationSectionSize {
		t.Fatalf("setup: config %d bytes, representations %d bytes", len(configData), len(repData))
	}
	idx.representations = reps
	var buf bytes.Buffer
	if err := writeRepresentations(&buf, idx); err == nil || !strings.Contains(err.Error(), "representation metadata size") {
		t.Fatalf("writeRepresentations: err = %v, want representation size error", err)
	}
}
