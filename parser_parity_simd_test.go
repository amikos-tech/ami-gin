//go:build simdjson

package gin

import (
	stderrors "errors"
	"strings"
	"testing"
)

func softSkipParityFixture() parityFixture {
	return parityFixture{
		Name: "soft-skip-and-malformed-json-parity",
		Config: func() GINConfig {
			cfg := DefaultConfig()
			if err := WithParserFailureMode(IngestFailureSoft)(&cfg); err != nil {
				panic(err)
			}
			if err := WithNumericFailureMode(IngestFailureSoft)(&cfg); err != nil {
				panic(err)
			}
			return cfg
		},
		NumRGs: 4,
		JSONDocs: [][]byte{
			[]byte(`{"name":"alice","age":30}`),
			[]byte(`{"name":}`),
			[]byte(`{"name":"bob","overflow":1e400}`),
			[]byte(`{"name":"carol","age":40}`),
		},
	}
}

func TestSIMDParserSoftSkipAndMalformedJSONByteParity(t *testing.T) {
	fx := softSkipParityFixture()

	stdlibEncoded := buildAndEncodeWithParser(t, fx, stdlibParser{})

	parser := newTestSIMDParser(t)
	simdEncoded := buildAndEncodeWithParser(t, fx, parser)

	assertByteIdentical(t, fx.Name, simdEncoded, stdlibEncoded)
}

func TestSIMDParserNestedArraysStageOnlyWildcardPaths(t *testing.T) {
	const depth = 8
	document := []byte(strings.Repeat("[", depth) + "1" + strings.Repeat("]", depth))

	parser := newTestSIMDParser(t)
	builder, err := NewBuilder(DefaultConfig(), 1, WithParser(parser))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	if err := builder.AddDocument(0, document); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	idx := builder.Finalize()

	if got, want := len(idx.PathDirectory), depth+1; got != want {
		t.Fatalf("PathDirectory count = %d, want %d", got, want)
	}
	for _, entry := range idx.PathDirectory {
		if hasNumericArrayIndex(entry.PathName) {
			t.Fatalf("PathDirectory contains private numeric path %q", entry.PathName)
		}
	}
}

// TestSIMDParserBudgetRejectsLexicalOrderPath pins that object keys are
// staged in lexical order, not document order. Under document order (z, a,
// m) the budget would instead reject $.m; the rejection landing on $.z
// proves the walker sorts keys before staging.
func TestSIMDParserBudgetRejectsLexicalOrderPath(t *testing.T) {
	config, err := NewConfig(WithMaxStagedPaths(3))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	parser := newTestSIMDParser(t)
	builder, err := NewBuilder(config, 1, WithParser(parser))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	err = builder.AddDocument(0, []byte(`{"z":{},"a":{},"m":{}}`))
	var ingestErr *IngestError
	if !stderrors.As(err, &ingestErr) {
		t.Fatalf("AddDocument error = %T %v, want *IngestError", err, err)
	}
	if got := ingestErr.Path(); got != "$.z" {
		t.Fatalf("IngestError.Path() = %q, want $.z", got)
	}
	if builder.numDocs != 0 {
		t.Fatalf("rejected document was committed: numDocs=%d", builder.numDocs)
	}
}

func TestSIMDParserPropagatesMarkPresentBudgetFailure(t *testing.T) {
	tests := []struct {
		name string
		doc  []byte
	}{
		{name: "object", doc: []byte(`{"a":{},"b":{},"c":{}}`)},
		{name: "array", doc: []byte(`{"a":[],"b":[],"c":[]}`)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config, err := NewConfig(WithMaxStagedPaths(2))
			if err != nil {
				t.Fatalf("NewConfig: %v", err)
			}
			parser := newTestSIMDParser(t)
			builder, err := NewBuilder(config, 1, WithParser(parser))
			if err != nil {
				t.Fatalf("NewBuilder: %v", err)
			}

			err = builder.AddDocument(0, tc.doc)
			var ingestErr *IngestError
			if !stderrors.As(err, &ingestErr) {
				t.Fatalf("AddDocument error = %T %v, want *IngestError", err, err)
			}
			if got := ingestErr.Path(); got != "$.b" {
				t.Fatalf("IngestError.Path() = %q, want $.b", got)
			}
			if builder.numDocs != 0 {
				t.Fatalf("rejected document was committed: numDocs=%d", builder.numDocs)
			}
		})
	}
}
