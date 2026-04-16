package gin

import (
	"encoding/json"
	stderrors "errors"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestWalkJSONPropagatesStagingErrors(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 1)

	err := builder.walkJSON("$.score", complex(1, 2), 0)
	if err == nil {
		t.Fatal("walkJSON() error = nil, want unsupported type error")
	}
	if !strings.Contains(err.Error(), "unsupported transformed value type") {
		t.Fatalf("walkJSON() error = %v, want unsupported transformed value type", err)
	}

	if got := builder.Finalize().PathDirectory; len(got) != 0 {
		t.Fatalf("walkJSON() merged rejected path state: PathDirectory len = %d, want 0", len(got))
	}
}

func TestSortedObjectKeys(t *testing.T) {
	got := sortedObjectKeys(map[string]any{
		"z": 1,
		"a": 2,
		"m": 3,
	})
	want := []string{"a", "m", "z"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sortedObjectKeys() = %v, want %v", got, want)
	}
}

func TestAddDocumentReportsLexicographicallyFirstObjectFieldError(t *testing.T) {
	cfg, err := NewConfig(
		WithCustomTransformer("$.a", "invalid", func(value any) (any, bool) { return complex(1, 0), true }),
		WithCustomTransformer("$.z", "invalid", func(value any) (any, bool) { return complex(1, 0), true }),
	)
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	builder := mustNewBuilder(t, cfg, 1)
	err = builder.AddDocument(0, []byte(`{"z":"bad","a":"bad"}`))
	if err == nil {
		t.Fatal("AddDocument() error = nil, want staged field error")
	}
	if !strings.Contains(err.Error(), "$.a") {
		t.Fatalf("AddDocument() error = %v, want $.a to fail first", err)
	}
}

func TestPrepareTransformerValueRecursesThroughNestedArrays(t *testing.T) {
	input := map[string]any{
		"items": []any{
			json.Number("1"),
			map[string]any{
				"values": []any{
					json.Number("2.5"),
					map[string]any{"deep": json.Number("3e2")},
				},
			},
		},
	}

	want := map[string]any{
		"items": []any{
			float64(1),
			map[string]any{
				"values": []any{
					float64(2.5),
					map[string]any{"deep": float64(300)},
				},
			},
		},
	}

	if got := prepareTransformerValue(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("prepareTransformerValue() = %#v, want %#v", got, want)
	}
}

func TestParseJSONNumberLiteralEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantInt   bool
		wantI64   int64
		wantF64   float64
		wantErrIs error
	}{
		{
			name:    "negative zero stays integral",
			raw:     "-0",
			wantInt: true,
			wantI64: 0,
		},
		{
			name:    "scientific notation stays float",
			raw:     "1e2",
			wantF64: 100,
		},
		{
			name:      "int overflow preserves parse error",
			raw:       "9223372036854775808",
			wantErrIs: strconv.ErrRange,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isInt, intVal, floatVal, err := parseJSONNumberLiteral(tt.raw)
			if tt.wantErrIs != nil {
				if err == nil {
					t.Fatalf("parseJSONNumberLiteral(%q) error = nil, want %v", tt.raw, tt.wantErrIs)
				}
				var numErr *strconv.NumError
				if !stderrors.As(err, &numErr) {
					t.Fatalf("parseJSONNumberLiteral(%q) error = %T, want *strconv.NumError", tt.raw, err)
				}
				if !stderrors.Is(err, tt.wantErrIs) {
					t.Fatalf("parseJSONNumberLiteral(%q) error = %v, want errors.Is(..., %v)", tt.raw, err, tt.wantErrIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseJSONNumberLiteral(%q) error = %v", tt.raw, err)
			}
			if isInt != tt.wantInt {
				t.Fatalf("parseJSONNumberLiteral(%q) isInt = %v, want %v", tt.raw, isInt, tt.wantInt)
			}
			if intVal != tt.wantI64 {
				t.Fatalf("parseJSONNumberLiteral(%q) intVal = %d, want %d", tt.raw, intVal, tt.wantI64)
			}
			if floatVal != tt.wantF64 {
				t.Fatalf("parseJSONNumberLiteral(%q) floatVal = %v, want %v", tt.raw, floatVal, tt.wantF64)
			}
		})
	}
}

func TestMixedNumericPathPromotesSafelyAcrossRowGroups(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 4)

	docs := []struct {
		docID DocID
		json  string
	}{
		{0, `{"score":10}`},
		{1, `{"score":20}`},
		{2, `{"score":30}`},
		{3, `{"score":15.5}`},
	}

	for _, doc := range docs {
		if err := builder.AddDocument(doc.docID, []byte(doc.json)); err != nil {
			t.Fatalf("AddDocument(%d) failed: %v", doc.docID, err)
		}
	}

	idx := builder.Finalize()
	scorePathID, ok := idx.pathLookup["$.score"]
	if !ok {
		t.Fatal("$.score missing from pathLookup")
	}
	ni, ok := idx.NumericIndexes[scorePathID]
	if !ok {
		t.Fatal("$.score missing numeric index")
	}
	if ni.ValueType != NumericValueTypeFloatMixed {
		t.Fatalf("ValueType = %v, want %v", ni.ValueType, NumericValueTypeFloatMixed)
	}

	wantBounds := []struct {
		min float64
		max float64
	}{
		{min: 10, max: 10},
		{min: 20, max: 20},
		{min: 30, max: 30},
		{min: 15.5, max: 15.5},
	}
	for rgID, want := range wantBounds {
		stat := ni.RGStats[rgID]
		if !stat.HasValue {
			t.Fatalf("RGStats[%d].HasValue = false, want true", rgID)
		}
		if stat.Min != want.min || stat.Max != want.max {
			t.Fatalf("RGStats[%d] bounds = [%v,%v], want [%v,%v]", rgID, stat.Min, stat.Max, want.min, want.max)
		}
	}

	if got := idx.Evaluate([]Predicate{EQ("$.score", int64(20))}).ToSlice(); len(got) != 1 || got[0] != 1 {
		t.Fatalf(`EQ("$.score", 20) = %v, want [1]`, got)
	}
}

func TestQueryNEOnIntOnlyNumericPath(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)

	docs := []struct {
		docID DocID
		json  string
	}{
		{0, `{"score":1}`},
		{1, `{"score":2}`},
		{2, `{"score":3}`},
	}

	for _, doc := range docs {
		if err := builder.AddDocument(doc.docID, []byte(doc.json)); err != nil {
			t.Fatalf("AddDocument(%d) failed: %v", doc.docID, err)
		}
	}

	idx := builder.Finalize()

	if got := idx.Evaluate([]Predicate{NE("$.score", int64(2))}).ToSlice(); len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Fatalf(`NE("$.score", 2) = %v, want [0 2]`, got)
	}
	if got := idx.Evaluate([]Predicate{NE("$.score", 2.5)}).ToSlice(); len(got) != 3 || got[0] != 0 || got[1] != 1 || got[2] != 2 {
		t.Fatalf(`NE("$.score", 2.5) = %v, want [0 1 2]`, got)
	}
	if got := idx.Evaluate([]Predicate{EQ("$.score", 2.0)}).ToSlice(); len(got) != 1 || got[0] != 1 {
		t.Fatalf(`EQ("$.score", 2.0) = %v, want [1]`, got)
	}
	if got := idx.Evaluate([]Predicate{EQ("$.score", 2.5)}).ToSlice(); len(got) != 0 {
		t.Fatalf(`EQ("$.score", 2.5) = %v, want []`, got)
	}
}

func TestAddDocumentRejectsTrailingJSONContent(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 1)

	err := builder.AddDocument(0, []byte(`{"score":1} {"extra":2}`))
	if err == nil {
		t.Fatal("AddDocument() error = nil, want trailing JSON error")
	}
	if !strings.Contains(err.Error(), "unexpected trailing JSON content") {
		t.Fatalf("AddDocument() error = %v, want trailing JSON error", err)
	}

	if builder.numDocs != 0 {
		t.Fatalf("numDocs = %d, want 0", builder.numDocs)
	}
	if got := builder.Finalize().PathDirectory; len(got) != 0 {
		t.Fatalf("AddDocument() merged rejected trailing JSON: PathDirectory len = %d, want 0", len(got))
	}
}

func TestToRoundedInt64EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		value any
		round func(float64) float64
		want  int64
		ok    bool
	}{
		{
			name:  "nan rejected",
			value: math.NaN(),
			round: math.Floor,
			ok:    false,
		},
		{
			name:  "positive infinity rejected",
			value: math.Inf(1),
			round: math.Floor,
			ok:    false,
		},
		{
			name:  "negative fraction floors away from zero",
			value: -1.2,
			round: math.Floor,
			want:  -2,
			ok:    true,
		},
		{
			name:  "negative fraction ceils toward zero",
			value: -1.2,
			round: math.Ceil,
			want:  -1,
			ok:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toRoundedInt64(tt.value, tt.round)
			if ok != tt.ok {
				t.Fatalf("toRoundedInt64(%v) ok = %v, want %v", tt.value, ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("toRoundedInt64(%v) = %d, want %d", tt.value, got, tt.want)
			}
		})
	}
}

func TestMustNewTrigramIndexPanicsOnInvalidOption(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("MustNewTrigramIndex() did not panic on invalid option")
		}
	}()

	_ = MustNewTrigramIndex(1, WithN(1))
}
