package gin

import (
	"bytes"
	stderrors "errors"
	"math"
	"strings"
	"testing"
)

type typedSinkTestParser struct {
	stage func(parserSink, *documentBuildState) error
}

func (typedSinkTestParser) Name() string { return "typed-sink-test" }

func (p typedSinkTestParser) Parse(_ []byte, rgID int, sink parserSink) error {
	state := sink.BeginDocument(rgID)
	return p.stage(sink, state)
}

func newTypedSinkTestBuilder(t *testing.T, cfg GINConfig, stage func(parserSink, *documentBuildState) error) *GINBuilder {
	t.Helper()
	builder, err := NewBuilder(cfg, 1, WithParser(typedSinkTestParser{stage: stage}))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	return builder
}

func buildCommittedTypedSinkDocument(
	t *testing.T,
	cfg GINConfig,
	parser Parser,
	jsonDoc []byte,
) (*GINIndex, []byte) {
	t.Helper()

	opts := make([]BuilderOption, 0, 1)
	if parser != nil {
		opts = append(opts, WithParser(parser))
	}
	builder, err := NewBuilder(cfg, 1, opts...)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	if err := builder.AddDocument(DocID(0), jsonDoc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if builder.numDocs != 1 || builder.nextPos != 1 {
		t.Fatalf("committed document counters = (numDocs=%d, nextPos=%d), want (1, 1)", builder.numDocs, builder.nextPos)
	}
	if _, ok := builder.docIDToPos[DocID(0)]; !ok {
		t.Fatal("committed document is absent from docIDToPos")
	}
	if _, ok := builder.pathData["$"]; !ok {
		t.Fatal("root path was not committed; encoded comparison would be empty")
	}

	idx := builder.Finalize()
	if idx == nil {
		t.Fatal("Finalize returned nil")
	}
	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return idx, encoded
}

func requireTypedSinkEncodedEqual(t *testing.T, got, want []byte) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Fatalf("encoded indexes differ (got %d bytes, want %d bytes)", len(got), len(want))
	}
}

func requireTypedSinkUncommitted(t *testing.T, builder *GINBuilder, wantSoftSkips uint64) {
	t.Helper()
	if builder.Err() != nil {
		t.Fatalf("builder.Err() = %v, want nil", builder.Err())
	}
	if builder.numDocs != 0 || builder.nextPos != 0 {
		t.Fatalf("rejected document changed counters: numDocs=%d nextPos=%d", builder.numDocs, builder.nextPos)
	}
	if len(builder.docIDToPos) != 0 {
		t.Fatalf("rejected document changed docIDToPos: %+v", builder.docIDToPos)
	}
	if len(builder.pathData) != 0 {
		t.Fatalf("rejected document committed path data: %+v", builder.pathData)
	}
	if got := builder.NumSoftSkippedDocuments(); got != wantSoftSkips {
		t.Fatalf("NumSoftSkippedDocuments() = %d, want %d", got, wantSoftSkips)
	}
}

func requireTypedSinkNumericError(t *testing.T, err error) *IngestError {
	t.Helper()
	if err == nil {
		t.Fatal("AddDocument error = nil, want numeric IngestError")
	}
	var ingestErr *IngestError
	if !stderrors.As(err, &ingestErr) {
		t.Fatalf("AddDocument error = %v, want *IngestError", err)
	}
	if ingestErr.Layer() != IngestLayerNumeric || ingestErr.Path() != "$" {
		t.Fatalf("IngestError = (layer=%q, path=%q), want (numeric, $)", ingestErr.Layer(), ingestErr.Path())
	}
	return ingestErr
}

func typedSinkNumericConfig(t *testing.T, mode IngestFailureMode) GINConfig {
	t.Helper()
	cfg := DefaultConfig()
	if err := WithNumericFailureMode(mode)(&cfg); err != nil {
		t.Fatalf("WithNumericFailureMode(%q): %v", mode, err)
	}
	return cfg
}

func TestTypedSinkInt64PreservesExactValue(t *testing.T) {
	const exact = int64(9007199254740993)
	stage := func(sink parserSink, state *documentBuildState) error {
		return sink.StageInt64(state, "$", exact)
	}

	typedIndex, typedEncoded := buildCommittedTypedSinkDocument(
		t,
		DefaultConfig(),
		typedSinkTestParser{stage: stage},
		[]byte(`null`),
	)
	_, stdlibEncoded := buildCommittedTypedSinkDocument(
		t,
		DefaultConfig(),
		nil,
		[]byte(`9007199254740993`),
	)
	requireTypedSinkEncodedEqual(t, typedEncoded, stdlibEncoded)

	rows := typedIndex.Evaluate([]Predicate{EQ("$", exact)}).ToSlice()
	if len(rows) != 1 || rows[0] != 0 {
		t.Fatalf("exact int64 query rows = %v, want [0]", rows)
	}
}

func TestTypedSinkFloat64PreservesLexemeClass(t *testing.T) {
	tests := []struct {
		name    string
		value   float64
		jsonDoc []byte
	}{
		{name: "whole-float", value: 1.0, jsonDoc: []byte(`1.0`)},
		{name: "exponent-float", value: 1e18, jsonDoc: []byte(`1e18`)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			typedStage := func(sink parserSink, state *documentBuildState) error {
				return sink.StageFloat64(state, "$", tc.value)
			}
			nativeStage := func(sink parserSink, state *documentBuildState) error {
				return sink.StageNativeNumeric(state, "$", tc.value)
			}

			_, typedEncoded := buildCommittedTypedSinkDocument(
				t,
				DefaultConfig(),
				typedSinkTestParser{stage: typedStage},
				[]byte(`null`),
			)
			_, stdlibEncoded := buildCommittedTypedSinkDocument(t, DefaultConfig(), nil, tc.jsonDoc)
			_, nativeEncoded := buildCommittedTypedSinkDocument(
				t,
				DefaultConfig(),
				typedSinkTestParser{stage: nativeStage},
				[]byte(`null`),
			)

			requireTypedSinkEncodedEqual(t, typedEncoded, stdlibEncoded)
			if bytes.Equal(typedEncoded, nativeEncoded) {
				t.Fatal("StageFloat64 encoded like StageNativeNumeric; whole-float coercion was not bypassed")
			}
		})
	}
}

func TestTypedSinkUint64OverflowModes(t *testing.T) {
	overflow := uint64(math.MaxInt64) + 1
	stage := func(sink parserSink, state *documentBuildState) error {
		return sink.StageUint64(state, "$", overflow)
	}

	t.Run("hard", func(t *testing.T) {
		typedBuilder := newTypedSinkTestBuilder(t, DefaultConfig(), stage)
		typedErr := typedBuilder.AddDocument(DocID(0), []byte(`null`))
		typedIngestErr := requireTypedSinkNumericError(t, typedErr)
		if !strings.Contains(typedIngestErr.Cause().Error(), "unsupported integer") {
			t.Fatalf("typed IngestError.Cause() = %v, want unsupported integer", typedIngestErr.Cause())
		}
		requireTypedSinkUncommitted(t, typedBuilder, 0)

		stdlibBuilder, err := NewBuilder(DefaultConfig(), 1)
		if err != nil {
			t.Fatalf("NewBuilder stdlib: %v", err)
		}
		stdlibErr := stdlibBuilder.AddDocument(DocID(0), []byte(`9223372036854775808`))
		requireTypedSinkNumericError(t, stdlibErr)
		requireTypedSinkUncommitted(t, stdlibBuilder, 0)
	})

	t.Run("soft", func(t *testing.T) {
		cfg := typedSinkNumericConfig(t, IngestFailureSoft)
		typedBuilder := newTypedSinkTestBuilder(t, cfg, stage)
		if err := typedBuilder.AddDocument(DocID(0), []byte(`null`)); err != nil {
			t.Fatalf("typed soft AddDocument: %v", err)
		}
		requireTypedSinkUncommitted(t, typedBuilder, 1)

		stdlibBuilder, err := NewBuilder(cfg, 1)
		if err != nil {
			t.Fatalf("NewBuilder stdlib: %v", err)
		}
		if err := stdlibBuilder.AddDocument(DocID(0), []byte(`9223372036854775808`)); err != nil {
			t.Fatalf("stdlib soft AddDocument: %v", err)
		}
		requireTypedSinkUncommitted(t, stdlibBuilder, 1)
	})
}

func TestTypedSinkFloat64NonFiniteModes(t *testing.T) {
	tests := []struct {
		name  string
		value float64
	}{
		{name: "nan", value: math.NaN()},
		{name: "positive-infinity", value: math.Inf(1)},
		{name: "negative-infinity", value: math.Inf(-1)},
	}

	for _, tc := range tests {
		t.Run(tc.name+"/hard", func(t *testing.T) {
			stage := func(sink parserSink, state *documentBuildState) error {
				return sink.StageFloat64(state, "$", tc.value)
			}
			builder := newTypedSinkTestBuilder(t, DefaultConfig(), stage)
			ingestErr := requireTypedSinkNumericError(t, builder.AddDocument(DocID(0), []byte(`null`)))
			if !strings.Contains(ingestErr.Cause().Error(), "non-finite numeric value") {
				t.Fatalf("IngestError.Cause() = %v, want non-finite numeric value", ingestErr.Cause())
			}
			requireTypedSinkUncommitted(t, builder, 0)
		})

		t.Run(tc.name+"/soft", func(t *testing.T) {
			stage := func(sink parserSink, state *documentBuildState) error {
				return sink.StageFloat64(state, "$", tc.value)
			}
			builder := newTypedSinkTestBuilder(t, typedSinkNumericConfig(t, IngestFailureSoft), stage)
			if err := builder.AddDocument(DocID(0), []byte(`null`)); err != nil {
				t.Fatalf("soft AddDocument: %v", err)
			}
			requireTypedSinkUncommitted(t, builder, 1)
		})
	}
}

var _ Parser = typedSinkTestParser{}
