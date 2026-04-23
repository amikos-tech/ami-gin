package gin

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/pkg/errors"
)

type atomicityDoc struct {
	// docID DocID is intentionally non-contiguous in fixtures and properties.
	docID      DocID
	doc        []byte
	shouldFail bool
}

type atomicityCorpus struct {
	all []atomicityDoc
	// cleanOnly []atomicityDoc preserves successful documents with original IDs.
	cleanOnly    []atomicityDoc
	failingCount int
	numRGs       int
}

func strictEmailAtomicityConfig() (GINConfig, error) {
	return NewConfig(WithEmailDomainTransformer("$.email", "strict"))
}

func buildAtomicityIndex(config GINConfig, docs []atomicityDoc, numRGs int) ([]byte, error) {
	if numRGs < 1 {
		numRGs = 1
	}
	builder, err := NewBuilder(config, numRGs)
	if err != nil {
		return nil, errors.Wrap(err, "new atomicity builder")
	}
	for _, doc := range docs {
		err := builder.AddDocument(doc.docID, doc.doc)
		switch {
		case doc.shouldFail && err == nil:
			return nil, errors.Errorf("AddDocument(%d) succeeded for expected failure", doc.docID)
		case !doc.shouldFail && err != nil:
			return nil, errors.Wrapf(err, "AddDocument(%d) failed for clean document", doc.docID)
		}
	}
	return Encode(builder.Finalize())
}

func mustAtomicityJSON(value map[string]any) []byte {
	doc, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return doc
}

func cleanAtomicityCorpus() []atomicityDoc {
	return []atomicityDoc{
		{
			docID: 3,
			doc: mustAtomicityJSON(map[string]any{
				"email":   "alice@example.com",
				"score":   int64(10),
				"ratio":   1.25,
				"active":  true,
				"deleted": nil,
				"tags":    []any{"red", int64(2), false, nil},
				"nested": map[string]any{
					"label": "alpha",
					"count": int64(1),
				},
			}),
		},
		{
			docID: 10,
			doc: mustAtomicityJSON(map[string]any{
				"email":   "bob@example.com",
				"score":   int64(20),
				"ratio":   2.5,
				"active":  false,
				"deleted": nil,
				"tags":    []any{"blue", int64(4), true},
				"nested": map[string]any{
					"label": "beta",
					"count": int64(2),
				},
			}),
		},
		{
			docID: 21,
			doc: mustAtomicityJSON(map[string]any{
				"email":   "carol@example.com",
				"score":   int64(30),
				"ratio":   3.75,
				"active":  true,
				"deleted": nil,
				"tags":    []any{"green", int64(6), nil},
				"nested": map[string]any{
					"label": "gamma",
					"count": int64(3),
				},
			}),
		},
		{
			docID: 34,
			doc: mustAtomicityJSON(map[string]any{
				"email":   "dana@example.com",
				"score":   int64(40),
				"ratio":   5.0,
				"active":  false,
				"deleted": nil,
				"tags":    []any{"yellow", int64(8), false},
				"nested": map[string]any{
					"label": "delta",
					"count": int64(4),
				},
			}),
		},
	}
}

func TestAddDocumentAtomicityEncodeDeterminism(t *testing.T) {
	config, err := strictEmailAtomicityConfig()
	if err != nil {
		t.Fatalf("strictEmailAtomicityConfig: %v", err)
	}
	docs := cleanAtomicityCorpus()
	numRGs := 16

	first, err := buildAtomicityIndex(config, docs, numRGs)
	if err != nil {
		t.Fatalf("first build: %v", err)
	}
	second, err := buildAtomicityIndex(config, docs, numRGs)
	if err != nil {
		t.Fatalf("second build: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("encoded index differs for identical clean corpus")
	}

	builder, err := NewBuilder(config, numRGs)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	for _, doc := range docs {
		if err := builder.AddDocument(doc.docID, doc.doc); err != nil {
			t.Fatalf("AddDocument(%d): %v", doc.docID, err)
		}
	}
	idx := builder.Finalize()
	if len(idx.DocIDMapping) != len(docs) {
		t.Fatalf("DocIDMapping len = %d, want %d", len(idx.DocIDMapping), len(docs))
	}
	for i, doc := range docs {
		if idx.DocIDMapping[i] != doc.docID {
			t.Fatalf("non-contiguous doc ids preserve DocIDMapping: index %d = %d, want %d", i, idx.DocIDMapping[i], doc.docID)
		}
	}
}

type unsupportedTokenAtomicityParser struct{}

func (unsupportedTokenAtomicityParser) Name() string { return "unsupported-token" }

func (unsupportedTokenAtomicityParser) Parse(_ []byte, rgID int, sink parserSink) error {
	state := sink.BeginDocument(rgID)
	return sink.StageScalar(state, "$.bad", complex(1, 2))
}

type malformedNumericLiteralAtomicityParser struct{}

func (malformedNumericLiteralAtomicityParser) Name() string { return "malformed-numeric-literal" }

func (malformedNumericLiteralAtomicityParser) Parse(_ []byte, rgID int, sink parserSink) error {
	state := sink.BeginDocument(rgID)
	return sink.StageJSONNumber(state, "$.score", "not-a-number")
}

type nonFiniteNumericAtomicityParser struct{}

func (nonFiniteNumericAtomicityParser) Name() string { return "non-finite-numeric" }

func (nonFiniteNumericAtomicityParser) Parse(_ []byte, rgID int, sink parserSink) error {
	state := sink.BeginDocument(rgID)
	return sink.StageNativeNumeric(state, "$.score", math.Inf(1))
}

type uint64OverflowAtomicityParser struct{}

func (uint64OverflowAtomicityParser) Name() string { return "uint64-overflow" }

func (uint64OverflowAtomicityParser) Parse(_ []byte, rgID int, sink parserSink) error {
	state := sink.BeginDocument(rgID)
	return sink.StageMaterialized(state, "$.big", uint64(math.MaxUint64), true)
}

func requireAddDocumentNonTragicFailure(t *testing.T, builder *GINBuilder, err error, wantDocs uint64, wantNextPos int) {
	t.Helper()
	if err == nil {
		t.Fatal("AddDocument error = nil, want non-tragic failure")
	}
	if builder.tragicErr != nil {
		t.Fatalf("builder.tragicErr != nil after public failure: %v", builder.tragicErr)
	}
	if builder.numDocs != wantDocs {
		t.Fatalf("numDocs = %d, want %d", builder.numDocs, wantDocs)
	}
	if builder.nextPos != wantNextPos {
		t.Fatalf("nextPos = %d, want %d", builder.nextPos, wantNextPos)
	}
}

func requireSubsequentValidDocument(t *testing.T, builder *GINBuilder, docID DocID) {
	t.Helper()
	builder.parser = stdlibParser{}
	builder.parserName = stdlibParserName
	if err := builder.AddDocument(docID, []byte(`{"ok":"after-failure"}`)); err != nil {
		t.Fatalf("subsequent valid AddDocument failed: %v", err)
	}
}

func TestAddDocumentPublicFailuresDoNotSetTragicErr(t *testing.T) {
	t.Run("malformed-json", func(t *testing.T) {
		builder := mustNewBuilder(t, DefaultConfig(), 4)
		err := builder.AddDocument(0, []byte("garbage"))
		requireAddDocumentNonTragicFailure(t, builder, err, 0, 0)
		requireSubsequentValidDocument(t, builder, 1)
	})

	t.Run("trailing-json", func(t *testing.T) {
		builder := mustNewBuilder(t, DefaultConfig(), 4)
		err := builder.AddDocument(0, []byte(`{"a":1} []`))
		requireAddDocumentNonTragicFailure(t, builder, err, 0, 0)
		requireSubsequentValidDocument(t, builder, 1)
	})

	t.Run("non-utf8-truncated-json", func(t *testing.T) {
		builder := mustNewBuilder(t, DefaultConfig(), 4)
		err := builder.AddDocument(0, []byte{0xff, '{', '"', 'a', '"', ':'})
		requireAddDocumentNonTragicFailure(t, builder, err, 0, 0)
		requireSubsequentValidDocument(t, builder, 1)
	})

	t.Run("unsupported-token", func(t *testing.T) {
		builder, err := NewBuilder(DefaultConfig(), 4, WithParser(unsupportedTokenAtomicityParser{}))
		if err != nil {
			t.Fatalf("NewBuilder: %v", err)
		}
		err = builder.AddDocument(0, []byte(`{"bad":true}`))
		requireAddDocumentNonTragicFailure(t, builder, err, 0, 0)
		requireSubsequentValidDocument(t, builder, 1)
	})

	t.Run("strict-transformer", func(t *testing.T) {
		config, err := strictEmailAtomicityConfig()
		if err != nil {
			t.Fatalf("strictEmailAtomicityConfig: %v", err)
		}
		builder := mustNewBuilder(t, config, 4)
		err = builder.AddDocument(0, []byte(`{"email":42}`))
		requireAddDocumentNonTragicFailure(t, builder, err, 0, 0)
		if err := builder.AddDocument(1, []byte(`{"email":"valid@example.com"}`)); err != nil {
			t.Fatalf("valid email after transformer failure: %v", err)
		}
	})

	t.Run("malformed-numeric-literal", func(t *testing.T) {
		builder, err := NewBuilder(DefaultConfig(), 4, WithParser(malformedNumericLiteralAtomicityParser{}))
		if err != nil {
			t.Fatalf("NewBuilder: %v", err)
		}
		err = builder.AddDocument(0, []byte(`{"score":1}`))
		requireAddDocumentNonTragicFailure(t, builder, err, 0, 0)
		requireSubsequentValidDocument(t, builder, 1)
	})

	t.Run("non-finite-numeric", func(t *testing.T) {
		builder, err := NewBuilder(DefaultConfig(), 4, WithParser(nonFiniteNumericAtomicityParser{}))
		if err != nil {
			t.Fatalf("NewBuilder: %v", err)
		}
		err = builder.AddDocument(0, []byte(`{"score":1}`))
		requireAddDocumentNonTragicFailure(t, builder, err, 0, 0)
		requireSubsequentValidDocument(t, builder, 1)
	})

	t.Run("validator-rejected-numeric-promotion", func(t *testing.T) {
		builder := mustNewBuilder(t, DefaultConfig(), 4)
		if err := builder.AddDocument(0, []byte(`{"score":9007199254740993}`)); err != nil {
			t.Fatalf("seed AddDocument failed: %v", err)
		}
		err := builder.AddDocument(1, []byte(`{"score":1.5}`))
		requireAddDocumentNonTragicFailure(t, builder, err, 1, 1)
		if err := builder.AddDocument(2, []byte(`{"score":9007199254740992}`)); err != nil {
			t.Fatalf("valid int after numeric promotion rejection: %v", err)
		}
	})

	t.Run("position-exceeds-numrgs", func(t *testing.T) {
		builder := mustNewBuilder(t, DefaultConfig(), 1)
		if err := builder.AddDocument(0, []byte(`{"name":"stable"}`)); err != nil {
			t.Fatalf("seed AddDocument failed: %v", err)
		}
		err := builder.AddDocument(1, []byte(`{"name":"overflow"}`))
		requireAddDocumentNonTragicFailure(t, builder, err, 1, 1)
		if err := builder.AddDocument(0, []byte(`{"name":"same-slot"}`)); err != nil {
			t.Fatalf("existing docID after capacity failure: %v", err)
		}
	})

	t.Run("missing-begin-document", func(t *testing.T) {
		builder, err := NewBuilder(DefaultConfig(), 4, WithParser(skipBeginDocumentParser{}))
		if err != nil {
			t.Fatalf("NewBuilder: %v", err)
		}
		err = builder.AddDocument(0, []byte(`{"a":1}`))
		requireAddDocumentNonTragicFailure(t, builder, err, 0, 0)
		requireSubsequentValidDocument(t, builder, 1)
	})

	t.Run("double-begin-document", func(t *testing.T) {
		builder, err := NewBuilder(DefaultConfig(), 4, WithParser(doubleBeginDocumentParser{}))
		if err != nil {
			t.Fatalf("NewBuilder: %v", err)
		}
		err = builder.AddDocument(0, []byte(`{"a":1}`))
		requireAddDocumentNonTragicFailure(t, builder, err, 0, 0)
		requireSubsequentValidDocument(t, builder, 1)
	})

	t.Run("wrong-rgid", func(t *testing.T) {
		builder, err := NewBuilder(DefaultConfig(), 4, WithParser(wrongRGIDParser{}))
		if err != nil {
			t.Fatalf("NewBuilder: %v", err)
		}
		err = builder.AddDocument(0, []byte(`{"a":1}`))
		requireAddDocumentNonTragicFailure(t, builder, err, 0, 0)
		requireSubsequentValidDocument(t, builder, 1)
	})

	t.Run("uint64-overflow", func(t *testing.T) {
		builder, err := NewBuilder(DefaultConfig(), 4, WithParser(uint64OverflowAtomicityParser{}))
		if err != nil {
			t.Fatalf("NewBuilder: %v", err)
		}
		err = builder.AddDocument(0, []byte(`{"big":1}`))
		requireAddDocumentNonTragicFailure(t, builder, err, 0, 0)
		requireSubsequentValidDocument(t, builder, 1)
	})

	t.Run("unsupported-number-without-partial-mutation-regression", func(t *testing.T) {
		builder := mustNewBuilder(t, DefaultConfig(), 4)
		if err := builder.AddDocument(0, []byte(`{"name":"stable","score":10}`)); err != nil {
			t.Fatalf("seed AddDocument failed: %v", err)
		}

		err := builder.AddDocument(1, []byte(`{"name":"leak","nested":{"label":"should-not-stick"},"score":9223372036854775808}`))
		requireAddDocumentNonTragicFailure(t, builder, err, 1, 1)
		if !strings.Contains(err.Error(), "$.score") {
			t.Fatalf("error should contain path context, got %v", err)
		}
		if _, exists := builder.pathData["$.nested.label"]; exists {
			t.Fatal("rejected document leaked $.nested.label into builder state")
		}
		if _, exists := builder.docIDToPos[DocID(1)]; exists {
			t.Fatalf("docIDToPos contains rejected document: %+v", builder.docIDToPos)
		}
		if len(builder.posToDocID) != 1 {
			t.Fatalf("posToDocID len = %d, want 1", len(builder.posToDocID))
		}

		idx := builder.Finalize()
		if _, exists := idx.pathLookup["$.nested.label"]; exists {
			t.Fatal("rejected document path was added to finalized index")
		}
	})
}
