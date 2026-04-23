package gin

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/pkg/errors"
)

func TestIngestFailureModeDefaultsAndValidation(t *testing.T) {
	defaults := DefaultConfig()
	if defaults.ParserFailureMode != IngestFailureHard {
		t.Fatalf("DefaultConfig().ParserFailureMode = %q, want %q", defaults.ParserFailureMode, IngestFailureHard)
	}
	if defaults.NumericFailureMode != IngestFailureHard {
		t.Fatalf("DefaultConfig().NumericFailureMode = %q, want %q", defaults.NumericFailureMode, IngestFailureHard)
	}

	cfg, err := NewConfig(
		WithParserFailureMode(IngestFailureSoft),
		WithNumericFailureMode(IngestFailureSoft),
		WithToLowerTransformer("$.email", "lower", WithTransformerFailureMode(IngestFailureSoft)),
	)
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}
	if cfg.ParserFailureMode != IngestFailureSoft {
		t.Fatalf("ParserFailureMode = %q, want %q", cfg.ParserFailureMode, IngestFailureSoft)
	}
	if cfg.NumericFailureMode != IngestFailureSoft {
		t.Fatalf("NumericFailureMode = %q, want %q", cfg.NumericFailureMode, IngestFailureSoft)
	}
	specs := cfg.representationSpecs["$.email"]
	if len(specs) != 1 {
		t.Fatalf("representationSpecs[$.email] len = %d, want 1", len(specs))
	}
	if specs[0].Transformer.FailureMode != IngestFailureSoft {
		t.Fatalf("Transformer.FailureMode = %q, want %q", specs[0].Transformer.FailureMode, IngestFailureSoft)
	}

	builder := mustNewBuilder(t, GINConfig{
		BloomFilterSize:   1024,
		BloomFilterHashes: 2,
		EnableTrigrams:    true,
		TrigramMinLength:  3,
		HLLPrecision:      12,
		PrefixBlockSize:   16,
	}, 2)
	if builder.config.ParserFailureMode != IngestFailureHard {
		t.Fatalf("builder.config.ParserFailureMode = %q, want %q", builder.config.ParserFailureMode, IngestFailureHard)
	}
	if builder.config.NumericFailureMode != IngestFailureHard {
		t.Fatalf("builder.config.NumericFailureMode = %q, want %q", builder.config.NumericFailureMode, IngestFailureHard)
	}

	if _, err := NewConfig(WithParserFailureMode(IngestFailureMode("invalid"))); err == nil {
		t.Fatal("NewConfig(WithParserFailureMode(invalid)) error = nil, want validation failure")
	} else if !strings.Contains(err.Error(), "invalid ingest failure mode") {
		t.Fatalf("parser mode error = %v, want invalid ingest failure mode", err)
	}
	if _, err := NewConfig(WithNumericFailureMode(IngestFailureMode("invalid"))); err == nil {
		t.Fatal("NewConfig(WithNumericFailureMode(invalid)) error = nil, want validation failure")
	} else if !strings.Contains(err.Error(), "invalid ingest failure mode") {
		t.Fatalf("numeric mode error = %v, want invalid ingest failure mode", err)
	}
	if _, err := NewConfig(WithToLowerTransformer("$.email", "lower", WithTransformerFailureMode(IngestFailureMode("invalid")))); err == nil {
		t.Fatal("NewConfig(WithTransformerFailureMode(invalid)) error = nil, want validation failure")
	} else if !strings.Contains(err.Error(), "invalid transformer failure mode") {
		t.Fatalf("transformer mode error = %v, want invalid transformer failure mode", err)
	}
}

func TestValidateIngestFailureModeRejectsLegacyTokens(t *testing.T) {
	if err := validateIngestFailureMode(IngestFailureMode("strict")); err == nil {
		t.Fatal(`validateIngestFailureMode(IngestFailureMode("strict")) error = nil, want validation failure`)
	} else if !strings.Contains(err.Error(), "invalid ingest failure mode") {
		t.Fatalf("strict ingest mode error = %v, want invalid ingest failure mode", err)
	}
	if err := validateIngestFailureMode(IngestFailureMode("soft_fail")); err == nil {
		t.Fatal(`validateIngestFailureMode(IngestFailureMode("soft_fail")) error = nil, want validation failure`)
	} else if !strings.Contains(err.Error(), "invalid ingest failure mode") {
		t.Fatalf("soft_fail ingest mode error = %v, want invalid ingest failure mode", err)
	}

	if err := validateTransformerFailureMode(IngestFailureMode("strict")); err != nil {
		t.Fatalf(`validateTransformerFailureMode(IngestFailureMode("strict")) error = %v, want nil`, err)
	}
	if err := validateTransformerFailureMode(IngestFailureMode("soft_fail")); err != nil {
		t.Fatalf(`validateTransformerFailureMode(IngestFailureMode("soft_fail")) error = %v, want nil`, err)
	}
	if got := normalizeTransformerFailureMode(IngestFailureMode("strict")); got != IngestFailureHard {
		t.Fatalf(`normalizeTransformerFailureMode(IngestFailureMode("strict")) = %q, want %q`, got, IngestFailureHard)
	}
	if got := normalizeTransformerFailureMode(IngestFailureMode("soft_fail")); got != IngestFailureSoft {
		t.Fatalf(`normalizeTransformerFailureMode(IngestFailureMode("soft_fail")) = %q, want %q`, got, IngestFailureSoft)
	}
}

func requireSoftSkippedDocument(t *testing.T, builder *GINBuilder, err error, rejectedDocID DocID, wantDocs uint64, wantNextPos int) {
	t.Helper()
	if err != nil {
		t.Fatalf("AddDocument soft skip error = %v, want nil", err)
	}
	if builder.tragicErr != nil {
		t.Fatalf("builder.tragicErr = %v, want nil", builder.tragicErr)
	}
	if builder.numDocs != wantDocs {
		t.Fatalf("numDocs = %d, want %d", builder.numDocs, wantDocs)
	}
	if builder.nextPos != wantNextPos {
		t.Fatalf("nextPos = %d, want %d", builder.nextPos, wantNextPos)
	}
	if _, exists := builder.docIDToPos[rejectedDocID]; exists {
		t.Fatalf("docIDToPos contains soft-skipped DocID(%d): %+v", rejectedDocID, builder.docIDToPos)
	}
}

func requireSingleDenseValidDocument(t *testing.T, builder *GINBuilder, docID DocID, body string) *GINIndex {
	t.Helper()
	if err := builder.AddDocument(docID, []byte(body)); err != nil {
		t.Fatalf("valid AddDocument(%d) after soft skip failed: %v", docID, err)
	}
	if got := builder.docIDToPos[docID]; got != 0 {
		t.Fatalf("docIDToPos[%d] = %d, want 0", docID, got)
	}
	if builder.nextPos != 1 {
		t.Fatalf("nextPos = %d, want 1", builder.nextPos)
	}
	if builder.numDocs != 1 {
		t.Fatalf("numDocs = %d, want 1", builder.numDocs)
	}
	return builder.Finalize()
}

func requireRows(t *testing.T, idx *GINIndex, predicate Predicate, want []int) {
	t.Helper()
	got := idx.Evaluate([]Predicate{predicate}).ToSlice()
	if len(got) != len(want) {
		t.Fatalf("Evaluate(%+v) = %v, want %v", predicate, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Evaluate(%+v) = %v, want %v", predicate, got, want)
		}
	}
}

func softFailureConfig(t *testing.T, opts ...ConfigOption) GINConfig {
	t.Helper()
	config, err := NewConfig(opts...)
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	return config
}

func TestParserFailureModeHardReturnsParseError(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)
	err := builder.AddDocument(DocID(0), []byte("not-json"))
	if err == nil {
		t.Fatal("AddDocument malformed JSON error = nil, want parser error")
	}
	if !strings.Contains(err.Error(), "read JSON token") {
		t.Fatalf("AddDocument malformed JSON error = %v, want read JSON token", err)
	}
	if builder.tragicErr != nil {
		t.Fatalf("builder.tragicErr = %v, want nil", builder.tragicErr)
	}
}

func TestParserFailureModeSoftSkipsOrdinaryParseError(t *testing.T) {
	config := softFailureConfig(t, WithParserFailureMode(IngestFailureSoft))
	builder := mustNewBuilder(t, config, 2)

	err := builder.AddDocument(DocID(0), []byte("not-json"))
	requireSoftSkippedDocument(t, builder, err, DocID(0), 0, 0)

	idx := requireSingleDenseValidDocument(t, builder, DocID(1), `{"name":"kept"}`)
	if idx.Header.NumDocs != 1 {
		t.Fatalf("Header.NumDocs = %d, want 1", idx.Header.NumDocs)
	}
	requireRows(t, idx, EQ("$.name", "kept"), []int{0})
}

func TestParserFailureModeSoftKeepsContractViolationsHard(t *testing.T) {
	config := softFailureConfig(t, WithParserFailureMode(IngestFailureSoft))
	cases := []struct {
		name       string
		parser     Parser
		wantSubstr string
	}{
		{name: "missing-begin", parser: skipBeginDocumentParser{}, wantSubstr: "did not call BeginDocument"},
		{name: "double-begin", parser: doubleBeginDocumentParser{}, wantSubstr: "called BeginDocument 2 times"},
		{name: "wrong-rgid", parser: wrongRGIDParser{}, wantSubstr: "BeginDocument rgID mismatch"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			builder, err := NewBuilder(config, 2, WithParser(tc.parser))
			if err != nil {
				t.Fatalf("NewBuilder: %v", err)
			}
			err = builder.AddDocument(DocID(0), []byte(`{"name":"bad"}`))
			if err == nil {
				t.Fatal("AddDocument contract error = nil, want hard error")
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("AddDocument contract error = %v, want %q", err, tc.wantSubstr)
			}
			if builder.tragicErr != nil {
				t.Fatalf("builder.tragicErr = %v, want nil", builder.tragicErr)
			}
			if builder.numDocs != 0 || builder.nextPos != 0 {
				t.Fatalf("builder advanced after contract failure: numDocs=%d nextPos=%d", builder.numDocs, builder.nextPos)
			}
		})
	}
}

func TestParserFailureModeSoftDoesNotSwallowStageHardErrors(t *testing.T) {
	config := softFailureConfig(t, WithParserFailureMode(IngestFailureSoft), WithNumericFailureMode(IngestFailureHard))
	builder, err := NewBuilder(config, 2, WithParser(malformedNumericLiteralAtomicityParser{}))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	err = builder.AddDocument(DocID(0), []byte(`{"score":1}`))
	if err == nil {
		t.Fatal("AddDocument hard stage error = nil, want numeric error")
	}
	if !strings.Contains(err.Error(), "parse numeric at $.score") {
		t.Fatalf("AddDocument hard stage error = %v, want parse numeric at $.score", err)
	}
	if builder.tragicErr != nil {
		t.Fatalf("builder.tragicErr = %v, want nil", builder.tragicErr)
	}
	if builder.numDocs != 0 || builder.nextPos != 0 {
		t.Fatalf("builder advanced after hard stage error: numDocs=%d nextPos=%d", builder.numDocs, builder.nextPos)
	}
}

func TestParserFailureModeSoftKeepsTragicStateHard(t *testing.T) {
	config := softFailureConfig(
		t,
		WithParserFailureMode(IngestFailureSoft),
		WithNumericFailureMode(IngestFailureSoft),
		WithToLowerTransformer("$.email", "lower", WithTransformerFailureMode(IngestFailureSoft)),
	)
	builder := mustNewBuilder(t, config, 2)
	builder.tragicErr = errors.New("simulated tragic state")

	err := builder.AddDocument(DocID(0), []byte("not-json"))
	if err == nil {
		t.Fatal("AddDocument after tragic state = nil, want refusal")
	}
	if !strings.Contains(err.Error(), "builder closed by prior tragic failure") {
		t.Fatalf("AddDocument after tragic state = %v, want tragic refusal", err)
	}
}

func TestParserFailureModeStageProvenanceDoesNotLeakAcrossDocuments(t *testing.T) {
	config := softFailureConfig(t, WithParserFailureMode(IngestFailureSoft), WithNumericFailureMode(IngestFailureHard))
	builder, err := NewBuilder(config, 3, WithParser(malformedNumericLiteralAtomicityParser{}))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	err = builder.AddDocument(DocID(0), []byte(`{"score":1}`))
	if err == nil {
		t.Fatal("AddDocument hard stage error = nil, want numeric error")
	}

	builder.parser = stdlibParser{}
	builder.parserName = stdlibParserName
	err = builder.AddDocument(DocID(1), []byte("not-json"))
	requireSoftSkippedDocument(t, builder, err, DocID(1), 0, 0)

	idx := requireSingleDenseValidDocument(t, builder, DocID(2), `{"name":"kept"}`)
	requireRows(t, idx, EQ("$.name", "kept"), []int{0})
}

func TestSoftSkippedDocIDCanBeRetriedWithoutPositionConsumption(t *testing.T) {
	config := softFailureConfig(t, WithParserFailureMode(IngestFailureSoft))
	builder := mustNewBuilder(t, config, 2)

	err := builder.AddDocument(DocID(7), []byte("not-json"))
	requireSoftSkippedDocument(t, builder, err, DocID(7), 0, 0)

	if err := builder.AddDocument(DocID(7), []byte(`{"name":"retried"}`)); err != nil {
		t.Fatalf("retry AddDocument(7) failed: %v", err)
	}
	if got := builder.docIDToPos[DocID(7)]; got != 0 {
		t.Fatalf("docIDToPos[7] = %d, want 0", got)
	}
	if builder.nextPos != 1 {
		t.Fatalf("nextPos = %d, want 1", builder.nextPos)
	}
	idx := builder.Finalize()
	requireRows(t, idx, EQ("$.name", "retried"), []int{0})
}

func TestNumericFailureModeHardReturnsErrors(t *testing.T) {
	t.Run("malformed-literal", func(t *testing.T) {
		builder, err := NewBuilder(DefaultConfig(), 2, WithParser(malformedNumericLiteralAtomicityParser{}))
		if err != nil {
			t.Fatalf("NewBuilder: %v", err)
		}
		err = builder.AddDocument(DocID(0), []byte(`{"score":1}`))
		if err == nil || !strings.Contains(err.Error(), "parse numeric at $.score") {
			t.Fatalf("AddDocument malformed literal error = %v, want parse numeric at $.score", err)
		}
	})

	t.Run("non-finite-native", func(t *testing.T) {
		builder, err := NewBuilder(DefaultConfig(), 2, WithParser(nonFiniteNumericAtomicityParser{}))
		if err != nil {
			t.Fatalf("NewBuilder: %v", err)
		}
		err = builder.AddDocument(DocID(0), []byte(`{"score":1}`))
		if err == nil || !strings.Contains(err.Error(), "non-finite numeric value") {
			t.Fatalf("AddDocument non-finite error = %v, want non-finite numeric value", err)
		}
	})

	t.Run("validator-rejected-promotion", func(t *testing.T) {
		builder := mustNewBuilder(t, DefaultConfig(), 3)
		if err := builder.AddDocument(DocID(0), []byte(`{"score":9007199254740993}`)); err != nil {
			t.Fatalf("seed AddDocument: %v", err)
		}
		err := builder.AddDocument(DocID(1), []byte(`{"score":1.5}`))
		if err == nil || !strings.Contains(err.Error(), mixedNumericPromotionScoreErr) {
			t.Fatalf("AddDocument mixed promotion error = %v, want %q", err, mixedNumericPromotionScoreErr)
		}
	})
}

func TestNumericFailureModeSoftSkipsMalformedLiteralAndNonFinite(t *testing.T) {
	config := softFailureConfig(t, WithNumericFailureMode(IngestFailureSoft))

	t.Run("malformed-literal", func(t *testing.T) {
		builder, err := NewBuilder(config, 2, WithParser(malformedNumericLiteralAtomicityParser{}))
		if err != nil {
			t.Fatalf("NewBuilder: %v", err)
		}
		err = builder.AddDocument(DocID(0), []byte(`{"score":1}`))
		requireSoftSkippedDocument(t, builder, err, DocID(0), 0, 0)
		builder.parser = stdlibParser{}
		builder.parserName = stdlibParserName
		idx := requireSingleDenseValidDocument(t, builder, DocID(1), `{"score":2}`)
		requireRows(t, idx, EQ("$.score", int64(2)), []int{0})
	})

	t.Run("non-finite-native", func(t *testing.T) {
		builder, err := NewBuilder(config, 2, WithParser(nonFiniteNumericAtomicityParser{}))
		if err != nil {
			t.Fatalf("NewBuilder: %v", err)
		}
		err = builder.AddDocument(DocID(0), []byte(`{"score":1}`))
		requireSoftSkippedDocument(t, builder, err, DocID(0), 0, 0)
		builder.parser = stdlibParser{}
		builder.parserName = stdlibParserName
		idx := requireSingleDenseValidDocument(t, builder, DocID(1), `{"score":2}`)
		requireRows(t, idx, EQ("$.score", int64(2)), []int{0})
	})
}

func TestNumericFailureModeSoftSkipsValidatorRejectedPromotion(t *testing.T) {
	config := softFailureConfig(t, WithNumericFailureMode(IngestFailureSoft))
	builder := mustNewBuilder(t, config, 3)
	if err := builder.AddDocument(DocID(0), []byte(`{"score":9007199254740993}`)); err != nil {
		t.Fatalf("seed AddDocument: %v", err)
	}

	err := builder.AddDocument(DocID(1), []byte(`{"score":1.5}`))
	requireSoftSkippedDocument(t, builder, err, DocID(1), 1, 1)

	if err := builder.AddDocument(DocID(2), []byte(`{"score":9007199254740992}`)); err != nil {
		t.Fatalf("valid int after numeric soft skip: %v", err)
	}
	if got := builder.docIDToPos[DocID(2)]; got != 1 {
		t.Fatalf("docIDToPos[2] = %d, want 1", got)
	}
	idx := builder.Finalize()
	requireRows(t, idx, EQ("$.score", int64(9007199254740993)), []int{0})
	requireRows(t, idx, EQ("$.score", int64(9007199254740992)), []int{1})
}

func TestNumericFailureModeSoftSkipsOversizedUnsignedTransformerValue(t *testing.T) {
	config := softFailureConfig(
		t,
		WithNumericFailureMode(IngestFailureSoft),
		WithCustomTransformer("$.token", "numeric", func(value any) (any, bool) {
			token, ok := value.(string)
			if !ok {
				return nil, false
			}
			if token == "skip" {
				return uint64(math.MaxUint64), true
			}
			return int64(7), true
		}),
	)
	builder := mustNewBuilder(t, config, 2)

	err := builder.AddDocument(DocID(0), []byte(`{"token":"skip"}`))
	requireSoftSkippedDocument(t, builder, err, DocID(0), 0, 0)

	if err := builder.AddDocument(DocID(1), []byte(`{"token":"keep"}`)); err != nil {
		t.Fatalf("valid AddDocument after oversized uint soft skip failed: %v", err)
	}
	if got := builder.docIDToPos[DocID(1)]; got != 0 {
		t.Fatalf("docIDToPos[1] = %d, want 0", got)
	}

	idx := builder.Finalize()
	requireRows(t, idx, EQ("$.token", As("numeric", int64(7))), []int{0})
}

func TestNumericFailureModeSoftKeepsMergeRecoveryTragic(t *testing.T) {
	config := softFailureConfig(t, WithNumericFailureMode(IngestFailureSoft))
	builder := mustNewBuilder(t, config, 2)
	builder.testHooks.mergeStagedPathsPanicHook = func() { panic("simulated merge panic") }

	err := builder.AddDocument(DocID(0), []byte(`{"score":1}`))
	if err == nil {
		t.Fatal("AddDocument recovered merge panic error = nil, want tragic error")
	}
	if builder.tragicErr == nil {
		t.Fatal("builder.tragicErr = nil, want recovered merge panic")
	}
	if !strings.Contains(err.Error(), "builder tragic: recovered panic in merge") {
		t.Fatalf("AddDocument recovered merge panic error = %v, want tragic merge error", err)
	}
	if builder.numDocs != 0 || builder.nextPos != 0 {
		t.Fatalf("builder advanced after tragic merge: numDocs=%d nextPos=%d", builder.numDocs, builder.nextPos)
	}
}

func TestTransformerFailureModeHardReturnsError(t *testing.T) {
	config := softFailureConfig(t, WithEmailDomainTransformer("$.email", "domain"))
	builder := mustNewBuilder(t, config, 2)

	err := builder.AddDocument(DocID(0), []byte(`{"email":42}`))
	if err == nil {
		t.Fatal("AddDocument transformer rejection error = nil, want hard error")
	}
	if !strings.Contains(err.Error(), `companion transformer "domain" on $.email failed to produce a value`) {
		t.Fatalf("AddDocument transformer rejection error = %v, want companion transformer context", err)
	}
	if builder.tragicErr != nil {
		t.Fatalf("builder.tragicErr = %v, want nil", builder.tragicErr)
	}
	if builder.numDocs != 0 || builder.nextPos != 0 {
		t.Fatalf("builder advanced after transformer hard failure: numDocs=%d nextPos=%d", builder.numDocs, builder.nextPos)
	}
}

func TestTransformerFailureModeSoftSkipsWholeDocument(t *testing.T) {
	config := softFailureConfig(t, WithEmailDomainTransformer("$.email", "domain", WithTransformerFailureMode(IngestFailureSoft)))
	builder := mustNewBuilder(t, config, 2)

	err := builder.AddDocument(DocID(0), []byte(`{"email":42}`))
	requireSoftSkippedDocument(t, builder, err, DocID(0), 0, 0)

	idx := requireSingleDenseValidDocument(t, builder, DocID(1), `{"email":"ok@example.com"}`)
	rawPathID := requirePathID(t, idx, "$.email")
	if _, ok := idx.NumericIndexes[rawPathID]; ok {
		t.Fatal(`NumericIndexes["$.email"] present, want soft-skipped raw numeric value absent`)
	}
	requireRows(t, idx, EQ("$.email", As("domain", "example.com")), []int{0})
}

func TestTransformerFailureModeSoftDiscardsPartiallyStagedDocument(t *testing.T) {
	config := softFailureConfig(t, WithEmailDomainTransformer("$.email", "domain", WithTransformerFailureMode(IngestFailureSoft)))
	builder := mustNewBuilder(t, config, 2)

	err := builder.AddDocument(DocID(0), []byte(`{"before":"not-finalized","email":42}`))
	requireSoftSkippedDocument(t, builder, err, DocID(0), 0, 0)

	if err := builder.AddDocument(DocID(1), []byte(`{"before":"kept","email":"ok@example.com"}`)); err != nil {
		t.Fatalf("valid AddDocument after transformer soft skip: %v", err)
	}
	if got := builder.docIDToPos[DocID(1)]; got != 0 {
		t.Fatalf("docIDToPos[1] = %d, want 0", got)
	}

	idx := builder.Finalize()
	requireRows(t, idx, EQ("$.before", "not-finalized"), []int{})
	rawPathID := requirePathID(t, idx, "$.email")
	if _, ok := idx.NumericIndexes[rawPathID]; ok {
		t.Fatal(`NumericIndexes["$.email"] present, want soft-skipped raw numeric value absent`)
	}
	requireRows(t, idx, EQ("$.before", "kept"), []int{0})
	requireRows(t, idx, EQ("$.email", As("domain", "example.com")), []int{0})
}

func buildSoftAttemptedIndex(t *testing.T, config GINConfig, docs []atomicityDoc, numRGs int) []byte {
	t.Helper()
	builder := mustNewBuilder(t, config, numRGs)
	for _, doc := range docs {
		if err := builder.AddDocument(doc.docID, doc.doc); err != nil {
			t.Fatalf("AddDocument(%d) error = %v, want nil under soft config", doc.docID, err)
		}
	}
	encoded, err := Encode(builder.Finalize())
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return encoded
}

func TestSoftFailureModesMatchCleanCorpus(t *testing.T) {
	config := softFailureConfig(
		t,
		WithParserFailureMode(IngestFailureSoft),
		WithNumericFailureMode(IngestFailureSoft),
		WithEmailDomainTransformer("$.email", "domain", WithTransformerFailureMode(IngestFailureSoft)),
	)
	fullCorpus := []atomicityDoc{
		{docID: DocID(0), doc: []byte(`{"email":"alice@example.com","score":9007199254740993}`)},
		{docID: DocID(1), doc: []byte(`{"before":"not-finalized","email":42,"score":9007199254740992}`)},
		{docID: DocID(2), doc: []byte(`{"email":"bob@example.com","score":1.5}`)},
		{docID: DocID(3), doc: []byte(`not-json`)},
		{docID: DocID(4), doc: []byte(`{"email":"carol@example.com","score":9007199254740992}`)},
	}
	cleanOnly := []atomicityDoc{
		{docID: DocID(0), doc: []byte(`{"email":"alice@example.com","score":9007199254740993}`)},
		{docID: DocID(4), doc: []byte(`{"email":"carol@example.com","score":9007199254740992}`)},
	}

	fullBytes := buildSoftAttemptedIndex(t, config, fullCorpus, 5)
	cleanBytes := buildSoftAttemptedIndex(t, config, cleanOnly, 5)
	if !bytes.Equal(fullBytes, cleanBytes) {
		t.Fatal("soft failure full corpus and clean-only corpus encoded bytes differ")
	}
}

func TestAllSoftFailureModesSilentlyDropFailures(t *testing.T) {
	config := softFailureConfig(
		t,
		WithParserFailureMode(IngestFailureSoft),
		WithNumericFailureMode(IngestFailureSoft),
		WithEmailDomainTransformer("$.email", "domain", WithTransformerFailureMode(IngestFailureSoft)),
	)
	builder := mustNewBuilder(t, config, 5)

	if err := builder.AddDocument(DocID(0), []byte(`{"email":"seed@example.com","score":9007199254740993}`)); err != nil {
		t.Fatalf("seed AddDocument: %v", err)
	}

	// This silent-drop behavior is opt-in: callers explicitly configured parser,
	// transformer, and numeric layers as soft. Phase 18 adds structured reporting.
	softFailures := []struct {
		docID DocID
		doc   []byte
	}{
		{docID: DocID(1), doc: []byte(`not-json`)},
		{docID: DocID(2), doc: []byte(`{"email":42}`)},
		{docID: DocID(3), doc: []byte(`{"email":"float@example.com","score":1.5}`)},
	}
	for _, failure := range softFailures {
		err := builder.AddDocument(failure.docID, failure.doc)
		requireSoftSkippedDocument(t, builder, err, failure.docID, 1, 1)
	}

	if err := builder.AddDocument(DocID(4), []byte(`{"email":"final@example.com","score":9007199254740992}`)); err != nil {
		t.Fatalf("final AddDocument: %v", err)
	}
	if got := builder.docIDToPos[DocID(4)]; got != 1 {
		t.Fatalf("docIDToPos[4] = %d, want 1", got)
	}
	idx := builder.Finalize()
	if idx.Header.NumDocs != 2 {
		t.Fatalf("Header.NumDocs = %d, want 2", idx.Header.NumDocs)
	}
	requireRows(t, idx, EQ("$.email", As("domain", "example.com")), []int{0, 1})
	rawPathID := requirePathID(t, idx, "$.email")
	if _, ok := idx.NumericIndexes[rawPathID]; ok {
		t.Fatal(`NumericIndexes["$.email"] present, want soft-skipped raw numeric value absent`)
	}
	requireRows(t, idx, EQ("$.email", "float@example.com"), []int{})
}

func TestDecodeRestoresHardFailureModeDefaults(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 1)
	if err := builder.AddDocument(DocID(0), []byte(`{"name":"kept"}`)); err != nil {
		t.Fatalf("seed AddDocument: %v", err)
	}

	encoded, err := Encode(builder.Finalize())
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.Config == nil {
		t.Fatal("decoded.Config = nil, want config")
	}
	if decoded.Config.ParserFailureMode != IngestFailureHard {
		t.Fatalf("decoded.Config.ParserFailureMode = %q, want %q", decoded.Config.ParserFailureMode, IngestFailureHard)
	}
	if decoded.Config.NumericFailureMode != IngestFailureHard {
		t.Fatalf("decoded.Config.NumericFailureMode = %q, want %q", decoded.Config.NumericFailureMode, IngestFailureHard)
	}
}
