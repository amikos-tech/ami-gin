package gin

import (
	"bytes"
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/pkg/errors"
)

// regexUnicodeRawSpec is a regex companion registered without freezing, as
// the code before #115 (or a tampered writer) would store it.
type regexUnicodeRawSpec struct {
	path, alias string
	id          TransformerID
	params      string
}

// regexUnicodeRawFunc compiles the stored pattern when it compiles here.
// Otherwise it returns a stand-in that still produces values, so the
// companion is materialized and Decode sees it in both sections.
func regexUnicodeRawFunc(spec regexUnicodeRawSpec) FieldTransformer {
	var p RegexParams
	if err := json.Unmarshal([]byte(spec.params), &p); err == nil && p.Group >= 0 {
		if re, err := regexp.Compile(p.Pattern); err == nil {
			return regexExtractFunc(re, p.Group, spec.id == TransformerRegexExtractInt)
		}
	}
	return func(v any) (any, bool) {
		s, ok := v.(string)
		if !ok {
			return nil, false
		}
		if spec.id == TransformerRegexExtractInt {
			return float64(len(s)), true
		}
		return s, true
	}
}

// regexUnicodeRawConfig registers raw specs through addRepresentation (no
// freezing, no reserved-key check).
func regexUnicodeRawConfig(t *testing.T, specs ...regexUnicodeRawSpec) GINConfig {
	t.Helper()
	cfg := DefaultConfig()
	for _, spec := range specs {
		canonicalPath, err := canonicalizeSupportedPath(spec.path)
		if err != nil {
			t.Fatal(err)
		}
		transformerSpec := NewTransformerSpec(canonicalPath, spec.id, json.RawMessage(spec.params))
		if err := cfg.addRepresentation(canonicalPath, spec.alias, transformerSpec, true, IngestFailureSoft, regexUnicodeRawFunc(spec)); err != nil {
			t.Fatalf("addRepresentation(%+v): %v", spec, err)
		}
	}
	return cfg
}

// regexUnicodeRawIndex builds the shared fixture docs with raw specs and
// returns the uncompressed encoded bytes.
func regexUnicodeRawIndex(t *testing.T, specs ...regexUnicodeRawSpec) []byte {
	t.Helper()
	return mustEncodeUncompressed(t, buildRegexUnicodeIndex(t, regexUnicodeRawConfig(t, specs...)))
}

func mustDecodeRegexUnicode(t *testing.T, data []byte) *GINIndex {
	t.Helper()
	idx, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return idx
}

var (
	regexUnicodeLegacyUpper = regexUnicodeRawSpec{"$.name", "upper", TransformerRegexExtract, `{"pattern":"(\\p{Lu}+)","group":1}`}
	regexUnicodeLegacyN2    = regexUnicodeRawSpec{"$.ref", "n2", TransformerRegexExtractInt, `{"pattern":"(\\d+)\\p{Lu}","group":1}`}
	regexUnicodeSafeNum     = regexUnicodeRawSpec{"$.code", "num", TransformerRegexExtractInt, `{"pattern":"order-(\\d+)","group":1}`}
)

func requireRegexUnicodeDegraded(t *testing.T, idx *GINIndex, want map[string]DegradeReason) {
	t.Helper()
	got := idx.DegradedCompanions()
	if len(got) != len(want) {
		t.Fatalf("DegradedCompanions = %+v, want %v", got, want)
	}
	for _, entry := range got {
		reason, ok := want[entry.SourcePath+"#"+entry.Alias]
		if !ok || reason != entry.Reason {
			t.Fatalf("DegradedCompanions = %+v, want %v", got, want)
		}
		if entry.Detail == "" {
			t.Fatalf("DegradedCompanions entry %+v has no detail", entry)
		}
	}
}

func TestRegexUnicodeClassifyLegacyAndSafe(t *testing.T) {
	treeOnly := regexUnicodeRawSpec{"$.name", "accent", TransformerRegexExtract, `{"pattern":"([é-ü]+|[A-Z]+)","group":1}`}
	if regexSourceSensitive(`([é-ü]+|[A-Z]+)`) {
		t.Fatal("setup: the tree-rule pattern must not trip the source rule")
	}
	idx := mustDecodeRegexUnicode(t, regexUnicodeRawIndex(t, regexUnicodeLegacyUpper, treeOnly, regexUnicodeSafeNum))
	requireRegexUnicodeDegraded(t, idx, map[string]DegradeReason{
		"$.name#upper":  DegradeReasonLegacyUnfrozenRegex,
		"$.name#accent": DegradeReasonLegacyUnfrozenRegex,
	})
	requireRows(t, idx, EQ("$.name", As("upper", "DEF")), []int{0, 1, 2, 3})
	requireRows(t, idx, EQ("$.name", As("accent", "DEF")), []int{0, 1, 2, 3})
	requireRows(t, idx, EQ("$.code", As("num", float64(7))), []int{0, 3})
	requireRows(t, idx, EQ("$.code", As("num", float64(8))), []int{1})
}

func TestRegexUnicodeClassifyCompileFailures(t *testing.T) {
	invalid := regexUnicodeRawSpec{"$.name", "bad", TransformerRegexExtract, `{"pattern":"[invalid","group":0}`}
	garay := regexUnicodeRawSpec{"$.name", "garay", TransformerRegexExtract, `{"pattern":"(\\p{Garay}+|[A-Z]+)","group":1}`}
	frozenBad := regexUnicodeRawSpec{"$.name", "fbad", TransformerRegexExtract,
		`{"pattern":"([A-Z]+","group":1,"source_pattern":"(\\p{Lu}+","frozen_unicode":"17.0.0"}`}
	idx := mustDecodeRegexUnicode(t, regexUnicodeRawIndex(t, invalid, garay, frozenBad, regexUnicodeSafeNum))

	garayReason := DegradeReasonRegexCompileFailed
	switch unicode.Version {
	case regexUnicodeV15:
	case regexUnicodeV17:
		garayReason = DegradeReasonLegacyUnfrozenRegex
	default:
		t.Fatalf("unicode.Version %s has no literal expectation", unicode.Version)
	}
	requireRegexUnicodeDegraded(t, idx, map[string]DegradeReason{
		"$.name#bad":   DegradeReasonRegexCompileFailed,
		"$.name#garay": garayReason,
		"$.name#fbad":  DegradeReasonRegexCompileFailed,
	})
	for _, entry := range idx.DegradedCompanions() {
		if entry.Alias == "bad" && !strings.Contains(entry.Detail, "missing closing ]") {
			t.Fatalf("detail %q does not hold the compile error", entry.Detail)
		}
	}
	requireRows(t, idx, EQ("$.name", As("bad", "ABC")), []int{0, 1, 2, 3})
	requireRows(t, idx, EQ("$.name", Normalized("garay", "ABC")), []int{0, 1, 2, 3})
	requireRows(t, idx, EQ("$.code", As("num", float64(7))), []int{0, 3})
}

func TestRegexUnicodeClassifyMarkedSpecs(t *testing.T) {
	requireRegexUnicodeKnownVersion(t)
	unverified := regexUnicodeRawSpec{"$.name", "upper", TransformerRegexExtract,
		`{"pattern":"(\\p{Lu}+)","group":1,"source_pattern":"(\\p{Lu}+)","frozen_unicode":"17.0.0"}`}
	idx := mustDecodeRegexUnicode(t, regexUnicodeRawIndex(t, unverified, regexUnicodeSafeNum))
	requireRegexUnicodeDegraded(t, idx, map[string]DegradeReason{"$.name#upper": DegradeReasonUnverifiedFrozenRegex})
	requireRows(t, idx, EQ("$.name", As("upper", "DEF")), []int{0, 1, 2, 3})

	frozen, err := freezeRegexPattern(regexUnicodeUpperPattern, maxConfigSize)
	if err != nil {
		t.Fatal(err)
	}
	params, err := json.Marshal(regexWireParams{Pattern: frozen, Group: 1, SourcePattern: regexUnicodeUpperPattern, FrozenUnicode: "99.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	future := regexUnicodeRawSpec{"$.name", "upper", TransformerRegexExtract, string(params)}
	idx = mustDecodeRegexUnicode(t, regexUnicodeRawIndex(t, future, regexUnicodeSafeNum))
	if got := idx.DegradedCompanions(); len(got) != 0 {
		t.Fatalf("frozen_unicode 99.0.0: DegradedCompanions = %+v, want none", got)
	}
	baseline := regexUnicodeBaselines[unicode.Version]
	queries := regexUnicodeQueries()
	for i := 0; i < 4; i++ {
		requireRows(t, idx, queries[i].predicates[0], baseline[i])
	}
}

// TestRegexUnicodeClassifyMarkedQuotedLiteral: a \Q...\E block can hide a
// later flag group from the verifier scan, so a marked spec that holds a \Q
// escape is unverified. An escaped backslash before Q stays a literal.
func TestRegexUnicodeClassifyMarkedQuotedLiteral(t *testing.T) {
	marked := func(alias, pattern string) regexUnicodeRawSpec {
		params, err := json.Marshal(regexWireParams{Pattern: pattern, Group: 0, SourcePattern: pattern, FrozenUnicode: "17.0.0"})
		if err != nil {
			t.Fatal(err)
		}
		return regexUnicodeRawSpec{"$.name", alias, TransformerRegexExtract, string(params)}
	}
	idx := mustDecodeRegexUnicode(t, regexUnicodeRawIndex(t,
		marked("q1", `\Q[\E(?i)x`),
		marked("q2", `\Q[\E(?i)(ƛ+)`),
		marked("q3", `\Q[\E(?i:ƛ)`),
		marked("lit", `(\\Q|[A-Z]+)`),
		regexUnicodeSafeNum,
	))
	requireRegexUnicodeDegraded(t, idx, map[string]DegradeReason{
		"$.name#q1": DegradeReasonUnverifiedFrozenRegex,
		"$.name#q2": DegradeReasonUnverifiedFrozenRegex,
		"$.name#q3": DegradeReasonUnverifiedFrozenRegex,
	})
	requireRows(t, idx, EQ("$.name", As("q2", "ƛ")), []int{0, 1, 2, 3})
	requireRows(t, idx, EQ("$.code", As("num", float64(8))), []int{1})
}

func TestRegexUnicodeClassifyTimeout(t *testing.T) {
	safeUpper := regexUnicodeRawSpec{"$.name", "upper", TransformerRegexExtract, `{"pattern":"([A-Z]+)","group":1}`}
	data := regexUnicodeRawIndex(t, safeUpper)

	release := make(chan struct{})
	finished := make(chan struct{}, 1)
	previous, previousDeadline := regexCompileFunc, regexCompileDeadline
	t.Cleanup(func() { regexCompileFunc, regexCompileDeadline = previous, previousDeadline })
	regexCompileDeadline = 20 * time.Millisecond
	regexCompileFunc = func(pattern string) (*regexp.Regexp, error) {
		<-release
		defer func() { finished <- struct{}{} }()
		return regexp.Compile(pattern)
	}

	idx := mustDecodeRegexUnicode(t, data)
	requireRegexUnicodeDegraded(t, idx, map[string]DegradeReason{"$.name#upper": DegradeReasonRegexCompileTimeout})
	before := idx.DegradedCompanions()

	close(release)
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("late compile did not finish")
	}
	if after := idx.DegradedCompanions(); !reflect.DeepEqual(before, after) {
		t.Fatalf("late compile changed DegradedCompanions: %+v -> %+v", before, after)
	}
	requireRows(t, idx, EQ("$.name", As("upper", "DEF")), []int{0, 1, 2, 3})

	// The reason is not stored: a later Decode with a normal compile prunes.
	regexCompileFunc, regexCompileDeadline = previous, previousDeadline
	again := mustDecodeRegexUnicode(t, data)
	if got := again.DegradedCompanions(); len(got) != 0 {
		t.Fatalf("second Decode: DegradedCompanions = %+v, want none", got)
	}
	requireRows(t, again, EQ("$.name", As("upper", "DEF")), []int{3})
	requireRows(t, again, EQ("$.name", Normalized("upper", "ABC")), []int{0, 1})
}

// regexUnicodeBadParams are params that must fail Decode.
var regexUnicodeBadParams = []string{
	`{"pattern":"([A-Z]+)","group":-1}`,
	`"not an object"`,
	`{"pattern":"([A-Z]+)","group":"1"}`,
	`{"pattern":"([A-Z]+)","group":1,"source_pattern":"x"}`,
	`{"pattern":"([A-Z]+)","group":1,"frozen_unicode":"17.0.0"}`,
	`{"pattern":"([A-Z]+)","group":1,"source_pattern":"","frozen_unicode":"17.0.0"}`,
	`{"pattern":"([A-Z]+)","group":1,"source_pattern":"x","frozen_unicode":null}`,
	`{"pattern":"([A-Z]+)","group":1,"source_pattern":5,"frozen_unicode":"17.0.0"}`,
}

func TestRegexUnicodeDecodeRejectsInvalidParams(t *testing.T) {
	for _, params := range regexUnicodeBadParams {
		spec := regexUnicodeRawSpec{"$.name", "upper", TransformerRegexExtract, params}

		// Config-backed: the spec is in both sections.
		data := regexUnicodeRawIndex(t, spec, regexUnicodeSafeNum)
		if _, err := Decode(data); !errors.Is(err, ErrInvalidFormat) {
			t.Errorf("config-backed %s: Decode err = %v, want ErrInvalidFormat", params, err)
		}

		// Representation-only: the config no longer lists the spec.
		idx := buildRegexUnicodeIndex(t, regexUnicodeRawConfig(t, spec, regexUnicodeSafeNum))
		other := regexUnicodeRawConfig(t, regexUnicodeSafeNum)
		idx.Config = &other
		if _, err := Decode(mustEncodeUncompressed(t, idx)); !errors.Is(err, ErrInvalidFormat) {
			t.Errorf("representation-only %s: Decode err = %v, want ErrInvalidFormat", params, err)
		}
	}

	// Pure readConfig error: invalid params JSON shape in the config section.
	sc := SerializedConfig{Transformers: []TransformerSpec{{
		Path: "$.name", Alias: "upper", ID: TransformerRegexExtract, Name: "regex_extract", Params: json.RawMessage(`[1,2]`),
	}}}
	buf := mustMarshalLengthPrefixedConfig(t, sc)
	if _, _, err := readConfig(&buf); !errors.Is(err, ErrInvalidFormat) {
		t.Fatalf("readConfig: err = %v, want ErrInvalidFormat", err)
	}
}

func TestRegexUnicodeRepresentationOnlyDegrades(t *testing.T) {
	idx := buildRegexUnicodeIndex(t, regexUnicodeRawConfig(t, regexUnicodeLegacyUpper, regexUnicodeSafeNum))
	other := regexUnicodeRawConfig(t, regexUnicodeSafeNum)
	idx.Config = &other
	decoded := mustDecodeRegexUnicode(t, mustEncodeUncompressed(t, idx))
	requireRegexUnicodeDegraded(t, decoded, map[string]DegradeReason{"$.name#upper": DegradeReasonLegacyUnfrozenRegex})
	requireRows(t, decoded, EQ("$.name", As("upper", "DEF")), []int{0, 1, 2, 3})
}

// regexUnicodeGuardPredicates covers every operator through As, and the
// operators Normalized supports through Normalized.
func regexUnicodeGuardPredicates() map[string]Predicate {
	return map[string]Predicate{
		"EQ As":              EQ("$.name", As("upper", "DEF")),
		"NE As":              NE("$.name", As("upper", "DEF")),
		"GT As":              GT("$.ref", As("n2", float64(5))),
		"GTE As":             GTE("$.ref", As("n2", float64(5))),
		"LT As":              LT("$.ref", As("n2", float64(0))),
		"LTE As":             LTE("$.ref", As("n2", float64(0))),
		"IN As":              IN("$.name", As("upper", "DEF")),
		"NIN As":             NIN("$.name", As("upper", "DEF")),
		"IsNull As":          {Path: "$.name", Operator: OpIsNull, Value: As("upper", nil)},
		"IsNotNull As":       {Path: "$.name", Operator: OpIsNotNull, Value: As("upper", nil)},
		"Contains As":        {Path: "$.name", Operator: OpContains, Value: As("upper", "zzz")},
		"Regex As":           {Path: "$.name", Operator: OpRegex, Value: As("upper", "zzz")},
		"absent As":          EQ("$.name", As("upper", "zzz-not-present")),
		"EQ Normalized":      EQ("$.name", Normalized("upper", "DEF")),
		"NE Normalized":      NE("$.name", Normalized("upper", "DEF")),
		"GT Normalized":      GT("$.ref", Normalized("n2", "9Z")),
		"GTE Normalized":     GTE("$.ref", Normalized("n2", "9Z")),
		"LT Normalized":      LT("$.ref", Normalized("n2", "0Z")),
		"LTE Normalized":     LTE("$.ref", Normalized("n2", "0Z")),
		"IN Normalized":      IN("$.name", Normalized("upper", "DEF")),
		"NIN Normalized":     NIN("$.name", Normalized("upper", "DEF")),
		"EQ Normalized n2 2": EQ("$.ref", Normalized("n2", "2A")),
	}
}

// regexUnicodeHealthyPrunes lists the guard rows that prune on a healthy
// (frozen) index, so the guard rows prove something.
var regexUnicodeHealthyPrunes = []string{
	"EQ As", "GT As", "GTE As", "LT As", "LTE As", "IN As", "absent As",
	"EQ Normalized", "GT Normalized", "GTE Normalized", "LT Normalized", "LTE Normalized", "IN Normalized", "EQ Normalized n2 2",
}

func TestRegexUnicodeGuardEveryOperator(t *testing.T) {
	degraded := mustDecodeRegexUnicode(t, regexUnicodeRawIndex(t, regexUnicodeLegacyUpper, regexUnicodeLegacyN2, regexUnicodeSafeNum))
	healthy := mustDecodeRegexUnicode(t, mustEncodeUncompressed(t, buildRegexUnicodeIndex(t, regexUnicodeConfig(t))))
	all := []int{0, 1, 2, 3}
	for label, predicate := range regexUnicodeGuardPredicates() {
		if got := degraded.Evaluate([]Predicate{predicate}).ToSlice(); !reflect.DeepEqual(got, all) {
			t.Errorf("%s: degraded = %v, want every row group", label, got)
		}
	}
	for _, label := range regexUnicodeHealthyPrunes {
		predicate := regexUnicodeGuardPredicates()[label]
		if got := healthy.Evaluate([]Predicate{predicate}).ToSlice(); len(got) >= len(all) {
			t.Errorf("%s: healthy = %v, want pruning: the guard row proves nothing", label, got)
		}
	}
	// Predicate-local: the other predicate of the same Evaluate call prunes.
	got := degraded.Evaluate([]Predicate{EQ("$.name", Normalized("upper", "ABCꟜ")), EQ("$.status", "error")}).ToSlice()
	if !reflect.DeepEqual(got, []int{0, 2}) {
		t.Fatalf("degraded AND status error = %v, want [0 2]", got)
	}
}

// regexUnicodeSectionParams returns the params of alias in the config
// section and the representation section of a decoded index.
func regexUnicodeSectionParams(t *testing.T, idx *GINIndex, alias string) (json.RawMessage, json.RawMessage) {
	t.Helper()
	var configParams, repParams json.RawMessage
	for _, representation := range collectRepresentationsFromConfig(idx.Config) {
		if representation.Alias == alias {
			configParams = representation.Transformer.Params
		}
	}
	for _, representation := range idx.representations {
		if representation.Alias == alias {
			repParams = representation.Transformer.Params
		}
	}
	if configParams == nil || repParams == nil {
		t.Fatalf("alias %s missing from a section", alias)
	}
	return configParams, repParams
}

func TestRegexUnicodeReencodeKeepsDegradation(t *testing.T) {
	invalid := regexUnicodeRawSpec{"$.name", "bad", TransformerRegexExtract, `{"pattern":"[invalid","group":0}`}
	first := mustDecodeRegexUnicode(t, regexUnicodeRawIndex(t, regexUnicodeLegacyUpper, regexUnicodeLegacyN2, invalid, regexUnicodeSafeNum))
	second := mustDecodeRegexUnicode(t, mustEncodeUncompressed(t, first))
	if !reflect.DeepEqual(first.DegradedCompanions(), second.DegradedCompanions()) {
		t.Fatalf("re-encode changed DegradedCompanions: %+v -> %+v", first.DegradedCompanions(), second.DegradedCompanions())
	}
	requireRegexUnicodeDegraded(t, second, map[string]DegradeReason{
		"$.name#upper": DegradeReasonLegacyUnfrozenRegex,
		"$.ref#n2":     DegradeReasonLegacyUnfrozenRegex,
		"$.name#bad":   DegradeReasonRegexCompileFailed,
	})
	for _, spec := range []regexUnicodeRawSpec{regexUnicodeLegacyUpper, regexUnicodeLegacyN2, invalid} {
		for _, idx := range []*GINIndex{first, second} {
			configParams, repParams := regexUnicodeSectionParams(t, idx, spec.alias)
			if string(configParams) != spec.params || string(repParams) != spec.params {
				t.Fatalf("%s params changed: config %s, representation %s, want %s", spec.alias, configParams, repParams, spec.params)
			}
		}
	}
}

func TestRegexUnicodeDegradedCompanionsSnapshot(t *testing.T) {
	healthy := mustDecodeRegexUnicode(t, mustEncodeUncompressed(t, buildRegexUnicodeIndex(t, regexUnicodeConfig(t))))
	if got := healthy.DegradedCompanions(); got != nil {
		t.Fatalf("healthy DegradedCompanions = %+v, want nil", got)
	}

	idx := mustDecodeRegexUnicode(t, regexUnicodeRawIndex(t, regexUnicodeLegacyUpper, regexUnicodeLegacyN2, regexUnicodeSafeNum))
	first := idx.DegradedCompanions()
	want := []DegradedCompanion{
		{SourcePath: "$.name", Alias: "upper", Reason: DegradeReasonLegacyUnfrozenRegex},
		{SourcePath: "$.ref", Alias: "n2", Reason: DegradeReasonLegacyUnfrozenRegex},
	}
	if len(first) != len(want) {
		t.Fatalf("DegradedCompanions = %+v", first)
	}
	for i := range want {
		if first[i].SourcePath != want[i].SourcePath || first[i].Alias != want[i].Alias || first[i].Reason != want[i].Reason || first[i].Detail == "" {
			t.Fatalf("DegradedCompanions[%d] = %+v, want %+v with detail", i, first[i], want[i])
		}
	}
	first[0].Reason = "changed"
	_ = append(first, DegradedCompanion{Alias: "extra"})
	second := idx.DegradedCompanions()
	if second[0].Reason != DegradeReasonLegacyUnfrozenRegex || len(second) != 2 {
		t.Fatalf("mutating the snapshot changed the index: %+v", second)
	}

	plain := DefaultConfig()
	idx.Config = &plain
	if got := idx.DegradedCompanions(); !reflect.DeepEqual(got, second) {
		t.Fatalf("replacing idx.Config changed DegradedCompanions: %+v", got)
	}
	requireRows(t, idx, EQ("$.name", As("upper", "DEF")), []int{0, 1, 2, 3})
}

func TestRegexUnicodeFinalizeFromDecodedConfig(t *testing.T) {
	decoded := mustDecodeRegexUnicode(t, regexUnicodeRawIndex(t, regexUnicodeLegacyUpper, regexUnicodeSafeNum))
	rebuilt := buildRegexUnicodeIndex(t, *decoded.Config)
	requireRegexUnicodeDegraded(t, rebuilt, map[string]DegradeReason{"$.name#upper": DegradeReasonLegacyUnfrozenRegex})
	requireRows(t, rebuilt, EQ("$.name", As("upper", "DEF")), []int{0, 1, 2, 3})
	requireRows(t, rebuilt, EQ("$.code", As("num", float64(7))), []int{0, 3})

	var buf bytes.Buffer
	if err := writeConfig(&buf, rebuilt.Config, ""); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`{"pattern":"(\\p{Lu}+)","group":1}`)) {
		t.Fatalf("rebuilt config does not keep the legacy params: %s", buf.Bytes())
	}
}

// TestRegexUnicodeDegradedCompanionsWithoutValues: a legacy companion that
// holds no values (no fixture doc has a Greek letter) is still listed, sorted
// with the others, and the list survives re-encode, Finalize from the decoded
// config and changes to idx.Config or the returned slice.
func TestRegexUnicodeDegradedCompanionsWithoutValues(t *testing.T) {
	greek := regexUnicodeRawSpec{"$.name", "greek", TransformerRegexExtract, `{"pattern":"(\\p{Greek}+)","group":1}`}
	idx := mustDecodeRegexUnicode(t, regexUnicodeRawIndex(t, regexUnicodeLegacyUpper, greek, regexUnicodeSafeNum))
	for _, info := range idx.Representations("$.name") {
		if info.Alias == "greek" {
			t.Fatal("setup: the greek companion must hold no values")
		}
	}
	want := []DegradedCompanion{
		{SourcePath: "$.name", Alias: "greek", Reason: DegradeReasonLegacyUnfrozenRegex},
		{SourcePath: "$.name", Alias: "upper", Reason: DegradeReasonLegacyUnfrozenRegex},
	}
	requireList := func(label string, got []DegradedCompanion) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s: DegradedCompanions = %+v, want %+v", label, got, want)
		}
		for i := range want {
			if got[i].SourcePath != want[i].SourcePath || got[i].Alias != want[i].Alias || got[i].Reason != want[i].Reason || got[i].Detail == "" {
				t.Fatalf("%s: DegradedCompanions[%d] = %+v, want %+v with detail", label, i, got[i], want[i])
			}
		}
	}
	first := idx.DegradedCompanions()
	requireList("decoded", first)
	requireRows(t, idx, EQ("$.name", As("greek", "αβ")), []int{0, 1, 2, 3})

	first[0].Alias = "changed"
	_ = append(first[:1], DegradedCompanion{Alias: "extra"})
	plain := DefaultConfig()
	reencoded := mustEncodeUncompressed(t, idx)
	idx.Config = &plain
	requireList("after mutation and Config swap", idx.DegradedCompanions())

	requireList("re-encoded", mustDecodeRegexUnicode(t, reencoded).DegradedCompanions())
	decoded := mustDecodeRegexUnicode(t, reencoded)
	requireList("finalized from decoded config", buildRegexUnicodeIndex(t, *decoded.Config).DegradedCompanions())
}

// TestRegexUnicodeNewBuilderRejectsPlaceholder: a decoded config whose regex
// companion did not compile has no executable transformer. NewBuilder fails
// and names the source path, alias and reason. Decode itself succeeds.
func TestRegexUnicodeNewBuilderRejectsPlaceholder(t *testing.T) {
	requireRejected := func(t *testing.T, cfg GINConfig, alias string, reason DegradeReason) {
		t.Helper()
		_, err := NewBuilder(cfg, 4)
		if err == nil {
			t.Fatalf("NewBuilder: err = nil, want error for alias %q", alias)
		}
		for _, want := range []string{"$.name", `"` + alias + `"`, string(reason), "no executable transformer"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("NewBuilder: err = %v, want it to hold %q", err, want)
			}
		}
	}

	t.Run("compile failed", func(t *testing.T) {
		invalid := regexUnicodeRawSpec{"$.name", "bad", TransformerRegexExtract, `{"pattern":"[invalid","group":0}`}
		idx := mustDecodeRegexUnicode(t, regexUnicodeRawIndex(t, invalid, regexUnicodeSafeNum))
		requireRejected(t, *idx.Config, "bad", DegradeReasonRegexCompileFailed)
	})

	t.Run("unverified and uncompilable", func(t *testing.T) {
		spec := regexUnicodeRawSpec{"$.name", "fbad", TransformerRegexExtract,
			`{"pattern":"(\\p{Lu}+","group":1,"source_pattern":"(\\p{Lu}+","frozen_unicode":"17.0.0"}`}
		idx := mustDecodeRegexUnicode(t, regexUnicodeRawIndex(t, spec, regexUnicodeSafeNum))
		requireRegexUnicodeDegraded(t, idx, map[string]DegradeReason{"$.name#fbad": DegradeReasonUnverifiedFrozenRegex})
		requireRejected(t, *idx.Config, "fbad", DegradeReasonUnverifiedFrozenRegex)
	})

	t.Run("compile timeout", func(t *testing.T) {
		safeUpper := regexUnicodeRawSpec{"$.name", "upper", TransformerRegexExtract, `{"pattern":"([A-Z]+)","group":1}`}
		data := regexUnicodeRawIndex(t, safeUpper)
		release := make(chan struct{})
		previous, previousDeadline := regexCompileFunc, regexCompileDeadline
		t.Cleanup(func() { regexCompileFunc, regexCompileDeadline = previous, previousDeadline })
		regexCompileDeadline = 20 * time.Millisecond
		regexCompileFunc = func(pattern string) (*regexp.Regexp, error) {
			<-release
			return regexp.Compile(pattern)
		}
		idx := mustDecodeRegexUnicode(t, data)
		close(release)
		regexCompileFunc, regexCompileDeadline = previous, previousDeadline
		requireRejected(t, *idx.Config, "upper", DegradeReasonRegexCompileTimeout)
	})

	t.Run("legacy compiled keeps building", func(t *testing.T) {
		idx := mustDecodeRegexUnicode(t, regexUnicodeRawIndex(t, regexUnicodeLegacyUpper, regexUnicodeSafeNum))
		if _, err := NewBuilder(*idx.Config, 4); err != nil {
			t.Fatalf("NewBuilder with a legacy compiled companion: %v", err)
		}
	})
}
