package gin

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"sync"
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

const (
	phase06BenchmarkDocs         = 4096
	phase06BenchmarkRowGroups    = 4096
	phase06BenchmarkBasePaths    = 14
	phase06BenchmarkEQValue      = "svc-07"
	phase06BenchmarkContainsText = "timeout shard-03"
	phase06BenchmarkRegexPattern = "timeout shard-(03|17)"
	phase06BenchmarkEQMatches    = 64
	phase06BenchmarkTextMatches  = 256
	phase06BenchmarkRegexMatches = 512
	phase06BenchmarkEQLabel      = "64of4096"
	phase06BenchmarkTextLabel    = "256of4096"
	phase06BenchmarkRegexLabel   = "512of4096"
)

var (
	phase06BenchmarkWidthTiers = []int{16, 128, 512, 2048}
	phase06ServicePathVariants = []struct {
		name string
		path string
	}{
		{name: "canonical", path: "$.service"},
		{name: "single-quoted", path: "$['service']"},
		{name: "double-quoted", path: "$[\"service\"]"},
	}
	phase06MessagePathVariants = []struct {
		name string
		path string
	}{
		{name: "canonical", path: "$.message"},
		{name: "single-quoted", path: "$['message']"},
		{name: "double-quoted", path: "$[\"message\"]"},
	}
	phase06WideIndexCache   = make(map[int]*GINIndex)
	phase06WideIndexCacheMu sync.Mutex

	phase07BenchmarkDocCounts = []int{100, 1000, 10000}
	phase07BenchmarkShapes    = []phase07BenchmarkShape{
		{
			name:      "int-only",
			buildDocs: generatePhase07IntDocs,
			newConfig: DefaultConfig,
		},
		{
			name:      "mixed-safe",
			buildDocs: generatePhase07MixedDocs,
			newConfig: DefaultConfig,
		},
		{
			name:      "wide-flat",
			buildDocs: generatePhase07WideDocs,
			newConfig: DefaultConfig,
		},
		{
			name:      "transformer-heavy",
			buildDocs: generatePhase07TransformerDocs,
			newConfig: newPhase07TransformerBenchmarkConfig,
		},
	}
)

type phase07BenchmarkShape struct {
	name      string
	buildDocs func(int) [][]byte
	newConfig func() GINConfig
}

func generatePhase06WideLogDoc(i int, width int) []byte {
	if width < phase06BenchmarkBasePaths {
		panic(fmt.Sprintf("phase 06 width %d must be >= %d", width, phase06BenchmarkBasePaths))
	}

	service := fmt.Sprintf("svc-%02d", i%64)
	if i%64 == 7 {
		service = phase06BenchmarkEQValue
	}

	message := fmt.Sprintf("ok request shard-%02d", i%64)
	switch i % 16 {
	case 3:
		message = "timeout shard-03 during downstream read"
	case 7:
		message = "timeout shard-17 during downstream read"
	}

	doc := map[string]any{
		"service":    service,
		"method":     []string{"GET", "POST", "PUT", "DELETE"}[i%4],
		"status":     []string{"ok", "warn", "error"}[i%3],
		"message":    message,
		"request_id": fmt.Sprintf("req-%04d", i),
		"host":       fmt.Sprintf("api-%02d.internal", i%32),
		"region":     []string{"eu-central-1", "us-east-1", "ap-southeast-1"}[i%3],
		"http": map[string]any{
			"path":   fmt.Sprintf("/v1/resource/%02d", i%32),
			"status": 200 + (i % 5),
		},
		"labels": map[string]any{
			"env":  []string{"prod", "staging", "dev"}[i%3],
			"team": []string{"core", "search", "platform", "data"}[i%4],
		},
	}

	for extra := 0; extra < width-phase06BenchmarkBasePaths; extra++ {
		doc[fmt.Sprintf("extra_%04d", extra)] = fmt.Sprintf("extra-value-%02d", extra%8)
	}

	data, _ := json.Marshal(doc)
	return data
}

func setupPhase06WideIndex(width int) *GINIndex {
	phase06WideIndexCacheMu.Lock()
	if idx, ok := phase06WideIndexCache[width]; ok {
		phase06WideIndexCacheMu.Unlock()
		return idx
	}
	phase06WideIndexCacheMu.Unlock()

	builder, _ := NewBuilder(DefaultConfig(), phase06BenchmarkRowGroups)
	for i := 0; i < phase06BenchmarkDocs; i++ {
		builder.AddDocument(DocID(i), generatePhase06WideLogDoc(i, width))
	}
	idx := builder.Finalize()
	if len(idx.PathDirectory) != width {
		panic(fmt.Sprintf("phase 06 fixture width mismatch: got %d paths, want %d", len(idx.PathDirectory), width))
	}

	phase06WideIndexCacheMu.Lock()
	defer phase06WideIndexCacheMu.Unlock()
	if cached, ok := phase06WideIndexCache[width]; ok {
		return cached
	}
	phase06WideIndexCache[width] = idx
	return idx
}

func benchmarkPhase06Predicate(b *testing.B, width int, spelling string, pred Predicate, expectedMatches int) {
	idx := setupPhase06WideIndex(width)
	result := idx.Evaluate([]Predicate{pred})
	if result.Count() != expectedMatches {
		b.Fatalf("unexpected selectivity for %s: got %d want %d", spelling, result.Count(), expectedMatches)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Evaluate([]Predicate{pred})
	}
}

func generatePhase07IntDocs(n int) [][]byte {
	docs := make([][]byte, n)
	for i := 0; i < n; i++ {
		doc := map[string]any{
			"id":           9223372036854775807 - int64(i%32),
			"account_id":   int64(900000000000000000 + i),
			"count":        int64(i),
			"score_bucket": int64((i % 16) * 10),
		}
		data, _ := json.Marshal(doc)
		docs[i] = data
	}
	return docs
}

func generatePhase07MixedDocs(n int) [][]byte {
	docs := make([][]byte, n)
	for i := 0; i < n; i++ {
		doc := map[string]any{
			"sensor_id":     fmt.Sprintf("sensor-%04d", i),
			"reading_int":   int64(9007199254740000 + i),
			"reading_float": 1.5 + float64(i%10)/10,
		}
		data, _ := json.Marshal(doc)
		docs[i] = data
	}
	return docs
}

func generatePhase07WideDocs(n int) [][]byte {
	docs := make([][]byte, n)
	for i := 0; i < n; i++ {
		doc := make(map[string]any, 514)
		doc["id"] = 9223372036854775807 - int64(i%32)
		doc["account_id"] = int64(900000000000000000 + i)
		for field := 0; field < 512; field++ {
			doc[fmt.Sprintf("field_%04d", field)] = fmt.Sprintf("value_%04d_%04d", field, i%17)
		}
		data, _ := json.Marshal(doc)
		docs[i] = data
	}
	return docs
}

func generatePhase07TransformerDocs(n int) [][]byte {
	docs := make([][]byte, n)
	for i := 0; i < n; i++ {
		doc := map[string]any{
			"timestamp":  fmt.Sprintf("2024-01-%02dT10:30:00Z", 1+i%28),
			"event_date": fmt.Sprintf("2024-02-%02d", 1+i%28),
			"version":    fmt.Sprintf("v2.%d.%d", i%10, i%100),
			"client_ip":  fmt.Sprintf("192.168.%d.%d", i%16, (i%200)+1),
			"build_ref":  fmt.Sprintf("build-%d", 100000+i),
		}
		data, _ := json.Marshal(doc)
		docs[i] = data
	}
	return docs
}

func newPhase07TransformerBenchmarkConfig() GINConfig {
	cfg, err := NewConfig(
		WithISODateTransformer("$.timestamp"),
		WithDateTransformer("$.event_date"),
		WithSemVerTransformer("$.version"),
		WithIPv4Transformer("$.client_ip"),
		WithRegexExtractIntTransformer("$.build_ref", `build-(\d+)`, 1),
	)
	if err != nil {
		panic(err)
	}
	return cfg
}

// KEEP IN SYNC WITH pre-Phase-07 AddDocument control path for BUILD-05
func benchmarkAddDocumentLegacy(builder *GINBuilder, docID DocID, jsonDoc []byte) error {
	pos, exists := builder.docIDToPos[docID]
	if !exists {
		pos = builder.nextPos
		if pos >= builder.numRGs {
			return fmt.Errorf("position %d exceeds numRGs %d", pos, builder.numRGs)
		}
		builder.docIDToPos[docID] = pos
		builder.posToDocID = append(builder.posToDocID, docID)
		builder.nextPos++
	}

	if pos > builder.maxRGID {
		builder.maxRGID = pos
	}
	builder.numDocs++

	var doc any
	if err := json.Unmarshal(jsonDoc, &doc); err != nil {
		return err
	}

	benchmarkWalkJSONLegacy(builder, "$", doc, pos)
	return nil
}

func benchmarkWalkJSONLegacy(builder *GINBuilder, path string, value any, rgID int) {
	canonicalPath := normalizeWalkPath(path)

	if builder.config.fieldTransformers != nil {
		if transformer, ok := builder.config.fieldTransformers[canonicalPath]; ok {
			if transformed, ok := transformer(value); ok {
				value = transformed
			}
		}
	}

	pd := builder.getOrCreatePath(canonicalPath)
	pd.presentRGs.Set(rgID)

	switch v := value.(type) {
	case nil:
		pd.observedTypes |= TypeNull
		pd.nullRGs.Set(rgID)
	case bool:
		pd.observedTypes |= TypeBool
		builder.addStringTerm(pd, strconv.FormatBool(v), rgID, canonicalPath)
	case float64:
		if v == math.Trunc(v) && v >= math.MinInt64 && v <= math.MaxInt64 {
			pd.observedTypes |= TypeInt
		} else {
			pd.observedTypes |= TypeFloat
		}
		benchmarkAddNumericValueLegacy(builder, pd, v, rgID, canonicalPath)
	case string:
		pd.observedTypes |= TypeString
		builder.addStringTerm(pd, v, rgID, canonicalPath)
	case []any:
		for i, item := range v {
			benchmarkWalkJSONLegacy(builder, fmt.Sprintf("%s[%d]", path, i), item, rgID)
		}
		for _, item := range v {
			benchmarkWalkJSONLegacy(builder, path+"[*]", item, rgID)
		}
	case map[string]any:
		for key, item := range v {
			benchmarkWalkJSONLegacy(builder, path+"."+key, item, rgID)
		}
	}
}

func benchmarkAddNumericValueLegacy(builder *GINBuilder, pd *pathBuildData, val float64, rgID int, path string) {
	if !pd.hasNumericValues {
		pd.hasNumericValues = true
		pd.numericValueType = NumericValueTypeFloatMixed
		pd.floatGlobalMin = val
		pd.floatGlobalMax = val
	} else {
		if val < pd.floatGlobalMin {
			pd.floatGlobalMin = val
		}
		if val > pd.floatGlobalMax {
			pd.floatGlobalMax = val
		}
	}

	stat, ok := pd.numericStats[rgID]
	if !ok {
		pd.numericStats[rgID] = &RGNumericStat{
			Min:      val,
			Max:      val,
			HasValue: true,
		}
	} else {
		if val < stat.Min {
			stat.Min = val
		}
		if val > stat.Max {
			stat.Max = val
		}
	}

	builder.bloom.AddString(path + "=" + strconv.FormatFloat(val, 'f', -1, 64))
}

func benchmarkAddDocumentLegacyReference(builder *GINBuilder, docID DocID, jsonDoc []byte) error {
	pos, exists := builder.docIDToPos[docID]
	if !exists {
		pos = builder.nextPos
		if pos >= builder.numRGs {
			return fmt.Errorf("position %d exceeds numRGs %d", pos, builder.numRGs)
		}
		builder.docIDToPos[docID] = pos
		builder.posToDocID = append(builder.posToDocID, docID)
		builder.nextPos++
	}

	if pos > builder.maxRGID {
		builder.maxRGID = pos
	}
	builder.numDocs++

	var doc any
	if err := json.Unmarshal(jsonDoc, &doc); err != nil {
		return err
	}

	benchmarkWalkJSONLegacyReference(builder, "$", doc, pos)
	return nil
}

func benchmarkWalkJSONLegacyReference(builder *GINBuilder, path string, value any, rgID int) {
	canonicalPath := normalizeWalkPath(path)

	if builder.config.fieldTransformers != nil {
		if transformer, ok := builder.config.fieldTransformers[canonicalPath]; ok {
			if transformed, ok := transformer(value); ok {
				value = transformed
			}
		}
	}

	pd := builder.getOrCreatePath(canonicalPath)
	pd.presentRGs.Set(rgID)

	switch v := value.(type) {
	case nil:
		pd.observedTypes |= TypeNull
		pd.nullRGs.Set(rgID)
	case bool:
		pd.observedTypes |= TypeBool
		builder.addStringTerm(pd, strconv.FormatBool(v), rgID, canonicalPath)
	case float64:
		if v == math.Trunc(v) && v >= math.MinInt64 && v <= math.MaxInt64 {
			pd.observedTypes |= TypeInt
		} else {
			pd.observedTypes |= TypeFloat
		}
		benchmarkAddNumericValueLegacyReference(builder, pd, v, rgID, canonicalPath)
	case string:
		pd.observedTypes |= TypeString
		builder.addStringTerm(pd, v, rgID, canonicalPath)
	case []any:
		for i, item := range v {
			benchmarkWalkJSONLegacyReference(builder, fmt.Sprintf("%s[%d]", path, i), item, rgID)
		}
		for _, item := range v {
			benchmarkWalkJSONLegacyReference(builder, path+"[*]", item, rgID)
		}
	case map[string]any:
		for key, item := range v {
			benchmarkWalkJSONLegacyReference(builder, path+"."+key, item, rgID)
		}
	}
}

func benchmarkAddNumericValueLegacyReference(builder *GINBuilder, pd *pathBuildData, val float64, rgID int, path string) {
	if !pd.hasNumericValues {
		pd.hasNumericValues = true
		pd.numericValueType = NumericValueTypeFloatMixed
		pd.floatGlobalMin = val
		pd.floatGlobalMax = val
	} else {
		if val < pd.floatGlobalMin {
			pd.floatGlobalMin = val
		}
		if val > pd.floatGlobalMax {
			pd.floatGlobalMax = val
		}
	}

	stat, ok := pd.numericStats[rgID]
	if !ok {
		pd.numericStats[rgID] = &RGNumericStat{
			Min:      val,
			Max:      val,
			HasValue: true,
		}
	} else {
		if val < stat.Min {
			stat.Min = val
		}
		if val > stat.Max {
			stat.Max = val
		}
	}

	builder.bloom.AddString(path + "=" + strconv.FormatFloat(val, 'f', -1, 64))
}

func benchmarkAddDocumentExplicit(builder *GINBuilder, docID DocID, jsonDoc []byte) error {
	return builder.AddDocument(docID, jsonDoc)
}

func benchmarkPhase07BuildIndex(docs [][]byte, config GINConfig, useLegacy bool) *GINIndex {
	builder, _ := NewBuilder(config, len(docs))
	for i, doc := range docs {
		if useLegacy {
			_ = benchmarkAddDocumentLegacy(builder, DocID(i), doc)
			continue
		}
		_ = benchmarkAddDocumentExplicit(builder, DocID(i), doc)
	}
	return builder.Finalize()
}

func TestBenchmarkAddDocumentLegacyMatchesReferenceNumericPath(t *testing.T) {
	tests := []struct {
		name       string
		docs       [][]byte
		config     GINConfig
		targetPath string
	}{
		{
			name:       "int-only",
			docs:       generatePhase07IntDocs(4),
			config:     DefaultConfig(),
			targetPath: "$.count",
		},
		{
			name:       "transformer-heavy",
			docs:       generatePhase07TransformerDocs(4),
			config:     newPhase07TransformerBenchmarkConfig(),
			targetPath: "$.build_ref",
		},
	}

	assertNumericIndexEqual := func(t *testing.T, label string, got, want *NumericIndex) {
		t.Helper()

		if got == nil || want == nil {
			t.Fatalf("%s: numeric index presence mismatch: got=%v want=%v", label, got != nil, want != nil)
		}
		if got.ValueType != want.ValueType {
			t.Fatalf("%s: ValueType = %v, want %v", label, got.ValueType, want.ValueType)
		}
		if got.IntGlobalMin != want.IntGlobalMin || got.IntGlobalMax != want.IntGlobalMax {
			t.Fatalf("%s: int globals = [%d,%d], want [%d,%d]", label, got.IntGlobalMin, got.IntGlobalMax, want.IntGlobalMin, want.IntGlobalMax)
		}
		if got.GlobalMin != want.GlobalMin || got.GlobalMax != want.GlobalMax {
			t.Fatalf("%s: float globals = [%v,%v], want [%v,%v]", label, got.GlobalMin, got.GlobalMax, want.GlobalMin, want.GlobalMax)
		}
		if len(got.RGStats) != len(want.RGStats) {
			t.Fatalf("%s: len(RGStats) = %d, want %d", label, len(got.RGStats), len(want.RGStats))
		}
		for i := range got.RGStats {
			if got.RGStats[i] != want.RGStats[i] {
				t.Fatalf("%s: RGStats[%d] = %+v, want %+v", label, i, got.RGStats[i], want.RGStats[i])
			}
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			legacyBuilder, err := NewBuilder(tt.config, len(tt.docs))
			if err != nil {
				t.Fatalf("NewBuilder() error = %v", err)
			}
			referenceBuilder, err := NewBuilder(tt.config, len(tt.docs))
			if err != nil {
				t.Fatalf("NewBuilder() error = %v", err)
			}

			for i, doc := range tt.docs {
				if err := benchmarkAddDocumentLegacy(legacyBuilder, DocID(i), doc); err != nil {
					t.Fatalf("benchmarkAddDocumentLegacy(%d) error = %v", i, err)
				}
				if err := benchmarkAddDocumentLegacyReference(referenceBuilder, DocID(i), doc); err != nil {
					t.Fatalf("benchmarkAddDocumentLegacyReference(%d) error = %v", i, err)
				}
			}

			legacyIdx := legacyBuilder.Finalize()
			referenceIdx := referenceBuilder.Finalize()

			legacyPathID, ok := legacyIdx.pathLookup[tt.targetPath]
			if !ok {
				t.Fatalf("legacy pathLookup missing %s", tt.targetPath)
			}
			referencePathID, ok := referenceIdx.pathLookup[tt.targetPath]
			if !ok {
				t.Fatalf("reference pathLookup missing %s", tt.targetPath)
			}

			assertNumericIndexEqual(
				t,
				tt.targetPath,
				legacyIdx.NumericIndexes[legacyPathID],
				referenceIdx.NumericIndexes[referencePathID],
			)
		})
	}
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

func BenchmarkAddDocumentPhase07(b *testing.B) {
	parserModes := []struct {
		name      string
		useLegacy bool
	}{
		{name: "legacy-unmarshal", useLegacy: true},
		{name: "explicit-number", useLegacy: false},
	}

	for _, shape := range phase07BenchmarkShapes {
		for _, docCount := range phase07BenchmarkDocCounts {
			docs := shape.buildDocs(docCount)
			for _, parserMode := range parserModes {
				name := fmt.Sprintf("parser=%s/docs=%d/shape=%s", parserMode.name, docCount, shape.name)
				b.Run(name, func(b *testing.B) {
					b.ReportAllocs()
					doc := docs[0]
					config := shape.newConfig()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						builder, _ := NewBuilder(config, docCount)
						if parserMode.useLegacy {
							_ = benchmarkAddDocumentLegacy(builder, 0, doc)
						} else {
							_ = benchmarkAddDocumentExplicit(builder, 0, doc)
						}
					}
				})
			}
		}
	}
}

func BenchmarkBuildPhase07(b *testing.B) {
	parserModes := []struct {
		name      string
		useLegacy bool
	}{
		{name: "legacy-unmarshal", useLegacy: true},
		{name: "explicit-number", useLegacy: false},
	}

	for _, shape := range phase07BenchmarkShapes {
		for _, docCount := range phase07BenchmarkDocCounts {
			docs := shape.buildDocs(docCount)
			for _, parserMode := range parserModes {
				name := fmt.Sprintf("parser=%s/docs=%d/shape=%s", parserMode.name, docCount, shape.name)
				b.Run(name, func(b *testing.B) {
					b.ReportAllocs()
					config := shape.newConfig()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						_ = benchmarkPhase07BuildIndex(docs, config, parserMode.useLegacy)
					}
				})
			}
		}
	}
}

func BenchmarkFinalizePhase07(b *testing.B) {
	parserModes := []struct {
		name      string
		useLegacy bool
	}{
		{name: "legacy-unmarshal", useLegacy: true},
		{name: "explicit-number", useLegacy: false},
	}

	for _, shape := range phase07BenchmarkShapes {
		for _, docCount := range phase07BenchmarkDocCounts {
			docs := shape.buildDocs(docCount)
			for _, parserMode := range parserModes {
				name := fmt.Sprintf("parser=%s/docs=%d/shape=%s", parserMode.name, docCount, shape.name)
				b.Run(name, func(b *testing.B) {
					b.ReportAllocs()
					config := shape.newConfig()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						b.StopTimer()
						builder, _ := NewBuilder(config, len(docs))
						for docID, doc := range docs {
							if parserMode.useLegacy {
								_ = benchmarkAddDocumentLegacy(builder, DocID(docID), doc)
							} else {
								_ = benchmarkAddDocumentExplicit(builder, DocID(docID), doc)
							}
						}
						b.StartTimer()
						_ = builder.Finalize()
					}
				})
			}
		}
	}
}

// =============================================================================
// Query Performance Benchmarks
// =============================================================================

func BenchmarkQueryEQ(b *testing.B) {
	for _, width := range phase06BenchmarkWidthTiers {
		for _, variant := range phase06ServicePathVariants {
			name := fmt.Sprintf("paths=%d/spelling=%s/selectivity=%s", width, variant.name, phase06BenchmarkEQLabel)
			b.Run(name, func(b *testing.B) {
				benchmarkPhase06Predicate(
					b,
					width,
					variant.path,
					EQ(variant.path, phase06BenchmarkEQValue),
					phase06BenchmarkEQMatches,
				)
			})
		}
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
	for _, width := range phase06BenchmarkWidthTiers {
		for _, variant := range phase06MessagePathVariants {
			name := fmt.Sprintf("paths=%d/spelling=%s/selectivity=%s", width, variant.name, phase06BenchmarkTextLabel)
			b.Run(name, func(b *testing.B) {
				benchmarkPhase06Predicate(
					b,
					width,
					variant.path,
					Contains(variant.path, phase06BenchmarkContainsText),
					phase06BenchmarkTextMatches,
				)
			})
		}
	}
}

func BenchmarkQueryRegex(b *testing.B) {
	for _, width := range phase06BenchmarkWidthTiers {
		for _, variant := range phase06MessagePathVariants {
			name := fmt.Sprintf("paths=%d/spelling=%s/selectivity=%s", width, variant.name, phase06BenchmarkRegexLabel)
			b.Run(name, func(b *testing.B) {
				benchmarkPhase06Predicate(
					b,
					width,
					variant.path,
					Regex(variant.path, phase06BenchmarkRegexPattern),
					phase06BenchmarkRegexMatches,
				)
			})
		}
	}
}

func BenchmarkPathLookup(b *testing.B) {
	for _, width := range phase06BenchmarkWidthTiers {
		idx := setupPhase06WideIndex(width)
		for _, variant := range phase06ServicePathVariants {
			name := fmt.Sprintf("paths=%d/spelling=%s", width, variant.name)
			b.Run(name, func(b *testing.B) {
				pathID, entry := idx.findPath(variant.path)
				if pathID < 0 || entry == nil {
					b.Fatalf("path lookup failed for %s", variant.path)
				}

				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					pathID, entry = idx.findPath(variant.path)
					if pathID < 0 || entry == nil {
						b.Fatalf("path lookup failed for %s", variant.path)
					}
				}
			})
		}
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

// =============================================================================
// Compression Level Benchmarks
// =============================================================================

func BenchmarkCompressionLevels(b *testing.B) {
	sizes := []int{100, 1000, 5000}

	levels := []struct {
		name  string
		level CompressionLevel
	}{
		{"None", CompressionNone},
		{"Zstd-1", CompressionFastest},
		{"Zstd-3", CompressionBalanced},
		{"Zstd-9", CompressionBetter},
		{"Zstd-15", CompressionBest},
		{"Zstd-19", CompressionMax},
	}

	for _, size := range sizes {
		idx := setupTestIndex(size)

		for _, l := range levels {
			b.Run(fmt.Sprintf("Encode/RGs=%d/%s", size, l.name), func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, _ = EncodeWithLevel(idx, l.level)
				}
			})
		}

		// Pre-encode for decode and size benchmarks
		encoded := make(map[string][]byte)
		for _, l := range levels {
			encoded[l.name], _ = EncodeWithLevel(idx, l.level)
		}

		for _, l := range levels {
			data := encoded[l.name]
			b.Run(fmt.Sprintf("Decode/RGs=%d/%s", size, l.name), func(b *testing.B) {
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, _ = Decode(data)
				}
			})
		}

		// Report sizes
		noneSize := len(encoded["None"])
		for _, l := range levels {
			data := encoded[l.name]
			b.Run(fmt.Sprintf("Size/RGs=%d/%s", size, l.name), func(b *testing.B) {
				b.ReportMetric(float64(len(data)), "bytes")
				b.ReportMetric(float64(len(data))/1024, "KB")
				if noneSize > 0 {
					b.ReportMetric(float64(len(data))/float64(noneSize)*100, "ratio%")
				}
			})
		}
	}
}

// =============================================================================
// Worst-Case Composite Query Benchmarks
// =============================================================================

func generateWorstCaseDoc(i int) []byte {
	doc := map[string]any{
		"id":          i,
		"name":        fmt.Sprintf("user_%d", i%100),
		"email":       fmt.Sprintf("user%d@example.com", i),
		"age":         20 + (i % 50),
		"score":       float64(i%1000) / 10.0,
		"active":      i%2 == 0,
		"status":      []string{"active", "pending", "inactive", "suspended"}[i%4],
		"description": fmt.Sprintf("This is a detailed description for user %d with various keywords like error_code_%d and warning_level_%d", i, i%50, i%20),
		"tags":        []string{fmt.Sprintf("tag_%d", i%20), fmt.Sprintf("category_%d", i%10), fmt.Sprintf("group_%d", i%5)},
		"metadata": map[string]any{
			"created": fmt.Sprintf("2024-01-%02d", (i%28)+1),
			"version": fmt.Sprintf("v%d.%d.%d", i%5, i%10, i%100),
		},
		"nullable_field": func() any {
			if i%3 == 0 {
				return nil
			}
			return fmt.Sprintf("value_%d", i)
		}(),
	}
	data, _ := json.Marshal(doc)
	return data
}

func setupWorstCaseIndex(numRGs int) *GINIndex {
	builder, _ := NewBuilder(DefaultConfig(), numRGs)
	for i := 0; i < numRGs; i++ {
		builder.AddDocument(DocID(i), generateWorstCaseDoc(i))
	}
	return builder.Finalize()
}

func BenchmarkCompositeWorstCase(b *testing.B) {
	idx := setupWorstCaseIndex(1000)

	b.Run("EQ+Range+Contains", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			idx.Evaluate([]Predicate{
				EQ("$.status", "active"),
				GTE("$.age", 25),
				LTE("$.age", 45),
				Contains("$.description", "error_code"),
			})
		}
	})

	b.Run("EQ+Range+Contains+IN", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			idx.Evaluate([]Predicate{
				EQ("$.active", true),
				GTE("$.score", 20.0),
				LT("$.score", 80.0),
				Contains("$.description", "warning"),
				IN("$.status", "active", "pending", "suspended"),
			})
		}
	})

	b.Run("EQ+Range+Contains+Null", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			idx.Evaluate([]Predicate{
				EQ("$.name", "user_42"),
				GT("$.age", 30),
				Contains("$.email", "example"),
				IsNotNull("$.nullable_field"),
			})
		}
	})

	b.Run("AllOperatorTypes", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			idx.Evaluate([]Predicate{
				EQ("$.status", "active"),
				NE("$.name", "user_0"),
				GTE("$.age", 25),
				LTE("$.score", 50.0),
				Contains("$.description", "user"),
				IN("$.tags[*]", "tag_1", "tag_5", "tag_10"),
				IsNotNull("$.nullable_field"),
			})
		}
	})

	b.Run("Regex+Range+EQ", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			idx.Evaluate([]Predicate{
				Regex("$.description", "error_code_[0-9]+"),
				GTE("$.age", 20),
				LTE("$.age", 40),
				EQ("$.active", true),
			})
		}
	})

	b.Run("LargeIN+Range", func(b *testing.B) {
		// IN with 20 values
		values := make([]any, 20)
		for i := 0; i < 20; i++ {
			values[i] = fmt.Sprintf("user_%d", i*5)
		}
		for i := 0; i < b.N; i++ {
			idx.Evaluate([]Predicate{
				IN("$.name", values...),
				GTE("$.age", 25),
				LTE("$.age", 45),
			})
		}
	})

	b.Run("MultipleContains", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			idx.Evaluate([]Predicate{
				Contains("$.description", "detailed"),
				Contains("$.description", "keywords"),
				Contains("$.email", "user"),
			})
		}
	})

	b.Run("NegationHeavy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			idx.Evaluate([]Predicate{
				NE("$.status", "inactive"),
				NE("$.name", "user_0"),
				NIN("$.tags[*]", "tag_0", "tag_19"),
				IsNotNull("$.nullable_field"),
			})
		}
	})

	b.Run("HighSelectivity", func(b *testing.B) {
		// Query that matches most row groups (worst case for bitmap ops)
		for i := 0; i < b.N; i++ {
			idx.Evaluate([]Predicate{
				GTE("$.age", 20), // matches all
				LTE("$.age", 69), // matches all
				IsNotNull("$.name"),
			})
		}
	})

	b.Run("LowSelectivity", func(b *testing.B) {
		// Query that matches very few row groups
		for i := 0; i < b.N; i++ {
			idx.Evaluate([]Predicate{
				EQ("$.name", "user_42"),
				EQ("$.age", 42),
				EQ("$.status", "active"),
				Contains("$.description", "user 42"),
			})
		}
	})
}

func BenchmarkCompositeVsPredicateCount(b *testing.B) {
	idx := setupWorstCaseIndex(1000)

	// Test scaling with predicate count using mixed operator types
	predicateSets := []struct {
		name  string
		preds []Predicate
	}{
		{"2_Mixed", []Predicate{
			EQ("$.status", "active"),
			GTE("$.age", 30),
		}},
		{"3_Mixed", []Predicate{
			EQ("$.status", "active"),
			GTE("$.age", 30),
			Contains("$.description", "error"),
		}},
		{"4_Mixed", []Predicate{
			EQ("$.status", "active"),
			GTE("$.age", 30),
			LTE("$.score", 50.0),
			Contains("$.description", "error"),
		}},
		{"5_Mixed", []Predicate{
			EQ("$.status", "active"),
			GTE("$.age", 30),
			LTE("$.score", 50.0),
			Contains("$.description", "error"),
			IsNotNull("$.nullable_field"),
		}},
		{"6_Mixed", []Predicate{
			EQ("$.status", "active"),
			GTE("$.age", 30),
			LTE("$.score", 50.0),
			Contains("$.description", "error"),
			IsNotNull("$.nullable_field"),
			IN("$.tags[*]", "tag_1", "tag_5"),
		}},
		{"8_Mixed", []Predicate{
			EQ("$.status", "active"),
			NE("$.name", "user_0"),
			GTE("$.age", 25),
			LTE("$.age", 45),
			GTE("$.score", 10.0),
			LTE("$.score", 80.0),
			Contains("$.description", "user"),
			IsNotNull("$.nullable_field"),
		}},
	}

	for _, ps := range predicateSets {
		b.Run(ps.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				idx.Evaluate(ps.preds)
			}
		})
	}
}

func BenchmarkCompositeVsIndexSize(b *testing.B) {
	sizes := []int{100, 500, 1000, 5000}

	preds := []Predicate{
		EQ("$.status", "active"),
		GTE("$.age", 25),
		LTE("$.score", 50.0),
		Contains("$.description", "error"),
		IsNotNull("$.nullable_field"),
	}

	for _, size := range sizes {
		idx := setupWorstCaseIndex(size)
		b.Run(fmt.Sprintf("RGs=%d", size), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				idx.Evaluate(preds)
			}
		})
	}
}

// =============================================================================
// Realistic Row Group Benchmarks (many rows per RG)
// =============================================================================

func BenchmarkRealisticRowGroups(b *testing.B) {
	// Simulate realistic Parquet scenario: many rows per row group
	rowsPerRG := []int{1000, 10000, 50000}
	numRGs := 20

	for _, rows := range rowsPerRG {
		b.Run(fmt.Sprintf("Build/RGs=%d/RowsPerRG=%d", numRGs, rows), func(b *testing.B) {
			docs := make([][]byte, rows)
			for i := 0; i < rows; i++ {
				docs[i] = generateWorstCaseDoc(i)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				builder, _ := NewBuilder(DefaultConfig(), numRGs)
				for rg := 0; rg < numRGs; rg++ {
					for _, doc := range docs {
						builder.AddDocument(DocID(rg), doc)
					}
				}
				builder.Finalize()
			}
		})
	}

	// Query performance with realistic index
	for _, rows := range rowsPerRG {
		// Build index once
		builder, _ := NewBuilder(DefaultConfig(), numRGs)
		for rg := 0; rg < numRGs; rg++ {
			for i := 0; i < rows; i++ {
				builder.AddDocument(DocID(rg), generateWorstCaseDoc(rg*rows+i))
			}
		}
		idx := builder.Finalize()

		totalRows := numRGs * rows

		b.Run(fmt.Sprintf("Query/Rows=%dk/RGs=%d", totalRows/1000, numRGs), func(b *testing.B) {
			preds := []Predicate{
				EQ("$.status", "active"),
				GTE("$.age", 25),
				Contains("$.description", "error"),
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				idx.Evaluate(preds)
			}
		})

		b.Run(fmt.Sprintf("IndexSize/Rows=%dk/RGs=%d", totalRows/1000, numRGs), func(b *testing.B) {
			data, _ := Encode(idx)
			b.ReportMetric(float64(len(data)), "bytes")
			b.ReportMetric(float64(len(data))/float64(numRGs), "bytes/RG")
			b.ReportMetric(float64(len(data))/float64(totalRows), "bytes/row")
		})
	}
}

func BenchmarkCompressionLevelsThroughput(b *testing.B) {
	idx := setupTestIndex(5000)

	levels := []struct {
		name  string
		level CompressionLevel
	}{
		{"None", CompressionNone},
		{"Zstd-1", CompressionFastest},
		{"Zstd-3", CompressionBalanced},
		{"Zstd-9", CompressionBetter},
		{"Zstd-15", CompressionBest},
		{"Zstd-19", CompressionMax},
	}

	for _, l := range levels {
		data, _ := EncodeWithLevel(idx, l.level)
		uncompressedData, _ := EncodeWithLevel(idx, CompressionNone)
		uncompressedSize := len(uncompressedData) - 4 // subtract magic bytes

		b.Run(fmt.Sprintf("EncodeMBps/%s", l.name), func(b *testing.B) {
			b.SetBytes(int64(uncompressedSize))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = EncodeWithLevel(idx, l.level)
			}
		})

		b.Run(fmt.Sprintf("DecodeMBps/%s", l.name), func(b *testing.B) {
			b.SetBytes(int64(uncompressedSize))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = Decode(data)
			}
		})
	}
}
