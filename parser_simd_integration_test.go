//go:build simdjson

package gin

import (
	stderrors "errors"
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
