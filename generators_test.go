package gin

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
)

// TestDoc carries both raw JSON and parsed data for verification
type TestDoc struct {
	JSON []byte
	Data map[string]any
}

func (td TestDoc) HasFieldValue(field string, value any) bool {
	v, ok := td.Data[field]
	if !ok {
		return false
	}
	switch expected := value.(type) {
	case string:
		actual, ok := v.(string)
		return ok && actual == expected
	case float64:
		actual, ok := v.(float64)
		return ok && actual == expected
	case bool:
		actual, ok := v.(bool)
		return ok && actual == expected
	default:
		return false
	}
}

func (td TestDoc) HasFieldNull(field string) bool {
	v, ok := td.Data[field]
	return ok && v == nil
}

func (td TestDoc) FieldAbsent(field string) bool {
	_, ok := td.Data[field]
	return !ok
}

func isSubset(a, b *RGSet) bool {
	aSlice := a.ToSlice()
	for _, v := range aSlice {
		if !b.IsSet(v) {
			return false
		}
	}
	return true
}

func unionAll(bitmaps []*RGSet) *RGSet {
	if len(bitmaps) == 0 {
		return MustNewRGSet(1) // Use 1 instead of 0 since 0 is invalid
	}
	result := bitmaps[0].Clone()
	for i := 1; i < len(bitmaps); i++ {
		result = result.Union(bitmaps[i])
	}
	return result
}

func findPathEntry(idx *GINIndex, pathName string) *PathEntry {
	for i := range idx.PathDirectory {
		if idx.PathDirectory[i].PathName == pathName {
			return &idx.PathDirectory[i]
		}
	}
	return nil
}

func GenSimpleJSONValue() gopter.Gen {
	return gen.OneGenOf(
		gen.AlphaString().Map(func(s string) any { return s }),
		gen.Float64Range(-1e6, 1e6).Map(func(f float64) any { return f }),
		gen.Bool().Map(func(b bool) any { return b }),
	)
}

func GenJSONDocument(maxDepth int) gopter.Gen {
	return genFlatJSONObject()
}

func genFlatJSONObject() gopter.Gen {
	return gopter.CombineGens(
		gen.AlphaString(),
		gen.AlphaString(),
		gen.Float64Range(-1e6, 1e6),
		gen.Bool(),
	).Map(func(vals []interface{}) []byte {
		m := map[string]any{
			"str1": vals[0].(string),
			"str2": vals[1].(string),
			"num":  vals[2].(float64),
			"flag": vals[3].(bool),
		}
		if m["str1"] == "" {
			m["str1"] = "default"
		}
		data, _ := json.Marshal(m)
		return data
	})
}

func GenValidJSONPath() gopter.Gen {
	return gen.AlphaString().Map(func(s string) string {
		if s == "" {
			s = "field"
		}
		return "$." + s
	})
}

func GenSortedStrings(minLen, maxLen int) gopter.Gen {
	return gen.SliceOfN(maxLen, gen.Identifier()).Map(func(strs []string) []string {
		result := make([]string, 0, len(strs))
		for _, s := range strs {
			if len(s) > 0 {
				result = append(result, s)
			}
		}
		if len(result) < minLen {
			for i := len(result); i < minLen; i++ {
				result = append(result, "term"+string(rune('a'+i%26)))
			}
		}
		sort.Strings(result)
		return result
	})
}

type RGSetPair struct {
	A      *RGSet
	B      *RGSet
	NumRGs int
}

type RGSetTriple struct {
	A      *RGSet
	B      *RGSet
	C      *RGSet
	NumRGs int
}

type NumericRangePair struct {
	Min float64
	Max float64
}

type HLLItemPair struct {
	Items1 []string
	Items2 []string
}

func GenRGSet(maxRGs int) gopter.Gen {
	return gen.IntRange(1, maxRGs).FlatMap(func(numRGs interface{}) gopter.Gen {
		n := numRGs.(int)
		return gen.SliceOf(gen.IntRange(0, n-1)).Map(func(bits []int) *RGSet {
			rs := MustNewRGSet(n)
			for _, bit := range bits {
				rs.Set(bit)
			}
			return rs
		})
	}, reflect.TypeOf((*RGSet)(nil)))
}

func GenRGSetPair(maxRGs int) gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(1, maxRGs),
		gen.SliceOf(gen.IntRange(0, maxRGs-1)),
		gen.SliceOf(gen.IntRange(0, maxRGs-1)),
	).Map(func(vals []interface{}) RGSetPair {
		numRGs := vals[0].(int)
		bits1 := vals[1].([]int)
		bits2 := vals[2].([]int)

		a := MustNewRGSet(numRGs)
		for _, bit := range bits1 {
			if bit < numRGs {
				a.Set(bit)
			}
		}
		b := MustNewRGSet(numRGs)
		for _, bit := range bits2 {
			if bit < numRGs {
				b.Set(bit)
			}
		}
		return RGSetPair{A: a, B: b, NumRGs: numRGs}
	})
}

func GenRGSetTriple(maxRGs int) gopter.Gen {
	return gopter.CombineGens(
		gen.IntRange(1, maxRGs),
		gen.SliceOf(gen.IntRange(0, maxRGs-1)),
		gen.SliceOf(gen.IntRange(0, maxRGs-1)),
		gen.SliceOf(gen.IntRange(0, maxRGs-1)),
	).Map(func(vals []interface{}) RGSetTriple {
		numRGs := vals[0].(int)
		bits1 := vals[1].([]int)
		bits2 := vals[2].([]int)
		bits3 := vals[3].([]int)

		a := MustNewRGSet(numRGs)
		for _, bit := range bits1 {
			if bit < numRGs {
				a.Set(bit)
			}
		}
		b := MustNewRGSet(numRGs)
		for _, bit := range bits2 {
			if bit < numRGs {
				b.Set(bit)
			}
		}
		c := MustNewRGSet(numRGs)
		for _, bit := range bits3 {
			if bit < numRGs {
				c.Set(bit)
			}
		}
		return RGSetTriple{A: a, B: b, C: c, NumRGs: numRGs}
	})
}

func GenNumericRange() gopter.Gen {
	return gopter.CombineGens(
		gen.Float64Range(-1e10, 1e10),
		gen.Float64Range(0, 1e10),
	).Map(func(vals []interface{}) NumericRangePair {
		base := vals[0].(float64)
		delta := vals[1].(float64)
		return NumericRangePair{Min: base, Max: base + delta}
	})
}

func GenHLLPair() gopter.Gen {
	return gopter.CombineGens(
		gen.SliceOfN(100, gen.AlphaString()),
		gen.SliceOfN(100, gen.AlphaString()),
	).Map(func(vals []interface{}) HLLItemPair {
		return HLLItemPair{
			Items1: vals[0].([]string),
			Items2: vals[1].([]string),
		}
	})
}

func rgSetEqual(a, b *RGSet) bool {
	if a.NumRGs != b.NumRGs {
		return false
	}
	aSlice := a.ToSlice()
	bSlice := b.ToSlice()
	if len(aSlice) != len(bSlice) {
		return false
	}
	for i := range aSlice {
		if aSlice[i] != bSlice[i] {
			return false
		}
	}
	return true
}

// genSingleTestDoc generates a single test document
func genSingleTestDoc() gopter.Gen {
	names := []string{"alice", "bob", "charlie", "diana", "eve"}
	statuses := []string{"active", "pending", "inactive"}

	return gopter.CombineGens(
		gen.IntRange(0, len(names)-1),
		gen.IntRange(18, 65),
		gen.Bool(),
		gen.IntRange(0, len(statuses)-1),
	).Map(func(vals []interface{}) TestDoc {
		name := names[vals[0].(int)]
		age := vals[1].(int)
		active := vals[2].(bool)
		status := statuses[vals[3].(int)]

		data := map[string]any{
			"name":   name,
			"age":    float64(age),
			"active": active,
			"status": status,
		}
		jsonBytes, _ := json.Marshal(data)
		return TestDoc{JSON: jsonBytes, Data: data}
	})
}

// GenTestDocs generates documents with name, age, active, status fields
func GenTestDocs(maxCount int) gopter.Gen {
	return gen.SliceOfN(maxCount, genSingleTestDoc())
}

// genSingleTestDocWithNulls generates a single test document that may have null values
func genSingleTestDocWithNulls() gopter.Gen {
	names := []string{"alice", "bob", "charlie"}

	return gopter.CombineGens(
		gen.IntRange(0, len(names)-1),
		gen.IntRange(18, 65),
		gen.Bool(),
		gen.Bool(),
		gen.Bool(),
	).Map(func(vals []interface{}) TestDoc {
		nameIdx := vals[0].(int)
		age := vals[1].(int)
		active := vals[2].(bool)
		nameIsNull := vals[3].(bool)
		ageIsNull := vals[4].(bool)

		data := make(map[string]any)
		if nameIsNull {
			data["name"] = nil
		} else {
			data["name"] = names[nameIdx]
		}
		if ageIsNull {
			data["age"] = nil
		} else {
			data["age"] = float64(age)
		}
		data["active"] = active

		jsonBytes, _ := json.Marshal(data)
		return TestDoc{JSON: jsonBytes, Data: data}
	})
}

// GenTestDocsWithNulls generates documents with random null values
func GenTestDocsWithNulls(maxCount int) gopter.Gen {
	return gen.SliceOfN(maxCount, genSingleTestDocWithNulls())
}

// genSingleMixedTypeDoc generates a single test document with name, age, active
func genSingleMixedTypeDoc() gopter.Gen {
	names := []string{"alice", "bob", "charlie"}

	return gopter.CombineGens(
		gen.IntRange(0, len(names)-1),
		gen.IntRange(18, 65),
		gen.Bool(),
	).Map(func(vals []interface{}) TestDoc {
		name := names[vals[0].(int)]
		age := vals[1].(int)
		active := vals[2].(bool)

		data := map[string]any{
			"name":   name,
			"age":    float64(age),
			"active": active,
		}
		jsonBytes, _ := json.Marshal(data)
		return TestDoc{JSON: jsonBytes, Data: data}
	})
}

// GenMixedTypeDocs generates documents with constrained values for multi-predicate testing
func GenMixedTypeDocs(maxCount int) gopter.Gen {
	return gen.SliceOfN(maxCount, genSingleMixedTypeDoc())
}

// GenPredicate generates random predicates for testing
func GenPredicate() gopter.Gen {
	names := []string{"alice", "bob", "charlie"}

	return gen.OneGenOf(
		// EQ on name
		gen.IntRange(0, len(names)-1).Map(func(idx int) Predicate {
			return EQ("$.name", names[idx])
		}),
		// GT/GTE/LT/LTE on age
		gopter.CombineGens(
			gen.IntRange(0, 3),
			gen.IntRange(18, 65),
		).Map(func(vals []interface{}) Predicate {
			op := vals[0].(int)
			age := float64(vals[1].(int))
			switch op {
			case 0:
				return GT("$.age", age)
			case 1:
				return GTE("$.age", age)
			case 2:
				return LT("$.age", age)
			default:
				return LTE("$.age", age)
			}
		}),
		// IsNull / IsNotNull on name
		gen.Bool().Map(func(isNull bool) Predicate {
			if isNull {
				return IsNull("$.name")
			}
			return IsNotNull("$.name")
		}),
		// EQ on active
		gen.Bool().Map(func(active bool) Predicate {
			return EQ("$.active", fmt.Sprintf("%t", active))
		}),
	)
}
