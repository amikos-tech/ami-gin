//go:build simdjson

package gin

import (
	stderrors "errors"
	"strings"
	"testing"

	purejson "github.com/amikos-tech/pure-simdjson"
)

func newTestSIMDParser(t *testing.T) CloseableParser {
	t.Helper()
	parser, err := NewSIMDParser()
	if err != nil {
		t.Fatalf("NewSIMDParser: %v", err)
	}
	t.Cleanup(func() {
		if err := parser.Close(); err != nil {
			t.Errorf("Close SIMD parser: %v", err)
		}
	})
	return parser
}

func TestSIMDParserGoldenAuthoredFixtures(t *testing.T) {
	parser := newTestSIMDParser(t)

	for _, fx := range authoredParityFixtures() {
		fx := fx
		t.Run(fx.Name, func(t *testing.T) {
			encoded := buildAndEncodeWithParser(t, fx, parser)
			golden := loadGolden(t, fx.Name)
			assertByteIdentical(t, fx.Name, encoded, golden)
		})
	}
}

func TestSIMDParserNativeNumericFailuresUseNumericPolicy(t *testing.T) {
	tests := []struct {
		name    string
		jsonDoc []byte
	}{
		{
			name:    "overflowed-exponent",
			jsonDoc: []byte(`{"n":1e400}`),
		},
		{
			name:    "uint64-above-int64",
			jsonDoc: []byte(`{"n":9223372036854775808}`),
		},
		{
			name:    "larger-than-uint64",
			jsonDoc: []byte(`{"n":18446744073709551616}`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("hard-numeric-soft-parser", func(t *testing.T) {
				parser := newTestSIMDParser(t)
				cfg, err := NewConfig(
					WithParserFailureMode(IngestFailureSoft),
					WithNumericFailureMode(IngestFailureHard),
				)
				if err != nil {
					t.Fatalf("NewConfig: %v", err)
				}
				builder, err := NewBuilder(cfg, 1, WithParser(parser))
				if err != nil {
					t.Fatalf("NewBuilder: %v", err)
				}

				addErr := builder.AddDocument(DocID(0), tc.jsonDoc)
				var ingestErr *IngestError
				if !stderrors.As(addErr, &ingestErr) {
					t.Fatalf("AddDocument error = %v, want *IngestError", addErr)
				}
				if ingestErr.Layer() != IngestLayerNumeric || ingestErr.Path() != "$.n" {
					t.Fatalf(
						"IngestError = (layer=%q, path=%q), want (numeric, $.n)",
						ingestErr.Layer(),
						ingestErr.Path(),
					)
				}
				requireTypedSinkUncommitted(t, builder, 0)
			})

			t.Run("soft-numeric-hard-parser", func(t *testing.T) {
				parser := newTestSIMDParser(t)
				cfg, err := NewConfig(
					WithParserFailureMode(IngestFailureHard),
					WithNumericFailureMode(IngestFailureSoft),
				)
				if err != nil {
					t.Fatalf("NewConfig: %v", err)
				}
				builder, err := NewBuilder(cfg, 1, WithParser(parser))
				if err != nil {
					t.Fatalf("NewBuilder: %v", err)
				}

				if err := builder.AddDocument(DocID(0), tc.jsonDoc); err != nil {
					t.Fatalf("AddDocument: %v", err)
				}
				requireTypedSinkUncommitted(t, builder, 1)
			})

			t.Run("hard-numeric-hard-parser", func(t *testing.T) {
				parser := newTestSIMDParser(t)
				cfg, err := NewConfig(
					WithParserFailureMode(IngestFailureHard),
					WithNumericFailureMode(IngestFailureHard),
				)
				if err != nil {
					t.Fatalf("NewConfig: %v", err)
				}
				builder, err := NewBuilder(cfg, 1, WithParser(parser))
				if err != nil {
					t.Fatalf("NewBuilder: %v", err)
				}

				addErr := builder.AddDocument(DocID(0), tc.jsonDoc)
				var ingestErr *IngestError
				if !stderrors.As(addErr, &ingestErr) {
					t.Fatalf("AddDocument error = %v, want *IngestError", addErr)
				}
				if ingestErr.Layer() != IngestLayerNumeric || ingestErr.Path() != "$.n" {
					t.Fatalf(
						"IngestError = (layer=%q, path=%q), want (numeric, $.n)",
						ingestErr.Layer(),
						ingestErr.Path(),
					)
				}
				requireTypedSinkUncommitted(t, builder, 0)
			})

			t.Run("soft-numeric-soft-parser", func(t *testing.T) {
				parser := newTestSIMDParser(t)
				cfg, err := NewConfig(
					WithParserFailureMode(IngestFailureSoft),
					WithNumericFailureMode(IngestFailureSoft),
				)
				if err != nil {
					t.Fatalf("NewConfig: %v", err)
				}
				builder, err := NewBuilder(cfg, 1, WithParser(parser))
				if err != nil {
					t.Fatalf("NewBuilder: %v", err)
				}

				if err := builder.AddDocument(DocID(0), tc.jsonDoc); err != nil {
					t.Fatalf("AddDocument: %v", err)
				}
				requireTypedSinkUncommitted(t, builder, 1)
			})
		})
	}
}

func TestSIMDParserRejectedNumericFallbackPreservesTransformerPolicy(t *testing.T) {
	parsers := []struct {
		name   string
		parser Parser
	}{
		{name: "stdlib", parser: stdlibParser{}},
		{name: "simd", parser: newTestSIMDParser(t)},
	}

	for _, tc := range parsers {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := NewConfig(
				WithParserFailureMode(IngestFailureHard),
				WithNumericFailureMode(IngestFailureSoft),
				WithCustomTransformer("$.z", "reject", func(any) (any, bool) {
					return nil, false
				}),
			)
			if err != nil {
				t.Fatalf("NewConfig: %v", err)
			}
			builder, err := NewBuilder(cfg, 1, WithParser(tc.parser))
			if err != nil {
				t.Fatalf("NewBuilder: %v", err)
			}

			addErr := builder.AddDocument(DocID(0), []byte(`{"z":1e400}`))
			var ingestErr *IngestError
			if !stderrors.As(addErr, &ingestErr) {
				t.Fatalf("AddDocument error = %v, want *IngestError", addErr)
			}
			if ingestErr.Layer() != IngestLayerTransformer || ingestErr.Path() != "$.z" {
				t.Fatalf(
					"IngestError = (layer=%q, path=%q), want (transformer, $.z)",
					ingestErr.Layer(),
					ingestErr.Path(),
				)
			}
			requireTypedSinkUncommitted(t, builder, 0)
		})
	}
}

func TestSIMDParserRejectedNumericFallbackHonorsLastKeyWins(t *testing.T) {
	fx := parityFixture{
		Name:   "simd-rejected-numeric-last-key-wins",
		Config: DefaultConfig,
		NumRGs: 2,
		JSONDocs: [][]byte{
			[]byte(`{"n":1e400,"n":1}`),
			[]byte(`{"n":2}`),
		},
	}

	stdlibEncoded := buildAndEncodeWithParser(t, fx, stdlibParser{})
	simdEncoded := buildAndEncodeWithParser(t, fx, newTestSIMDParser(t))
	assertByteIdentical(t, fx.Name, simdEncoded, stdlibEncoded)
}

func TestSIMDParserNumericRoutingRecursesIntoArraysAndObjects(t *testing.T) {
	tests := []struct {
		name     string
		jsonDoc  []byte
		wantPath string
	}{
		{
			name:     "nested-in-array",
			jsonDoc:  []byte(`{"items":[1,1e400]}`),
			wantPath: "$.items[1]",
		},
		{
			name:     "nested-in-object",
			jsonDoc:  []byte(`{"a":{"b":1e400}}`),
			wantPath: "$.a.b",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parser := newTestSIMDParser(t)
			cfg, err := NewConfig(
				WithParserFailureMode(IngestFailureSoft),
				WithNumericFailureMode(IngestFailureHard),
			)
			if err != nil {
				t.Fatalf("NewConfig: %v", err)
			}
			builder, err := NewBuilder(cfg, 1, WithParser(parser))
			if err != nil {
				t.Fatalf("NewBuilder: %v", err)
			}

			addErr := builder.AddDocument(DocID(0), tc.jsonDoc)
			var ingestErr *IngestError
			if !stderrors.As(addErr, &ingestErr) {
				t.Fatalf("AddDocument error = %v, want *IngestError", addErr)
			}
			if ingestErr.Layer() != IngestLayerNumeric || ingestErr.Path() != tc.wantPath {
				t.Fatalf(
					"IngestError = (layer=%q, path=%q), want (numeric, %s)",
					ingestErr.Layer(),
					ingestErr.Path(),
					tc.wantPath,
				)
			}
		})
	}
}

func TestSIMDParserNumericRoutingPicksDeterministicFirstOffender(t *testing.T) {
	tests := []struct {
		name     string
		jsonDoc  []byte
		wantPath string
	}{
		{
			name:     "object-sorted-key-order",
			jsonDoc:  []byte(`{"z_bad":1e400,"a_bad":1e400}`),
			wantPath: "$.a_bad",
		},
		{
			name:     "array-ascending-index-order",
			jsonDoc:  []byte(`{"nums":[1e400,2e400]}`),
			wantPath: "$.nums[0]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			parser := newTestSIMDParser(t)
			cfg, err := NewConfig(
				WithParserFailureMode(IngestFailureSoft),
				WithNumericFailureMode(IngestFailureHard),
			)
			if err != nil {
				t.Fatalf("NewConfig: %v", err)
			}
			builder, err := NewBuilder(cfg, 1, WithParser(parser))
			if err != nil {
				t.Fatalf("NewBuilder: %v", err)
			}

			addErr := builder.AddDocument(DocID(0), tc.jsonDoc)
			var ingestErr *IngestError
			if !stderrors.As(addErr, &ingestErr) {
				t.Fatalf("AddDocument error = %v, want *IngestError", addErr)
			}
			if ingestErr.Layer() != IngestLayerNumeric || ingestErr.Path() != tc.wantPath {
				t.Fatalf(
					"IngestError = (layer=%q, path=%q), want (numeric, %s)",
					ingestErr.Layer(),
					ingestErr.Path(),
					tc.wantPath,
				)
			}
		})
	}
}

func TestRouteSIMDWellFormedFallbackSkipsAtNestingDepthLimit(t *testing.T) {
	depth := simdNestingDepthLimit
	jsonDoc := []byte(strings.Repeat(`{"a":`, depth) + "1e400" + strings.Repeat("}", depth))

	builder, err := NewBuilder(DefaultConfig(), 1)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	fallbackErr, routed := routeSIMDWellFormedFallback(jsonDoc, 0, builder)
	if fallbackErr != nil || routed {
		t.Fatalf("routeSIMDWellFormedFallback() = (%v, %v), want (nil, false)", fallbackErr, routed)
	}
}

func TestSIMDParserNestingDepthBoundaryContract(t *testing.T) {
	acceptedFixture := parityFixture{
		Name:   "simd-nesting-depth-accepted-boundary",
		Config: DefaultConfig,
		NumRGs: 1,
		JSONDocs: [][]byte{
			nestedObjectJSON(simdNestingDepthLimit - 1),
		},
	}
	stdlibEncoded := buildAndEncodeWithParser(t, acceptedFixture, stdlibParser{})
	simdEncoded := buildAndEncodeWithParser(t, acceptedFixture, newTestSIMDParser(t))
	assertByteIdentical(t, acceptedFixture.Name, simdEncoded, stdlibEncoded)

	rejectedDoc := nestedObjectJSON(simdNestingDepthLimit)
	stdlibBuilder, err := NewBuilder(DefaultConfig(), 1)
	if err != nil {
		t.Fatalf("NewBuilder stdlib: %v", err)
	}
	if err := stdlibBuilder.AddDocument(DocID(0), rejectedDoc); err != nil {
		t.Fatalf("stdlib AddDocument at SIMD depth limit: %v", err)
	}

	simdBuilder, err := NewBuilder(DefaultConfig(), 1, WithParser(newTestSIMDParser(t)))
	if err != nil {
		t.Fatalf("NewBuilder SIMD: %v", err)
	}
	addErr := simdBuilder.AddDocument(DocID(0), rejectedDoc)
	var ingestErr *IngestError
	if !stderrors.As(addErr, &ingestErr) {
		t.Fatalf("SIMD AddDocument error = %v, want *IngestError", addErr)
	}
	if ingestErr.Layer() != IngestLayerParser {
		t.Fatalf("SIMD IngestError.Layer() = %q, want parser", ingestErr.Layer())
	}
	if !stderrors.Is(addErr, purejson.ErrDepthLimitExceeded) {
		t.Fatalf("errors.Is(%v, purejson.ErrDepthLimitExceeded) = false", addErr)
	}
	if simdBuilder.Err() != nil {
		t.Fatalf("SIMD builder.Err() = %v, want nil for document-local depth failure", simdBuilder.Err())
	}
}

func TestSIMDParserMalformedTrailingNumericKnownPolicyAsymmetry(t *testing.T) {
	cfg, err := NewConfig(
		WithParserFailureMode(IngestFailureHard),
		WithNumericFailureMode(IngestFailureSoft),
	)
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	jsonDoc := []byte(`1e400 garbage`)

	stdlibBuilder, err := NewBuilder(cfg, 1)
	if err != nil {
		t.Fatalf("NewBuilder stdlib: %v", err)
	}
	if err := stdlibBuilder.AddDocument(DocID(0), jsonDoc); err != nil {
		t.Fatalf("stdlib AddDocument: %v", err)
	}
	requireTypedSinkUncommitted(t, stdlibBuilder, 1)

	simdBuilder, err := NewBuilder(cfg, 1, WithParser(newTestSIMDParser(t)))
	if err != nil {
		t.Fatalf("NewBuilder SIMD: %v", err)
	}
	addErr := simdBuilder.AddDocument(DocID(0), jsonDoc)
	var ingestErr *IngestError
	if !stderrors.As(addErr, &ingestErr) {
		t.Fatalf("SIMD AddDocument error = %v, want *IngestError", addErr)
	}
	if ingestErr.Layer() != IngestLayerParser {
		t.Fatalf("SIMD IngestError.Layer() = %q, want parser", ingestErr.Layer())
	}
	requireTypedSinkUncommitted(t, simdBuilder, 0)
}

func nestedObjectJSON(depth int) []byte {
	return []byte(strings.Repeat(`{"a":`, depth) + "0" + strings.Repeat("}", depth))
}

func TestSIMDParserMalformedJSONStillUsesParserPolicy(t *testing.T) {
	parser := newTestSIMDParser(t)
	cfg, err := NewConfig(
		WithParserFailureMode(IngestFailureSoft),
		WithNumericFailureMode(IngestFailureHard),
	)
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	builder, err := NewBuilder(cfg, 1, WithParser(parser))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	if err := builder.AddDocument(DocID(0), []byte(`{"n":}`)); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	requireTypedSinkUncommitted(t, builder, 1)
}

func TestSIMDParserCloseIsIdempotent(t *testing.T) {
	parser, err := NewSIMDParser()
	if err != nil {
		t.Fatalf("NewSIMDParser: %v", err)
	}
	if err := parser.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := parser.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestSIMDParserCloseErrorPropagatesWhenParserBusy(t *testing.T) {
	cp, err := NewSIMDParser()
	if err != nil {
		t.Fatalf("NewSIMDParser: %v", err)
	}
	sp, ok := cp.(*simdParser)
	if !ok {
		t.Fatalf("NewSIMDParser() = %T, want *simdParser", cp)
	}

	doc, err := sp.parser.Parse([]byte(`{"a":1}`))
	if err != nil {
		t.Fatalf("underlying Parse: %v", err)
	}

	closeErr := cp.Close()
	if closeErr == nil {
		t.Fatal("Close() while document is live = nil, want ErrParserBusy")
	}
	if !stderrors.Is(closeErr, purejson.ErrParserBusy) {
		t.Fatalf("errors.Is(%v, purejson.ErrParserBusy) = false", closeErr)
	}
	if !strings.Contains(closeErr.Error(), "close pure-simdjson SIMD parser") {
		t.Fatalf("Close() error = %q, want cleanup context", closeErr.Error())
	}

	if err := doc.Close(); err != nil {
		t.Fatalf("doc.Close: %v", err)
	}
	if err := cp.Close(); err != nil {
		t.Fatalf("Close() after releasing live doc = %v, want nil", err)
	}
}

func TestSIMDParserParseAfterExternalCloseReturnsUsableBuilderError(t *testing.T) {
	parser, err := NewSIMDParser()
	if err != nil {
		t.Fatalf("NewSIMDParser: %v", err)
	}
	if err := parser.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	cfg, err := NewConfig(WithParserFailureMode(IngestFailureHard))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	builder, err := NewBuilder(cfg, 2, WithParser(parser))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	jsonDocs := [][]byte{[]byte(`{"a":1}`), []byte(`{"b":2}`)}
	for i, jsonDoc := range jsonDocs {
		addErr := builder.AddDocument(DocID(i), jsonDoc)
		var ingestErr *IngestError
		if !stderrors.As(addErr, &ingestErr) {
			t.Fatalf("AddDocument[%d] error = %v, want *IngestError", i, addErr)
		}
		if ingestErr.Layer() != IngestLayerParser {
			t.Fatalf("AddDocument[%d] IngestError.Layer() = %q, want parser", i, ingestErr.Layer())
		}
		if builder.Err() != nil {
			t.Fatalf("builder.Err() after AddDocument[%d] = %v, want nil", i, builder.Err())
		}
	}
}
