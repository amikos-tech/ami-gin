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
		if strings.Contains(entry.PathName, "[0]") {
			t.Fatalf("PathDirectory contains private numeric path %q", entry.PathName)
		}
	}
}

func TestSIMDParserPropagatesMarkPresentBudgetFailure(t *testing.T) {
	config, err := NewConfig(WithMaxStagedPaths(2))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	parser := newTestSIMDParser(t)
	builder, err := NewBuilder(config, 1, WithParser(parser))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	err = builder.AddDocument(0, []byte(`{"a":{},"b":{},"c":{}}`))
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
}
