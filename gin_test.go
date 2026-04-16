package gin

import (
	"bytes"
	stderrors "errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"testing"
)

func mustNewBuilder(t *testing.T, config GINConfig, numRGs int) *GINBuilder {
	t.Helper()
	builder, err := NewBuilder(config, numRGs)
	if err != nil {
		t.Fatalf("failed to create builder: %v", err)
	}
	return builder
}

func outOfRangePathID(t *testing.T, idx *GINIndex) uint16 {
	t.Helper()
	if len(idx.PathDirectory) > int(^uint16(0)) {
		t.Fatalf("PathDirectory len = %d exceeds uint16 range", len(idx.PathDirectory))
	}
	return uint16(len(idx.PathDirectory))
}

func TestBloomFilter(t *testing.T) {
	bf := MustNewBloomFilter(1024, 3)
	bf.AddString("hello")
	bf.AddString("world")

	if !bf.MayContainString("hello") {
		t.Error("bloom filter should contain 'hello'")
	}
	if !bf.MayContainString("world") {
		t.Error("bloom filter should contain 'world'")
	}
	if bf.MayContainString("notpresent") {
		t.Log("bloom filter false positive (expected occasionally)")
	}
}

func TestRGSet(t *testing.T) {
	rs := MustNewRGSet(100)
	rs.Set(5)
	rs.Set(10)
	rs.Set(99)

	if !rs.IsSet(5) {
		t.Error("bit 5 should be set")
	}
	if !rs.IsSet(10) {
		t.Error("bit 10 should be set")
	}
	if !rs.IsSet(99) {
		t.Error("bit 99 should be set")
	}
	if rs.IsSet(0) {
		t.Error("bit 0 should not be set")
	}
	if rs.Count() != 3 {
		t.Errorf("expected count 3, got %d", rs.Count())
	}
}

func TestRGSetIntersect(t *testing.T) {
	a := MustNewRGSet(64)
	a.Set(1)
	a.Set(2)
	a.Set(3)

	b := MustNewRGSet(64)
	b.Set(2)
	b.Set(3)
	b.Set(4)

	result := a.Intersect(b)
	if result.Count() != 2 {
		t.Errorf("expected 2 bits set, got %d", result.Count())
	}
	if !result.IsSet(2) || !result.IsSet(3) {
		t.Error("bits 2 and 3 should be set")
	}
}

func TestRGSetUnion(t *testing.T) {
	a := MustNewRGSet(64)
	a.Set(1)
	a.Set(2)

	b := MustNewRGSet(64)
	b.Set(3)
	b.Set(4)

	result := a.Union(b)
	if result.Count() != 4 {
		t.Errorf("expected 4 bits set, got %d", result.Count())
	}
}

func TestBuilderSimple(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)

	docs := []struct {
		docID DocID
		json  string
	}{
		{0, `{"name": "alice", "age": 30}`},
		{0, `{"name": "bob", "age": 25}`},
		{1, `{"name": "charlie", "age": 35}`},
		{2, `{"name": "alice", "age": 40}`},
	}

	for _, d := range docs {
		if err := builder.AddDocument(d.docID, []byte(d.json)); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	idx := builder.Finalize()

	if idx.Header.NumDocs != 4 {
		t.Errorf("expected 4 docs, got %d", idx.Header.NumDocs)
	}
	if idx.Header.NumRowGroups != 3 {
		t.Errorf("expected 3 row groups, got %d", idx.Header.NumRowGroups)
	}
	if len(idx.PathDirectory) == 0 {
		t.Error("path directory should not be empty")
	}
}

func TestQueryEQ(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)

	builder.AddDocument(0, []byte(`{"name": "alice"}`))
	builder.AddDocument(1, []byte(`{"name": "bob"}`))
	builder.AddDocument(2, []byte(`{"name": "alice"}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{EQ("$.name", "alice")})
	rgs := result.ToSlice()

	if len(rgs) != 2 {
		t.Errorf("expected 2 matching RGs, got %d", len(rgs))
	}
	if !result.IsSet(0) || !result.IsSet(2) {
		t.Error("RG 0 and 2 should match")
	}
}

func TestQueryNumericRange(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 4)

	builder.AddDocument(0, []byte(`{"score": 10}`))
	builder.AddDocument(1, []byte(`{"score": 20}`))
	builder.AddDocument(2, []byte(`{"score": 30}`))
	builder.AddDocument(3, []byte(`{"score": 40}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{GT("$.score", 15)})
	if result.Count() != 3 {
		t.Errorf("expected 3 matching RGs for score > 15, got %d", result.Count())
	}

	result = idx.Evaluate([]Predicate{LTE("$.score", 20)})
	if result.Count() != 2 {
		t.Errorf("expected 2 matching RGs for score <= 20, got %d", result.Count())
	}
}

func TestQueryIN(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)

	builder.AddDocument(0, []byte(`{"status": "active"}`))
	builder.AddDocument(1, []byte(`{"status": "pending"}`))
	builder.AddDocument(2, []byte(`{"status": "inactive"}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{IN("$.status", "active", "pending")})
	if result.Count() != 2 {
		t.Errorf("expected 2 matching RGs, got %d", result.Count())
	}
}

func TestAdaptivePromotesHotTermsToExactBitmaps(t *testing.T) {
	hot := "h" + "ot"

	config := DefaultConfig()
	if config.AdaptiveMinRGCoverage != 2 {
		t.Fatalf("AdaptiveMinRGCoverage = %d, want 2", config.AdaptiveMinRGCoverage)
	}
	if config.AdaptivePromotedTermCap != 64 {
		t.Fatalf("AdaptivePromotedTermCap = %d, want 64", config.AdaptivePromotedTermCap)
	}
	if config.AdaptiveCoverageCeiling != 0.80 {
		t.Fatalf("AdaptiveCoverageCeiling = %v, want 0.80", config.AdaptiveCoverageCeiling)
	}
	if config.AdaptiveBucketCount != 128 {
		t.Fatalf("AdaptiveBucketCount = %d, want 128", config.AdaptiveBucketCount)
	}
	config.CardinalityThreshold = 4

	builder := mustNewBuilder(t, config, 8)
	for rgID := 0; rgID < 4; rgID++ {
		if err := builder.AddDocument(DocID(rgID), []byte(fmt.Sprintf(`{"field":"%s"}`, hot))); err != nil {
			t.Fatalf("AddDocument(hot, rg=%d) failed: %v", rgID, err)
		}
	}
	for rgID := 0; rgID < 8; rgID++ {
		doc := []byte(fmt.Sprintf(`{"field":"tail_%d"}`, rgID))
		if err := builder.AddDocument(DocID(rgID), doc); err != nil {
			t.Fatalf("AddDocument(tail, rg=%d) failed: %v", rgID, err)
		}
	}

	idx := builder.Finalize()
	entry := findPathEntry(idx, "$.field")
	if entry == nil {
		t.Fatal("expected $.field path entry")
	}
	if entry.Mode != PathModeAdaptiveHybrid {
		t.Fatalf("Mode = %v, want adaptive hybrid", entry.Mode)
	}
	if entry.AdaptivePromotedTerms != 1 {
		t.Fatalf("AdaptivePromotedTerms = %d, want 1", entry.AdaptivePromotedTerms)
	}
	if entry.AdaptiveBucketCount != 128 {
		t.Fatalf("AdaptiveBucketCount = %d, want 128", entry.AdaptiveBucketCount)
	}
	if _, ok := idx.StringIndexes[entry.PathID]; ok {
		t.Fatal("full StringIndex should be omitted for adaptive path")
	}

	adaptive, ok := idx.AdaptiveStringIndexes[entry.PathID]
	if !ok {
		t.Fatal("expected adaptive string index for high-cardinality path")
	}
	if len(adaptive.BucketRGBitmaps) != 128 {
		t.Fatalf("len(BucketRGBitmaps) = %d, want 128", len(adaptive.BucketRGBitmaps))
	}
	if len(adaptive.Terms) != 1 || adaptive.Terms[0] != hot {
		t.Fatalf("promoted terms = %v, want [hot]", adaptive.Terms)
	}
	if len(adaptive.RGBitmaps) != 1 {
		t.Fatalf("len(RGBitmaps) = %d, want 1", len(adaptive.RGBitmaps))
	}
	if got := adaptive.RGBitmaps[0].ToSlice(); fmt.Sprint(got) != fmt.Sprint([]int{0, 1, 2, 3}) {
		t.Fatalf("promoted bitmap = %v, want [0 1 2 3]", got)
	}
}

func TestNewBuilderAllowsLegacyConfigLiteralWhenAdaptiveDisabled(t *testing.T) {
	config := GINConfig{
		CardinalityThreshold: 128,
		BloomFilterSize:      1 << 20,
		BloomFilterHashes:    7,
		EnableTrigrams:       true,
		TrigramMinLength:     3,
		HLLPrecision:         12,
		PrefixBlockSize:      16,
	}

	if _, err := NewBuilder(config, 2); err != nil {
		t.Fatalf("NewBuilder() error = %v, want legacy struct literal to remain valid", err)
	}
}

func TestNewBuilderRejectsOversizedAdaptiveSettings(t *testing.T) {
	tests := []struct {
		name   string
		config func() GINConfig
	}{
		{
			name: "bucket count exceeds serialized limit",
			config: func() GINConfig {
				cfg := DefaultConfig()
				cfg.AdaptiveBucketCount = maxAdaptiveBucketsPerPath * 2
				return cfg
			},
		},
		{
			name: "promoted term cap exceeds path metadata limit",
			config: func() GINConfig {
				cfg := DefaultConfig()
				cfg.AdaptivePromotedTermCap = maxAdaptiveTermsPerPath + 1
				return cfg
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewBuilder(tt.config(), 2); err == nil {
				t.Fatal("NewBuilder() error = nil, want oversized adaptive setting rejection")
			}
		})
	}
}

func TestAddDocumentDoesNotLeakStagedPathsOnMergeError(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)

	if err := builder.AddDocument(0, []byte(`{"score":9007199254740993}`)); err != nil {
		t.Fatalf("AddDocument(seed) failed: %v", err)
	}

	err := builder.AddDocument(1, []byte(`{"alpha":"leak","score":1.5}`))
	if err == nil {
		t.Fatal("AddDocument(leaking doc) error = nil, want mixed numeric promotion failure")
	}

	idx := builder.Finalize()
	if entry := findPathEntry(idx, "$.alpha"); entry != nil {
		t.Fatalf("$.alpha leaked into finalized index with path id %d", entry.PathID)
	}
}

func TestAddDocumentRefusesAfterMergeFailurePoisonsBuilder(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)
	if err := builder.AddDocument(0, []byte(`{"name":"alice"}`)); err != nil {
		t.Fatalf("AddDocument(seed) failed: %v", err)
	}

	// Simulate a mid-loop mergeStagedPaths failure by poisoning the builder
	// directly. The natural trigger path (mixed numeric promotion) is caught
	// by validateStagedPaths' preview before mergeStagedPaths runs, so poison
	// is defensive; we still need to prove the refusal contract.
	builder.poisonErr = stderrors.New("simulated merge failure")

	err := builder.AddDocument(1, []byte(`{"name":"bob"}`))
	if err == nil {
		t.Fatal("AddDocument after poison = nil, want wrapped poison error")
	}
	if !strings.Contains(err.Error(), "builder poisoned") {
		t.Fatalf("AddDocument error = %q, want 'builder poisoned' context", err.Error())
	}
	if !strings.Contains(err.Error(), "simulated merge failure") {
		t.Fatalf("AddDocument error = %q, want original cause preserved", err.Error())
	}
}

func TestAdaptiveFallbackHasNoFalseNegatives(t *testing.T) {
	config := DefaultConfig()
	config.CardinalityThreshold = 1
	config.AdaptiveBucketCount = 4

	builder := mustNewBuilder(t, config, 6)
	for rgID := 0; rgID < 2; rgID++ {
		if err := builder.AddDocument(DocID(rgID), []byte(`{"field":"hot"}`)); err != nil {
			t.Fatalf("AddDocument(hot, rg=%d) failed: %v", rgID, err)
		}
	}
	for rgID := 2; rgID < 6; rgID++ {
		doc := []byte(fmt.Sprintf(`{"field":"tail_%d"}`, rgID))
		if err := builder.AddDocument(DocID(rgID), doc); err != nil {
			t.Fatalf("AddDocument(tail, rg=%d) failed: %v", rgID, err)
		}
	}

	idx := builder.Finalize()
	entry := findPathEntry(idx, "$.field")
	if entry == nil {
		t.Fatal("expected $.field path entry")
	}
	if entry.Mode != PathModeAdaptiveHybrid {
		t.Fatalf("Mode = %v, want adaptive hybrid", entry.Mode)
	}

	result := idx.Evaluate([]Predicate{EQ("$.field", "tail_2")})
	if !result.IsSet(2) {
		t.Fatalf("tail bucket result = %v, want RG 2 present", result.ToSlice())
	}
	if result.Count() <= 1 {
		t.Fatalf("tail bucket result = %v, want lossy bucket superset", result.ToSlice())
	}
	if result.Count() == 6 {
		t.Fatalf("tail bucket result = %v, should not degrade to AllRGs()", result.ToSlice())
	}
	if result.IsSet(0) || result.IsSet(1) {
		t.Fatalf("tail bucket result = %v, promoted-only RGs should not be in tail bucket", result.ToSlice())
	}
}

func TestAdaptiveNegativePredicatesStayConservative(t *testing.T) {
	config := DefaultConfig()
	config.CardinalityThreshold = 2
	config.AdaptiveBucketCount = 4

	builder := mustNewBuilder(t, config, 6)
	for rgID := 0; rgID < 2; rgID++ {
		if err := builder.AddDocument(DocID(rgID), []byte(`{"field":"hot"}`)); err != nil {
			t.Fatalf("AddDocument(hot, rg=%d) failed: %v", rgID, err)
		}
	}
	for rgID := 2; rgID < 6; rgID++ {
		doc := []byte(fmt.Sprintf(`{"field":"tail_%d"}`, rgID))
		if err := builder.AddDocument(DocID(rgID), doc); err != nil {
			t.Fatalf("AddDocument(tail, rg=%d) failed: %v", rgID, err)
		}
	}

	idx := builder.Finalize()

	if got := idx.Evaluate([]Predicate{NE("$.field", "tail_2")}).ToSlice(); fmt.Sprint(got) != fmt.Sprint([]int{0, 1, 2, 3, 4, 5}) {
		t.Fatalf("NE non-promoted result = %v, want all present RGs", got)
	}
	if got := idx.Evaluate([]Predicate{NIN("$.field", "tail_2", "tail_3")}).ToSlice(); fmt.Sprint(got) != fmt.Sprint([]int{0, 1, 2, 3, 4, 5}) {
		t.Fatalf("NIN non-promoted result = %v, want all present RGs", got)
	}
	if got := idx.Evaluate([]Predicate{NE("$.field", "hot")}).ToSlice(); fmt.Sprint(got) != fmt.Sprint([]int{2, 3, 4, 5}) {
		t.Fatalf("NE promoted result = %v, want exact promoted inversion", got)
	}
	if got := idx.Evaluate([]Predicate{NIN("$.field", "hot")}).ToSlice(); fmt.Sprint(got) != fmt.Sprint([]int{2, 3, 4, 5}) {
		t.Fatalf("NIN promoted result = %v, want exact promoted inversion", got)
	}
}

func TestFinalizeHighCardinalityNumericOnlyPathUsesBloomOnlyMode(t *testing.T) {
	config := DefaultConfig()
	config.CardinalityThreshold = 1

	builder := mustNewBuilder(t, config, 2)
	pd := builder.getOrCreatePath("$.score")
	pd.observedTypes = TypeInt
	pd.presentRGs.Set(0)
	pd.presentRGs.Set(1)
	pd.hll.AddString("10")
	pd.hll.AddString("20")

	idx := builder.Finalize()
	entry := findPathEntry(idx, "$.score")
	if entry == nil {
		t.Fatal("expected $.score path entry")
	}
	if entry.Mode != PathModeBloomOnly {
		t.Fatalf("Mode = %v, want bloom-only for non-adaptive high-cardinality path", entry.Mode)
	}
	if entry.AdaptiveBucketCount != 0 {
		t.Fatalf("AdaptiveBucketCount = %d, want 0", entry.AdaptiveBucketCount)
	}
	if _, ok := idx.AdaptiveStringIndexes[entry.PathID]; ok {
		t.Fatal("unexpected adaptive string index for numeric-only path")
	}
}

func TestAdaptiveINVariants(t *testing.T) {
	config := DefaultConfig()
	config.CardinalityThreshold = 1
	config.AdaptiveMinRGCoverage = 2
	config.AdaptivePromotedTermCap = 2
	config.AdaptiveCoverageCeiling = 0.80
	config.AdaptiveBucketCount = 4

	builder := mustNewBuilder(t, config, 6)
	docs := []string{
		`{"field":"hot","flag":true}`,
		`{"field":"hot","flag":true}`,
		`{"field":"tail_2","flag":false}`,
		`{"field":"tail_3","flag":false}`,
		`{"field":"tail_4","flag":true}`,
		`{"field":"tail_5","flag":false}`,
	}
	for rgID, doc := range docs {
		if err := builder.AddDocument(DocID(rgID), []byte(doc)); err != nil {
			t.Fatalf("AddDocument(rg=%d) failed: %v", rgID, err)
		}
	}

	idx := builder.Finalize()
	fieldEntry := findPathEntry(idx, "$.field")
	if fieldEntry == nil {
		t.Fatal("expected $.field path entry")
	}
	if fieldEntry.Mode != PathModeAdaptiveHybrid {
		t.Fatalf("$.field Mode = %v, want adaptive hybrid", fieldEntry.Mode)
	}
	flagEntry := findPathEntry(idx, "$.flag")
	if flagEntry == nil {
		t.Fatal("expected $.flag path entry")
	}
	if flagEntry.Mode != PathModeAdaptiveHybrid {
		t.Fatalf("$.flag Mode = %v, want adaptive hybrid", flagEntry.Mode)
	}

	tests := []struct {
		name        string
		pred        Predicate
		wantExact   []int
		mustContain []int
		mustExclude []int
	}{
		{
			name:      "all promoted",
			pred:      IN("$.field", "hot"),
			wantExact: []int{0, 1},
		},
		{
			name:        "all tail",
			pred:        IN("$.field", "tail_2", "tail_3"),
			mustContain: []int{2, 3},
			mustExclude: []int{0, 1},
		},
		{
			name:        "mixed promoted and tail",
			pred:        IN("$.field", "hot", "tail_4"),
			mustContain: []int{0, 1, 4},
		},
		{
			name:      "bool terms",
			pred:      IN("$.flag", true),
			wantExact: []int{0, 1, 4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := idx.Evaluate([]Predicate{tt.pred}).ToSlice()
			if tt.wantExact != nil {
				if fmt.Sprint(got) != fmt.Sprint(tt.wantExact) {
					t.Fatalf("%s = %v, want %v", tt.pred.Operator, got, tt.wantExact)
				}
				return
			}
			for _, rgID := range tt.mustContain {
				if !containsInt(got, rgID) {
					t.Fatalf("%s = %v, want RG %d present", tt.pred.Operator, got, rgID)
				}
			}
			for _, rgID := range tt.mustExclude {
				if containsInt(got, rgID) {
					t.Fatalf("%s = %v, want RG %d absent", tt.pred.Operator, got, rgID)
				}
			}
		})
	}
}

func containsInt(values []int, want int) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestAdaptiveThresholdBoundarySelection(t *testing.T) {
	tests := []struct {
		name      string
		threshold uint32
		wantMode  PathMode
	}{
		{name: "below threshold stays exact", threshold: 3, wantMode: PathModeClassic},
		{name: "at threshold stays exact", threshold: 2, wantMode: PathModeClassic},
		{name: "above threshold becomes adaptive", threshold: 1, wantMode: PathModeAdaptiveHybrid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			config.CardinalityThreshold = tt.threshold
			config.AdaptiveMinRGCoverage = 2
			config.AdaptivePromotedTermCap = 2
			config.AdaptiveCoverageCeiling = 0.80
			config.AdaptiveBucketCount = 4

			builder := mustNewBuilder(t, config, 3)
			docs := []string{
				`{"field":"hot"}`,
				`{"field":"hot"}`,
				`{"field":"tail"}`,
			}
			for rgID, doc := range docs {
				if err := builder.AddDocument(DocID(rgID), []byte(doc)); err != nil {
					t.Fatalf("AddDocument(rg=%d) failed: %v", rgID, err)
				}
			}

			entry := findPathEntry(builder.Finalize(), "$.field")
			if entry == nil {
				t.Fatal("expected $.field path entry")
			}
			if entry.Mode != tt.wantMode {
				t.Fatalf("Mode = %v, want %v", entry.Mode, tt.wantMode)
			}
		})
	}
}

func TestAdaptiveCardinalitySweepAtFixedThreshold(t *testing.T) {
	tests := []struct {
		name         string
		docs         []string
		wantMode     PathMode
		wantAdaptive bool
	}{
		{
			name: "below threshold",
			docs: []string{
				`{"field":"hot"}`,
				`{"field":"hot"}`,
				`{"field":"hot"}`,
			},
			wantMode: PathModeClassic,
		},
		{
			name: "at threshold",
			docs: []string{
				`{"field":"hot"}`,
				`{"field":"hot"}`,
				`{"field":"tail"}`,
			},
			wantMode: PathModeClassic,
		},
		{
			name: "above threshold",
			docs: []string{
				`{"field":"hot"}`,
				`{"field":"hot"}`,
				`{"field":"tail_a"}`,
				`{"field":"tail_b"}`,
			},
			wantMode: PathModeAdaptiveHybrid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			config.CardinalityThreshold = 2
			config.AdaptiveMinRGCoverage = 2
			config.AdaptivePromotedTermCap = 2
			config.AdaptiveCoverageCeiling = 0.80
			config.AdaptiveBucketCount = 4

			builder := mustNewBuilder(t, config, len(tt.docs))
			for rgID, doc := range tt.docs {
				if err := builder.AddDocument(DocID(rgID), []byte(doc)); err != nil {
					t.Fatalf("AddDocument(rg=%d) failed: %v", rgID, err)
				}
			}

			entry := findPathEntry(builder.Finalize(), "$.field")
			if entry == nil {
				t.Fatal("expected $.field path entry")
			}
			if entry.Mode != tt.wantMode {
				t.Fatalf("Mode = %v, want %v", entry.Mode, tt.wantMode)
			}
		})
	}
}

func TestSelectAdaptivePromotedTermsHonorsCoverageFiltersAndSortOrder(t *testing.T) {
	config := DefaultConfig()
	config.AdaptiveMinRGCoverage = 2
	config.AdaptivePromotedTermCap = 2
	config.AdaptiveCoverageCeiling = 0.80
	config.AdaptiveBucketCount = 4

	builder := mustNewBuilder(t, config, 6)
	pd := builder.getOrCreatePath("$.field")
	for rgID := 0; rgID < 3; rgID++ {
		builder.addStringTerm(pd, "alpha", rgID, "$.field")
	}
	for rgID := 0; rgID < 3; rgID++ {
		builder.addStringTerm(pd, "beta", rgID, "$.field")
	}
	builder.addStringTerm(pd, "gamma", 0, "$.field")
	for rgID := 0; rgID < 5; rgID++ {
		builder.addStringTerm(pd, "omega", rgID, "$.field")
	}

	got := builder.selectAdaptivePromotedTerms(pd)
	if len(got) != 2 {
		t.Fatalf("len(promoted) = %d, want 2", len(got))
	}
	if _, ok := got["alpha"]; !ok {
		t.Fatal("expected alpha to be promoted")
	}
	if _, ok := got["beta"]; !ok {
		t.Fatal("expected beta to be promoted")
	}
	if _, ok := got["gamma"]; ok {
		t.Fatal("gamma should be filtered out by min coverage")
	}
	if _, ok := got["omega"]; ok {
		t.Fatal("omega should be filtered out by coverage ceiling")
	}
}

func TestSelectAdaptivePromotedTermsRespectsMinCoverage(t *testing.T) {
	config := DefaultConfig()
	config.AdaptiveMinRGCoverage = 2
	config.AdaptivePromotedTermCap = 8
	config.AdaptiveCoverageCeiling = 0.99
	config.AdaptiveBucketCount = 4

	builder := mustNewBuilder(t, config, 4)
	pd := builder.getOrCreatePath("$.field")
	builder.addStringTerm(pd, "single", 0, "$.field")
	builder.addStringTerm(pd, "double", 0, "$.field")
	builder.addStringTerm(pd, "double", 1, "$.field")

	got := builder.selectAdaptivePromotedTerms(pd)
	if _, ok := got["single"]; ok {
		t.Fatal("single should be filtered out by min coverage")
	}
	if _, ok := got["double"]; !ok {
		t.Fatal("double should satisfy min coverage")
	}
}

func TestSelectAdaptivePromotedTermsRespectsCoverageCeiling(t *testing.T) {
	config := DefaultConfig()
	config.AdaptiveMinRGCoverage = 1
	config.AdaptivePromotedTermCap = 8
	config.AdaptiveCoverageCeiling = 0.50
	config.AdaptiveBucketCount = 4

	builder := mustNewBuilder(t, config, 4)
	pd := builder.getOrCreatePath("$.field")
	for rgID := 0; rgID < 3; rgID++ {
		builder.addStringTerm(pd, "too_hot", rgID, "$.field")
	}
	for rgID := 0; rgID < 2; rgID++ {
		builder.addStringTerm(pd, "okay", rgID, "$.field")
	}

	got := builder.selectAdaptivePromotedTerms(pd)
	if _, ok := got["too_hot"]; ok {
		t.Fatal("too_hot should be filtered out by coverage ceiling")
	}
	if _, ok := got["okay"]; !ok {
		t.Fatal("okay should satisfy coverage ceiling")
	}
}

func TestSelectAdaptivePromotedTermsRespectsCapAndStableSortOrder(t *testing.T) {
	config := DefaultConfig()
	config.AdaptiveMinRGCoverage = 1
	config.AdaptivePromotedTermCap = 2
	config.AdaptiveCoverageCeiling = 0.99
	config.AdaptiveBucketCount = 4

	builder := mustNewBuilder(t, config, 4)
	pd := builder.getOrCreatePath("$.field")
	for _, term := range []string{"beta", "alpha", "gamma"} {
		builder.addStringTerm(pd, term, 0, "$.field")
	}

	got := builder.selectAdaptivePromotedTerms(pd)
	if len(got) != 2 {
		t.Fatalf("len(promoted) = %d, want 2", len(got))
	}
	if _, ok := got["alpha"]; !ok {
		t.Fatal("alpha should win tie-break by lexical order")
	}
	if _, ok := got["beta"]; !ok {
		t.Fatal("beta should win tie-break by lexical order")
	}
	if _, ok := got["gamma"]; ok {
		t.Fatal("gamma should be dropped by cap")
	}
}

func TestNewConfigAdaptiveOptionValidators(t *testing.T) {
	tests := []struct {
		name string
		opts []ConfigOption
	}{
		{
			name: "negative min coverage",
			opts: []ConfigOption{WithAdaptiveMinRGCoverage(-1)},
		},
		{
			name: "negative term cap",
			opts: []ConfigOption{WithAdaptivePromotedTermCap(-1)},
		},
		{
			name: "invalid coverage ceiling",
			opts: []ConfigOption{WithAdaptiveCoverageCeiling(1)},
		},
		{
			name: "zero bucket count",
			opts: []ConfigOption{WithAdaptiveBucketCount(0)},
		},
		{
			name: "non power of two bucket count",
			opts: []ConfigOption{WithAdaptiveBucketCount(3)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewConfig(tt.opts...); err == nil {
				t.Fatal("NewConfig() error = nil, want adaptive option validation failure")
			}
		})
	}
}

func TestNewAdaptiveStringIndexValidatesInvariants(t *testing.T) {
	rg0 := MustNewRGSet(2)
	rg0.Set(0)
	rg1 := MustNewRGSet(2)
	rg1.Set(1)

	bucket0 := MustNewRGSet(2)
	bucket1 := MustNewRGSet(2)

	if _, err := NewAdaptiveStringIndex([]string{"beta", "alpha"}, []*RGSet{rg0, rg1}, []*RGSet{bucket0, bucket1}); err == nil {
		t.Fatal("NewAdaptiveStringIndex() error = nil, want unsorted terms rejection")
	}
	if _, err := NewAdaptiveStringIndex([]string{"alpha"}, []*RGSet{rg0, rg1}, []*RGSet{bucket0, bucket1}); err == nil {
		t.Fatal("NewAdaptiveStringIndex() error = nil, want mismatched bitmap length rejection")
	}
	if _, err := NewAdaptiveStringIndex([]string{"alpha"}, []*RGSet{rg0}, []*RGSet{}); err == nil {
		t.Fatal("NewAdaptiveStringIndex() error = nil, want empty bucket set rejection")
	}
	if _, err := NewAdaptiveStringIndex([]string{"alpha"}, []*RGSet{rg0}, []*RGSet{bucket0, bucket1, bucket0}); err == nil {
		t.Fatal("NewAdaptiveStringIndex() error = nil, want non-power-of-two bucket count rejection")
	}
	if _, err := NewAdaptiveStringIndex([]string{"alpha", "beta"}, []*RGSet{rg0, rg1}, []*RGSet{bucket0, bucket1}); err != nil {
		t.Fatalf("NewAdaptiveStringIndex() error = %v, want valid index", err)
	}
}

func TestAdaptiveBucketIndexPanicsOnZeroBuckets(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("adaptiveBucketIndex() panic = nil, want panic for zero buckets")
		}
	}()

	_ = adaptiveBucketIndex("alpha", 0)
}

func TestPathModeStringMapping(t *testing.T) {
	tests := []struct {
		mode PathMode
		want string
	}{
		{mode: PathModeClassic, want: "exact"},
		{mode: PathModeBloomOnly, want: "bloom-only"},
		{mode: PathModeAdaptiveHybrid, want: "adaptive-hybrid"},
		{mode: PathMode(99), want: "unknown"},
	}

	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.want {
			t.Fatalf("PathMode(%d).String() = %q, want %q", tt.mode, got, tt.want)
		}
	}
}

func TestPathModeIsValid(t *testing.T) {
	tests := []struct {
		mode PathMode
		want bool
	}{
		{mode: PathModeClassic, want: true},
		{mode: PathModeBloomOnly, want: true},
		{mode: PathModeAdaptiveHybrid, want: true},
		{mode: PathMode(3), want: false},
		{mode: PathMode(99), want: false},
		{mode: PathMode(255), want: false},
	}

	for _, tt := range tests {
		if got := tt.mode.IsValid(); got != tt.want {
			t.Fatalf("PathMode(%d).IsValid() = %v, want %v", tt.mode, got, tt.want)
		}
	}
}

func TestAdaptiveNegativePredicatesMixedPromotedAndTailStayConservative(t *testing.T) {
	config := DefaultConfig()
	config.CardinalityThreshold = 1
	config.AdaptiveMinRGCoverage = 2
	config.AdaptivePromotedTermCap = 2
	config.AdaptiveCoverageCeiling = 0.80
	config.AdaptiveBucketCount = 4

	builder := mustNewBuilder(t, config, 6)
	docs := []string{
		`{"field":"hot"}`,
		`{"field":"hot"}`,
		`{"field":"tail_2"}`,
		`{"field":"tail_3"}`,
		`{"field":"tail_4"}`,
		`{"field":"tail_5"}`,
	}
	for rgID, doc := range docs {
		if err := builder.AddDocument(DocID(rgID), []byte(doc)); err != nil {
			t.Fatalf("AddDocument(rg=%d) failed: %v", rgID, err)
		}
	}

	idx := builder.Finalize()
	if got := idx.Evaluate([]Predicate{NIN("$.field", "hot", "tail_2")}).ToSlice(); fmt.Sprint(got) != fmt.Sprint([]int{0, 1, 2, 3, 4, 5}) {
		t.Fatalf("NIN mixed promoted+tail result = %v, want all present RGs", got)
	}
}

func TestAdaptiveContainsUsesTrigramIndex(t *testing.T) {
	config := DefaultConfig()
	config.CardinalityThreshold = 1
	config.AdaptiveMinRGCoverage = 2
	config.AdaptivePromotedTermCap = 4
	config.AdaptiveCoverageCeiling = 0.80
	config.AdaptiveBucketCount = 4

	builder := mustNewBuilder(t, config, 4)
	docs := []string{
		`{"text":"alpha needle"}`,
		`{"text":"beta haystack"}`,
		`{"text":"gamma needle"}`,
		`{"text":"delta haystack"}`,
	}
	for rgID, doc := range docs {
		if err := builder.AddDocument(DocID(rgID), []byte(doc)); err != nil {
			t.Fatalf("AddDocument(rg=%d) failed: %v", rgID, err)
		}
	}

	idx := builder.Finalize()
	entry := findPathEntry(idx, "$.text")
	if entry == nil {
		t.Fatal("expected $.text path entry")
	}
	if entry.Mode != PathModeAdaptiveHybrid {
		t.Fatalf("Mode = %v, want adaptive hybrid", entry.Mode)
	}
	if entry.Flags&FlagTrigramIndex == 0 {
		t.Fatal("adaptive path should retain trigram index for CONTAINS")
	}

	assertContains := func(t *testing.T, idx *GINIndex) {
		t.Helper()
		if got := idx.Evaluate([]Predicate{Contains("$.text", "needle")}).ToSlice(); fmt.Sprint(got) != fmt.Sprint([]int{0, 2}) {
			t.Fatalf("Contains($.text, needle) = %v, want [0 2]", got)
		}
	}

	assertContains(t, idx)

	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	assertContains(t, decoded)
}

func TestAdaptiveInvariantViolationLogs(t *testing.T) {
	var logBuf bytes.Buffer
	prev := currentAdaptiveInvariantLogger()
	SetAdaptiveInvariantLogger(log.New(&logBuf, "", 0))
	defer SetAdaptiveInvariantLogger(prev)

	idx := NewGINIndex()
	idx.Header.NumRowGroups = 3
	idx.PathDirectory = []PathEntry{{
		PathID:   0,
		PathName: "$.field",
		Mode:     PathModeAdaptiveHybrid,
	}}
	idx.pathLookup["$.field"] = 0
	idx.GlobalBloom = MustNewBloomFilter(64, 3)
	idx.GlobalBloom.AddString("$.field=hot")

	if got := idx.Evaluate([]Predicate{EQ("$.field", "hot")}).ToSlice(); fmt.Sprint(got) != fmt.Sprint([]int{0, 1, 2}) {
		t.Fatalf("Evaluate(EQ) = %v, want all row groups", got)
	}
	if !strings.Contains(logBuf.String(), "adaptive path invariant violation") {
		t.Fatalf("log output = %q, want invariant violation message", logBuf.String())
	}
}

func TestSetAdaptiveInvariantLoggerNilSilences(t *testing.T) {
	prev := currentAdaptiveInvariantLogger()
	SetAdaptiveInvariantLogger(nil)
	defer SetAdaptiveInvariantLogger(prev)

	idx := NewGINIndex()
	idx.Header.NumRowGroups = 2
	idx.PathDirectory = []PathEntry{{
		PathID:   0,
		PathName: "$.field",
		Mode:     PathModeAdaptiveHybrid,
	}}
	idx.pathLookup["$.field"] = 0
	idx.GlobalBloom = MustNewBloomFilter(64, 3)
	idx.GlobalBloom.AddString("$.field=hot")

	// Must not panic or race when logger is nil.
	got := idx.Evaluate([]Predicate{EQ("$.field", "hot")}).ToSlice()
	if fmt.Sprint(got) != fmt.Sprint([]int{0, 1}) {
		t.Fatalf("Evaluate(EQ) = %v, want all row groups (safe fallback)", got)
	}
}

func TestQueryNIN(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 4)

	builder.AddDocument(0, []byte(`{"status": "active"}`))
	builder.AddDocument(1, []byte(`{"status": "pending"}`))
	builder.AddDocument(2, []byte(`{"status": "inactive"}`))
	builder.AddDocument(3, []byte(`{"status": "archived"}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{NIN("$.status", "active", "pending")})
	if result.Count() != 2 {
		t.Errorf("expected 2 matching RGs for NOT IN, got %d", result.Count())
	}
	if !result.IsSet(2) || !result.IsSet(3) {
		t.Error("RG 2 and 3 should match NOT IN")
	}
	if result.IsSet(0) || result.IsSet(1) {
		t.Error("RG 0 and 1 should not match NOT IN")
	}
}

func TestQueryNull(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)

	builder.AddDocument(0, []byte(`{"value": null}`))
	builder.AddDocument(1, []byte(`{"value": 42}`))
	builder.AddDocument(2, []byte(`{"other": "field"}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{IsNull("$.value")})
	if !result.IsSet(0) {
		t.Error("RG 0 should match IS NULL")
	}

	result = idx.Evaluate([]Predicate{IsNotNull("$.value")})
	if !result.IsSet(0) || !result.IsSet(1) {
		t.Error("RG 0 and 1 should match IS NOT NULL")
	}
}

func TestSerializeRoundTrip(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)

	builder.AddDocument(0, []byte(`{"name": "alice", "age": 30, "active": true}`))
	builder.AddDocument(1, []byte(`{"name": "bob", "age": 25, "active": false}`))
	builder.AddDocument(2, []byte(`{"name": "charlie", "age": null}`))

	idx := builder.Finalize()

	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if decoded.Header.NumDocs != idx.Header.NumDocs {
		t.Errorf("NumDocs mismatch: %d vs %d", decoded.Header.NumDocs, idx.Header.NumDocs)
	}
	if decoded.Header.NumRowGroups != idx.Header.NumRowGroups {
		t.Errorf("NumRowGroups mismatch")
	}
	if len(decoded.PathDirectory) != len(idx.PathDirectory) {
		t.Errorf("PathDirectory length mismatch")
	}

	result := decoded.Evaluate([]Predicate{EQ("$.name", "alice")})
	if !result.IsSet(0) {
		t.Error("query on decoded index failed")
	}
}

func TestSerializeRoundTripWithArrays(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)

	builder.AddDocument(0, []byte(`{"items": ["x", "y"], "name": "alice"}`))
	builder.AddDocument(1, []byte(`{"items": ["z"], "nested": [{"a": 1}]}`))

	idx := builder.Finalize()

	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if decoded.Header.NumDocs != idx.Header.NumDocs {
		t.Errorf("NumDocs mismatch: %d vs %d", decoded.Header.NumDocs, idx.Header.NumDocs)
	}
	if len(decoded.PathDirectory) != len(idx.PathDirectory) {
		t.Errorf("PathDirectory length mismatch: %d vs %d", len(decoded.PathDirectory), len(idx.PathDirectory))
	}

	result := decoded.Evaluate([]Predicate{EQ("$.items[*]", "x")})
	if !result.IsSet(0) || result.IsSet(1) {
		t.Errorf("array query on decoded index failed: got %v", result.ToSlice())
	}

	result = decoded.Evaluate([]Predicate{EQ("$.name", "alice")})
	if !result.IsSet(0) || result.IsSet(1) {
		t.Errorf("flat field query on decoded index failed: got %v", result.ToSlice())
	}
}

func TestCompressionLevels(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)
	builder.AddDocument(0, []byte(`{"name": "alice", "age": 30, "active": true}`))
	builder.AddDocument(1, []byte(`{"name": "bob", "age": 25, "active": false}`))
	builder.AddDocument(2, []byte(`{"name": "charlie", "age": 35}`))
	idx := builder.Finalize()

	levels := []struct {
		name  string
		level CompressionLevel
	}{
		{"None", CompressionNone},
		{"Fastest", CompressionFastest},
		{"Balanced", CompressionBalanced},
		{"Better", CompressionBetter},
		{"Best", CompressionBest},
		{"Max", CompressionMax},
	}

	for _, tc := range levels {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := EncodeWithLevel(idx, tc.level)
			if err != nil {
				t.Fatalf("encode with level %d failed: %v", tc.level, err)
			}

			decoded, err := Decode(encoded)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			if decoded.Header.NumDocs != idx.Header.NumDocs {
				t.Errorf("NumDocs mismatch: %d vs %d", decoded.Header.NumDocs, idx.Header.NumDocs)
			}
			if decoded.Header.NumRowGroups != idx.Header.NumRowGroups {
				t.Errorf("NumRowGroups mismatch")
			}

			result := decoded.Evaluate([]Predicate{EQ("$.name", "bob")})
			if !result.IsSet(1) {
				t.Error("query on decoded index failed")
			}
		})
	}
}

func TestCompressionUncompressedMagic(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)
	builder.AddDocument(0, []byte(`{"value": 1}`))
	builder.AddDocument(1, []byte(`{"value": 2}`))
	idx := builder.Finalize()

	encoded, err := EncodeWithLevel(idx, CompressionNone)
	if err != nil {
		t.Fatalf("encode uncompressed failed: %v", err)
	}

	if len(encoded) < 4 {
		t.Fatal("encoded data too short")
	}
	if string(encoded[:4]) != "GINu" {
		t.Errorf("expected uncompressed magic 'GINu', got %q", string(encoded[:4]))
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("decode uncompressed failed: %v", err)
	}

	result := decoded.Evaluate([]Predicate{EQ("$.value", float64(2))})
	if !result.IsSet(1) {
		t.Error("query on decoded uncompressed index failed")
	}
}

func TestCompressionCompressedMagic(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)
	builder.AddDocument(0, []byte(`{"value": 1}`))
	builder.AddDocument(1, []byte(`{"value": 2}`))
	idx := builder.Finalize()

	encoded, err := EncodeWithLevel(idx, CompressionFastest)
	if err != nil {
		t.Fatalf("encode compressed failed: %v", err)
	}

	if len(encoded) < 4 {
		t.Fatal("encoded data too short")
	}
	if string(encoded[:4]) != "GINc" {
		t.Errorf("expected compressed magic 'GINc', got %q", string(encoded[:4]))
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("decode compressed failed: %v", err)
	}

	result := decoded.Evaluate([]Predicate{EQ("$.value", float64(2))})
	if !result.IsSet(1) {
		t.Error("query on decoded compressed index failed")
	}
}

func TestCompressionSizeReduction(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 100)
	for i := 0; i < 100; i++ {
		builder.AddDocument(DocID(i), []byte(fmt.Sprintf(`{"id": %d, "name": "user_%d", "value": %d}`, i, i%10, i*100)))
	}
	idx := builder.Finalize()

	uncompressed, _ := EncodeWithLevel(idx, CompressionNone)
	fastest, _ := EncodeWithLevel(idx, CompressionFastest)

	if len(fastest) >= len(uncompressed) {
		t.Errorf("zstd-1 should be smaller than uncompressed: %d >= %d", len(fastest), len(uncompressed))
	}

	// Note: For small datasets, higher compression levels may not always produce smaller output
	// due to compression dictionary overhead. The benefit is more pronounced with larger data.
	t.Logf("Sizes - Uncompressed: %d, Zstd-1: %d (%.1f%% of original)",
		len(uncompressed), len(fastest), float64(len(fastest))/float64(len(uncompressed))*100)
}

func TestCompressionInvalidLevel(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)
	builder.AddDocument(0, []byte(`{"value": 1}`))
	idx := builder.Finalize()

	tests := []struct {
		name  string
		level CompressionLevel
	}{
		{"negative", CompressionLevel(-1)},
		{"too_high", CompressionLevel(20)},
		{"way_too_high", CompressionLevel(100)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := EncodeWithLevel(idx, tc.level)
			if err == nil {
				t.Errorf("expected error for compression level %d", tc.level)
			}
		})
	}
}

func TestNestedJSON(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)

	builder.AddDocument(0, []byte(`{"user": {"name": "alice", "address": {"city": "NYC"}}}`))
	builder.AddDocument(1, []byte(`{"user": {"name": "bob", "address": {"city": "LA"}}}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{EQ("$.user.name", "alice")})
	if !result.IsSet(0) || result.IsSet(1) {
		t.Error("nested query failed")
	}

	result = idx.Evaluate([]Predicate{EQ("$.user.address.city", "LA")})
	if result.IsSet(0) || !result.IsSet(1) {
		t.Error("deep nested query failed")
	}
}

func TestArrayJSON(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)

	builder.AddDocument(0, []byte(`{"tags": ["go", "rust"]}`))
	builder.AddDocument(1, []byte(`{"tags": ["python", "java"]}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{EQ("$.tags[*]", "go")})
	if !result.IsSet(0) || result.IsSet(1) {
		t.Error("array wildcard query failed")
	}
}

func TestMultiplePredicates(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 4)

	builder.AddDocument(0, []byte(`{"name": "alice", "age": 30}`))
	builder.AddDocument(1, []byte(`{"name": "alice", "age": 25}`))
	builder.AddDocument(2, []byte(`{"name": "bob", "age": 30}`))
	builder.AddDocument(3, []byte(`{"name": "bob", "age": 25}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{
		EQ("$.name", "alice"),
		GTE("$.age", 28),
	})
	if result.Count() != 1 || !result.IsSet(0) {
		t.Errorf("expected only RG 0, got %v", result.ToSlice())
	}
}

func TestEmptyIndex(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 5)
	idx := builder.Finalize()

	if idx.Header.NumDocs != 0 {
		t.Error("empty index should have 0 docs")
	}

	result := idx.Evaluate([]Predicate{EQ("$.foo", "bar")})
	if result.Count() != 5 {
		t.Error("query on empty index should return all RGs")
	}
}

func TestBloomFastPath(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)

	builder.AddDocument(0, []byte(`{"status": "active"}`))
	builder.AddDocument(1, []byte(`{"status": "inactive"}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{EQ("$.status", "nonexistent")})
	if result.Count() != 0 {
		t.Error("bloom filter should eliminate all RGs for nonexistent value")
	}
}

func TestBooleanValues(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)

	builder.AddDocument(0, []byte(`{"active": true}`))
	builder.AddDocument(1, []byte(`{"active": false}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{EQ("$.active", true)})
	if !result.IsSet(0) || result.IsSet(1) {
		t.Error("boolean true query failed")
	}

	result = idx.Evaluate([]Predicate{EQ("$.active", false)})
	if result.IsSet(0) || !result.IsSet(1) {
		t.Error("boolean false query failed")
	}
}

func TestLargeRGCount(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 100)

	for i := 0; i < 100; i++ {
		builder.AddDocument(DocID(i), []byte(`{"value": "test"}`))
	}

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{EQ("$.value", "test")})
	if result.Count() != 100 {
		t.Errorf("expected 100 matching RGs, got %d", result.Count())
	}
}

func TestTrigramIndex(t *testing.T) {
	ti, err := NewTrigramIndex(3)
	if err != nil {
		t.Fatalf("failed to create trigram index: %v", err)
	}
	ti.Add("hello world", 0)
	ti.Add("hello there", 1)
	ti.Add("goodbye world", 2)

	result := ti.Search("hello")
	if !result.IsSet(0) || !result.IsSet(1) || result.IsSet(2) {
		t.Error("trigram search for 'hello' failed")
	}

	result = ti.Search("world")
	if !result.IsSet(0) || result.IsSet(1) || !result.IsSet(2) {
		t.Error("trigram search for 'world' failed")
	}

	result = ti.Search("xyz")
	if !result.IsEmpty() {
		t.Error("trigram search for non-existent pattern should return empty")
	}
}

func TestTrigramExtraction(t *testing.T) {
	trigrams := ExtractTrigrams("hello")
	if len(trigrams) == 0 {
		t.Error("should extract trigrams from 'hello'")
	}

	hasHel := false
	for _, tg := range trigrams {
		if tg == " he" || tg == "hel" || tg == "ell" || tg == "llo" {
			hasHel = true
			break
		}
	}
	if !hasHel {
		t.Error("expected to find trigrams from 'hello'")
	}
}

func TestHyperLogLog(t *testing.T) {
	hll := MustNewHyperLogLog(12)

	for i := 0; i < 10000; i++ {
		hll.AddString(string(rune('a' + i%26)))
	}

	estimate := hll.Estimate()
	if estimate < 20 || estimate > 35 {
		t.Errorf("HLL estimate for 26 unique values is off: got %d", estimate)
	}
}

func TestHyperLogLogLargeCardinality(t *testing.T) {
	hll := MustNewHyperLogLog(14)

	for i := 0; i < 100000; i++ {
		hll.AddString(fmt.Sprintf("value_%d", i))
	}

	estimate := hll.Estimate()
	diff := float64(estimate) - 100000.0
	errorRate := diff / 100000.0
	if errorRate < -0.05 || errorRate > 0.05 {
		t.Errorf("HLL estimate error too high: got %d, expected ~100000, error rate: %.2f%%", estimate, errorRate*100)
	}
}

func TestHyperLogLogMerge(t *testing.T) {
	hll1 := MustNewHyperLogLog(12)
	hll2 := MustNewHyperLogLog(12)

	for i := 0; i < 1000; i++ {
		hll1.AddString(fmt.Sprintf("set1_%d", i))
	}
	for i := 0; i < 1000; i++ {
		hll2.AddString(fmt.Sprintf("set2_%d", i))
	}

	hll1.Merge(hll2)
	estimate := hll1.Estimate()
	if estimate < 1800 || estimate > 2200 {
		t.Errorf("merged HLL estimate is off: got %d, expected ~2000", estimate)
	}
}

func TestPrefixCompression(t *testing.T) {
	terms := []string{
		"application",
		"application_config",
		"application_data",
		"application_log",
		"database",
		"database_config",
		"database_pool",
	}

	pc := MustNewPrefixCompressor(4)
	blocks := pc.Compress(terms)

	if len(blocks) == 0 {
		t.Error("expected at least one block")
	}

	decompressed := pc.Decompress(blocks)
	if len(decompressed) != len(terms) {
		t.Errorf("decompressed length mismatch: got %d, expected %d", len(decompressed), len(terms))
	}

	for i, term := range decompressed {
		found := false
		for _, orig := range terms {
			if term == orig {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("decompressed term %d '%s' not in original terms", i, term)
		}
	}
}

func TestPrefixCompressionStats(t *testing.T) {
	terms := []string{
		"prefix_a",
		"prefix_ab",
		"prefix_abc",
		"prefix_abcd",
	}

	compressed, original, ratio := CompressionStats(terms)
	if ratio >= 1.0 {
		t.Errorf("compression ratio should be < 1.0 for terms with shared prefixes, got %.2f", ratio)
	}
	t.Logf("Compression: %d bytes -> %d bytes (ratio: %.2f)", original, compressed, ratio)
}

func TestContainsQuery(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)

	builder.AddDocument(0, []byte(`{"description": "hello world application"}`))
	builder.AddDocument(1, []byte(`{"description": "goodbye world service"}`))
	builder.AddDocument(2, []byte(`{"description": "hello universe"}`))

	idx := builder.Finalize()

	result := idx.Evaluate([]Predicate{Contains("$.description", "hello")})
	if !result.IsSet(0) || result.IsSet(1) || !result.IsSet(2) {
		t.Errorf("CONTAINS 'hello' failed: got RGs %v", result.ToSlice())
	}

	result = idx.Evaluate([]Predicate{Contains("$.description", "world")})
	if !result.IsSet(0) || !result.IsSet(1) || result.IsSet(2) {
		t.Errorf("CONTAINS 'world' failed: got RGs %v", result.ToSlice())
	}
}

func TestContainsWithSerialize(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)

	builder.AddDocument(0, []byte(`{"text": "the quick brown fox"}`))
	builder.AddDocument(1, []byte(`{"text": "lazy dog sleeps"}`))

	idx := builder.Finalize()

	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	result := decoded.Evaluate([]Predicate{Contains("$.text", "quick")})
	if !result.IsSet(0) || result.IsSet(1) {
		t.Error("CONTAINS query on decoded index failed")
	}
}

func TestHLLCardinality(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)

	builder.AddDocument(0, []byte(`{"status": "active"}`))
	builder.AddDocument(0, []byte(`{"status": "inactive"}`))
	builder.AddDocument(0, []byte(`{"status": "pending"}`))
	builder.AddDocument(1, []byte(`{"status": "active"}`))

	idx := builder.Finalize()

	var statusPath *PathEntry
	for i := range idx.PathDirectory {
		if idx.PathDirectory[i].PathName == "$.status" {
			statusPath = &idx.PathDirectory[i]
			break
		}
	}

	if statusPath == nil {
		t.Fatal("$.status path not found")
	}

	if statusPath.Cardinality < 2 || statusPath.Cardinality > 5 {
		t.Errorf("cardinality estimate is off: got %d, expected ~3", statusPath.Cardinality)
	}
}

func TestJSONPathValidation(t *testing.T) {
	validPaths := []string{
		"$",
		"$.foo",
		"$.foo.bar",
		"$.foo.bar.baz",
		"$['foo']",
		`$["foo"]`,
		"$.foo['bar']",
		"$.items[*]",
		"$.foo_bar",
		"$['foo-bar']['baz']",
	}

	for _, path := range validPaths {
		if err := ValidateJSONPath(path); err != nil {
			t.Errorf("expected valid path %q, got error: %v", path, err)
		}
	}

	// Invalid syntax (caught by ojg parser)
	syntaxErrors := []struct {
		path string
		desc string
	}{
		{"", "empty path"},
		{"foo", "missing $ prefix"},
		{"$[", "unclosed bracket"},
		{"$['foo", "unclosed string"},
	}

	for _, tc := range syntaxErrors {
		if err := ValidateJSONPath(tc.path); err == nil {
			t.Errorf("expected invalid path %q (%s) to fail validation", tc.path, tc.desc)
		}
	}

	// Unsupported features (valid JSONPath but not supported by GIN)
	unsupportedPaths := []struct {
		path string
		desc string
	}{
		{"$.items[0]", "array index"},
		{"$.items[123]", "array index"},
		{"$.data[0].name", "array index in path"},
		{"$..foo", "recursive descent"},
		{"$.items[0:5]", "slice notation"},
		{"$.items[?(@.price > 10)]", "filter expression"},
	}

	for _, tc := range unsupportedPaths {
		err := ValidateJSONPath(tc.path)
		if err == nil {
			t.Errorf("expected unsupported path %q (%s) to fail validation", tc.path, tc.desc)
		}
	}
}

func TestJSONPathNormalize(t *testing.T) {
	// Just verify normalization doesn't panic and produces consistent output
	paths := []string{
		"$",
		"$.foo",
		"$.foo.bar",
		"$['foo']",
		"$.items[*]",
	}

	for _, path := range paths {
		result := NormalizePath(path)
		if result == "" {
			t.Errorf("NormalizePath(%q) returned empty string", path)
		}
		// Normalized path should be valid
		if !IsValidJSONPath(result) {
			t.Errorf("NormalizePath(%q) = %q is not valid", path, result)
		}
	}
}

func TestJSONPathCanonicalizeSupportedPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{path: "$.foo", want: "$.foo"},
		{path: "$['foo']", want: "$.foo"},
		{path: `$["foo"]`, want: "$.foo"},
	}

	for _, tc := range tests {
		got, err := canonicalizeSupportedPath(tc.path)
		if err != nil {
			t.Fatalf("canonicalizeSupportedPath(%q) error = %v", tc.path, err)
		}
		if got != tc.want {
			t.Errorf("canonicalizeSupportedPath(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestJSONPathCanonicalizeUnsupportedPath(t *testing.T) {
	unsupported := []string{
		"$.items[0]",
		"$..foo",
		"$.items[0:5]",
		"$.items[?(@.price > 10)]",
	}

	for _, path := range unsupported {
		got, err := canonicalizeSupportedPath(path)
		if err == nil {
			t.Fatalf("canonicalizeSupportedPath(%q) = %q, want error", path, got)
		}
	}
}

func TestIsValidJSONPath(t *testing.T) {
	if !IsValidJSONPath("$.foo.bar") {
		t.Error("expected $.foo.bar to be valid")
	}
	if IsValidJSONPath("invalid") {
		t.Error("expected 'invalid' to be invalid")
	}
}

func TestMustValidateJSONPath(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid path")
		}
	}()
	MustValidateJSONPath("invalid")
}

func TestWithFTSPathsExact(t *testing.T) {
	config, err := NewConfig(WithFTSPaths("$.description"))
	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}
	builder := mustNewBuilder(t, config, 3)

	builder.AddDocument(0, []byte(`{"description": "hello world", "title": "test title"}`))
	builder.AddDocument(1, []byte(`{"description": "goodbye world", "title": "another title"}`))
	builder.AddDocument(2, []byte(`{"description": "hello universe", "title": "hello title"}`))

	idx := builder.Finalize()

	// CONTAINS on $.description should prune (trigrams enabled)
	result := idx.Evaluate([]Predicate{Contains("$.description", "hello")})
	if result.Count() != 2 || !result.IsSet(0) || !result.IsSet(2) {
		t.Errorf("CONTAINS on $.description should prune, got RGs %v", result.ToSlice())
	}

	// CONTAINS on $.title should return all RGs (no trigrams)
	result = idx.Evaluate([]Predicate{Contains("$.title", "hello")})
	if result.Count() != 3 {
		t.Errorf("CONTAINS on $.title should return all RGs (graceful degradation), got %d", result.Count())
	}
}

func TestWithFTSPathsPrefix(t *testing.T) {
	config, err := NewConfig(WithFTSPaths("$.content.*"))
	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}
	builder := mustNewBuilder(t, config, 3)

	builder.AddDocument(0, []byte(`{"content": {"body": "hello world", "summary": "test summary"}, "meta": "hello meta"}`))
	builder.AddDocument(1, []byte(`{"content": {"body": "goodbye world", "summary": "another summary"}, "meta": "world meta"}`))
	builder.AddDocument(2, []byte(`{"content": {"body": "hello universe", "summary": "hello summary"}, "meta": "something"}`))

	idx := builder.Finalize()

	// CONTAINS on $.content.body should prune
	result := idx.Evaluate([]Predicate{Contains("$.content.body", "hello")})
	if result.Count() != 2 || !result.IsSet(0) || !result.IsSet(2) {
		t.Errorf("CONTAINS on $.content.body should prune, got RGs %v", result.ToSlice())
	}

	// CONTAINS on $.content.summary should also prune
	result = idx.Evaluate([]Predicate{Contains("$.content.summary", "hello")})
	if result.Count() != 1 || !result.IsSet(2) {
		t.Errorf("CONTAINS on $.content.summary should prune, got RGs %v", result.ToSlice())
	}

	// CONTAINS on $.meta should return all RGs (no trigrams)
	result = idx.Evaluate([]Predicate{Contains("$.meta", "hello")})
	if result.Count() != 3 {
		t.Errorf("CONTAINS on $.meta should return all RGs, got %d", result.Count())
	}
}

func TestWithFTSPathsCanonicalizesEquivalentSupportedPaths(t *testing.T) {
	config, err := NewConfig(WithFTSPaths("$['description']", `$["content"].*`))
	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}
	builder := mustNewBuilder(t, config, 3)

	builder.AddDocument(0, []byte(`{"description": "hello world", "content": {"body": "hello body"}, "meta": "alpha"}`))
	builder.AddDocument(1, []byte(`{"description": "goodbye world", "content": {"body": "goodbye body"}, "meta": "beta"}`))
	builder.AddDocument(2, []byte(`{"description": "hello universe", "content": {"body": "hello again"}, "meta": "gamma"}`))

	idx := builder.Finalize()

	if got, want := idx.Config.ftsPaths, []string{"$.description", "$.content.*"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("fresh ftsPaths = %v, want %v", got, want)
	}

	assertContainsRGs := func(t *testing.T, queryPath string, value string, want []int) {
		t.Helper()
		result := idx.Evaluate([]Predicate{Contains(queryPath, value)})
		if got := result.ToSlice(); len(got) != len(want) {
			t.Fatalf("Contains(%q, %q) RGs = %v, want %v", queryPath, value, got, want)
		} else {
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("Contains(%q, %q) RGs = %v, want %v", queryPath, value, got, want)
				}
			}
		}
	}

	assertContainsRGs(t, "$.description", "hello", []int{0, 2})
	assertContainsRGs(t, "$.content.body", "hello", []int{0, 2})

	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	if got, want := decoded.Config.ftsPaths, []string{"$.description", "$.content.*"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("decoded ftsPaths = %v, want %v", got, want)
	}

	idx = decoded
	assertContainsRGs(t, "$.description", "hello", []int{0, 2})
	assertContainsRGs(t, "$.content.body", "hello", []int{0, 2})
}

func TestWithFTSPathsBackwardCompatible(t *testing.T) {
	// Without WithFTSPaths, all string paths should have trigrams
	config := DefaultConfig()
	builder := mustNewBuilder(t, config, 2)

	builder.AddDocument(0, []byte(`{"description": "hello world", "title": "test title"}`))
	builder.AddDocument(1, []byte(`{"description": "goodbye world", "title": "hello title"}`))

	idx := builder.Finalize()

	// Both paths should support CONTAINS with pruning
	result := idx.Evaluate([]Predicate{Contains("$.description", "hello")})
	if result.Count() != 1 || !result.IsSet(0) {
		t.Errorf("CONTAINS on $.description should prune, got RGs %v", result.ToSlice())
	}

	result = idx.Evaluate([]Predicate{Contains("$.title", "hello")})
	if result.Count() != 1 || !result.IsSet(1) {
		t.Errorf("CONTAINS on $.title should prune, got RGs %v", result.ToSlice())
	}
}

func TestWithFTSPathsOnNumericField(t *testing.T) {
	// Configuring FTS path on a field that has numeric values should gracefully degrade
	config, err := NewConfig(WithFTSPaths("$.score"))
	if err != nil {
		t.Fatalf("failed to create config: %v", err)
	}
	builder := mustNewBuilder(t, config, 2)

	builder.AddDocument(0, []byte(`{"score": 100, "name": "alice"}`))
	builder.AddDocument(1, []byte(`{"score": 200, "name": "bob"}`))

	idx := builder.Finalize()

	// CONTAINS on numeric field should return all RGs (graceful degradation)
	result := idx.Evaluate([]Predicate{Contains("$.score", "100")})
	if result.Count() != 2 {
		t.Errorf("CONTAINS on numeric field should return all RGs, got %d", result.Count())
	}
}

func TestWithFTSPathsRejectsDuplicateCanonicalPaths(t *testing.T) {
	_, err := NewConfig(WithFTSPaths("$.foo", "$['foo']"))
	if err == nil {
		t.Fatal("expected error for duplicate canonical FTS paths, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate canonical FTS path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigAllowsMultipleTransformersPerSourcePath(t *testing.T) {
	cfg, err := NewConfig(
		WithToLowerTransformer("$['email']", "lower"),
		WithEmailDomainTransformer("$.email", "domain"),
	)
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	specs := cfg.representationSpecs["$.email"]
	if len(specs) != 2 {
		t.Fatalf("representationSpecs[$.email] len = %d, want 2", len(specs))
	}

	if got := specs[0].TargetPath; got != "__derived:$.email#lower" {
		t.Fatalf("representationSpecs[$.email][0].TargetPath = %q, want %q", got, "__derived:$.email#lower")
	}
	if got := specs[1].TargetPath; got != "__derived:$.email#domain" {
		t.Fatalf("representationSpecs[$.email][1].TargetPath = %q, want %q", got, "__derived:$.email#domain")
	}

	regs := cfg.representationTransformers["$.email"]
	if len(regs) != 2 {
		t.Fatalf("representationTransformers[$.email] len = %d, want 2", len(regs))
	}
	if regs[0].Alias != "lower" || regs[1].Alias != "domain" {
		t.Fatalf("representationTransformers[$.email] aliases = [%q %q], want [lower domain]", regs[0].Alias, regs[1].Alias)
	}
}

func TestConfigRejectsDuplicateTransformerAlias(t *testing.T) {
	_, err := NewConfig(
		WithToLowerTransformer("$.email", "lower"),
		WithEmailDomainTransformer("$['email']", "lower"),
	)
	if err == nil {
		t.Fatal("NewConfig() error = nil, want duplicate alias failure")
	}
	if !strings.Contains(err.Error(), "duplicate transformer alias") {
		t.Fatalf("NewConfig() error = %v, want duplicate transformer alias", err)
	}
}

func TestBuilderIndexesRawAndCompanionRepresentations(t *testing.T) {
	config, err := NewConfig(
		WithToLowerTransformer("$.email", "lower"),
		WithEmailDomainTransformer("$.email", "domain"),
	)
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	builder := mustNewBuilder(t, config, 2)
	if err := builder.AddDocument(0, []byte(`{"email":"Alice@Example.COM"}`)); err != nil {
		t.Fatalf("AddDocument(0) error = %v", err)
	}
	if err := builder.AddDocument(1, []byte(`{"email":"bob@other.com"}`)); err != nil {
		t.Fatalf("AddDocument(1) error = %v", err)
	}

	idx := builder.Finalize()

	if got := idx.Evaluate([]Predicate{EQ("$.email", "Alice@Example.COM")}).ToSlice(); fmt.Sprint(got) != fmt.Sprint([]int{0}) {
		t.Fatalf(`EQ("$.email", "Alice@Example.COM") = %v, want [0]`, got)
	}

	rawPathID, ok := idx.pathLookup["$.email"]
	if !ok {
		t.Fatal(`pathLookup["$.email"] missing`)
	}
	rawIndex, ok := idx.StringIndexes[rawPathID]
	if !ok {
		t.Fatal(`StringIndexes["$.email"] missing`)
	}
	if got := fmt.Sprint(rawIndex.Terms); got != fmt.Sprint([]string{"Alice@Example.COM", "bob@other.com"}) {
		t.Fatalf("raw terms = %s, want %v", got, []string{"Alice@Example.COM", "bob@other.com"})
	}

	lowerPathID, ok := idx.pathLookup["__derived:$.email#lower"]
	if !ok {
		t.Fatal(`pathLookup["__derived:$.email#lower"] missing`)
	}
	lowerIndex, ok := idx.StringIndexes[lowerPathID]
	if !ok {
		t.Fatal(`StringIndexes["__derived:$.email#lower"] missing`)
	}
	if got := fmt.Sprint(lowerIndex.Terms); got != fmt.Sprint([]string{"alice@example.com", "bob@other.com"}) {
		t.Fatalf("lower terms = %s, want %v", got, []string{"alice@example.com", "bob@other.com"})
	}

	domainPathID, ok := idx.pathLookup["__derived:$.email#domain"]
	if !ok {
		t.Fatal(`pathLookup["__derived:$.email#domain"] missing`)
	}
	domainIndex, ok := idx.StringIndexes[domainPathID]
	if !ok {
		t.Fatal(`StringIndexes["__derived:$.email#domain"] missing`)
	}
	if got := fmt.Sprint(domainIndex.Terms); got != fmt.Sprint([]string{"example.com", "other.com"}) {
		t.Fatalf("domain terms = %s, want %v", got, []string{"example.com", "other.com"})
	}
}

func TestBuilderCanonicalizesSupportedPathVariants(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)

	if err := builder.walkJSON("$['foo']", "alpha", 0); err != nil {
		t.Fatalf("walkJSON() error = %v", err)
	}
	if err := builder.walkJSON(`$["foo"]`, "beta", 1); err != nil {
		t.Fatalf("walkJSON() error = %v", err)
	}

	idx := builder.Finalize()

	if len(idx.PathDirectory) != 1 {
		t.Fatalf("PathDirectory len = %d, want 1", len(idx.PathDirectory))
	}

	entry := idx.PathDirectory[0]
	if entry.PathName != "$.foo" {
		t.Fatalf("PathDirectory[0].PathName = %q, want $.foo", entry.PathName)
	}

	if len(idx.pathLookup) != 1 {
		t.Fatalf("pathLookup len = %d, want 1", len(idx.pathLookup))
	}

	if got, ok := idx.pathLookup["$.foo"]; !ok || got != entry.PathID {
		t.Fatalf("pathLookup[$.foo] = (%d, %v), want (%d, true)", got, ok, entry.PathID)
	}
}

func TestRebuildPathLookupRejectsDuplicateCanonicalPaths(t *testing.T) {
	idx := NewGINIndex()
	idx.PathDirectory = []PathEntry{
		{PathID: 0, PathName: "$.foo"},
		{PathID: 1, PathName: "$['foo']"},
	}

	err := idx.rebuildPathLookup()
	if err == nil {
		t.Fatal("rebuildPathLookup() error = nil, want ErrInvalidFormat")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("rebuildPathLookup() error = %v, want ErrInvalidFormat", err)
	}
}

func TestRebuildPathLookupPreservesInternalRepresentationPaths(t *testing.T) {
	idx := NewGINIndex()
	idx.PathDirectory = []PathEntry{
		{PathID: 0, PathName: "$.email"},
		{PathID: 1, PathName: "__derived:$.email#lower"},
	}

	if err := idx.rebuildPathLookup(); err != nil {
		t.Fatalf("rebuildPathLookup() error = %v", err)
	}
	if got := idx.PathDirectory[1].PathName; got != "__derived:$.email#lower" {
		t.Fatalf("PathDirectory[1].PathName = %q, want %q", got, "__derived:$.email#lower")
	}
	if got, ok := idx.pathLookup["__derived:$.email#lower"]; !ok || got != 1 {
		t.Fatalf(`pathLookup["__derived:$.email#lower"] = (%d, %v), want (1, true)`, got, ok)
	}
}

func TestRebuildPathLookupEmptyDirectory(t *testing.T) {
	idx := NewGINIndex()
	idx.pathLookup = nil

	if err := idx.rebuildPathLookup(); err != nil {
		t.Fatalf("rebuildPathLookup() error = %v", err)
	}
	if idx.pathLookup == nil {
		t.Fatal("pathLookup = nil, want empty map")
	}
	if len(idx.pathLookup) != 0 {
		t.Fatalf("len(pathLookup) = %d, want 0", len(idx.pathLookup))
	}
}

func TestRebuildPathLookupMidDirectoryTruncationPreservesExistingLookupOnError(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)
	builder.AddDocument(0, []byte(`{"foo": "bar", "bar": "baz"}`))
	builder.AddDocument(1, []byte(`{"foo": "qux", "bar": "zap"}`))

	idx := builder.Finalize()

	originalLookup := make(map[string]uint16, len(idx.pathLookup))
	for path, pathID := range idx.pathLookup {
		originalLookup[path] = pathID
	}

	idx.PathDirectory = append(idx.PathDirectory[:1], idx.PathDirectory[2:]...)

	err := idx.rebuildPathLookup()
	if err == nil {
		t.Fatal("rebuildPathLookup() error = nil, want ErrInvalidFormat")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("rebuildPathLookup() error = %v, want ErrInvalidFormat", err)
	}
	if len(idx.pathLookup) != len(originalLookup) {
		t.Fatalf("pathLookup len = %d, want %d", len(idx.pathLookup), len(originalLookup))
	}
	for path, wantPathID := range originalLookup {
		gotPathID, ok := idx.pathLookup[path]
		if !ok || gotPathID != wantPathID {
			t.Fatalf("pathLookup[%q] = (%d, %v), want (%d, true)", path, gotPathID, ok, wantPathID)
		}
	}
}

func TestRebuildPathLookupValidationErrorPreservesExistingLookupOnError(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)
	builder.AddDocument(0, []byte(`{"foo": "bar", "bar": "baz"}`))
	builder.AddDocument(1, []byte(`{"foo": "qux", "bar": "zap"}`))

	idx := builder.Finalize()

	originalLookup := make(map[string]uint16, len(idx.pathLookup))
	for path, pathID := range idx.pathLookup {
		originalLookup[path] = pathID
	}

	idx.PathDirectory[1].PathName = "$.foo_renamed"
	idx.StringIndexes[outOfRangePathID(t, idx)] = &StringIndex{}

	err := idx.rebuildPathLookup()
	if err == nil {
		t.Fatal("rebuildPathLookup() error = nil, want ErrInvalidFormat")
	}
	if !stderrors.Is(err, ErrInvalidFormat) {
		t.Fatalf("rebuildPathLookup() error = %v, want ErrInvalidFormat", err)
	}
	if len(idx.pathLookup) != len(originalLookup) {
		t.Fatalf("pathLookup len = %d, want %d", len(idx.pathLookup), len(originalLookup))
	}
	for path, wantPathID := range originalLookup {
		gotPathID, ok := idx.pathLookup[path]
		if !ok || gotPathID != wantPathID {
			t.Fatalf("pathLookup[%q] = (%d, %v), want (%d, true)", path, gotPathID, ok, wantPathID)
		}
	}
	if _, ok := idx.pathLookup["$.foo_renamed"]; ok {
		t.Fatal("pathLookup unexpectedly contains rebuilt key after validation error")
	}
}

func TestValidateJSONPathRejectsInternalRepresentationPaths(t *testing.T) {
	if err := ValidateJSONPath("__derived:$.email#lower"); err == nil {
		t.Fatal("ValidateJSONPath(internal path) error = nil, want rejection")
	}
}

func TestQueryAliasRoutingUsesAsWrapper(t *testing.T) {
	config, err := NewConfig(
		WithToLowerTransformer("$.email", "lower"),
		WithEmailDomainTransformer("$.email", "domain"),
	)
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	builder := mustNewBuilder(t, config, 3)
	docs := []string{
		`{"email":"Alice@Example.COM"}`,
		`{"email":"bob@other.dev"}`,
		`{"email":"CHARLIE@EXAMPLE.COM"}`,
	}
	for i, doc := range docs {
		if err := builder.AddDocument(DocID(i), []byte(doc)); err != nil {
			t.Fatalf("AddDocument(%d) error = %v", i, err)
		}
	}

	idx := builder.Finalize()

	lower := idx.Evaluate([]Predicate{EQ("$.email", As("lower", "alice@example.com"))}).ToSlice()
	if got := fmt.Sprint(lower); got != fmt.Sprint([]int{0}) {
		t.Fatalf(`EQ("$.email", As("lower", ...)) = %v, want [0]`, lower)
	}

	domain := idx.Evaluate([]Predicate{EQ("$.email", As("domain", "example.com"))}).ToSlice()
	if got := fmt.Sprint(domain); got != fmt.Sprint([]int{0, 2}) {
		t.Fatalf(`EQ("$.email", As("domain", ...)) = %v, want [0 2]`, domain)
	}
}

func TestRawPathQueriesRemainDefaultWithDerivedAliases(t *testing.T) {
	config, err := NewConfig(
		WithToLowerTransformer("$.email", "lower"),
	)
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	builder := mustNewBuilder(t, config, 2)
	if err := builder.AddDocument(0, []byte(`{"email":"Alice@Example.COM"}`)); err != nil {
		t.Fatalf("AddDocument(0) error = %v", err)
	}
	if err := builder.AddDocument(1, []byte(`{"email":"BOB@EXAMPLE.COM"}`)); err != nil {
		t.Fatalf("AddDocument(1) error = %v", err)
	}

	idx := builder.Finalize()

	raw := idx.Evaluate([]Predicate{EQ("$.email", "alice@example.com")}).ToSlice()
	if len(raw) != 0 {
		t.Fatalf(`EQ("$.email", "alice@example.com") = %v, want []`, raw)
	}

	aliased := idx.Evaluate([]Predicate{EQ("$.email", As("lower", "alice@example.com"))}).ToSlice()
	if got := fmt.Sprint(aliased); got != fmt.Sprint([]int{0}) {
		t.Fatalf(`EQ("$.email", As("lower", "alice@example.com")) = %v, want [0]`, aliased)
	}
}

func TestRepresentationsIntrospectionListsAliases(t *testing.T) {
	config, err := NewConfig(
		WithToLowerTransformer("$.email", "lower"),
		WithEmailDomainTransformer("$.email", "domain"),
	)
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	builder := mustNewBuilder(t, config, 1)
	if err := builder.AddDocument(0, []byte(`{"email":"Alice@Example.COM"}`)); err != nil {
		t.Fatalf("AddDocument() error = %v", err)
	}

	idx := builder.Finalize()
	got := idx.Representations("$['email']")
	want := []RepresentationInfo{
		{SourcePath: "$.email", Alias: "domain", Transformer: "email_domain"},
		{SourcePath: "$.email", Alias: "lower", Transformer: "to_lower"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Representations($['email']) = %#v, want %#v", got, want)
	}
}

func TestFinalizePopulatesRepresentationLookup(t *testing.T) {
	config, err := NewConfig(
		WithToLowerTransformer("$.email", "lower"),
		WithEmailDomainTransformer("$.email", "domain"),
	)
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	builder := mustNewBuilder(t, config, 1)
	if err := builder.AddDocument(0, []byte(`{"email":"Alice@Example.COM"}`)); err != nil {
		t.Fatalf("AddDocument() error = %v", err)
	}

	idx := builder.Finalize()
	lookup := idx.representationLookup["$.email"]
	if len(lookup) != 2 {
		t.Fatalf(`representationLookup["$.email"] len = %d, want 2`, len(lookup))
	}

	for alias, target := range map[string]string{
		"lower":  "__derived:$.email#lower",
		"domain": "__derived:$.email#domain",
	} {
		pathID, ok := lookup[alias]
		if !ok {
			t.Fatalf(`representationLookup["$.email"]["%s"] missing`, alias)
		}
		if got := idx.PathDirectory[pathID].PathName; got != target {
			t.Fatalf(`representationLookup["$.email"]["%s"] -> %q, want %q`, alias, got, target)
		}
	}
}

func TestFindPathCanonicalLookupAndFallback(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)
	builder.AddDocument(0, []byte(`{"foo": "bar"}`))
	builder.AddDocument(1, []byte(`{"foo": "baz"}`))

	idx := builder.Finalize()

	for _, path := range []string{"$.foo", "$['foo']", `$["foo"]`} {
		pathID, entry := idx.findPath(path)
		if pathID != 1 {
			t.Fatalf("findPath(%q) pathID = %d, want 1", path, pathID)
		}
		if entry == nil || entry.PathName != "$.foo" {
			t.Fatalf("findPath(%q) entry = %#v, want canonical $.foo entry", path, entry)
		}
	}

	for _, path := range []string{"$.missing", "$.nonexistent"} {
		pathID, entry := idx.findPath(path)
		if pathID != -1 || entry != nil {
			t.Fatalf("findPath(%q) = (%d, %#v), want (-1, nil)", path, pathID, entry)
		}

		result := idx.Evaluate([]Predicate{EQ(path, "bar")})
		if result.Count() != int(idx.Header.NumRowGroups) {
			t.Fatalf("Evaluate(EQ(%q, bar)) count = %d, want %d", path, result.Count(), idx.Header.NumRowGroups)
		}
	}
}

func TestFindPathInvalidPathFallsBackToNoMatch(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)
	builder.AddDocument(0, []byte(`{"foo": "bar"}`))
	builder.AddDocument(1, []byte(`{"foo": "baz"}`))

	idx := builder.Finalize()

	for _, path := range []string{"$.items[0]", "invalid"} {
		pathID, entry := idx.findPath(path)
		if pathID != -1 || entry != nil {
			t.Fatalf("findPath(%q) = (%d, %#v), want (-1, nil)", path, pathID, entry)
		}
	}
}

func TestEvaluateMixedPredicatesPreservesValidPathPruning(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)
	builder.AddDocument(0, []byte(`{"foo": "x"}`))
	builder.AddDocument(1, []byte(`{"foo": "y"}`))
	builder.AddDocument(2, []byte(`{"foo": "x"}`))

	idx := builder.Finalize()

	cases := []struct {
		name       string
		predicates []Predicate
	}{
		{
			name: "missing path after valid predicate",
			predicates: []Predicate{
				EQ("$.foo", "x"),
				EQ("$.nonexistent", "ignored"),
			},
		},
		{
			name: "missing path before valid predicate",
			predicates: []Predicate{
				EQ("$.nonexistent", "ignored"),
				EQ("$.foo", "x"),
			},
		},
		{
			name: "unsupported path after valid predicate",
			predicates: []Predicate{
				EQ("$.foo", "x"),
				EQ("$.items[0]", "ignored"),
			},
		},
		{
			name: "unsupported path before valid predicate",
			predicates: []Predicate{
				EQ("$.items[0]", "ignored"),
				EQ("$.foo", "x"),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := idx.Evaluate(tc.predicates).ToSlice()
			want := []int{0, 2}
			if len(got) != len(want) {
				t.Fatalf("Evaluate(%v) = %v, want %v", tc.predicates, got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("Evaluate(%v) = %v, want %v", tc.predicates, got, want)
				}
			}
		})
	}
}

func TestQueryEQCanonicalPathDecodeParity(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)
	builder.AddDocument(0, []byte(`{"foo": "x"}`))
	builder.AddDocument(1, []byte(`{"foo": "y"}`))
	builder.AddDocument(2, []byte(`{"foo": "x"}`))

	idx := builder.Finalize()

	assertMatches := func(t *testing.T, current *GINIndex, path string, want []int) {
		t.Helper()
		result := current.Evaluate([]Predicate{EQ(path, "x")})
		got := result.ToSlice()
		if len(got) != len(want) {
			t.Fatalf("Evaluate(EQ(%q, x)) = %v, want %v", path, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("Evaluate(EQ(%q, x)) = %v, want %v", path, got, want)
			}
		}
	}

	for _, path := range []string{"$.foo", "$['foo']", `$["foo"]`} {
		assertMatches(t, idx, path, []int{0, 2})
	}

	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	for _, path := range []string{"$.foo", "$['foo']", `$["foo"]`} {
		assertMatches(t, decoded, path, []int{0, 2})
	}
}

func TestEvaluateUnsupportedPathsReturnAllRowGroups(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)
	builder.AddDocument(0, []byte(`{"items": [{"foo": "x"}]}`))
	builder.AddDocument(1, []byte(`{"items": [{"foo": "y"}]}`))

	idx := builder.Finalize()

	for _, path := range []string{"$.items[0]", "$..foo", "$.items[0:5]", "$.items[?(@.price > 10)]", "invalid"} {
		result := idx.Evaluate([]Predicate{EQ(path, "x")})
		if result.Count() != int(idx.Header.NumRowGroups) {
			t.Fatalf("Evaluate(EQ(%q, x)) count = %d, want %d", path, result.Count(), idx.Header.NumRowGroups)
		}
	}
}

func TestStringLengthIndex(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)
	builder.AddDocument(0, []byte(`{"name": "ab"}`))
	builder.AddDocument(1, []byte(`{"name": "abcdef"}`))
	builder.AddDocument(2, []byte(`{"name": "abcd"}`))
	idx := builder.Finalize()

	sli, ok := idx.StringLengthIndexes[1]
	if !ok {
		t.Fatal("StringLengthIndex should exist for $.name")
	}
	if sli.GlobalMin != 2 {
		t.Errorf("expected GlobalMin=2, got %d", sli.GlobalMin)
	}
	if sli.GlobalMax != 6 {
		t.Errorf("expected GlobalMax=6, got %d", sli.GlobalMax)
	}

	result := idx.Evaluate([]Predicate{EQ("$.name", "a")})
	if !result.IsEmpty() {
		t.Error("len=1 query should return empty (min=2)")
	}

	result = idx.Evaluate([]Predicate{EQ("$.name", "abcdefghij")})
	if !result.IsEmpty() {
		t.Error("len=10 query should return empty (max=6)")
	}

	result = idx.Evaluate([]Predicate{EQ("$.name", "abcd")})
	if !result.IsSet(2) {
		t.Error("len=4 query should match RG 2")
	}
}

func TestStringLengthIndexSerialization(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)
	builder.AddDocument(0, []byte(`{"name": "short"}`))
	builder.AddDocument(1, []byte(`{"name": "mediumname"}`))
	builder.AddDocument(2, []byte(`{"name": "verylongname"}`))
	idx := builder.Finalize()

	data, err := Encode(idx)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	idx2, err := Decode(data)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	sli, ok := idx2.StringLengthIndexes[1]
	if !ok {
		t.Fatal("StringLengthIndex should exist after round-trip")
	}
	if sli.GlobalMin != 5 {
		t.Errorf("expected GlobalMin=5, got %d", sli.GlobalMin)
	}
	if sli.GlobalMax != 12 {
		t.Errorf("expected GlobalMax=12, got %d", sli.GlobalMax)
	}

	result := idx2.Evaluate([]Predicate{EQ("$.name", "ab")})
	if !result.IsEmpty() {
		t.Error("len=2 query should return empty after round-trip (min=5)")
	}
}

func TestAddDocumentUsesExplicitParser(t *testing.T) {
	src, err := os.ReadFile("builder.go")
	if err != nil {
		t.Fatalf("read builder.go: %v", err)
	}

	text := string(src)
	if strings.Contains(text, "json.Unmarshal(jsonDoc, &doc)") {
		t.Fatal("AddDocument still uses eager generic unmarshal")
	}
	if !strings.Contains(text, "json.NewDecoder(") {
		t.Fatal("AddDocument should use json.NewDecoder for streaming parse")
	}
	if !strings.Contains(text, ".UseNumber()") {
		t.Fatal("AddDocument should enable UseNumber on the decoder")
	}
}

func TestAddDocumentRejectsUnsupportedNumberWithoutPartialMutation(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 4)

	if err := builder.AddDocument(0, []byte(`{"name":"stable","score":10}`)); err != nil {
		t.Fatalf("seed AddDocument failed: %v", err)
	}

	err := builder.AddDocument(1, []byte(`{"name":"leak","nested":{"label":"should-not-stick"},"score":9223372036854775808}`))
	if err == nil {
		t.Fatal("expected unsupported numeric literal to fail")
	}
	if !strings.Contains(err.Error(), "$.score") {
		t.Fatalf("error should contain path context, got %v", err)
	}

	if builder.numDocs != 1 {
		t.Fatalf("numDocs = %d, want 1", builder.numDocs)
	}
	if _, exists := builder.docIDToPos[DocID(1)]; exists {
		t.Fatalf("docIDToPos contains rejected document: %+v", builder.docIDToPos)
	}
	if len(builder.posToDocID) != 1 {
		t.Fatalf("posToDocID len = %d, want 1", len(builder.posToDocID))
	}
	if builder.nextPos != 1 {
		t.Fatalf("nextPos = %d, want 1", builder.nextPos)
	}
	if _, exists := builder.pathData["$.nested.label"]; exists {
		t.Fatal("rejected document leaked nested path into builder state")
	}

	idx := builder.Finalize()
	if idx.Header.NumDocs != 1 {
		t.Fatalf("finalized NumDocs = %d, want 1", idx.Header.NumDocs)
	}

	namePathID, ok := idx.pathLookup["$.name"]
	if !ok {
		t.Fatal("$.name missing from pathLookup")
	}
	stringIndex, ok := idx.StringIndexes[namePathID]
	if !ok {
		t.Fatal("$.name string index missing")
	}
	for _, term := range stringIndex.Terms {
		if term == "leak" {
			t.Fatal("rejected document term was indexed")
		}
	}

	if _, exists := idx.pathLookup["$.nested.label"]; exists {
		t.Fatal("rejected document path was added to finalized index")
	}
}

func TestNumericIndexPreservesInt64Exactness(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)

	docs := []struct {
		docID DocID
		json  string
	}{
		{0, `{"score":9223372036854775806}`},
		{1, `{"score":9223372036854775807}`},
	}

	for _, doc := range docs {
		if err := builder.AddDocument(doc.docID, []byte(doc.json)); err != nil {
			t.Fatalf("AddDocument(%d) failed: %v", doc.docID, err)
		}
	}

	idx := builder.Finalize()

	exact := idx.Evaluate([]Predicate{EQ("$.score", int64(9223372036854775807))})
	if exact.Count() != 1 || !exact.IsSet(1) || exact.IsSet(0) {
		t.Fatalf("exact int64 EQ result = %v, want [1]", exact.ToSlice())
	}

	rangeResult := idx.Evaluate([]Predicate{GTE("$.score", int64(9223372036854775807))})
	if rangeResult.Count() != 1 || !rangeResult.IsSet(1) || rangeResult.IsSet(0) {
		t.Fatalf("exact int64 GTE result = %v, want [1]", rangeResult.ToSlice())
	}

	lower := idx.Evaluate([]Predicate{LT("$.score", int64(9223372036854775807))})
	if lower.Count() != 1 || !lower.IsSet(0) || lower.IsSet(1) {
		t.Fatalf("exact int64 LT result = %v, want [0]", lower.ToSlice())
	}
}

func TestAddDocumentDuplicateObjectKeysUseLastValue(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 1)

	if err := builder.AddDocument(0, []byte(`{"name":"old","name":"new"}`)); err != nil {
		t.Fatalf("AddDocument() failed: %v", err)
	}

	idx := builder.Finalize()

	oldResult := idx.Evaluate([]Predicate{EQ("$.name", "old")})
	if oldResult.Count() != 0 {
		t.Fatalf(`EQ("$.name", "old") = %v, want []`, oldResult.ToSlice())
	}

	newResult := idx.Evaluate([]Predicate{EQ("$.name", "new")})
	if newResult.Count() != 1 || !newResult.IsSet(0) {
		t.Fatalf(`EQ("$.name", "new") = %v, want [0]`, newResult.ToSlice())
	}
}

func TestIntOnlyNumericIndexExportsPerRGFloatBounds(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)

	docs := []struct {
		docID DocID
		json  string
	}{
		{0, `{"score":10}`},
		{1, `{"score":20}`},
	}

	for _, doc := range docs {
		if err := builder.AddDocument(doc.docID, []byte(doc.json)); err != nil {
			t.Fatalf("AddDocument(%d) failed: %v", doc.docID, err)
		}
	}

	assertNumericBounds := func(t *testing.T, label string, idx *GINIndex) {
		t.Helper()

		scorePathID, ok := idx.pathLookup["$.score"]
		if !ok {
			t.Fatalf("%s: $.score missing from pathLookup", label)
		}

		ni, ok := idx.NumericIndexes[scorePathID]
		if !ok {
			t.Fatalf("%s: $.score missing numeric index", label)
		}
		if ni.ValueType != NumericValueTypeIntOnly {
			t.Fatalf("%s: ValueType = %v, want %v", label, ni.ValueType, NumericValueTypeIntOnly)
		}

		want := []struct {
			intMin int64
			intMax int64
			min    float64
			max    float64
		}{
			{intMin: 10, intMax: 10, min: 10, max: 10},
			{intMin: 20, intMax: 20, min: 20, max: 20},
		}

		for rgID, expected := range want {
			stat := ni.RGStats[rgID]
			if !stat.HasValue {
				t.Fatalf("%s: RGStats[%d].HasValue = false, want true", label, rgID)
			}
			if stat.IntMin != expected.intMin || stat.IntMax != expected.intMax {
				t.Fatalf("%s: RGStats[%d] int bounds = [%d,%d], want [%d,%d]", label, rgID, stat.IntMin, stat.IntMax, expected.intMin, expected.intMax)
			}
			if stat.Min != expected.min || stat.Max != expected.max {
				t.Fatalf("%s: RGStats[%d] float bounds = [%v,%v], want [%v,%v]", label, rgID, stat.Min, stat.Max, expected.min, expected.max)
			}
		}
	}

	idx := builder.Finalize()
	assertNumericBounds(t, "finalized", idx)

	data, err := Encode(idx)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decoded, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	assertNumericBounds(t, "decoded", decoded)
}

func TestIntOnlyRangeQueriesWithFractionalBounds(t *testing.T) {
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

	tests := []struct {
		name string
		pred Predicate
		want []int
	}{
		{
			name: "GT uses floor bound",
			pred: GT("$.score", 1.5),
			want: []int{1, 2},
		},
		{
			name: "GTE uses ceil bound",
			pred: GTE("$.score", 1.5),
			want: []int{1, 2},
		},
		{
			name: "LT uses ceil bound",
			pred: LT("$.score", 2.5),
			want: []int{0, 1},
		},
		{
			name: "LTE uses floor bound",
			pred: LTE("$.score", 2.5),
			want: []int{0, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := idx.Evaluate([]Predicate{tt.pred}).ToSlice(); fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Fatalf("%s = %v, want %v", tt.pred, got, tt.want)
			}
		})
	}
}

func TestMixedNumericPathRejectsLossyPromotion(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)

	if err := builder.AddDocument(0, []byte(`{"score":9007199254740993}`)); err != nil {
		t.Fatalf("seed AddDocument failed: %v", err)
	}

	err := builder.AddDocument(1, []byte(`{"score":1.5}`))
	if err == nil {
		t.Fatal("expected lossy mixed numeric promotion to fail")
	}
	if !strings.Contains(err.Error(), "$.score") {
		t.Fatalf("error should contain path context, got %v", err)
	}

	if builder.numDocs != 1 {
		t.Fatalf("numDocs = %d, want 1", builder.numDocs)
	}

	idx := builder.Finalize()
	result := idx.Evaluate([]Predicate{EQ("$.score", int64(9007199254740993))})
	if result.Count() != 1 || !result.IsSet(0) || result.IsSet(1) {
		t.Fatalf("exact int64 EQ after rejected promotion = %v, want [0]", result.ToSlice())
	}
}

func TestIntOnlyNumericDecodeParity(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)

	docs := []struct {
		docID DocID
		json  string
	}{
		{0, `{"score":9223372036854775806}`},
		{1, `{"score":9223372036854775807}`},
	}

	for _, doc := range docs {
		if err := builder.AddDocument(doc.docID, []byte(doc.json)); err != nil {
			t.Fatalf("AddDocument(%d) failed: %v", doc.docID, err)
		}
	}

	encoded, err := Encode(builder.Finalize())
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	exact := decoded.Evaluate([]Predicate{EQ("$.score", int64(9223372036854775807))})
	if exact.Count() != 1 || !exact.IsSet(1) || exact.IsSet(0) {
		t.Fatalf("decoded exact int64 EQ result = %v, want [1]", exact.ToSlice())
	}

	rangeResult := decoded.Evaluate([]Predicate{GTE("$.score", int64(9223372036854775807))})
	if rangeResult.Count() != 1 || !rangeResult.IsSet(1) || rangeResult.IsSet(0) {
		t.Fatalf("decoded exact int64 GTE result = %v, want [1]", rangeResult.ToSlice())
	}
}
