//go:build simdjson

package gin

import (
	"bytes"
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/pkg/errors"
)

const (
	fuzzMaxInputBytes = 4096
	fuzzMaxArrayDepth = 8
)

type fuzzBuildOutcome struct {
	committed   bool
	softSkipped uint64
	err         error
	encoded     []byte
}

type fuzzOutcomeClass string

const (
	fuzzOutcomeBothCommitted                fuzzOutcomeClass = "both_committed"
	fuzzOutcomeByteDivergence               fuzzOutcomeClass = "byte_divergence"
	fuzzOutcomeUnexpectedOneSidedCommit     fuzzOutcomeClass = "unexpected_one_sided_commit"
	fuzzOutcomeRejectionAgreement           fuzzOutcomeClass = "rejection_agreement"
	fuzzOutcomeKnownMalformedLayerAsymmetry fuzzOutcomeClass = "known_malformed_layer_asymmetry"
)

func arrayNestingDepth(doc []byte) int {
	depth, maxDepth := 0, 0
	inString, escaped := false, false
	for _, c := range doc {
		switch {
		case escaped:
			escaped = false
		case c == '\\' && inString:
			escaped = true
		case c == '"':
			inString = !inString
		case inString:
			continue
		case c == '[':
			depth++
			if depth > maxDepth {
				maxDepth = depth
			}
		case c == ']':
			depth--
		}
	}
	return maxDepth
}

func fuzzInputWithinBounds(doc []byte) bool {
	return len(doc) <= fuzzMaxInputBytes && arrayNestingDepth(doc) <= fuzzMaxArrayDepth
}

func fuzzParserParityConfig(tb testing.TB) GINConfig {
	tb.Helper()
	cfg, err := NewConfig(
		WithParserFailureMode(IngestFailureHard),
		WithNumericFailureMode(IngestFailureSoft),
	)
	if err != nil {
		tb.Fatalf("NewConfig for parser parity fuzzing: %v", err)
	}
	return cfg
}

func buildFuzzParserOutcome(cfg GINConfig, parser Parser, doc []byte) fuzzBuildOutcome {
	var opts []BuilderOption
	if parser != nil {
		opts = append(opts, WithParser(parser))
	}
	builder, err := NewBuilder(cfg, 1, opts...)
	if err != nil {
		return fuzzBuildOutcome{err: err}
	}

	outcome := fuzzBuildOutcome{
		err:         builder.AddDocument(DocID(0), doc),
		softSkipped: builder.NumSoftSkippedDocuments(),
	}
	switch builder.numDocs {
	case 0:
		return outcome
	case 1:
		outcome.committed = true
	default:
		outcome.err = errors.Errorf("fresh fuzz builder committed %d documents, want at most 1", builder.numDocs)
		return outcome
	}

	idx := builder.Finalize()
	if idx == nil {
		outcome.err = errors.New("fresh fuzz builder finalized to nil after committing one document")
		return outcome
	}
	encoded, err := Encode(idx)
	if err != nil {
		outcome.err = errors.Wrap(err, "encode committed fuzz document")
		return outcome
	}
	outcome.encoded = encoded
	return outcome
}

func classifyFuzzParserOutcomes(doc []byte, stdlib, simd fuzzBuildOutcome) fuzzOutcomeClass {
	if stdlib.committed && simd.committed {
		if bytes.Equal(stdlib.encoded, simd.encoded) {
			return fuzzOutcomeBothCommitted
		}
		return fuzzOutcomeByteDivergence
	}
	if stdlib.committed != simd.committed {
		return fuzzOutcomeUnexpectedOneSidedCommit
	}
	if isKnownMalformedLayerAsymmetry(doc, stdlib, simd) {
		return fuzzOutcomeKnownMalformedLayerAsymmetry
	}
	return fuzzOutcomeRejectionAgreement
}

func isKnownMalformedLayerAsymmetry(doc []byte, stdlib, simd fuzzBuildOutcome) bool {
	if !bytes.Equal(doc, []byte(`1e400 garbage`)) || stdlib.err != nil || stdlib.softSkipped != 1 || simd.softSkipped != 0 {
		return false
	}
	var ingestErr *IngestError
	return errors.As(simd.err, &ingestErr) && ingestErr.Layer() == IngestLayerParser
}

func formatUnexpectedOneSidedCommit(stdlib, simd fuzzBuildOutcome) string {
	return fmt.Sprintf(
		"SIMD_FUZZ_OUTCOME class=unexpected_one_sided_commit stdlib=%s simd=%s",
		formatFuzzBuildOutcome(stdlib),
		formatFuzzBuildOutcome(simd),
	)
}

func formatFuzzBuildOutcome(outcome fuzzBuildOutcome) string {
	errText := ""
	if outcome.err != nil {
		errText = outcome.err.Error()
	}
	return fmt.Sprintf(
		"{committed=%t soft_skipped=%d err=%q encoded_bytes=%d}",
		outcome.committed,
		outcome.softSkipped,
		errText,
		len(outcome.encoded),
	)
}

func runFuzzParserParityInput(
	t *testing.T,
	doc []byte,
	stdlibArm func([]byte) fuzzBuildOutcome,
	simdArm func([]byte) fuzzBuildOutcome,
) {
	t.Helper()
	if !fuzzInputWithinBounds(doc) {
		return
	}

	stdlibOutcome := stdlibArm(doc)
	simdOutcome := simdArm(doc)
	switch classifyFuzzParserOutcomes(doc, stdlibOutcome, simdOutcome) {
	case fuzzOutcomeByteDivergence:
		assertByteIdentical(t, "fuzz-hard-stop", simdOutcome.encoded, stdlibOutcome.encoded)
	case fuzzOutcomeUnexpectedOneSidedCommit:
		t.Log(formatUnexpectedOneSidedCommit(stdlibOutcome, simdOutcome))
	}
}

func decodeFuzzCorpusSeed(contents []byte) ([]byte, error) {
	lines := strings.Split(string(contents), "\n")
	if len(lines) < 2 || lines[0] != "go test fuzz v1" {
		return nil, errors.New("missing standard Go fuzz corpus header")
	}
	payload := strings.TrimSpace(strings.Join(lines[1:], "\n"))
	expr, err := goparser.ParseExpr(payload)
	if err != nil {
		return nil, errors.Wrap(err, "parse fuzz corpus value")
	}
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, errors.New("fuzz corpus value must be one []byte conversion")
	}
	arrayType, ok := call.Fun.(*ast.ArrayType)
	if !ok || arrayType.Len != nil {
		return nil, errors.New("fuzz corpus value must use []byte")
	}
	ident, ok := arrayType.Elt.(*ast.Ident)
	if !ok || ident.Name != "byte" {
		return nil, errors.New("fuzz corpus value must use []byte")
	}
	literal, ok := call.Args[0].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return nil, errors.New("fuzz corpus []byte value must contain one string literal")
	}
	decoded, err := strconv.Unquote(literal.Value)
	if err != nil {
		return nil, errors.Wrap(err, "unquote fuzz corpus bytes")
	}
	return []byte(decoded), nil
}

func readFuzzCorpusSeed(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", "fuzz", "FuzzParserParity", name)
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fuzz corpus seed %s: %v", name, err)
	}
	doc, err := decodeFuzzCorpusSeed(contents)
	if err != nil {
		t.Fatalf("decode fuzz corpus seed %s: %v", name, err)
	}
	return doc
}

func FuzzParserParity(f *testing.F) {
	parser := newTestSIMDParser(f)
	cfg := fuzzParserParityConfig(f)

	f.Fuzz(func(t *testing.T, doc []byte) {
		runFuzzParserParityInput(
			t,
			doc,
			func(doc []byte) fuzzBuildOutcome {
				return buildFuzzParserOutcome(cfg, nil, doc)
			},
			func(doc []byte) fuzzBuildOutcome {
				return buildFuzzParserOutcome(cfg, parser, doc)
			},
		)
	})
}

func TestArrayNestingDepthStringAware(t *testing.T) {
	tests := []struct {
		name string
		doc  []byte
		want int
	}{
		{name: "brackets in string", doc: []byte(`{"value":"[[]]"}`), want: 0},
		{name: "escaped quote and backslash", doc: []byte(`{"value":"escaped quote: \"[\" and slash: \\["}`), want: 0},
		{name: "real arrays after quoted bracket", doc: []byte(`{"value":"[", "items":[[]]}`), want: 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := arrayNestingDepth(tc.doc); got != tc.want {
				t.Fatalf("arrayNestingDepth(%q) = %d, want %d", tc.doc, got, tc.want)
			}
		})
	}
}

func TestFuzzInputWithinBounds(t *testing.T) {
	tests := []struct {
		name string
		doc  []byte
		want bool
	}{
		{name: "exact byte limit", doc: bytes.Repeat([]byte(" "), 4096), want: true},
		{name: "over byte limit", doc: bytes.Repeat([]byte(" "), 4097), want: false},
		{name: "exact array depth", doc: []byte(strings.Repeat("[", 8) + "0" + strings.Repeat("]", 8)), want: true},
		{name: "over array depth", doc: []byte(strings.Repeat("[", 9) + "0" + strings.Repeat("]", 9)), want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := fuzzInputWithinBounds(tc.doc); got != tc.want {
				t.Fatalf("fuzzInputWithinBounds(len=%d, depth=%d) = %v, want %v", len(tc.doc), arrayNestingDepth(tc.doc), got, tc.want)
			}
		})
	}
}

func TestFuzzParserParityRejectsOverLimitBeforeArms(t *testing.T) {
	tests := []struct {
		name      string
		doc       []byte
		wantCalls int
	}{
		{name: "over byte limit", doc: bytes.Repeat([]byte("x"), 4097), wantCalls: 0},
		{name: "over depth limit", doc: []byte(strings.Repeat("[", 9) + "0" + strings.Repeat("]", 9)), wantCalls: 0},
		{name: "within limits", doc: []byte(`[0]`), wantCalls: 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			arm := func([]byte) fuzzBuildOutcome {
				calls++
				return fuzzBuildOutcome{committed: true, encoded: []byte("same")}
			}
			runFuzzParserParityInput(t, tc.doc, arm, arm)
			if calls != tc.wantCalls {
				t.Fatalf("parser/build arm calls = %d, want %d", calls, tc.wantCalls)
			}
		})
	}
}

func TestFuzzBuildOutcomeUsesBuilderState(t *testing.T) {
	cfg := fuzzParserParityConfig(t)

	committed := buildFuzzParserOutcome(cfg, nil, []byte(`{"a":1}`))
	if !committed.committed || committed.softSkipped != 0 || committed.err != nil || len(committed.encoded) == 0 {
		t.Fatalf("committed outcome = %+v, want committed bytes and no skip/error", committed)
	}

	softSkipped := buildFuzzParserOutcome(cfg, nil, []byte(`{"a":1e400}`))
	if softSkipped.committed || softSkipped.softSkipped != 1 || softSkipped.err != nil || softSkipped.encoded != nil {
		t.Fatalf("soft-skip outcome = %+v, want uncommitted soft skip without encoded bytes", softSkipped)
	}
}

func TestClassifyFuzzParserOutcomes(t *testing.T) {
	parserErr := newIngestErrorString(
		IngestLayerParser,
		"",
		"1e400 garbage",
		errors.New("malformed trailing input"),
	)
	tests := []struct {
		name   string
		doc    []byte
		stdlib fuzzBuildOutcome
		simd   fuzzBuildOutcome
		want   fuzzOutcomeClass
	}{
		{
			name:   "equal committed bytes",
			doc:    []byte(`{"a":1}`),
			stdlib: fuzzBuildOutcome{committed: true, encoded: []byte("same")},
			simd:   fuzzBuildOutcome{committed: true, encoded: []byte("same")},
			want:   fuzzOutcomeBothCommitted,
		},
		{
			name:   "committed byte divergence",
			doc:    []byte(`{"a":1}`),
			stdlib: fuzzBuildOutcome{committed: true, encoded: []byte("stdlib")},
			simd:   fuzzBuildOutcome{committed: true, encoded: []byte("simd")},
			want:   fuzzOutcomeByteDivergence,
		},
		{
			name:   "unexpected one-sided commit",
			doc:    []byte(`{"a":1}`),
			stdlib: fuzzBuildOutcome{committed: true, encoded: []byte("stdlib")},
			simd:   fuzzBuildOutcome{err: errors.New("parser rejection")},
			want:   fuzzOutcomeUnexpectedOneSidedCommit,
		},
		{
			name:   "both hard reject",
			doc:    []byte(`{"a":}`),
			stdlib: fuzzBuildOutcome{err: errors.New("stdlib rejection")},
			simd:   fuzzBuildOutcome{err: errors.New("simd rejection")},
			want:   fuzzOutcomeRejectionAgreement,
		},
		{
			name:   "both soft skip",
			doc:    []byte(`{"a":1e400}`),
			stdlib: fuzzBuildOutcome{softSkipped: 1},
			simd:   fuzzBuildOutcome{softSkipped: 1},
			want:   fuzzOutcomeRejectionAgreement,
		},
		{
			name:   "known malformed layer asymmetry",
			doc:    []byte(`1e400 garbage`),
			stdlib: fuzzBuildOutcome{softSkipped: 1},
			simd:   fuzzBuildOutcome{err: parserErr},
			want:   fuzzOutcomeKnownMalformedLayerAsymmetry,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyFuzzParserOutcomes(tc.doc, tc.stdlib, tc.simd)
			if got != tc.want {
				t.Fatalf("classifyFuzzParserOutcomes() = %q, want %q", got, tc.want)
			}
			if got == fuzzOutcomeUnexpectedOneSidedCommit {
				record := formatUnexpectedOneSidedCommit(tc.stdlib, tc.simd)
				const prefix = "SIMD_FUZZ_OUTCOME class=unexpected_one_sided_commit"
				if !strings.HasPrefix(record, prefix) || strings.Count(record, prefix) != 1 {
					t.Fatalf("one-sided record = %q, want exactly one stable prefix", record)
				}
				if !strings.Contains(record, "stdlib=") || !strings.Contains(record, "simd=") {
					t.Fatalf("one-sided record = %q, want both arm outcomes", record)
				}
				if strings.Contains(record, string(fuzzOutcomeKnownMalformedLayerAsymmetry)) {
					t.Fatalf("one-sided record = %q, must not claim the known malformed exclusion", record)
				}
				t.Log(record)
			}
		})
	}
}

func TestFuzzParserParityCorpus(t *testing.T) {
	tests := []struct {
		name     string
		category string
		want     []byte
	}{
		{
			name:     "authored-int-boundaries",
			category: "authored",
			want:     authoredFixtureDocument(t, "int64-boundaries", 2),
		},
		{
			name:     "authored-transformer-nested",
			category: "authored",
			want:     authoredFixtureDocument(t, "transformer-buffered-container-numerics", 0),
		},
		{
			name:     "phase20-nested",
			category: "phase20",
			want:     phase20FixtureDocument(t, "testdata/phase20/nested_high_cardinality.jsonl", 0),
		},
		{
			name:     "phase20-mixed-array",
			category: "phase20",
			want:     phase20FixtureDocument(t, "testdata/phase20/mixed_type_arrays.jsonl", 0),
		},
		{
			name:     "known-malformed-layer-asymmetry",
			category: "known-exclusion",
			want:     []byte(`1e400 garbage`),
		},
	}

	categoryCounts := make(map[string]int)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := readFuzzCorpusSeed(t, tc.name)
			if !bytes.Equal(doc, tc.want) {
				t.Fatalf("corpus seed differs from %s source\ngot:  %q\nwant: %q", tc.category, doc, tc.want)
			}
			if !fuzzInputWithinBounds(doc) {
				t.Fatalf("corpus seed exceeds fuzz bounds: len=%d depth=%d", len(doc), arrayNestingDepth(doc))
			}
			categoryCounts[tc.category]++
		})
	}

	if categoryCounts["authored"] != 2 || categoryCounts["phase20"] != 2 || categoryCounts["known-exclusion"] != 1 {
		t.Fatalf("corpus source categories = %v, want authored=2 phase20=2 known-exclusion=1", categoryCounts)
	}
}

func authoredFixtureDocument(t *testing.T, name string, index int) []byte {
	t.Helper()
	for _, fixture := range authoredParityFixtures() {
		if fixture.Name == name {
			if index < 0 || index >= len(fixture.JSONDocs) {
				t.Fatalf("authored fixture %s index %d out of range", name, index)
			}
			return fixture.JSONDocs[index]
		}
	}
	t.Fatalf("authored fixture %s not found", name)
	return nil
}

func phase20FixtureDocument(t *testing.T, path string, index int) []byte {
	t.Helper()
	docs, err := phase20LoadRawJSONL(path)
	if err != nil {
		t.Fatalf("load Phase 20 fixture %s: %v", path, err)
	}
	if index < 0 || index >= len(docs) {
		t.Fatalf("Phase 20 fixture %s index %d out of range", path, index)
	}
	return docs[index]
}
