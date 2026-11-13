package gin

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"unicode"
)

// Goldens in testdata/regex-unicode (see its README.md).
var (
	regexUnicodeLegacyRegexGoldens = []string{
		filepath.Join("testdata", "regex-unicode", "legacy-regex-go1.26.9.gin"),
		filepath.Join("testdata", "regex-unicode", "legacy-regex-go1.27.3.gin"),
	}
	regexUnicodeLegacySafeGolden   = filepath.Join("testdata", "regex-unicode", "legacy-safe-go1.26.9.gin")
	regexUnicodeLegacyGarayGolden  = filepath.Join("testdata", "regex-unicode", "legacy-garay-go1.27.3.gin")
	regexUnicodeFrozenGarayGolden  = filepath.Join("testdata", "regex-unicode", "frozen-garay-go1.27.3.gin")
	regexUnicodeGarayPattern       = `(\p{Garay}+)`
	regexUnicodeSafeNumParamsBytes = `{"pattern":"order-(\\d+)","group":1}`
)

// regexUnicodeGarayDocs: RG0 holds two Garay letters, RG1 none, RG2 one.
var regexUnicodeGarayDocs = []string{
	`{"name":"ab` + "\U00010D4A\U00010D4B" + `cd"}`,
	`{"name":"abcd"}`,
	`{"name":"` + "\U00010D4C" + `"}`,
}

// buildRegexUnicodeGarayIndex builds the Garay docs with the current builder.
// It needs Unicode 16 or later to register the pattern.
func buildRegexUnicodeGarayIndex(t *testing.T) *GINIndex {
	t.Helper()
	cfg, err := NewConfig(WithRegexExtractTransformer("$.name", "garay", regexUnicodeGarayPattern, 1, WithTransformerFailureMode(IngestFailureSoft)))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	builder := mustNewBuilder(t, cfg, len(regexUnicodeGarayDocs))
	for rg, doc := range regexUnicodeGarayDocs {
		if err := builder.AddDocument(DocID(rg), []byte(doc)); err != nil {
			t.Fatalf("AddDocument(%d): %v", rg, err)
		}
	}
	return builder.Finalize()
}

func TestRegexUnicodeLegacyRegexGoldens(t *testing.T) {
	all := []int{0, 1, 2, 3}
	for _, path := range regexUnicodeLegacyRegexGoldens {
		idx := mustDecodeRegexUnicode(t, readTestFile(t, path))
		requireRegexUnicodeDegraded(t, idx, map[string]DegradeReason{
			"$.name#upper": DegradeReasonLegacyUnfrozenRegex,
			"$.ref#n2":     DegradeReasonLegacyUnfrozenRegex,
		})
		got := idx.DegradedCompanions()
		if got[0].Alias != "upper" || got[1].Alias != "n2" {
			t.Fatalf("%s: DegradedCompanions order %+v", path, got)
		}
		for _, query := range regexUnicodeQueries() {
			predicate := query.predicates[0]
			if predicate.Path == "$.code" || len(query.predicates) > 1 {
				continue
			}
			if rows := idx.Evaluate(query.predicates).ToSlice(); !reflect.DeepEqual(rows, all) {
				t.Errorf("%s: %s = %v, want every row group", path, query.name, rows)
			}
		}
		requireRows(t, idx, EQ("$.code", As("num", float64(7))), []int{0, 3})
		if rows := idx.Evaluate([]Predicate{EQ("$.name", Normalized("upper", "ABCꟜ")), EQ("$.status", "error")}).ToSlice(); !reflect.DeepEqual(rows, []int{0, 2}) {
			t.Errorf("%s: upper AND status error = %v, want [0 2]", path, rows)
		}
	}
}

func TestRegexUnicodeLegacySafeGolden(t *testing.T) {
	golden := readTestFile(t, regexUnicodeLegacySafeGolden)
	idx := mustDecodeRegexUnicode(t, golden)
	configParams, repParams := regexUnicodeSectionParams(t, idx, "num")
	if string(configParams) != regexUnicodeSafeNumParamsBytes || string(repParams) != regexUnicodeSafeNumParamsBytes {
		t.Fatalf("stored params: config %s, representation %s", configParams, repParams)
	}
	requireRows(t, idx, EQ("$.code", As("num", float64(7))), []int{0, 3})
	if unicode.Version != "15.0.0" {
		t.Logf("byte identity is checked on 15.0.0 only (running %s)", unicode.Version)
		return
	}
	cfg, err := NewConfig(WithRegexExtractIntTransformer("$.code", "num", regexUnicodeCodePattern, 1, WithTransformerFailureMode(IngestFailureSoft)))
	if err != nil {
		t.Fatal(err)
	}
	if data := mustEncodeUncompressed(t, buildRegexUnicodeIndex(t, cfg)); !bytes.Equal(data, golden) {
		t.Fatalf("current builder writes %d bytes, pre-#115 golden has %d: safe patterns must stay byte-identical", len(data), len(golden))
	}
}

func TestRegexUnicodeLegacyGarayGolden(t *testing.T) {
	idx := mustDecodeRegexUnicode(t, readTestFile(t, regexUnicodeLegacyGarayGolden))
	var reason DegradeReason
	switch unicode.Version {
	case "15.0.0":
		reason = DegradeReasonRegexCompileFailed
	case "17.0.0":
		reason = DegradeReasonLegacyUnfrozenRegex
	default:
		t.Fatalf("unicode.Version %s has no literal expectation", unicode.Version)
	}
	requireRegexUnicodeDegraded(t, idx, map[string]DegradeReason{"$.name#garay": reason})
	requireRows(t, idx, EQ("$.name", As("garay", "\U00010D4A\U00010D4B")), []int{0, 1, 2})
	requireRows(t, idx, EQ("$.name", Normalized("garay", "x\U00010D4Cy")), []int{0, 1, 2})
}

func TestRegexUnicodeFrozenGarayGolden(t *testing.T) {
	golden := readTestFile(t, regexUnicodeFrozenGarayGolden)
	idx := mustDecodeRegexUnicode(t, golden)
	if got := idx.DegradedCompanions(); len(got) != 0 {
		t.Fatalf("DegradedCompanions = %+v, want none", got)
	}
	requireRows(t, idx, EQ("$.name", As("garay", "\U00010D4A\U00010D4B")), []int{0})
	requireRows(t, idx, EQ("$.name", Normalized("garay", "x\U00010D4Cy")), []int{2})

	configParams, repParams := regexUnicodeSectionParams(t, idx, "garay")
	wire, marked, err := decodeRegexWireParams(configParams)
	if err != nil || !marked || wire.SourcePattern != regexUnicodeGarayPattern || wire.FrozenUnicode != "17.0.0" {
		t.Fatalf("stored params %s: marked %v err %v", configParams, marked, err)
	}
	reencoded := mustEncodeUncompressed(t, idx)
	if !bytes.Equal(reencoded, golden) {
		t.Fatalf("Decode -> Encode is %d bytes, golden %d: not byte-identical", len(reencoded), len(golden))
	}
	again := mustDecodeRegexUnicode(t, reencoded)
	againConfig, againRep := regexUnicodeSectionParams(t, again, "garay")
	if !bytes.Equal(againConfig, configParams) || !bytes.Equal(againRep, repParams) {
		t.Fatalf("params changed on re-encode: %s / %s", againConfig, againRep)
	}
	if !json.Valid(configParams) {
		t.Fatalf("params are not valid JSON: %s", configParams)
	}
}
