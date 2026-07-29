//go:build simdjson

package gin

import "testing"

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

	parser, err := NewSIMDParser()
	if err != nil {
		t.Fatalf("NewSIMDParser: %v", err)
	}
	t.Cleanup(func() {
		if err := parser.Close(); err != nil {
			t.Errorf("Close SIMD parser: %v", err)
		}
	})
	simdEncoded := buildAndEncodeWithParser(t, fx, parser)

	assertByteIdentical(t, fx.Name, simdEncoded, stdlibEncoded)
}
