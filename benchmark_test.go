package gin

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
)

// =============================================================================
// Test Data Generators
// =============================================================================

func generateTestDoc(i int) []byte {
	doc := map[string]any{
		"id":     i,
		"name":   fmt.Sprintf("user_%d", i%100),
		"age":    20 + (i % 50),
		"active": i%2 == 0,
		"status": []string{"active", "pending", "inactive"}[i%3],
		"score":  float64(i%1000) / 10.0,
		"tags":   []string{fmt.Sprintf("tag_%d", i%10), fmt.Sprintf("category_%d", i%5)},
	}
	data, _ := json.Marshal(doc)
	return data
}

func generateTestDocWithText(i int) []byte {
	texts := []string{
		"the quick brown fox jumps over the lazy dog",
		"hello world this is a test document",
		"golang is a statically typed programming language",
		"data lake indexing with parquet files",
		"row group pruning for efficient queries",
	}
	doc := map[string]any{
		"id":          i,
		"description": texts[i%len(texts)] + fmt.Sprintf(" variant %d", i),
	}
	data, _ := json.Marshal(doc)
	return data
}

func generateLargeDoc(i int, numFields int) []byte {
	doc := make(map[string]any)
	doc["id"] = i
	for j := 0; j < numFields; j++ {
		doc[fmt.Sprintf("field_%d", j)] = fmt.Sprintf("value_%d_%d", i, j)
	}
	data, _ := json.Marshal(doc)
	return data
}

func generateNestedDoc(i int, depth int) []byte {
	doc := make(map[string]any)
	current := doc
	for d := 0; d < depth; d++ {
		if d == depth-1 {
			current[fmt.Sprintf("level_%d", d)] = fmt.Sprintf("value_%d", i)
		} else {
			nested := make(map[string]any)
			current[fmt.Sprintf("level_%d", d)] = nested
			current = nested
		}
	}
	data, _ := json.Marshal(doc)
	return data
}

func generateHighCardinalityDocs(n int, cardinality int) [][]byte {
	docs := make([][]byte, n)
	for i := 0; i < n; i++ {
		doc := map[string]any{
			"id":     i,
			"unique": fmt.Sprintf("unique_value_%d", i%cardinality),
		}
		data, _ := json.Marshal(doc)
		docs[i] = data
	}
	return docs
}

func setupTestIndex(numRGs int) *GINIndex {
	builder, _ := NewBuilder(DefaultConfig(), numRGs)
	for i := 0; i < numRGs; i++ {
		builder.AddDocument(DocID(i), generateTestDoc(i))
	}
	return builder.Finalize()
}

func setupTestIndexWithText(numRGs int) *GINIndex {
	builder, _ := NewBuilder(DefaultConfig(), numRGs)
	for i := 0; i < numRGs; i++ {
		builder.AddDocument(DocID(i), generateTestDocWithText(i))
	}
	return builder.Finalize()
}

// =============================================================================
// Builder Performance Benchmarks
// =============================================================================

func BenchmarkAddDocument(b *testing.B) {
	config := DefaultConfig()
	doc := generateTestDoc(0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder, _ := NewBuilder(config, 1000)
		builder.AddDocument(DocID(i%1000), doc)
	}
}

func BenchmarkAddDocumentBatch(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Docs=%d", size), func(b *testing.B) {
			config := DefaultConfig()
			docs := make([][]byte, size)
			for i := 0; i < size; i++ {
				docs[i] = generateTestDoc(i)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				builder, _ := NewBuilder(config, size)
				for j := 0; j < size; j++ {
					builder.AddDocument(DocID(j), docs[j])
				}
			}
		})
	}
}

func BenchmarkFinalize(b *testing.B) {
	sizes := []int{100, 1000, 5000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("RGs=%d", size), func(b *testing.B) {
			docs := make([][]byte, size)
			for i := 0; i < size; i++ {
				docs[i] = generateTestDoc(i)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				builder, _ := NewBuilder(DefaultConfig(), size)
				for j := 0; j < size; j++ {
					builder.AddDocument(DocID(j), docs[j])
				}
				b.StartTimer()

				_ = builder.Finalize()
			}
		})
	}
}

func BenchmarkBuilderMemory(b *testing.B) {
	sizes := []int{100, 1000, 5000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Docs=%d", size), func(b *testing.B) {
			docs := make([][]byte, size)
			for i := 0; i < size; i++ {
				docs[i] = generateTestDoc(i)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				builder, _ := NewBuilder(DefaultConfig(), size)
				for j := 0; j < size; j++ {
					builder.AddDocument(DocID(j), docs[j])
				}
				_ = builder.Finalize()
			}
		})
	}
}

// =============================================================================
// Query Performance Benchmarks
// =============================================================================

func BenchmarkQueryEQ(b *testing.B) {
	idx := setupTestIndex(1000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		idx.Evaluate([]Predicate{EQ("$.name", "user_42")})
	}
}

func BenchmarkQueryEQParallel(b *testing.B) {
	idx := setupTestIndex(1000)
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			idx.Evaluate([]Predicate{EQ("$.name", "user_42")})
		}
	})
}

func BenchmarkQueryEQMiss(b *testing.B) {
	idx := setupTestIndex(1000)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		idx.Evaluate([]Predicate{EQ("$.name", "nonexistent_user")})
	}
}

func BenchmarkQueryRange(b *testing.B) {
	ops := []struct {
		name string
		pred Predicate
	}{
		{"GT", GT("$.age", 30)},
		{"GTE", GTE("$.age", 30)},
		{"LT", LT("$.age", 30)},
		{"LTE", LTE("$.age", 30)},
	}

	idx := setupTestIndex(1000)

	for _, op := range ops {
		b.Run(op.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				idx.Evaluate([]Predicate{op.pred})
			}
		})
	}
}

func BenchmarkQueryIN(b *testing.B) {
	sizes := []int{2, 5, 10, 20}

	idx := setupTestIndex(1000)

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Values=%d", size), func(b *testing.B) {
			values := make([]any, size)
			for i := 0; i < size; i++ {
				values[i] = fmt.Sprintf("user_%d", i*10)
			}
			pred := Predicate{Path: "$.name", Operator: OpIN, Value: values}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				idx.Evaluate([]Predicate{pred})
			}
		})
	}
}

func BenchmarkQueryContains(b *testing.B) {
	patterns := []string{"quick", "hello world", "programming language"}

	idx := setupTestIndexWithText(1000)

	for _, pattern := range patterns {
		b.Run(fmt.Sprintf("Len=%d", len(pattern)), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				idx.Evaluate([]Predicate{Contains("$.description", pattern)})
			}
		})
	}
}

func BenchmarkQueryNull(b *testing.B) {
	builder, _ := NewBuilder(DefaultConfig(), 1000)
	for i := 0; i < 1000; i++ {
		var doc []byte
		if i%3 == 0 {
			doc = []byte(`{"name": null, "value": 42}`)
		} else {
			doc = []byte(fmt.Sprintf(`{"name": "user_%d", "value": %d}`, i, i))
		}
		builder.AddDocument(DocID(i), doc)
	}
	idx := builder.Finalize()

	b.Run("IsNull", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			idx.Evaluate([]Predicate{IsNull("$.name")})
		}
	})

	b.Run("IsNotNull", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			idx.Evaluate([]Predicate{IsNotNull("$.name")})
		}
	})
}

func BenchmarkQueryMultiplePreds(b *testing.B) {
	counts := []int{1, 2, 3, 4, 5}

	idx := setupTestIndex(1000)

	for _, count := range counts {
		b.Run(fmt.Sprintf("Preds=%d", count), func(b *testing.B) {
			preds := make([]Predicate, count)
			for i := 0; i < count; i++ {
				switch i % 3 {
				case 0:
					preds[i] = EQ("$.name", "user_42")
				case 1:
					preds[i] = GTE("$.age", 30)
				case 2:
					preds[i] = EQ("$.active", true)
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				idx.Evaluate(preds)
			}
		})
	}
}

func BenchmarkQueryVsIndexSize(b *testing.B) {
	sizes := []int{10, 100, 1000, 5000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("RGs=%d", size), func(b *testing.B) {
			idx := setupTestIndex(size)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				idx.Evaluate([]Predicate{EQ("$.name", "user_42")})
			}
		})
	}
}

// =============================================================================
// Serialization Performance Benchmarks
// =============================================================================

func BenchmarkEncode(b *testing.B) {
	sizes := []int{100, 500, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("RGs=%d", size), func(b *testing.B) {
			idx := setupTestIndex(size)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _ = Encode(idx)
			}
		})
	}
}

func BenchmarkDecode(b *testing.B) {
	sizes := []int{100, 500, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("RGs=%d", size), func(b *testing.B) {
			idx := setupTestIndex(size)
			data, _ := Encode(idx)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, _ = Decode(data)
			}
		})
	}
}

func BenchmarkEncodedSize(b *testing.B) {
	sizes := []int{100, 500, 1000, 2000}

	for _, size := range sizes {
		idx := setupTestIndex(size)
		data, _ := Encode(idx)
		b.Run(fmt.Sprintf("RGs=%d", size), func(b *testing.B) {
			b.ReportMetric(float64(len(data)), "bytes")
			b.ReportMetric(float64(len(data))/float64(size), "bytes/RG")
		})
	}
}

func BenchmarkSerializeRoundTrip(b *testing.B) {
	sizes := []int{100, 500, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("RGs=%d", size), func(b *testing.B) {
			idx := setupTestIndex(size)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				data, _ := Encode(idx)
				_, _ = Decode(data)
			}
		})
	}
}

// =============================================================================
// Component Benchmarks: Bloom Filter
// =============================================================================

func BenchmarkBloomAdd(b *testing.B) {
	sizes := []uint32{1024, 65536, 262144}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size=%d", size), func(b *testing.B) {
			bf, _ := NewBloomFilter(size, 5)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				bf.AddString(fmt.Sprintf("test_value_%d", i))
			}
		})
	}
}

func BenchmarkBloomLookup(b *testing.B) {
	bf, _ := NewBloomFilter(65536, 5)
	for i := 0; i < 10000; i++ {
		bf.AddString(fmt.Sprintf("test_value_%d", i))
	}

	b.Run("Hit", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			bf.MayContainString(fmt.Sprintf("test_value_%d", i%10000))
		}
	})

	b.Run("Miss", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			bf.MayContainString(fmt.Sprintf("nonexistent_%d", i))
		}
	})
}

func BenchmarkBloomFalsePositiveRate(b *testing.B) {
	bf, _ := NewBloomFilter(65536, 5)
	for i := 0; i < 10000; i++ {
		bf.AddString(fmt.Sprintf("value_%d", i))
	}

	falsePositives := 0
	total := 100000
	for i := 0; i < total; i++ {
		if bf.MayContainString(fmt.Sprintf("nonexistent_%d", i)) {
			falsePositives++
		}
	}

	b.ReportMetric(float64(falsePositives)/float64(total)*100, "FP%")
}

// =============================================================================
// Component Benchmarks: RGSet (Bitmap)
// =============================================================================

func BenchmarkRGSetIntersect(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size=%d", size), func(b *testing.B) {
			a, _ := NewRGSet(size)
			bb, _ := NewRGSet(size)
			for i := 0; i < size; i++ {
				if i%2 == 0 {
					a.Set(i)
				}
				if i%3 == 0 {
					bb.Set(i)
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = a.Intersect(bb)
			}
		})
	}
}

func BenchmarkRGSetUnion(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size=%d", size), func(b *testing.B) {
			a, _ := NewRGSet(size)
			bb, _ := NewRGSet(size)
			for i := 0; i < size; i++ {
				if i%2 == 0 {
					a.Set(i)
				}
				if i%3 == 0 {
					bb.Set(i)
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = a.Union(bb)
			}
		})
	}
}

func BenchmarkRGSetInvert(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size=%d", size), func(b *testing.B) {
			rs, _ := NewRGSet(size)
			for i := 0; i < size; i += 2 {
				rs.Set(i)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = rs.Invert()
			}
		})
	}
}

func BenchmarkRGSetVsSparsity(b *testing.B) {
	size := 10000
	densities := []float64{0.01, 0.1, 0.5, 0.9}

	for _, density := range densities {
		b.Run(fmt.Sprintf("Density=%.0f%%", density*100), func(b *testing.B) {
			a, _ := NewRGSet(size)
			bb, _ := NewRGSet(size)
			count := int(float64(size) * density)
			for i := 0; i < count; i++ {
				a.Set(rand.Intn(size))
				bb.Set(rand.Intn(size))
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = a.Intersect(bb)
			}
		})
	}
}

// =============================================================================
// Component Benchmarks: Trigram Index
// =============================================================================

func BenchmarkTrigramAdd(b *testing.B) {
	texts := []string{
		"short",
		"medium length text",
		"this is a longer text with more trigrams to extract",
	}

	for _, text := range texts {
		b.Run(fmt.Sprintf("Len=%d", len(text)), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ti, _ := NewTrigramIndex(100)
				ti.Add(text, 0)
			}
		})
	}
}

func BenchmarkTrigramSearch(b *testing.B) {
	ti, err := NewTrigramIndex(1000)
	if err != nil {
		b.Fatalf("failed to create trigram index: %v", err)
	}
	texts := []string{
		"the quick brown fox jumps over the lazy dog",
		"hello world this is a test document",
		"golang is a statically typed programming language",
		"data lake indexing with parquet files",
		"row group pruning for efficient queries",
	}
	for i := 0; i < 1000; i++ {
		ti.Add(texts[i%len(texts)]+fmt.Sprintf(" variant %d", i), i)
	}

	patterns := []string{"quick", "hello", "programming"}

	for _, pattern := range patterns {
		b.Run(fmt.Sprintf("Pattern=%s", pattern), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ti.Search(pattern)
			}
		})
	}
}

func BenchmarkTrigramVsPatternLen(b *testing.B) {
	ti, err := NewTrigramIndex(1000)
	if err != nil {
		b.Fatalf("failed to create trigram index: %v", err)
	}
	for i := 0; i < 1000; i++ {
		ti.Add(fmt.Sprintf("document number %d contains various words and phrases", i), i)
	}

	patterns := []struct {
		len     int
		pattern string
	}{
		{3, "doc"},
		{5, "docum"},
		{10, "document n"},
		{20, "document number cont"},
	}

	for _, p := range patterns {
		b.Run(fmt.Sprintf("Len=%d", p.len), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				ti.Search(p.pattern)
			}
		})
	}
}

// =============================================================================
// Component Benchmarks: HyperLogLog
// =============================================================================

func BenchmarkHLLAdd(b *testing.B) {
	precisions := []uint8{10, 12, 14}

	for _, p := range precisions {
		b.Run(fmt.Sprintf("Precision=%d", p), func(b *testing.B) {
			hll, _ := NewHyperLogLog(p)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				hll.AddString(fmt.Sprintf("value_%d", i))
			}
		})
	}
}

func BenchmarkHLLEstimate(b *testing.B) {
	precisions := []uint8{10, 12, 14}
	counts := []int{1000, 10000, 100000}

	for _, p := range precisions {
		for _, count := range counts {
			b.Run(fmt.Sprintf("Precision=%d/Count=%d", p, count), func(b *testing.B) {
				hll, _ := NewHyperLogLog(p)
				for i := 0; i < count; i++ {
					hll.AddString(fmt.Sprintf("value_%d", i))
				}
				b.ResetTimer()

				for i := 0; i < b.N; i++ {
					_ = hll.Estimate()
				}
			})
		}
	}
}

func BenchmarkHLLMerge(b *testing.B) {
	precisions := []uint8{10, 12, 14}

	for _, p := range precisions {
		b.Run(fmt.Sprintf("Precision=%d", p), func(b *testing.B) {
			hll1, _ := NewHyperLogLog(p)
			hll2, _ := NewHyperLogLog(p)
			for i := 0; i < 10000; i++ {
				hll1.AddString(fmt.Sprintf("set1_%d", i))
				hll2.AddString(fmt.Sprintf("set2_%d", i))
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				clone := hll1.Clone()
				clone.Merge(hll2)
			}
		})
	}
}

// =============================================================================
// Component Benchmarks: Prefix Compression
// =============================================================================

func BenchmarkPrefixCompress(b *testing.B) {
	termCounts := []int{100, 1000, 5000}

	for _, count := range termCounts {
		b.Run(fmt.Sprintf("Terms=%d", count), func(b *testing.B) {
			terms := make([]string, count)
			for i := 0; i < count; i++ {
				terms[i] = fmt.Sprintf("application_config_setting_%d", i)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				pc, _ := NewPrefixCompressor(16)
				_ = pc.Compress(terms)
			}
		})
	}
}

func BenchmarkPrefixDecompress(b *testing.B) {
	termCounts := []int{100, 1000, 5000}

	for _, count := range termCounts {
		b.Run(fmt.Sprintf("Terms=%d", count), func(b *testing.B) {
			terms := make([]string, count)
			for i := 0; i < count; i++ {
				terms[i] = fmt.Sprintf("application_config_setting_%d", i)
			}
			pc, _ := NewPrefixCompressor(16)
			blocks := pc.Compress(terms)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = pc.Decompress(blocks)
			}
		})
	}
}

func BenchmarkPrefixRatio(b *testing.B) {
	scenarios := []struct {
		name  string
		terms []string
	}{
		{
			name: "HighPrefix",
			terms: func() []string {
				t := make([]string, 1000)
				for i := 0; i < 1000; i++ {
					t[i] = fmt.Sprintf("very_long_common_prefix_setting_%d", i)
				}
				return t
			}(),
		},
		{
			name: "LowPrefix",
			terms: func() []string {
				t := make([]string, 1000)
				for i := 0; i < 1000; i++ {
					t[i] = fmt.Sprintf("x%d_unique", i)
				}
				return t
			}(),
		},
		{
			name: "Random",
			terms: func() []string {
				t := make([]string, 1000)
				for i := 0; i < 1000; i++ {
					t[i] = fmt.Sprintf("%c%d_term", 'a'+rune(i%26), i)
				}
				return t
			}(),
		},
	}

	for _, s := range scenarios {
		compressed, original, ratio := CompressionStats(s.terms)
		b.Run(s.name, func(b *testing.B) {
			b.ReportMetric(float64(original), "original_bytes")
			b.ReportMetric(float64(compressed), "compressed_bytes")
			b.ReportMetric(ratio*100, "ratio%")
		})
	}
}

// =============================================================================
// Scaling Benchmarks
// =============================================================================

func BenchmarkScaleRowGroups(b *testing.B) {
	sizes := []int{10, 100, 1000, 5000}

	for _, numRGs := range sizes {
		b.Run(fmt.Sprintf("RGs=%d", numRGs), func(b *testing.B) {
			idx := setupTestIndex(numRGs)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				idx.Evaluate([]Predicate{EQ("$.name", "user_42")})
			}
		})
	}
}

func BenchmarkScaleDocSize(b *testing.B) {
	fieldCounts := []int{5, 20, 50, 100}

	for _, numFields := range fieldCounts {
		b.Run(fmt.Sprintf("Fields=%d", numFields), func(b *testing.B) {
			docs := make([][]byte, 100)
			for i := 0; i < 100; i++ {
				docs[i] = generateLargeDoc(i, numFields)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				builder, _ := NewBuilder(DefaultConfig(), 100)
				for j := 0; j < 100; j++ {
					builder.AddDocument(DocID(j), docs[j])
				}
				_ = builder.Finalize()
			}
		})
	}
}

func BenchmarkScaleCardinality(b *testing.B) {
	cardinalities := []int{10, 100, 1000, 10000}

	for _, card := range cardinalities {
		b.Run(fmt.Sprintf("Cardinality=%d", card), func(b *testing.B) {
			docs := generateHighCardinalityDocs(1000, card)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				builder, _ := NewBuilder(DefaultConfig(), 1000)
				for j := 0; j < 1000; j++ {
					builder.AddDocument(DocID(j), docs[j])
				}
				_ = builder.Finalize()
			}
		})
	}
}

func BenchmarkScalePaths(b *testing.B) {
	depths := []int{2, 5, 10}

	for _, depth := range depths {
		b.Run(fmt.Sprintf("Depth=%d", depth), func(b *testing.B) {
			docs := make([][]byte, 100)
			for i := 0; i < 100; i++ {
				docs[i] = generateNestedDoc(i, depth)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				builder, _ := NewBuilder(DefaultConfig(), 100)
				for j := 0; j < 100; j++ {
					builder.AddDocument(DocID(j), docs[j])
				}
				_ = builder.Finalize()
			}
		})
	}
}

// =============================================================================
// End-to-End Benchmarks
// =============================================================================

func BenchmarkE2EBuildQuerySerialize(b *testing.B) {
	numRGs := 1000
	docs := make([][]byte, numRGs)
	for i := 0; i < numRGs; i++ {
		docs[i] = generateTestDoc(i)
	}

	b.Run("Build", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			builder, _ := NewBuilder(DefaultConfig(), numRGs)
			for j := 0; j < numRGs; j++ {
				builder.AddDocument(DocID(j), docs[j])
			}
			_ = builder.Finalize()
		}
	})

	idx := setupTestIndex(numRGs)

	b.Run("Query", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			idx.Evaluate([]Predicate{EQ("$.name", "user_42"), GTE("$.age", 30)})
		}
	})

	b.Run("Serialize", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			data, _ := Encode(idx)
			_, _ = Decode(data)
		}
	})
}

func BenchmarkRealWorldScenario(b *testing.B) {
	numRGs := 1000
	docs := make([][]byte, numRGs)
	for i := 0; i < numRGs; i++ {
		docs[i] = generateTestDocWithText(i)
	}

	builder, _ := NewBuilder(DefaultConfig(), numRGs)
	for j := 0; j < numRGs; j++ {
		builder.AddDocument(DocID(j), docs[j])
	}
	idx := builder.Finalize()

	data, _ := Encode(idx)
	decoded, _ := Decode(data)

	b.Run("EQ_Query", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			decoded.Evaluate([]Predicate{EQ("$.id", float64(500))})
		}
	})

	b.Run("Contains_Query", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			decoded.Evaluate([]Predicate{Contains("$.description", "quick")})
		}
	})

	b.Run("Combined_Query", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			decoded.Evaluate([]Predicate{
				Contains("$.description", "hello"),
				GTE("$.id", float64(100)),
			})
		}
	})
}
