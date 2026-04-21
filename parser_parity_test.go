package gin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/prop"
	"github.com/pkg/errors"
)

func loadGolden(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", "parity-golden", name+".bin")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("load golden %s: %v (goldens are committed by Plan 02 Task 4; if missing, regenerate via `go test -tags regenerate_goldens -run TestRegenerateParityGoldens .`)", name, err)
	}
	return b
}

func assertByteIdentical(t *testing.T, fixtureName string, encoded, golden []byte) {
	t.Helper()
	if len(encoded) != len(golden) {
		t.Fatalf("parity %s: byte length differs (encoded=%d golden=%d)", fixtureName, len(encoded), len(golden))
	}
	if !bytes.Equal(encoded, golden) {
		for i := range encoded {
			if encoded[i] != golden[i] {
				t.Fatalf("parity %s: first diff at byte offset %d (encoded=0x%02x golden=0x%02x)", fixtureName, i, encoded[i], golden[i])
			}
		}
	}
}

func buildAndEncodeWithParser(t *testing.T, fx parityFixture, parser Parser) []byte {
	t.Helper()
	cfg := fx.Config()
	builder, err := NewBuilder(cfg, fx.NumRGs, WithParser(parser))
	if err != nil {
		t.Fatalf("NewBuilder for %s: %v", fx.Name, err)
	}
	for i, doc := range fx.JSONDocs {
		if err := builder.AddDocument(DocID(i), doc); err != nil {
			t.Fatalf("AddDocument[%d] for %s: %v", i, fx.Name, err)
		}
	}
	idx := builder.Finalize()
	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("Encode for %s: %v", fx.Name, err)
	}
	return encoded
}

func buildAndEncode(t *testing.T, fx parityFixture) []byte {
	t.Helper()
	return buildAndEncodeWithParser(t, fx, stdlibParser{})
}

func TestParserParity_AuthoredFixtures(t *testing.T) {
	for _, fx := range authoredParityFixtures() {
		fx := fx
		t.Run(fx.Name, func(t *testing.T) {
			encoded := buildAndEncode(t, fx)
			golden := loadGolden(t, fx.Name)
			assertByteIdentical(t, fx.Name, encoded, golden)
		})
	}
}

type materializingParser struct{}

func (materializingParser) Name() string { return "materializing" }

func (materializingParser) Parse(jsonDoc []byte, rgID int, sink parserSink) error {
	decoder := json.NewDecoder(bytes.NewReader(jsonDoc))
	decoder.UseNumber()

	value, err := decodeAny(decoder)
	if err != nil {
		return err
	}
	if err := ensureDecoderEOF(decoder); err != nil {
		return errors.Wrap(err, "failed to parse JSON")
	}

	state := sink.BeginDocument(rgID)
	return stageMaterializedDocument(sink, state, "$", value)
}

func stageMaterializedDocument(sink parserSink, state *documentBuildState, path string, value any) error {
	canonicalPath := normalizeWalkPath(path)
	if sink.ShouldBufferForTransform(canonicalPath) {
		return sink.StageMaterialized(state, path, value, true)
	}

	switch v := value.(type) {
	case map[string]any:
		sink.MarkPresent(state, canonicalPath)
		for _, key := range sortedObjectKeys(v) {
			if err := sink.StageMaterialized(state, path+"."+key, v[key], true); err != nil {
				return err
			}
		}
		return nil
	case []any:
		sink.MarkPresent(state, canonicalPath)
		for i, item := range v {
			if err := sink.StageMaterialized(state, fmt.Sprintf("%s[%d]", path, i), item, true); err != nil {
				return err
			}
			if err := sink.StageMaterialized(state, path+"[*]", item, true); err != nil {
				return err
			}
		}
		return nil
	default:
		return sink.StageScalar(state, canonicalPath, value)
	}
}

func parserEquivalenceFixtures() []parityFixture {
	fixtures := append([]parityFixture{}, authoredParityFixtures()...)
	fixtures = append(fixtures,
		parityFixture{
			Name:   "mixed-float-int",
			Config: DefaultConfig,
			NumRGs: 3,
			JSONDocs: [][]byte{
				[]byte(`{"metrics":{"score":1,"ratio":1.25},"status":"warm"}`),
				[]byte(`{"metrics":{"score":2.5,"ratio":2},"status":"cold"}`),
				[]byte(`{"metrics":{"score":3,"ratio":3.75},"status":"hot"}`),
			},
		},
		parityFixture{
			Name:   "single-rg-array-siblings",
			Config: DefaultConfig,
			NumRGs: 1,
			JSONDocs: [][]byte{
				[]byte(`{"items":[{"label":"alpha","score":1.5},{"label":"beta","score":2}],"meta":{"flag":true}}`),
			},
		},
	)
	return fixtures
}

func TestParserParity_StdlibMatchesMaterializingParser(t *testing.T) {
	for _, fx := range parserEquivalenceFixtures() {
		fx := fx
		t.Run(fx.Name, func(t *testing.T) {
			stdlibEncoded := buildAndEncodeWithParser(t, fx, stdlibParser{})
			materializedEncoded := buildAndEncodeWithParser(t, fx, materializingParser{})
			assertByteIdentical(t, fx.Name, materializedEncoded, stdlibEncoded)
		})
	}
}

func TestParserSeam_DeterministicAcrossRuns(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 50
	properties := gopter.NewProperties(params)

	properties.Property("determinism across GenTestDocs", prop.ForAll(
		func(docs []TestDoc) bool {
			if len(docs) == 0 {
				return true
			}
			a, err := encodeDocs(docs)
			if err != nil {
				return false
			}
			b, err := encodeDocs(docs)
			if err != nil {
				return false
			}
			return bytes.Equal(a, b)
		},
		GenTestDocs(25),
	))

	properties.Property("determinism across GenTestDocsWithNulls", prop.ForAll(
		func(docs []TestDoc) bool {
			if len(docs) == 0 {
				return true
			}
			a, err := encodeDocs(docs)
			if err != nil {
				return false
			}
			b, err := encodeDocs(docs)
			if err != nil {
				return false
			}
			return bytes.Equal(a, b)
		},
		GenTestDocsWithNulls(25),
	))

	properties.Property("determinism across GenMixedTypeDocs", prop.ForAll(
		func(docs []TestDoc) bool {
			if len(docs) == 0 {
				return true
			}
			a, err := encodeDocs(docs)
			if err != nil {
				return false
			}
			b, err := encodeDocs(docs)
			if err != nil {
				return false
			}
			return bytes.Equal(a, b)
		},
		GenMixedTypeDocs(25),
	))

	properties.TestingRun(t)
}

func TestParserSeam_EquivalentAcrossParsers(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 50
	properties := gopter.NewProperties(params)

	properties.Property("stdlib equals materializing across GenTestDocs", prop.ForAll(
		func(docs []TestDoc) bool {
			if len(docs) == 0 {
				return true
			}
			stdlibEncoded, err := encodeDocsWithParser(docs, stdlibParser{})
			if err != nil {
				return false
			}
			materializedEncoded, err := encodeDocsWithParser(docs, materializingParser{})
			if err != nil {
				return false
			}
			return bytes.Equal(stdlibEncoded, materializedEncoded)
		},
		GenTestDocs(25),
	))

	properties.Property("stdlib equals materializing across GenTestDocsWithNulls", prop.ForAll(
		func(docs []TestDoc) bool {
			if len(docs) == 0 {
				return true
			}
			stdlibEncoded, err := encodeDocsWithParser(docs, stdlibParser{})
			if err != nil {
				return false
			}
			materializedEncoded, err := encodeDocsWithParser(docs, materializingParser{})
			if err != nil {
				return false
			}
			return bytes.Equal(stdlibEncoded, materializedEncoded)
		},
		GenTestDocsWithNulls(25),
	))

	properties.Property("stdlib equals materializing across GenMixedTypeDocs", prop.ForAll(
		func(docs []TestDoc) bool {
			if len(docs) == 0 {
				return true
			}
			stdlibEncoded, err := encodeDocsWithParser(docs, stdlibParser{})
			if err != nil {
				return false
			}
			materializedEncoded, err := encodeDocsWithParser(docs, materializingParser{})
			if err != nil {
				return false
			}
			return bytes.Equal(stdlibEncoded, materializedEncoded)
		},
		GenMixedTypeDocs(25),
	))

	properties.TestingRun(t)
}

func encodeDocs(docs []TestDoc) ([]byte, error) {
	return encodeDocsWithParser(docs, stdlibParser{})
}

func encodeDocsWithParser(docs []TestDoc, parser Parser) ([]byte, error) {
	if len(docs) == 0 {
		return nil, errors.New("empty docs slice")
	}
	builder, err := NewBuilder(DefaultConfig(), len(docs), WithParser(parser))
	if err != nil {
		return nil, errors.Wrap(err, "NewBuilder")
	}
	for i, doc := range docs {
		if err := builder.AddDocument(DocID(i), doc.JSON); err != nil {
			return nil, errors.Wrapf(err, "AddDocument[%d]", i)
		}
	}
	idx := builder.Finalize()
	encoded, err := Encode(idx)
	if err != nil {
		return nil, errors.Wrap(err, "Encode")
	}
	return encoded, nil
}

// IMPORTANT: query.go returns AllRGs when a predicate path is absent from the
// index. That is an unknown-path fallback, not pruning. Every *-prune case
// below targets a known path and must return a proper subset of the full
// corpus or the empty set.

func evaluateMatrixFixture() parityFixture {
	return parityFixture{
		Name:   "evaluate-matrix",
		Config: DefaultConfig,
		NumRGs: 4,
		JSONDocs: [][]byte{
			[]byte(`{"name":"alice","age":30,"status":"active","bio":"hello world"}`),
			[]byte(`{"name":"bob","age":25,"status":"inactive","bio":"foo bar baz"}`),
			[]byte(`{"name":"alice","age":40,"status":null,"bio":"test message qux"}`),
			[]byte(`{"name":"carol","age":35,"bio":"hello again"}`),
		},
	}
}

func buildEvaluateMatrixIndex(t *testing.T) *GINIndex {
	t.Helper()
	fx := evaluateMatrixFixture()
	cfg := fx.Config()
	builder, err := NewBuilder(cfg, fx.NumRGs)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	for i, doc := range fx.JSONDocs {
		if err := builder.AddDocument(DocID(i), doc); err != nil {
			t.Fatalf("AddDocument[%d]: %v", i, err)
		}
	}
	return builder.Finalize()
}

func intSliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestParserParity_EvaluateMatrix(t *testing.T) {
	idx := buildEvaluateMatrixIndex(t)

	cases := []struct {
		name    string
		pred    Predicate
		wantRGs []int
	}{
		{"EQ-match", EQ("$.name", "alice"), []int{0, 2}},
		{"EQ-prune", EQ("$.name", "nobody"), []int{}},
		{"NE-match", NE("$.name", "alice"), []int{1, 3}},
		{"NE-prune", NE("$.age", int64(35)), []int{0, 1, 2}},
		{"GT-match", GT("$.age", int64(30)), []int{2, 3}},
		{"GT-prune", GT("$.age", int64(1000)), []int{}},
		{"GTE-match", GTE("$.age", int64(30)), []int{0, 2, 3}},
		{"GTE-prune", GTE("$.age", int64(1000)), []int{}},
		{"LT-match", LT("$.age", int64(30)), []int{1}},
		{"LT-prune", LT("$.age", int64(0)), []int{}},
		{"LTE-match", LTE("$.age", int64(30)), []int{0, 1}},
		{"LTE-prune", LTE("$.age", int64(-1)), []int{}},
		{"IN-match", IN("$.name", "alice", "bob"), []int{0, 1, 2}},
		{"IN-prune", IN("$.name", "xxx", "yyy"), []int{}},
		{"NIN-match", NIN("$.name", "nobody"), []int{0, 1, 2, 3}},
		{"NIN-prune", NIN("$.name", "alice", "bob", "carol"), []int{}},
		{"IsNull-match", IsNull("$.status"), []int{2}},
		{"IsNull-prune", IsNull("$.name"), []int{}},
		{"IsNotNull-match", IsNotNull("$.name"), []int{0, 1, 2, 3}},
		{"IsNotNull-prune", IsNotNull("$.status"), []int{0, 1, 2}},
		{"Contains-match", Contains("$.bio", "hello"), []int{0, 3}},
		{"Contains-prune", Contains("$.bio", "zzzzzz"), []int{}},
		{"Regex-match", Regex("$.bio", "^hello"), []int{0, 3}},
		{"Regex-prune", Regex("$.bio", "zzzzzz"), []int{}},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got := idx.Evaluate([]Predicate{c.pred}).ToSlice()
			if !intSliceEqual(got, c.wantRGs) {
				t.Errorf("op=%s pred=%+v got=%v want=%v", c.name, c.pred, got, c.wantRGs)
			}
		})
	}
}
