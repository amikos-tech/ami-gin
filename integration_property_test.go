package gin

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// High Priority Tests

func TestPropertyIntegrationFullPipelineNoFalseNegatives(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("query results superset of actual matches", prop.ForAll(
		func(docs []TestDoc, queryNameIdx int) bool {
			if len(docs) == 0 {
				return true
			}

			names := []string{"alice", "bob", "charlie", "diana", "eve"}
			queryValue := names[queryNameIdx%len(names)]

			numRGs := len(docs)
			builder, _ := NewBuilder(DefaultConfig(), numRGs)
			for i, doc := range docs {
				_ = builder.AddDocument(DocID(i), doc.JSON)
			}
			idx := builder.Finalize()

			rgSet := idx.Evaluate([]Predicate{EQ("$.name", queryValue)})

			for i, doc := range docs {
				if doc.HasFieldValue("name", queryValue) {
					if !rgSet.IsSet(i) {
						return false
					}
				}
			}
			return true
		},
		GenTestDocs(50),
		gen.IntRange(0, 4),
	))

	properties.TestingRun(t)
}

func TestPropertyIntegrationSerializationPreservesQueries(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("queries identical after round-trip", prop.ForAll(
		func(docs []TestDoc, predicate Predicate) bool {
			if len(docs) == 0 {
				return true
			}

			numRGs := len(docs)
			builder, _ := NewBuilder(DefaultConfig(), numRGs)
			for i, doc := range docs {
				_ = builder.AddDocument(DocID(i), doc.JSON)
			}
			original := builder.Finalize()

			beforeResult := original.Evaluate([]Predicate{predicate})

			encoded, err := Encode(original)
			if err != nil {
				return true
			}

			decoded, err := Decode(encoded)
			if err != nil {
				return false
			}

			afterResult := decoded.Evaluate([]Predicate{predicate})

			return rgSetEqual(beforeResult, afterResult)
		},
		GenTestDocs(20),
		GenPredicate(),
	))

	properties.TestingRun(t)
}

func TestPropertyIntegrationMultiPredicateIntersection(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("multi-predicate returns intersection", prop.ForAll(
		func(docs []TestDoc) bool {
			if len(docs) == 0 {
				return true
			}

			numRGs := len(docs)
			builder, _ := NewBuilder(DefaultConfig(), numRGs)
			for i, doc := range docs {
				_ = builder.AddDocument(DocID(i), doc.JSON)
			}
			idx := builder.Finalize()

			combined := idx.Evaluate([]Predicate{
				EQ("$.name", "alice"),
				GT("$.age", 25.0),
			})

			nameResult := idx.Evaluate([]Predicate{EQ("$.name", "alice")})
			ageResult := idx.Evaluate([]Predicate{GT("$.age", 25.0)})
			expected := nameResult.Intersect(ageResult)

			return rgSetEqual(combined, expected)
		},
		GenMixedTypeDocs(50),
	))

	properties.TestingRun(t)
}

// Medium Priority Tests

func TestPropertyIntegrationNullPresentConsistency(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("null and present bitmaps are consistent", prop.ForAll(
		func(docs []TestDoc) bool {
			if len(docs) == 0 {
				return true
			}

			numRGs := len(docs)
			builder, _ := NewBuilder(DefaultConfig(), numRGs)
			for i, doc := range docs {
				_ = builder.AddDocument(DocID(i), doc.JSON)
			}
			idx := builder.Finalize()

			for pathID, nullIdx := range idx.NullIndexes {
				if !isSubset(nullIdx.NullRGBitmap, nullIdx.PresentRGBitmap) {
					return false
				}

				if si, ok := idx.StringIndexes[pathID]; ok {
					valueRGs := unionAll(si.RGBitmaps)
					overlap := valueRGs.Intersect(nullIdx.NullRGBitmap)
					if !overlap.IsEmpty() {
						return false
					}
				}
			}
			return true
		},
		GenTestDocsWithNulls(50),
	))

	properties.TestingRun(t)
}

func TestPropertyIntegrationCardinalityThreshold(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("cardinality threshold controls index type", prop.ForAll(
		func(values []string, threshold uint32) bool {
			if len(values) == 0 || threshold == 0 {
				return true
			}

			validValues := make([]string, 0, len(values))
			for _, v := range values {
				if v != "" {
					validValues = append(validValues, v)
				}
			}
			if len(validValues) == 0 {
				return true
			}

			config := DefaultConfig()
			config.CardinalityThreshold = threshold

			numRGs := len(validValues)
			if numRGs > 100 {
				numRGs = 100
			}
			builder, _ := NewBuilder(config, numRGs)
			for i := 0; i < numRGs; i++ {
				doc := []byte(`{"field": "` + validValues[i] + `"}`)
				_ = builder.AddDocument(DocID(i), doc)
			}
			idx := builder.Finalize()

			entry := findPathEntry(idx, "$.field")
			if entry == nil {
				return true
			}

			// Use the stored HLL-estimated cardinality (which is what the builder uses)
			// rather than actual unique count since HLL is approximate
			estimatedCardinality := entry.Cardinality

			if estimatedCardinality > threshold {
				return entry.Flags&FlagBloomOnly != 0
			}
			_, hasStringIdx := idx.StringIndexes[entry.PathID]
			return hasStringIdx && entry.Flags&FlagBloomOnly == 0
		},
		gen.SliceOfN(100, gen.Identifier()),
		gen.UInt32Range(10, 1000),
	))

	properties.TestingRun(t)
}

func TestPropertyIntegrationArrayWildcardSuperset(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("wildcard includes all element matches", prop.ForAll(
		func(arrays [][]string, searchIdx int) bool {
			if len(arrays) == 0 {
				return true
			}

			searchValues := []string{"alpha", "beta", "gamma"}
			searchValue := searchValues[searchIdx%len(searchValues)]

			numRGs := len(arrays)
			if numRGs > 50 {
				numRGs = 50
			}
			builder, _ := NewBuilder(DefaultConfig(), numRGs)
			for i := 0; i < numRGs; i++ {
				arr := arrays[i]
				for j := range arr {
					if arr[j] == "" {
						arr[j] = searchValues[j%len(searchValues)]
					}
				}
				doc, _ := json.Marshal(map[string]any{"items": arr})
				_ = builder.AddDocument(DocID(i), doc)
			}
			idx := builder.Finalize()

			wildcardResult := idx.Evaluate([]Predicate{EQ("$.items[*]", searchValue)})

			for i := 0; i < numRGs; i++ {
				for _, v := range arrays[i] {
					if v == searchValue && !wildcardResult.IsSet(i) {
						return false
					}
				}
			}
			return true
		},
		gen.SliceOfN(20, gen.SliceOfN(5, gen.OneConstOf("alpha", "beta", "gamma", "delta"))),
		gen.IntRange(0, 2),
	))

	properties.TestingRun(t)
}

// Low Priority Tests

func TestPropertyIntegrationTrigramContainsSuperset(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("Contains superset of actual substrings", prop.ForAll(
		func(values []string, pattern string) bool {
			if len(pattern) < 3 || len(values) == 0 {
				return true
			}

			validValues := make([]string, 0, len(values))
			for _, v := range values {
				if len(v) >= 3 {
					validValues = append(validValues, v)
				}
			}
			if len(validValues) == 0 {
				return true
			}

			numRGs := len(validValues)
			if numRGs > 30 {
				numRGs = 30
			}

			builder, _ := NewBuilder(DefaultConfig(), numRGs)
			for i := 0; i < numRGs; i++ {
				doc := []byte(`{"text": "` + validValues[i] + `"}`)
				_ = builder.AddDocument(DocID(i), doc)
			}
			idx := builder.Finalize()

			result := idx.Evaluate([]Predicate{Contains("$.text", pattern)})

			patternLower := strings.ToLower(pattern)
			for i := 0; i < numRGs; i++ {
				if strings.Contains(strings.ToLower(validValues[i]), patternLower) {
					if !result.IsSet(i) {
						return false
					}
				}
			}
			return true
		},
		gen.SliceOfN(30, gen.Identifier()),
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

// Edge Case Tests

func TestPropertyIntegrationEmptyDocsHandling(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("empty docs don't cause panics", prop.ForAll(
		func(numDocs int) bool {
			if numDocs <= 0 {
				numDocs = 1
			}
			if numDocs > 50 {
				numDocs = 50
			}

			builder, _ := NewBuilder(DefaultConfig(), numDocs)
			for i := 0; i < numDocs; i++ {
				_ = builder.AddDocument(DocID(i), []byte(`{}`))
			}
			idx := builder.Finalize()

			_ = idx.Evaluate([]Predicate{EQ("$.name", "test")})
			_ = idx.Evaluate([]Predicate{GT("$.age", 25.0)})
			_ = idx.Evaluate([]Predicate{IsNull("$.field")})
			_ = idx.Evaluate([]Predicate{IsNotNull("$.field")})
			_ = idx.Evaluate([]Predicate{Contains("$.text", "abc")})

			return true
		},
		gen.IntRange(1, 50),
	))

	properties.TestingRun(t)
}

func TestPropertyIntegrationSpecialValuesHandling(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("special values handled correctly", prop.ForAll(
		func(numDocs int) bool {
			if numDocs <= 0 {
				numDocs = 1
			}
			if numDocs > 20 {
				numDocs = 20
			}

			builder, _ := NewBuilder(DefaultConfig(), numDocs)

			specialDocs := [][]byte{
				[]byte(`{"num": 0}`),
				[]byte(`{"num": -0}`),
				[]byte(`{"num": 1e10}`),
				[]byte(`{"num": -1e10}`),
				[]byte(`{"str": ""}`),
				[]byte(`{"bool": true}`),
				[]byte(`{"bool": false}`),
				[]byte(`{"null_field": null}`),
			}

			for i := 0; i < numDocs; i++ {
				doc := specialDocs[i%len(specialDocs)]
				_ = builder.AddDocument(DocID(i), doc)
			}
			idx := builder.Finalize()

			_ = idx.Evaluate([]Predicate{EQ("$.num", 0.0)})
			_ = idx.Evaluate([]Predicate{GT("$.num", 0.0)})
			_ = idx.Evaluate([]Predicate{LT("$.num", 0.0)})
			_ = idx.Evaluate([]Predicate{EQ("$.str", "")})
			_ = idx.Evaluate([]Predicate{EQ("$.bool", "true")})
			_ = idx.Evaluate([]Predicate{EQ("$.bool", "false")})
			_ = idx.Evaluate([]Predicate{IsNull("$.null_field")})

			encoded, err := Encode(idx)
			if err != nil {
				return false
			}
			_, err = Decode(encoded)
			return err == nil
		},
		gen.IntRange(1, 20),
	))

	properties.TestingRun(t)
}

func TestPropertyIntegrationNumericRangeQueryNoFalseNegatives(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("numeric range queries have no false negatives", prop.ForAll(
		func(docs []TestDoc, queryAge int) bool {
			if len(docs) == 0 {
				return true
			}

			numRGs := len(docs)
			builder, _ := NewBuilder(DefaultConfig(), numRGs)
			for i, doc := range docs {
				_ = builder.AddDocument(DocID(i), doc.JSON)
			}
			idx := builder.Finalize()

			queryVal := float64(queryAge)

			gtResult := idx.Evaluate([]Predicate{GT("$.age", queryVal)})
			for i, doc := range docs {
				age, ok := doc.Data["age"].(float64)
				if ok && age > queryVal && !gtResult.IsSet(i) {
					return false
				}
			}

			gteResult := idx.Evaluate([]Predicate{GTE("$.age", queryVal)})
			for i, doc := range docs {
				age, ok := doc.Data["age"].(float64)
				if ok && age >= queryVal && !gteResult.IsSet(i) {
					return false
				}
			}

			ltResult := idx.Evaluate([]Predicate{LT("$.age", queryVal)})
			for i, doc := range docs {
				age, ok := doc.Data["age"].(float64)
				if ok && age < queryVal && !ltResult.IsSet(i) {
					return false
				}
			}

			lteResult := idx.Evaluate([]Predicate{LTE("$.age", queryVal)})
			for i, doc := range docs {
				age, ok := doc.Data["age"].(float64)
				if ok && age <= queryVal && !lteResult.IsSet(i) {
					return false
				}
			}

			return true
		},
		GenMixedTypeDocs(50),
		gen.IntRange(18, 65),
	))

	properties.TestingRun(t)
}

func TestPropertyIntegrationStringLengthPruningNoFalseNegatives(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("string length pruning never causes false negatives", prop.ForAll(
		func(values []string, queryIdx int) bool {
			validValues := make([]string, 0, len(values))
			for _, v := range values {
				if v != "" {
					validValues = append(validValues, v)
				}
			}
			if len(validValues) == 0 {
				return true
			}

			numRGs := len(validValues)
			if numRGs > 50 {
				numRGs = 50
			}

			builder, _ := NewBuilder(DefaultConfig(), numRGs)
			for i := 0; i < numRGs; i++ {
				doc := []byte(`{"field": "` + validValues[i] + `"}`)
				_ = builder.AddDocument(DocID(i), doc)
			}
			idx := builder.Finalize()

			queryValue := validValues[queryIdx%len(validValues)]
			result := idx.Evaluate([]Predicate{EQ("$.field", queryValue)})

			for i := 0; i < numRGs; i++ {
				if validValues[i] == queryValue && !result.IsSet(i) {
					return false
				}
			}
			return true
		},
		gen.SliceOfN(50, gen.Identifier()),
		gen.IntRange(0, 49),
	))

	properties.TestingRun(t)
}

func TestPropertyIntegrationStringLengthIndexSerializationRoundTrip(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("StringLengthIndex preserved after serialization", prop.ForAll(
		func(values []string) bool {
			validValues := make([]string, 0, len(values))
			for _, v := range values {
				if v != "" {
					validValues = append(validValues, v)
				}
			}
			if len(validValues) == 0 {
				return true
			}

			numRGs := len(validValues)
			if numRGs > 30 {
				numRGs = 30
			}

			builder, _ := NewBuilder(DefaultConfig(), numRGs)
			for i := 0; i < numRGs; i++ {
				doc := []byte(`{"name": "` + validValues[i] + `"}`)
				_ = builder.AddDocument(DocID(i), doc)
			}
			original := builder.Finalize()

			encoded, err := Encode(original)
			if err != nil {
				return true
			}

			decoded, err := Decode(encoded)
			if err != nil {
				return false
			}

			for pathID, origSLI := range original.StringLengthIndexes {
				decodedSLI, ok := decoded.StringLengthIndexes[pathID]
				if !ok {
					return false
				}
				if origSLI.GlobalMin != decodedSLI.GlobalMin ||
					origSLI.GlobalMax != decodedSLI.GlobalMax {
					return false
				}
				if len(origSLI.RGStats) != len(decodedSLI.RGStats) {
					return false
				}
				for i, stat := range origSLI.RGStats {
					if stat.Min != decodedSLI.RGStats[i].Min ||
						stat.Max != decodedSLI.RGStats[i].Max ||
						stat.HasValue != decodedSLI.RGStats[i].HasValue {
						return false
					}
				}
			}

			for _, v := range validValues[:numRGs] {
				beforeResult := original.Evaluate([]Predicate{EQ("$.name", v)})
				afterResult := decoded.Evaluate([]Predicate{EQ("$.name", v)})
				if !rgSetEqual(beforeResult, afterResult) {
					return false
				}
			}

			return true
		},
		gen.SliceOfN(30, gen.Identifier()),
	))

	properties.TestingRun(t)
}

func TestPropertyIntegrationStringLengthStatsCorrectness(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("StringLengthIndex stats are correct", prop.ForAll(
		func(values []string) bool {
			validValues := make([]string, 0, len(values))
			for _, v := range values {
				if v != "" {
					validValues = append(validValues, v)
				}
			}
			if len(validValues) == 0 {
				return true
			}

			numRGs := len(validValues)
			if numRGs > 30 {
				numRGs = 30
			}

			builder, _ := NewBuilder(DefaultConfig(), numRGs)
			for i := 0; i < numRGs; i++ {
				doc := []byte(`{"text": "` + validValues[i] + `"}`)
				_ = builder.AddDocument(DocID(i), doc)
			}
			idx := builder.Finalize()

			entry := findPathEntry(idx, "$.text")
			if entry == nil {
				return true
			}

			sli, ok := idx.StringLengthIndexes[entry.PathID]
			if !ok {
				return true
			}

			var actualMin, actualMax uint32
			first := true
			for i := 0; i < numRGs; i++ {
				length := uint32(len(validValues[i]))
				if first {
					actualMin = length
					actualMax = length
					first = false
				} else {
					if length < actualMin {
						actualMin = length
					}
					if length > actualMax {
						actualMax = length
					}
				}
			}

			if sli.GlobalMin != actualMin || sli.GlobalMax != actualMax {
				return false
			}

			for i := 0; i < numRGs; i++ {
				stat := sli.RGStats[i]
				if !stat.HasValue {
					return false
				}
				expectedLen := uint32(len(validValues[i]))
				if stat.Min != expectedLen || stat.Max != expectedLen {
					return false
				}
			}

			return true
		},
		gen.SliceOfN(30, gen.Identifier()),
	))

	properties.TestingRun(t)
}
