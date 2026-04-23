package gin

type parityFixture struct {
	Name     string
	Config   func() GINConfig
	NumRGs   int
	JSONDocs [][]byte
}

func authoredParityFixtures() []parityFixture {
	return []parityFixture{
		{
			Name:   "int64-boundaries",
			Config: DefaultConfig,
			NumRGs: 4,
			JSONDocs: [][]byte{
				[]byte(`{"a": 9223372036854775807}`),
				[]byte(`{"a": -9223372036854775807}`),
				[]byte(`{"a": 9007199254740993}`),
				[]byte(`{"a": 0}`),
			},
		},
		{
			Name:   "nulls-and-missing",
			Config: DefaultConfig,
			NumRGs: 4,
			JSONDocs: [][]byte{
				[]byte(`{"a": null, "b": "x"}`),
				[]byte(`{"b": "y"}`),
				[]byte(`{"a": null}`),
				[]byte(`{"a": "present", "b": null}`),
			},
		},
		{
			Name:   "deep-nested",
			Config: DefaultConfig,
			NumRGs: 2,
			JSONDocs: [][]byte{
				[]byte(`{"l1": {"l2": {"l3": {"l4": {"leaf": 42}}}}}`),
				[]byte(`{"l1": {"l2": [1, 2, {"leaf": "deep"}]}}`),
			},
		},
		{
			Name:   "unicode-keys",
			Config: DefaultConfig,
			NumRGs: 2,
			JSONDocs: [][]byte{
				[]byte(`{"ключ": "value", "日本語": 1}`),
				[]byte(`{"emoji🎉": true, "ascii": "mix"}`),
			},
		},
		{
			Name:   "empty-arrays",
			Config: DefaultConfig,
			NumRGs: 3,
			JSONDocs: [][]byte{
				[]byte(`{"arr": []}`),
				[]byte(`{"arr": [[], [], []]}`),
				[]byte(`{"nested": {"inner": []}}`),
			},
		},
		{
			Name:   "large-strings",
			Config: DefaultConfig,
			NumRGs: 2,
			JSONDocs: [][]byte{
				[]byte(`{"text": "` + repeatASCII("the quick brown fox jumps over the lazy dog ", 20) + `"}`),
				[]byte(`{"text": "` + repeatASCII("abcdefghijklmnopqrstuvwxyz0123456789 ", 30) + `"}`),
			},
		},
		{
			Name: "transformers-iso-date-and-lower",
			Config: func() GINConfig {
				cfg := DefaultConfig()
				if err := WithISODateTransformer("$.created_at", "epoch_ms")(&cfg); err != nil {
					panic(err)
				}
				if err := WithToLowerTransformer("$.email", "lower")(&cfg); err != nil {
					panic(err)
				}
				return cfg
			},
			NumRGs: 4,
			JSONDocs: [][]byte{
				[]byte(`{"created_at": "2024-01-15T10:30:00Z", "email": "Alice@EXAMPLE.COM"}`),
				[]byte(`{"created_at": "2024-02-20T08:00:00Z", "email": "bob@example.com"}`),
				[]byte(`{"created_at": "2024-03-01T00:00:00Z", "email": "CHARLIE@example.com"}`),
				[]byte(`{"created_at": "2024-04-10T23:59:59Z", "email": "david@EXAMPLE.COM"}`),
			},
		},
		{
			Name: "transformers-soft-fail-wire",
			Config: func() GINConfig {
				cfg := DefaultConfig()
				if err := WithToLowerTransformer("$.email", "lower", WithTransformerFailureMode(IngestFailureSoft))(&cfg); err != nil {
					panic(err)
				}
				return cfg
			},
			NumRGs: 2,
			JSONDocs: [][]byte{
				[]byte(`{"email":"Alice@Example.COM"}`),
				[]byte(`{"email":"Bob@Example.COM"}`),
			},
		},
	}
}

func repeatASCII(s string, n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
