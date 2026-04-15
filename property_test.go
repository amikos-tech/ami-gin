package gin

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

const (
	propertyTestDefaultMinSuccessfulTests = 1000
	propertyTestHLLMinSuccessfulTests     = 250
	propertyTestHLLEstimateSampleSize     = 256
)

func propertyTestParametersWithMinSuccessfulTests(minSuccessfulTests int) *gopter.TestParameters {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = minSuccessfulTests
	return params
}

func propertyTestParameters() *gopter.TestParameters {
	return propertyTestParametersWithMinSuccessfulTests(propertyTestDefaultMinSuccessfulTests)
}

func TestPropertyIdentityCodecRoundTrip(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())
	codec := NewIdentityCodec()

	properties.Property("encode/decode is lossless", prop.ForAll(
		func(val int) bool {
			if val < 0 {
				val = -val
			}
			encoded := codec.Encode(val)
			decoded := codec.Decode(encoded)
			return len(decoded) == 1 && decoded[0] == val
		},
		gen.Int(),
	))

	properties.TestingRun(t)
}

func TestPropertyRowGroupCodecRoundTrip(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("encode/decode is lossless", prop.ForAll(
		func(rgPerFile, fileIdx, rgIdx int) bool {
			if rgPerFile <= 0 {
				rgPerFile = 1
			}
			if fileIdx < 0 {
				fileIdx = -fileIdx
			}
			if rgIdx < 0 {
				rgIdx = -rgIdx
			}
			rgIdx %= rgPerFile

			codec := NewRowGroupCodec(rgPerFile)
			encoded := codec.Encode(fileIdx, rgIdx)
			decoded := codec.Decode(encoded)
			return len(decoded) == 2 && decoded[0] == fileIdx && decoded[1] == rgIdx
		},
		gen.IntRange(1, 1000),
		gen.IntRange(0, 10000),
		gen.IntRange(0, 10000),
	))

	properties.TestingRun(t)
}

func TestPropertyBloomFilterNoFalseNegatives(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("if added, MayContain returns true", prop.ForAll(
		func(items []string) bool {
			bf, _ := NewBloomFilter(8192, 5)
			for _, item := range items {
				bf.AddString(item)
			}
			for _, item := range items {
				if !bf.MayContainString(item) {
					return false
				}
			}
			return true
		},
		gen.SliceOfN(100, gen.AlphaString()),
	))

	properties.TestingRun(t)
}

func TestPropertyBloomFilterIdempotentAdd(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("adding twice doesn't change membership", prop.ForAll(
		func(item string) bool {
			bf, _ := NewBloomFilter(1024, 3)
			bf.AddString(item)
			contains1 := bf.MayContainString(item)
			bf.AddString(item)
			contains2 := bf.MayContainString(item)
			return contains1 && contains2
		},
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

func TestPropertyRGSetIntersectCommutative(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("A ∩ B = B ∩ A", prop.ForAll(
		func(pair RGSetPair) bool {
			ab := pair.A.Intersect(pair.B)
			ba := pair.B.Intersect(pair.A)
			return rgSetEqual(ab, ba)
		},
		GenRGSetPair(100),
	))

	properties.TestingRun(t)
}

func TestPropertyRGSetIntersectAssociative(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("(A ∩ B) ∩ C = A ∩ (B ∩ C)", prop.ForAll(
		func(triple RGSetTriple) bool {
			ab := triple.A.Intersect(triple.B)
			ab_c := ab.Intersect(triple.C)

			bc := triple.B.Intersect(triple.C)
			a_bc := triple.A.Intersect(bc)

			return rgSetEqual(ab_c, a_bc)
		},
		GenRGSetTriple(100),
	))

	properties.TestingRun(t)
}

func TestPropertyRGSetUnionCommutative(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("A ∪ B = B ∪ A", prop.ForAll(
		func(pair RGSetPair) bool {
			ab := pair.A.Union(pair.B)
			ba := pair.B.Union(pair.A)
			return rgSetEqual(ab, ba)
		},
		GenRGSetPair(100),
	))

	properties.TestingRun(t)
}

func TestPropertyRGSetUnionAssociative(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("(A ∪ B) ∪ C = A ∪ (B ∪ C)", prop.ForAll(
		func(triple RGSetTriple) bool {
			ab := triple.A.Union(triple.B)
			ab_c := ab.Union(triple.C)

			bc := triple.B.Union(triple.C)
			a_bc := triple.A.Union(bc)

			return rgSetEqual(ab_c, a_bc)
		},
		GenRGSetTriple(100),
	))

	properties.TestingRun(t)
}

func TestPropertyRGSetInvertSelfInverse(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("~~A = A", prop.ForAll(
		func(pair RGSetPair) bool {
			a := pair.A
			inverted := a.Invert()
			doubleInverted := inverted.Invert()
			return rgSetEqual(a, doubleInverted)
		},
		GenRGSetPair(100),
	))

	properties.TestingRun(t)
}

func TestPropertyRGSetDeMorgans(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("~(A ∩ B) = ~A ∪ ~B", prop.ForAll(
		func(pair RGSetPair) bool {
			intersection := pair.A.Intersect(pair.B)
			notIntersection := intersection.Invert()

			notA := pair.A.Invert()
			notB := pair.B.Invert()
			unionOfNots := notA.Union(notB)

			return rgSetEqual(notIntersection, unionOfNots)
		},
		GenRGSetPair(100),
	))

	properties.TestingRun(t)
}

func TestPropertyPrefixCompressionRoundTrip(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("compress/decompress is lossless for sorted strings", prop.ForAll(
		func(terms []string) bool {
			if len(terms) == 0 {
				return true
			}
			pc, _ := NewPrefixCompressor(16)
			compressed := pc.Compress(terms)
			decompressed := pc.Decompress(compressed)

			if len(decompressed) != len(terms) {
				return false
			}
			termSet := make(map[string]bool)
			for _, t := range terms {
				termSet[t] = true
			}
			for _, d := range decompressed {
				if !termSet[d] {
					return false
				}
			}
			return true
		},
		GenSortedStrings(1, 50),
	))

	properties.TestingRun(t)
}

func TestPropertySerializationRoundTrip(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("encode/decode produces equivalent index", prop.ForAll(
		func(docs [][]byte) bool {
			validDocs := make([][]byte, 0)
			for _, doc := range docs {
				if len(doc) > 2 {
					validDocs = append(validDocs, doc)
				}
			}
			if len(validDocs) == 0 {
				return true
			}

			numRGs := len(validDocs)
			if numRGs > 100 {
				numRGs = 100
			}
			builder, _ := NewBuilder(DefaultConfig(), numRGs)
			for i, doc := range validDocs {
				if i >= numRGs {
					break
				}
				_ = builder.AddDocument(DocID(i), doc)
			}
			idx := builder.Finalize()

			encoded, err := Encode(idx)
			if err != nil {
				return true
			}

			decoded, err := Decode(encoded)
			if err != nil {
				return false
			}

			if idx.Header.NumRowGroups != decoded.Header.NumRowGroups {
				return false
			}
			if idx.Header.NumPaths != decoded.Header.NumPaths {
				return false
			}
			if len(idx.StringIndexes) != len(decoded.StringIndexes) {
				return false
			}
			if len(idx.NumericIndexes) != len(decoded.NumericIndexes) {
				return false
			}
			return true
		},
		gen.SliceOfN(10, GenJSONDocument(1)),
	))

	properties.TestingRun(t)
}

func TestPropertyTrigramSearchSuperset(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("results contain all exact substring matches", prop.ForAll(
		func(values []string, searchPattern string) bool {
			paddedValues := make([]string, len(values))
			for i, v := range values {
				if len(v) < 3 {
					paddedValues[i] = v + "abc"
				} else {
					paddedValues[i] = v
				}
			}
			if len(searchPattern) < 3 {
				searchPattern += "abc"
			}
			if len(paddedValues) == 0 {
				return true
			}

			numRGs := len(paddedValues)
			if numRGs > 50 {
				numRGs = 50
			}
			ti, _ := NewTrigramIndex(numRGs)
			for i := 0; i < numRGs; i++ {
				ti.Add(paddedValues[i], i)
			}

			result := ti.Search(searchPattern)
			searchLower := strings.ToLower(searchPattern)

			for i := 0; i < numRGs; i++ {
				valLower := strings.ToLower(paddedValues[i])
				if strings.Contains(valLower, searchLower) {
					if !result.IsSet(i) {
						return false
					}
				}
			}
			return true
		},
		gen.SliceOfN(20, gen.Identifier()),
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

func TestPropertyHLLMergeCommutative(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("merge(A, B) = merge(B, A)", prop.ForAll(
		func(pair HLLItemPair) bool {
			hllA1, _ := NewHyperLogLog(12)
			hllB1, _ := NewHyperLogLog(12)
			for _, item := range pair.Items1 {
				hllA1.AddString(item)
			}
			for _, item := range pair.Items2 {
				hllB1.AddString(item)
			}

			hllA2 := hllA1.Clone()
			hllB2 := hllB1.Clone()

			hllA1.Merge(hllB1)
			hllB2.Merge(hllA2)

			return hllA1.Estimate() == hllB2.Estimate()
		},
		GenHLLPair(),
	))

	properties.TestingRun(t)
}

func TestPropertyHLLEstimateWithinBounds(t *testing.T) {
	// This property is still statistically meaningful with fewer large samples
	// and otherwise dominates the Go 1.25 race-enabled CI budget.
	properties := gopter.NewProperties(propertyTestParametersWithMinSuccessfulTests(propertyTestHLLMinSuccessfulTests))

	properties.Property("estimate within 3σ of expected error", prop.ForAll(
		func(items []string) bool {
			if len(items) < 100 {
				return true
			}

			uniqueItems := make(map[string]struct{})
			for _, item := range items {
				uniqueItems[item] = struct{}{}
			}
			actualCardinality := len(uniqueItems)
			if actualCardinality < 50 {
				return true
			}

			hll, _ := NewHyperLogLog(12)
			for _, item := range items {
				hll.AddString(item)
			}

			estimate := float64(hll.Estimate())
			actual := float64(actualCardinality)

			m := float64(1 << 12)
			stdError := 1.04 / math.Sqrt(m)
			threeStdDev := 3 * stdError * actual

			diff := math.Abs(estimate - actual)
			return diff <= threeStdDev || diff <= 10
		},
		gen.SliceOfN(propertyTestHLLEstimateSampleSize, gen.AlphaString()),
	))

	properties.TestingRun(t)
}

func TestPropertyNumericRangeCorrectness(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("range queries return correct RGs", prop.ForAll(
		func(values []float64, queryVal float64) bool {
			if len(values) == 0 {
				return true
			}

			numRGs := len(values)
			if numRGs > 50 {
				numRGs = 50
			}

			builder, _ := NewBuilder(DefaultConfig(), numRGs)
			for i := 0; i < numRGs && i < len(values); i++ {
				doc := []byte(`{"value": ` + formatFloat(values[i]) + `}`)
				_ = builder.AddDocument(DocID(i), doc)
			}
			idx := builder.Finalize()

			gtResult := idx.Evaluate([]Predicate{GT("$.value", queryVal)})
			for i := 0; i < numRGs && i < len(values); i++ {
				if values[i] > queryVal {
					if !gtResult.IsSet(i) {
						return false
					}
				}
			}

			ltResult := idx.Evaluate([]Predicate{LT("$.value", queryVal)})
			for i := 0; i < numRGs && i < len(values); i++ {
				if values[i] < queryVal {
					if !ltResult.IsSet(i) {
						return false
					}
				}
			}

			return true
		},
		gen.SliceOfN(20, gen.Float64Range(-1e6, 1e6)),
		gen.Float64Range(-1e6, 1e6),
	))

	properties.TestingRun(t)
}

func formatFloat(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "0"
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}
