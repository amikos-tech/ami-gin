//go:build simdjson

package gin

import "testing"

func TestSIMDParserPhase20Parity(t *testing.T) {
	parser := newTestSIMDParser(t)

	for _, fixture := range phase20SmokeFixtures {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			docs, err := phase20LoadRawJSONL(fixture.path)
			if err != nil {
				t.Fatalf("fixture=%s load %q: %v", fixture.name, fixture.path, err)
			}

			stdlibIndex, err := phase20BuildBenchmarkIndex(docs)
			if err != nil {
				t.Fatalf("fixture=%s parser=stdlib build: %v", fixture.name, err)
			}
			simdIndex, err := phase20BuildBenchmarkIndex(docs, WithParser(parser))
			if err != nil {
				t.Fatalf("fixture=%s parser=simd build: %v", fixture.name, err)
			}

			stdlibEncoded, err := Encode(stdlibIndex)
			if err != nil {
				t.Fatalf("fixture=%s parser=stdlib encode: %v", fixture.name, err)
			}
			simdEncoded, err := Encode(simdIndex)
			if err != nil {
				t.Fatalf("fixture=%s parser=simd encode: %v", fixture.name, err)
			}
			assertByteIdentical(t, "phase20/"+fixture.name, simdEncoded, stdlibEncoded)

			predicate := fixture.query.predicate
			if err := phase20PreflightQuery(stdlibIndex, predicate); err != nil {
				t.Fatalf("fixture=%s query=%s parser=stdlib preflight: %v", fixture.name, fixture.query.name, err)
			}
			if err := phase20PreflightQuery(simdIndex, predicate); err != nil {
				t.Fatalf("fixture=%s query=%s parser=simd preflight: %v", fixture.name, fixture.query.name, err)
			}
			stdlibRGs := stdlibIndex.Evaluate([]Predicate{predicate}).ToSlice()
			simdRGs := simdIndex.Evaluate([]Predicate{predicate}).ToSlice()
			if !intSliceEqual(simdRGs, stdlibRGs) {
				t.Fatalf(
					"fixture=%s query=%s predicate=%+v parser=simd got=%v parser=stdlib expected=%v",
					fixture.name,
					fixture.query.name,
					predicate,
					simdRGs,
					stdlibRGs,
				)
			}
		})
	}
}

func TestSIMDParserEvaluateParity(t *testing.T) {
	parser := newTestSIMDParser(t)
	stdlibIndex := buildEvaluateMatrixIndex(t)
	simdIndex := buildEvaluateMatrixIndex(t, WithParser(parser))

	for _, testCase := range evaluateMatrixCases() {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			stdlibRGs := stdlibIndex.Evaluate([]Predicate{testCase.pred}).ToSlice()
			if !intSliceEqual(stdlibRGs, testCase.wantRGs) {
				t.Fatalf(
					"case=%s predicate=%+v parser=stdlib got=%v expected=%v",
					testCase.name,
					testCase.pred,
					stdlibRGs,
					testCase.wantRGs,
				)
			}

			simdRGs := simdIndex.Evaluate([]Predicate{testCase.pred}).ToSlice()
			if !intSliceEqual(simdRGs, testCase.wantRGs) {
				t.Fatalf(
					"case=%s predicate=%+v parser=simd got=%v expected=%v",
					testCase.name,
					testCase.pred,
					simdRGs,
					testCase.wantRGs,
				)
			}
			if !intSliceEqual(simdRGs, stdlibRGs) {
				t.Fatalf(
					"case=%s predicate=%+v parser=simd got=%v parser=stdlib expected=%v",
					testCase.name,
					testCase.pred,
					simdRGs,
					stdlibRGs,
				)
			}
		})
	}
}
