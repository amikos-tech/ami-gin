package gin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// aggregatedDoc is one document placed into a row group that may hold others.
type aggregatedDoc struct {
	rg   int
	data map[string]any
}

func buildAggregated(t *testing.T, numRGs int, docs []aggregatedDoc) *GINIndex {
	t.Helper()
	builder := mustNewBuilder(t, DefaultConfig(), numRGs)
	for _, doc := range docs {
		raw, err := json.Marshal(doc.data)
		if err != nil {
			t.Fatalf("marshal %v: %v", doc.data, err)
		}
		if err := builder.AddDocument(DocID(doc.rg), raw); err != nil {
			t.Fatalf("AddDocument(%d, %s): %v", doc.rg, raw, err)
		}
	}
	idx := builder.Finalize()
	if idx == nil {
		t.Fatal("Finalize returned nil")
	}
	return idx
}

// docMatches is the document-level oracle for the row-group semantics ami-gin
// implements: a path counts as present when the key exists, null included.
// An absent key satisfies IsNull only when the row group aggregates several
// documents (aggregated); a lone document keeps the historical reading.
func docMatches(p Predicate, data map[string]any, aggregated bool) bool {
	field := strings.TrimPrefix(p.Path, "$.")
	value, present := data[field]
	switch p.Operator {
	case OpEQ:
		return present && scalarEquals(value, p.Value)
	case OpNE:
		return present && !scalarEquals(value, p.Value)
	case OpIN:
		return present && scalarIn(value, p.Value)
	case OpNIN:
		return present && !scalarIn(value, p.Value)
	case OpIsNull:
		return (present && value == nil) || (!present && aggregated)
	case OpIsNotNull:
		return present
	case OpGT, OpGTE, OpLT, OpLTE:
		have, ok := value.(float64)
		want := toFloat64(p.Value)
		if !ok || want == nil {
			return false
		}
		switch p.Operator {
		case OpGT:
			return have > *want
		case OpGTE:
			return have >= *want
		case OpLT:
			return have < *want
		default:
			return have <= *want
		}
	case OpContains:
		s, ok := value.(string)
		return ok && strings.Contains(s, p.Value.(string))
	case OpRegex:
		s, ok := value.(string)
		return ok && regexp.MustCompile(p.Value.(string)).MatchString(s)
	}
	return false
}

func scalarEquals(have, want any) bool {
	switch w := want.(type) {
	case string:
		s, ok := have.(string)
		return ok && s == w
	case bool:
		b, ok := have.(bool)
		return ok && b == w
	default:
		f, ok := have.(float64)
		wf := toFloat64(want)
		return ok && wf != nil && f == *wf
	}
}

func scalarIn(have, want any) bool {
	values, ok := want.([]any)
	if !ok {
		return false
	}
	for _, v := range values {
		if scalarEquals(have, v) {
			return true
		}
	}
	return false
}

// docIDSet names the matching row groups by DocID. RGSet positions follow
// first-seen DocID order, so tests compare DocIDs, never raw positions.
type docIDSet map[DocID]struct{}

func (s docIDSet) sorted() []int {
	out := make([]int, 0, len(s))
	for id := range s {
		out = append(out, int(id))
	}
	sort.Ints(out)
	return out
}

func (s docIDSet) superset(of docIDSet) bool {
	for id := range of {
		if _, ok := s[id]; !ok {
			return false
		}
	}
	return true
}

func (s docIDSet) equals(other docIDSet) bool {
	return len(s) == len(other) && s.superset(other)
}

func (s docIDSet) intersect(other docIDSet) docIDSet {
	out := docIDSet{}
	for id := range s {
		if _, ok := other[id]; ok {
			out[id] = struct{}{}
		}
	}
	return out
}

func indexDocIDs(idx *GINIndex, p Predicate) docIDSet {
	out := docIDSet{}
	for _, id := range idx.MatchingDocIDs(idx.Evaluate([]Predicate{p})) {
		out[id] = struct{}{}
	}
	return out
}

func oracleDocIDs(docs []aggregatedDoc, p Predicate) docIDSet {
	perRG := map[int]int{}
	for _, doc := range docs {
		perRG[doc.rg]++
	}
	out := docIDSet{}
	for _, doc := range docs {
		if docMatches(p, doc.data, perRG[doc.rg] >= 2) {
			out[DocID(doc.rg)] = struct{}{}
		}
	}
	return out
}

// TestNegationUnderAggregationIssue60 pins the three reproducer cases from
// issue #60: every one used to return the empty set.
func TestNegationUnderAggregationIssue60(t *testing.T) {
	idx := buildAggregated(t, 1, []aggregatedDoc{
		{0, map[string]any{"env": "prod"}},
		{0, map[string]any{"env": "staging"}},
	})
	for _, p := range []Predicate{NE("$.env", "prod"), NIN("$.env", "prod")} {
		if got := idx.Evaluate([]Predicate{p}).ToSlice(); !reflect.DeepEqual(got, []int{0}) {
			t.Errorf("%s = %v, want [0]", p, got)
		}
	}

	idx = buildAggregated(t, 1, []aggregatedDoc{
		{0, map[string]any{"env": "prod"}},
		{0, map[string]any{"other": "x"}},
	})
	if got := idx.Evaluate([]Predicate{IsNull("$.env")}).ToSlice(); !reflect.DeepEqual(got, []int{0}) {
		t.Errorf("IsNull($.env) = %v, want [0]", got)
	}
}

func aggregatedCorpus() (int, []aggregatedDoc) {
	return 4, []aggregatedDoc{
		// rg 0: two values for env, one app, n differs
		{0, map[string]any{"env": "prod", "app": "payments", "n": 1.0}},
		{0, map[string]any{"env": "canary", "app": "payments", "n": 2.0}},
		// rg 1: single env value repeated, one doc lacks app and n
		{1, map[string]any{"env": "prod", "app": "payments", "n": 5.0}},
		{1, map[string]any{"env": "prod"}},
		// rg 2: env only null and staging, canary present once
		{2, map[string]any{"env": nil, "app": "ledger", "n": 5.0, "canary": true}},
		{2, map[string]any{"env": "staging", "app": "ledger", "n": 5.0}},
		// rg 3: three docs, every value repeated, no absences
		{3, map[string]any{"env": "prod", "app": "ledger", "n": 7.0, "canary": false}},
		{3, map[string]any{"env": "prod", "app": "ledger", "n": 7.0, "canary": false}},
		{3, map[string]any{"env": "prod", "app": "ledger", "n": 7.0, "canary": false}},
	}
}

func aggregatedPredicates() []Predicate {
	return []Predicate{
		EQ("$.env", "prod"), EQ("$.env", "canary"), EQ("$.n", 5.0), EQ("$.canary", false),
		NE("$.env", "prod"), NE("$.env", "staging"), NE("$.env", "nobody"), NE("$.n", 5.0), NE("$.n", 7.0), NE("$.canary", false),
		IN("$.env", "prod", "canary"), IN("$.app", "ledger"),
		NIN("$.env", "prod"), NIN("$.env", "prod", "canary"), NIN("$.app", "payments"), NIN("$.app", "payments", "ledger"), NIN("$.n", 5.0, 7.0),
		IsNull("$.env"), IsNull("$.app"), IsNull("$.canary"), IsNull("$.n"),
		IsNotNull("$.env"), IsNotNull("$.canary"),
		GT("$.n", 4.0), GTE("$.n", 5.0), LT("$.n", 2.0), LTE("$.n", 1.0),
		Contains("$.app", "pay"), Regex("$.app", "^led"),
	}
}

// TestNegationUnderAggregationOracle checks every operator against a
// brute-force oracle over a corpus with several documents per DocID. Every
// operator must over-select or match; NE and IsNull must match exactly.
func TestNegationUnderAggregationOracle(t *testing.T) {
	numRGs, docs := aggregatedCorpus()
	idx := buildAggregated(t, numRGs, docs)

	for _, p := range aggregatedPredicates() {
		got := indexDocIDs(idx, p)
		want := oracleDocIDs(docs, p)
		if !got.superset(want) {
			t.Errorf("%s under-selects: index %v, oracle %v", p, got.sorted(), want.sorted())
			continue
		}
		switch p.Operator {
		case OpNE, OpIsNull:
			if !got.equals(want) {
				t.Errorf("%s = %v, want exactly %v", p, got.sorted(), want.sorted())
			}
		case OpNIN:
			// env, app hold only strings/null, so the distinct count is
			// known and NIN is exact. n spans a range in rg 0 (unknown).
			if p.Path != "$.n" && !got.equals(want) {
				t.Errorf("%s = %v, want exactly %v", p, got.sorted(), want.sorted())
			}
		}
	}
}

// TestPreciseNINIssue63 pins the expectations E1-E7 from the issue 63
// context: a multi-value row group is dropped when every distinct value it
// holds is in the NIN list, and kept otherwise.
func TestPreciseNINIssue63(t *testing.T) {
	t.Run("E1 E2 issue example", func(t *testing.T) {
		idx := buildAggregated(t, 3, []aggregatedDoc{
			{0, map[string]any{"color": "red"}},
			{0, map[string]any{"color": "red"}},
			{1, map[string]any{"color": "red"}},
			{1, map[string]any{"color": "blue"}},
			{2, map[string]any{"color": "red"}},
			{2, map[string]any{"color": "blue"}},
			{2, map[string]any{"color": "green"}},
		})
		if got := indexDocIDs(idx, NIN("$.color", "red", "blue")).sorted(); !reflect.DeepEqual(got, []int{2}) {
			t.Errorf("NIN(red, blue) = %v, want [2]", got)
		}
		if got := indexDocIDs(idx, NIN("$.color", "red")).sorted(); !reflect.DeepEqual(got, []int{1, 2}) {
			t.Errorf("NIN(red) = %v, want [1 2]", got)
		}
		if got := indexDocIDs(idx, NIN("$.color", "red", "blue", "green")).sorted(); len(got) != 0 {
			t.Errorf("NIN(red, blue, green) = %v, want []", got)
		}
	})

	t.Run("E3 duplicate query values count once", func(t *testing.T) {
		idx := buildAggregated(t, 1, []aggregatedDoc{
			{0, map[string]any{"p": "a"}},
			{0, map[string]any{"p": "b"}},
		})
		if got := indexDocIDs(idx, NIN("$.p", "a", "a")).sorted(); !reflect.DeepEqual(got, []int{0}) {
			t.Errorf("NIN(a, a) = %v, want [0]", got)
		}
	})

	t.Run("E4 string true and bool true are one term", func(t *testing.T) {
		idx := buildAggregated(t, 1, []aggregatedDoc{
			{0, map[string]any{"p": "true"}},
			{0, map[string]any{"p": true}},
		})
		if len(idx.AggregateIndexes) != 0 {
			t.Fatalf("AggregateIndexes = %d entries, want none: both documents index the same term", len(idx.AggregateIndexes))
		}
		for _, p := range []Predicate{NIN("$.p", "true"), NIN("$.p", true), NIN("$.p", "true", true)} {
			if got := indexDocIDs(idx, p).sorted(); len(got) != 0 {
				t.Errorf("%s = %v, want []", p, got)
			}
		}
	})

	t.Run("E5 single numeric value counts as one", func(t *testing.T) {
		idx := buildAggregated(t, 1, []aggregatedDoc{
			{0, map[string]any{"p": "a"}},
			{0, map[string]any{"p": 5.0}},
		})
		if got := indexDocIDs(idx, NIN("$.p", "a", 5.0)).sorted(); len(got) != 0 {
			t.Errorf("NIN(a, 5) = %v, want []", got)
		}
		if got := indexDocIDs(idx, NIN("$.p", "a", 5)).sorted(); len(got) != 0 {
			t.Errorf("NIN(a, int 5) = %v, want []", got)
		}
		if got := indexDocIDs(idx, NIN("$.p", "a")).sorted(); !reflect.DeepEqual(got, []int{0}) {
			t.Errorf("NIN(a) = %v, want [0]", got)
		}
		if got := indexDocIDs(idx, NIN("$.p", "a", "5")).sorted(); !reflect.DeepEqual(got, []int{0}) {
			t.Errorf("NIN(a, \"5\") = %v, want [0]: the string 5 is not the number 5", got)
		}
	})

	t.Run("E6 numeric range keeps the row group", func(t *testing.T) {
		idx := buildAggregated(t, 1, []aggregatedDoc{
			{0, map[string]any{"p": "a"}},
			{0, map[string]any{"p": 5.0}},
			{0, map[string]any{"p": 7.0}},
		})
		ai := idx.AggregateIndexes[idx.pathLookup["$.p"]]
		if ai == nil || !reflect.DeepEqual(ai.DistinctCounts, []uint32{0}) {
			t.Fatalf("DistinctCounts = %+v, want [0] (unknown)", ai)
		}
		if got := indexDocIDs(idx, NIN("$.p", "a", 5.0, 7.0)).sorted(); !reflect.DeepEqual(got, []int{0}) {
			t.Errorf("NIN(a, 5, 7) = %v, want [0]", got)
		}
	})

	t.Run("E7 explicit null is a distinct value", func(t *testing.T) {
		idx := buildAggregated(t, 1, []aggregatedDoc{
			{0, map[string]any{"p": "a"}},
			{0, map[string]any{"p": nil}},
		})
		if got := indexDocIDs(idx, NIN("$.p", "a")).sorted(); !reflect.DeepEqual(got, []int{0}) {
			t.Errorf("NIN(a) = %v, want [0]", got)
		}
	})
}

// TestDistinctCountsAlignToMultiValueRGs pins the sparse layout (E12): one
// count per set bit of MultiValueRGs, in bit order, and no entry at all for
// a path whose multi-document row groups each hold one value.
func TestDistinctCountsAlignToMultiValueRGs(t *testing.T) {
	numRGs, docs := aggregatedCorpus()
	idx := buildAggregated(t, numRGs, docs)
	env := idx.AggregateIndexes[idx.pathLookup["$.env"]]
	if env == nil {
		t.Fatal("no aggregate index for $.env")
	}
	// rg 0 {prod, canary}, rg 2 {null, staging}; rg 1 and rg 3 hold one value.
	if got := env.MultiValueRGs.ToSlice(); !reflect.DeepEqual(got, []int{0, 2}) {
		t.Fatalf("MultiValueRGs = %v, want [0 2]", got)
	}
	if !reflect.DeepEqual(env.DistinctCounts, []uint32{2, 2}) {
		t.Errorf("DistinctCounts = %v, want [2 2]", env.DistinctCounts)
	}
	n := idx.AggregateIndexes[idx.pathLookup["$.n"]]
	// rg 0 spans 1..2 (unknown); rg 1 lacks n in one doc but holds one value.
	if got := n.MultiValueRGs.ToSlice(); !reflect.DeepEqual(got, []int{0}) {
		t.Fatalf("n MultiValueRGs = %v, want [0]", got)
	}
	if !reflect.DeepEqual(n.DistinctCounts, []uint32{0}) {
		t.Errorf("n DistinctCounts = %v, want [0]", n.DistinctCounts)
	}
	if _, ok := idx.AggregateIndexes[idx.pathLookup["$.id"]]; ok {
		t.Error("aggregate index emitted for a path with no multi-value or absent row group")
	}
}

// TestNegationSingleDocumentSemanticsUnchanged pins the one-document-per-DocID
// behaviour: an array-valued document still counts as "value appears", and a
// document without the key is not null. No AggregateIndex is built.
func TestNegationSingleDocumentSemanticsUnchanged(t *testing.T) {
	idx := buildAggregated(t, 3, []aggregatedDoc{
		{0, map[string]any{"tags": []any{"a", "b"}}},
		{1, map[string]any{"tags": []any{"b"}}},
		{2, map[string]any{"other": "x"}},
	})
	if len(idx.AggregateIndexes) != 0 {
		t.Fatalf("AggregateIndexes = %d entries, want none for a 1:1 index", len(idx.AggregateIndexes))
	}
	if got := idx.Evaluate([]Predicate{NE("$.tags[*]", "a")}).ToSlice(); !reflect.DeepEqual(got, []int{1}) {
		t.Errorf("NE($.tags[*], a) = %v, want [1]", got)
	}
	if got := idx.Evaluate([]Predicate{IsNull("$.tags[*]")}).ToSlice(); len(got) != 0 {
		t.Errorf("IsNull($.tags[*]) = %v, want [] for a 1:1 index", got)
	}
}

// TestNegationArrayValuesUnderAggregation covers array elements, which are
// staged only under their wildcard path (#64). A row group holding several
// documents keeps NE/NIN when any document holds another element value and
// keeps IsNull when any document lacks the array.
func TestNegationArrayValuesUnderAggregation(t *testing.T) {
	idx := buildAggregated(t, 3, []aggregatedDoc{
		// rg 0: two docs, second holds b only -> NE(a) and NIN(a) must keep
		{0, map[string]any{"tags": []any{"a", "b"}}},
		{0, map[string]any{"tags": []any{"b"}}},
		// rg 1: two docs, both hold only a -> NE(a) must drop
		{1, map[string]any{"tags": []any{"a", "a"}}},
		{1, map[string]any{"tags": []any{"a"}}},
		// rg 2: two docs, one without tags -> IsNull must keep
		{2, map[string]any{"tags": []any{"a"}}},
		{2, map[string]any{"other": "x"}},
	})
	if got := indexDocIDs(idx, NE("$.tags[*]", "a")).sorted(); !reflect.DeepEqual(got, []int{0}) {
		t.Errorf("NE($.tags[*], a) = %v, want [0]", got)
	}
	if got := indexDocIDs(idx, NIN("$.tags[*]", "a")).sorted(); !reflect.DeepEqual(got, []int{0}) {
		t.Errorf("NIN($.tags[*], a) = %v, want [0]", got)
	}
	if got := indexDocIDs(idx, IsNull("$.tags[*]")).sorted(); !reflect.DeepEqual(got, []int{2}) {
		t.Errorf("IsNull($.tags[*]) = %v, want [2]", got)
	}
	for _, path := range idx.PathDirectory {
		if strings.Contains(path.PathName, "[0]") || strings.Contains(path.PathName, "[1]") {
			t.Errorf("private numeric array path %q staged; expected wildcard only", path.PathName)
		}
	}
}

func TestAggregateIndexSerializationRoundTrip(t *testing.T) {
	numRGs, docs := aggregatedCorpus()
	idx := buildAggregated(t, numRGs, docs)
	if len(idx.AggregateIndexes) == 0 {
		t.Fatal("expected aggregate indexes for an aggregated corpus")
	}

	for _, level := range []CompressionLevel{CompressionNone, CompressionBalanced} {
		data, err := EncodeWithLevel(idx, level)
		if err != nil {
			t.Fatalf("EncodeWithLevel(%d): %v", level, err)
		}
		decoded, err := Decode(data)
		if err != nil {
			t.Fatalf("Decode(level %d): %v", level, err)
		}
		if len(decoded.AggregateIndexes) != len(idx.AggregateIndexes) {
			t.Fatalf("decoded %d aggregate indexes, want %d", len(decoded.AggregateIndexes), len(idx.AggregateIndexes))
		}
		for pathID, want := range idx.AggregateIndexes {
			got := decoded.AggregateIndexes[pathID]
			if got == nil {
				t.Fatalf("path %d missing after decode", pathID)
			}
			if !reflect.DeepEqual(got.MultiValueRGs.ToSlice(), want.MultiValueRGs.ToSlice()) ||
				!reflect.DeepEqual(got.AbsentRGs.ToSlice(), want.AbsentRGs.ToSlice()) ||
				!reflect.DeepEqual(got.DistinctCounts, want.DistinctCounts) {
				t.Fatalf("path %d round trip mismatch: got %+v want %+v", pathID, got, want)
			}
		}
		for _, p := range aggregatedPredicates() {
			if a, b := indexDocIDs(idx, p), indexDocIDs(decoded, p); !a.equals(b) {
				t.Fatalf("%s differs after decode: %v vs %v", p, a.sorted(), b.sorted())
			}
		}
	}
}

func TestDecodeRejectsAggregateIndexForUnknownPath(t *testing.T) {
	numRGs, docs := aggregatedCorpus()
	idx := buildAggregated(t, numRGs, docs)
	idx.AggregateIndexes[uint16(len(idx.PathDirectory))] = &AggregateIndex{
		MultiValueRGs: MustNewRGSet(numRGs),
		AbsentRGs:     MustNewRGSet(numRGs),
	}
	if _, err := Encode(idx); err == nil {
		t.Fatal("Encode accepted an aggregate index for a path outside the directory")
	}
}

func TestEncodeRejectsMisalignedDistinctCounts(t *testing.T) {
	numRGs, docs := aggregatedCorpus()
	idx := buildAggregated(t, numRGs, docs)
	env := idx.AggregateIndexes[idx.pathLookup["$.env"]]
	env.DistinctCounts = append(env.DistinctCounts, 3)
	if _, err := Encode(idx); err == nil || !strings.Contains(err.Error(), "distinct counts") {
		t.Fatalf("Encode error = %v, want distinct count mismatch", err)
	}
}

func TestDecodeRejectsMisalignedDistinctCounts(t *testing.T) {
	numRGs, docs := aggregatedCorpus()
	idx := buildAggregated(t, numRGs, docs)
	data, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatal(err)
	}
	env := idx.AggregateIndexes[idx.pathLookup["$.env"]]
	// The count length field precedes the counts; locate it by its payload.
	needle := make([]byte, 0, 4+4*len(env.DistinctCounts))
	needle = append(needle, byte(len(env.DistinctCounts)), 0, 0, 0)
	for _, c := range env.DistinctCounts {
		needle = append(needle, byte(c), 0, 0, 0)
	}
	at := bytes.Index(data, needle)
	if at < 0 {
		t.Fatal("distinct count payload not found in uncompressed encoding")
	}
	data[at]++
	_, err = Decode(data)
	if err == nil || !strings.Contains(err.Error(), "aggregate index") {
		t.Fatalf("Decode error = %v, want aggregate index rejection", err)
	}
}

// genAggregatedDocs yields flat documents spread over numRGs row groups, so
// most row groups hold several documents and a few hold one or none. env is a
// low-cardinality string, n a small integer; either may be absent or null.
func genAggregatedDocs(numRGs int) gopter.Gen {
	envs := []any{"prod", "staging", "canary", nil, absentMarker{}}
	nums := []any{1.0, 2.0, 3.0, nil, absentMarker{}}
	genDoc := gopter.CombineGens(
		gen.IntRange(0, numRGs-1),
		gen.IntRange(0, len(envs)-1),
		gen.IntRange(0, len(nums)-1),
	).Map(func(vals []any) aggregatedDoc {
		data := map[string]any{"id": "x"}
		if v := envs[vals[1].(int)]; !isAbsent(v) {
			data["env"] = v
		}
		if v := nums[vals[2].(int)]; !isAbsent(v) {
			data["n"] = v
		}
		return aggregatedDoc{rg: vals[0].(int), data: data}
	})
	return gen.SliceOfN(numRGs*3, genDoc)
}

type absentMarker struct{}

func isAbsent(v any) bool {
	_, ok := v.(absentMarker)
	return ok
}

// TestPropertyNegationUnderAggregation: for random aggregated corpora the
// index never under-selects on any operator, and NE, NIN with one value and
// IsNull are exact on row groups holding at least two documents.
func TestPropertyNegationUnderAggregation(t *testing.T) {
	const numRGs = 6
	properties := gopter.NewProperties(propertyTestParametersWithBudgets(300, 60))

	predicates := []Predicate{
		EQ("$.env", "prod"), NE("$.env", "prod"), NE("$.env", "canary"), NE("$.n", 2.0),
		IN("$.env", "prod", "staging"), NIN("$.env", "prod"), NIN("$.env", "prod", "staging"),
		NIN("$.env", "prod", "staging", "canary"), NIN("$.env", "prod", "prod"), NIN("$.n", 1.0), NIN("$.n", 1.0, 2.0),
		IsNull("$.env"), IsNull("$.n"), IsNotNull("$.env"),
		GT("$.n", 1.0), LTE("$.n", 2.0), Contains("$.env", "sta"), Regex("$.env", "^ca"),
	}

	properties.Property("negation is sound and exact under aggregation", prop.ForAll(
		func(docs []aggregatedDoc) (bool, error) {
			perRG := make([]int, numRGs)
			multiDoc := docIDSet{}
			for _, doc := range docs {
				perRG[doc.rg]++
				if perRG[doc.rg] >= 2 {
					multiDoc[DocID(doc.rg)] = struct{}{}
				}
			}
			builder, err := NewBuilder(DefaultConfig(), numRGs)
			if err != nil {
				return false, err
			}
			for _, doc := range docs {
				raw, _ := json.Marshal(doc.data)
				if err := builder.AddDocument(DocID(doc.rg), raw); err != nil {
					return false, err
				}
			}
			idx := builder.Finalize()
			for _, p := range predicates {
				got := indexDocIDs(idx, p)
				want := oracleDocIDs(docs, p)
				if !got.superset(want) {
					return false, fmt.Errorf("%s under-selects: index %v oracle %v docs %v", p, got.sorted(), want.sorted(), docs)
				}
				exact := p.Operator == OpNE || p.Operator == OpIsNull ||
					(p.Operator == OpNIN && (p.Path == "$.env" || len(p.Value.([]any)) == 1))
				if exact && !got.intersect(multiDoc).equals(want.intersect(multiDoc)) {
					return false, fmt.Errorf("%s not exact on aggregated row groups: index %v oracle %v docs %v", p, got.sorted(), want.sorted(), docs)
				}
			}
			return true, nil
		},
		genAggregatedDocs(numRGs),
	))

	properties.TestingRun(t)
}
