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
			Name:   "simd-numeric-parity",
			Config: DefaultConfig,
			NumRGs: 1,
			JSONDocs: [][]byte{
				// Keep each numeric class on its own path. Mixing the exact
				// large integer with floats on one path is intentionally invalid.
				[]byte(`{"plain_int":1,"whole_float":1.0,"exp_float":1e18,"fraction":1.5,"exact_large_int":9007199254740993}`),
			},
		},
		{
			Name:   "mixed-float-int",
			Config: DefaultConfig,
			NumRGs: 3,
			JSONDocs: [][]byte{
				[]byte(`{"metrics":{"score":1,"ratio":1.25},"status":"warm"}`),
				[]byte(`{"metrics":{"score":2.5,"ratio":2},"status":"cold"}`),
				[]byte(`{"metrics":{"score":3,"ratio":3.75},"status":"hot"}`),
			},
		},
		{
			Name:   "single-rg-array-siblings",
			Config: DefaultConfig,
			NumRGs: 1,
			JSONDocs: [][]byte{
				[]byte(`{"items":[{"label":"alpha","score":1.5},{"label":"beta","score":2}],"meta":{"flag":true}}`),
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
			Name: "transformer-buffered-container-numerics",
			Config: func() GINConfig {
				cfg := DefaultConfig()
				if err := WithToLowerTransformer(
					"$.payload",
					"lower",
					WithTransformerFailureMode(IngestFailureSoft),
				)(&cfg); err != nil {
					panic(err)
				}
				return cfg
			},
			NumRGs: 2,
			JSONDocs: [][]byte{
				[]byte(`{"payload":{"integer":2,"whole":2.0,"fraction":2.5,"items":[3,3.0,{"nested":4.0}]}}`),
				[]byte(`{"payload":[{"integer":-5,"whole":6.0},7.25]}`),
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
