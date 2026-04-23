package gin

import (
	"bytes"
	"encoding/json"
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
