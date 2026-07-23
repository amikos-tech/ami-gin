//go:build simdjson

package gin

import (
	stderrors "errors"
	"strings"
	"testing"
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
		})
	}
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

func TestRouteSIMDNumericParseFailureSkipsWhenNestingExceedsMaxDepth(t *testing.T) {
	depth := simdMaxNestingDepth + 10
	jsonDoc := []byte(strings.Repeat(`{"a":`, depth) + "1e400" + strings.Repeat("}", depth))

	builder, err := NewBuilder(DefaultConfig(), 1)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	numericErr, routed := routeSIMDNumericParseFailure(jsonDoc, 0, builder)
	if numericErr != nil || routed {
		t.Fatalf("routeSIMDNumericParseFailure() = (%v, %v), want (nil, false)", numericErr, routed)
	}
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
