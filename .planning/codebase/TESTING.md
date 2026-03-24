# Testing Patterns

**Analysis Date:** 2026-03-24

## Test Framework

**Runner:**
- Go standard `testing` package
- `gotest.tools/gotestsum` for CI output (JUnit XML, short-verbose format)
- Config: `Makefile` target `test`

**Property Testing:**
- `github.com/leanovate/gopter` for property-based testing
- 1000 minimum successful tests per property (see `propertyTestParameters()` in `property_test.go:14`)

**Run Commands:**
```bash
go test -v                          # Run all tests verbose
go test -v -run TestQueryEQ         # Run specific test
go test -bench=.                    # Run all benchmarks
go test -bench=BenchmarkQueryEQ     # Run specific benchmark
go test -coverprofile=coverage.out  # Generate coverage
make test                           # Full CI run: gotestsum + JUnit XML + coverage
make lint                           # golangci-lint
```

## Test File Organization

**Location:** All tests co-located in root package `gin` (same directory as source).

**Naming convention:**
| File | Purpose | Lines |
|------|---------|-------|
| `gin_test.go` | Core unit tests (builder, query, serialize, components) | 994 |
| `benchmark_test.go` | Performance benchmarks for all components | 1343 |
| `property_test.go` | Property-based tests for data structures | 471 |
| `integration_property_test.go` | Property-based integration tests (full pipeline) | 656 |
| `generators_test.go` | Test data generators and helpers for property tests | 414 |
| `transformers_test.go` | Unit tests for all field transformers | 951 |
| `transformer_registry_test.go` | Transformer reconstruction/serialization tests | 350 |
| `regex_test.go` | Regex literal extraction and query tests | 275 |
| `docid_test.go` | DocID codec unit tests | 222 |
| `parquet_test.go` | Parquet integration tests (file I/O) | 435 |

**Test-to-source ratio:** ~6111 lines of tests vs ~4526 lines of source (1.35:1).

## Test Structure

### Unit Test Pattern

Individual `TestXxx` functions per behavior. No subtests for simple cases:

```go
func TestBloomFilter(t *testing.T) {
    bf := MustNewBloomFilter(1024, 3)
    bf.AddString("hello")
    bf.AddString("world")

    if !bf.MayContainString("hello") {
        t.Error("bloom filter should contain 'hello'")
    }
    if bf.MayContainString("notpresent") {
        t.Log("bloom filter false positive (expected occasionally)")
    }
}
```

### Table-Driven Test Pattern

Used extensively for testing multiple inputs against expected outputs:

```go
func TestExtractLiterals(t *testing.T) {
    tests := []struct {
        name     string
        pattern  string
        expected []string
    }{
        {
            name:     "simple literal",
            pattern:  "hello",
            expected: []string{"hello"},
        },
        {
            name:     "alternation with common suffix",
            pattern:  "foo|bar|baz",
            expected: []string{"foo", "ba"},
        },
        // ...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            literals, err := ExtractLiterals(tt.pattern)
            if err != nil {
                t.Fatalf("ExtractLiterals(%q) error: %v", tt.pattern, err)
            }
            if len(literals) != len(tt.expected) {
                t.Errorf("ExtractLiterals(%q) = %v, want %v", tt.pattern, literals, tt.expected)
                return
            }
            for i, lit := range literals {
                if lit != tt.expected[i] {
                    t.Errorf("ExtractLiterals(%q)[%d] = %q, want %q", tt.pattern, i, lit, tt.expected[i])
                }
            }
        })
    }
}
```

**Conventions for table-driven tests:**
- Variable named `tests` (plural) with anonymous struct slice
- Struct fields: `name string` always first, then inputs, then expected outputs
- Loop variable: `tt` (from "test table")
- Use `t.Run(tt.name, ...)` for subtests
- Use `t.Fatalf` for setup failures, `t.Errorf` for assertion failures

### Integration Test Pattern (Build-Query)

Most integration tests follow build-finalize-query-assert:

```go
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
```

## Test Helper Functions

**`mustNewBuilder`** - Helper in `gin_test.go:8` that creates a builder or fails the test:
```go
func mustNewBuilder(t *testing.T, config GINConfig, numRGs int, opts ...BuilderOption) *GINBuilder {
    t.Helper()
    builder, err := NewBuilder(config, numRGs, opts...)
    if err != nil {
        t.Fatalf("failed to create builder: %v", err)
    }
    return builder
}
```

**Test data generators** in `generators_test.go`:
- `GenSimpleJSONValue()` - Random JSON values (string, float64, bool)
- `GenJSONDocument(maxDepth)` - Random flat JSON documents
- `GenTestDocs(maxCount)` - Documents with name/age/active/status fields
- `GenTestDocsWithNulls(maxCount)` - Documents with random null values
- `GenMixedTypeDocs(maxCount)` - Constrained docs for multi-predicate testing
- `GenPredicate()` - Random predicates (EQ, GT/GTE/LT/LTE, IsNull/IsNotNull)
- `GenRGSet(maxRGs)`, `GenRGSetPair(maxRGs)`, `GenRGSetTriple(maxRGs)` - Random bitmap sets
- `GenSortedStrings(minLen, maxLen)` - Sorted string slices
- `GenNumericRange()`, `GenHLLPair()` - Specialized generators

**Benchmark helpers** in `benchmark_test.go`:
- `generateTestDoc(i)` - Standard JSON doc with id, name, age, active, status, score, tags
- `generateTestDocWithText(i)` - Doc with description field for text search
- `generateLargeDoc(i, numFields)` - Doc with configurable number of fields
- `generateNestedDoc(i, depth)` - Nested JSON doc with configurable depth
- `generateHighCardinalityDocs(n, cardinality)` - Docs with controlled unique values
- `setupTestIndex(numRGs)` - Build and finalize a standard test index
- `setupTestIndexWithText(numRGs)` - Build and finalize a text search index

**Comparison helpers** in `generators_test.go`:
- `rgSetEqual(a, b)` - Compare two RGSets for equality
- `isSubset(a, b)` - Check if a is a subset of b
- `unionAll(bitmaps)` - Union multiple RGSets
- `findPathEntry(idx, pathName)` - Find a PathEntry by name

**Test types** in `generators_test.go`:
- `TestDoc` - Carries both raw JSON and parsed data for verification
  - `HasFieldValue(field, value)`, `HasFieldNull(field)`, `FieldAbsent(field)` helper methods
- `RGSetPair`, `RGSetTriple` - Pairs/triples for property testing
- `NumericRangePair`, `HLLItemPair` - Specialized test data types

## Property-Based Testing

**Framework:** `gopter` with `prop.ForAll` pattern.

**Configuration:** 1000 minimum successful tests defined in shared helper:
```go
func propertyTestParameters() *gopter.TestParameters {
    params := gopter.DefaultTestParameters()
    params.MinSuccessfulTests = 1000
    return params
}
```

**Pattern:**
```go
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
```

**Property test categories in `property_test.go`:**
- Codec round-trip: `TestPropertyIdentityCodecRoundTrip`, `TestPropertyRowGroupCodecRoundTrip`
- Bloom filter: no false negatives, idempotent add
- RGSet algebra: commutative/associative intersect and union, double-invert identity, De Morgan's laws
- Prefix compression round-trip
- Serialization round-trip
- Trigram search superset guarantee
- HLL merge commutativity, estimate bounds

**Integration property tests in `integration_property_test.go`:**
- Full pipeline no false negatives (query results are superset of actual matches)
- Serialization preserves query results
- Multi-predicate returns intersection of individual results
- Null/present bitmap consistency
- Cardinality threshold controls index type

## Benchmark Practices

**Organization:** Benchmarks in `benchmark_test.go` (1343 lines), organized by section with comment headers:

```
// =============================================================================
// Builder Performance Benchmarks
// =============================================================================
```

**Sections:**
1. Test Data Generators (reusable helpers)
2. Builder Performance (AddDocument, Finalize, memory)
3. Query Performance (EQ, Range, IN, Contains, Null, multi-predicate, vs index size)
4. Serialization Performance (Encode, Decode, round-trip, encoded size)
5. Component Benchmarks - Bloom Filter (add, lookup hit/miss, false positive rate)
6. Component Benchmarks - RGSet (intersect, union, invert, sparsity)
7. Component Benchmarks - Trigram Index (add, search, pattern length)
8. Component Benchmarks - HyperLogLog (add, estimate, merge)
9. Component Benchmarks - Prefix Compression (compress, decompress, ratio)
10. Scaling Benchmarks (row groups, doc size, cardinality, path depth)
11. End-to-End Benchmarks (build+query+serialize)
12. Worst-case Composite Query Benchmarks

**Patterns:**

Sub-benchmarks with parameterized sizes:
```go
func BenchmarkAddDocumentBatch(b *testing.B) {
    sizes := []int{100, 1000, 10000}
    for _, size := range sizes {
        b.Run(fmt.Sprintf("Docs=%d", size), func(b *testing.B) {
            // setup
            b.ResetTimer()
            for i := 0; i < b.N; i++ {
                // benchmark body
            }
        })
    }
}
```

Memory allocation tracking:
```go
func BenchmarkBuilderMemory(b *testing.B) {
    b.ReportAllocs()
    b.ResetTimer()
    // ...
}
```

Custom metrics:
```go
func BenchmarkEncodedSize(b *testing.B) {
    b.ReportMetric(float64(len(data)), "bytes")
    b.ReportMetric(float64(len(data))/float64(size), "bytes/RG")
}

func BenchmarkBloomFalsePositiveRate(b *testing.B) {
    b.ReportMetric(float64(falsePositives)/float64(total)*100, "FP%")
}
```

Setup outside timer for fair measurement:
```go
func BenchmarkFinalize(b *testing.B) {
    for i := 0; i < b.N; i++ {
        b.StopTimer()
        builder, _ := NewBuilder(DefaultConfig(), size)
        for j := 0; j < size; j++ {
            builder.AddDocument(DocID(j), docs[j])
        }
        b.StartTimer()
        _ = builder.Finalize()
    }
}
```

Parallel benchmarks:
```go
func BenchmarkQueryEQParallel(b *testing.B) {
    idx := setupTestIndex(1000)
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            idx.Evaluate([]Predicate{EQ("$.name", "user_42")})
        }
    })
}
```

## Mocking

**No mocking framework used.** Tests use real implementations throughout. This is appropriate because:
- The library has no external service dependencies in core paths
- All data structures are in-memory
- Parquet tests use real temp files via `t.TempDir()`

**What to mock (if needed):** S3 operations in `s3.go` are not tested in the test suite (no `s3_test.go` exists). Parquet tests create real files.

## Fixtures and Factories

**No external fixture files.** All test data generated inline:

```go
// Inline JSON literals for simple tests
builder.AddDocument(0, []byte(`{"name": "alice", "age": 30}`))

// Factory functions for benchmarks
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
```

**Parquet test fixtures** created at test time:
```go
func createTestParquetFile(t *testing.T, path string, records []testRecord, rowsPerRG int64) {
    t.Helper()
    f, err := os.Create(path)
    // ... write records with parquet.NewGenericWriter ...
}
```

## Coverage

**Requirements:** No enforced minimum. Coverage generated by `make test` to `coverage.out`.

**View Coverage:**
```bash
make test                              # Generates coverage.out
go tool cover -html=coverage.out       # View in browser
go tool cover -func=coverage.out       # Print per-function coverage
```

## Test Types

### Unit Tests
- **Scope:** Individual functions and methods
- **Files:** `gin_test.go`, `docid_test.go`, `regex_test.go`, `transformers_test.go`, `transformer_registry_test.go`
- **Approach:** Direct call with known inputs, assert outputs
- **Example functions:** `TestBloomFilter`, `TestRGSet`, `TestIdentityCodec`, `TestISODateToEpochMs`

### Integration Tests
- **Scope:** Full build-query pipeline, serialize-deserialize round-trips
- **Files:** `gin_test.go` (TestSerializeRoundTrip, TestContainsWithSerialize), `parquet_test.go`, `integration_property_test.go`
- **Approach:** Build index from documents, query, verify results; serialize and verify queries produce same results
- **Example functions:** `TestSerializeRoundTrip`, `TestContainsWithSerialize`, `TestPropertyIntegrationFullPipelineNoFalseNegatives`

### Property-Based Tests
- **Scope:** Mathematical properties and invariants
- **Files:** `property_test.go`, `integration_property_test.go`
- **Approach:** gopter generates random inputs, verify properties hold for all (1000 runs)
- **Properties tested:**
  - Set algebra (commutativity, associativity, De Morgan's)
  - Round-trip fidelity (codec, serialization, compression)
  - No false negatives (bloom filter, trigram search, full pipeline)
  - Estimate bounds (HLL within 3σ)

### Benchmarks
- **Scope:** Performance of all components and operations
- **File:** `benchmark_test.go` (1343 lines)
- **Approach:** Parameterized sub-benchmarks testing various sizes and scenarios
- **Categories:** Builder, Query, Serialization, Bloom, RGSet, Trigram, HLL, Prefix, Scaling, E2E

### E2E Tests
- **No separate E2E framework.** End-to-end coverage achieved through integration property tests that exercise build -> serialize -> deserialize -> query pipeline.

## Test Coverage Gaps

**Untested areas:**
- `s3.go` - No `s3_test.go` exists. S3 operations (read/write sidecar, build from S3 Parquet) are entirely untested.
- `serialize.go` - Individual encoding functions not unit-tested; only tested via round-trip integration tests. Edge cases in binary format parsing not directly tested.
- Error paths in `serialize.go` - Corrupted data decoding, truncated input.
- `jsonpath.go` - `ParseJSONPath()` return value not tested (only validation tested).
- Concurrent access patterns - Only `BenchmarkQueryEQParallel` tests parallelism. No concurrent builder tests.

## Common Patterns

**Async Testing:** Not applicable - library is synchronous. Tests are all synchronous.

**Error Testing:**
```go
func TestBuilderWithNilCodec(t *testing.T) {
    _, err := NewBuilder(DefaultConfig(), 3, WithCodec(nil))
    if err == nil {
        t.Fatal("expected error when creating builder with nil codec")
    }
}

func TestCompressionInvalidLevel(t *testing.T) {
    tests := []struct {
        name  string
        level CompressionLevel
    }{
        {"negative", CompressionLevel(-1)},
        {"too_high", CompressionLevel(20)},
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
```

**Round-trip testing (serialize/deserialize):**
```go
func TestSerializeRoundTrip(t *testing.T) {
    // Build index
    builder := mustNewBuilder(t, DefaultConfig(), 3)
    builder.AddDocument(0, []byte(`{"name": "alice", "age": 30}`))
    idx := builder.Finalize()

    // Encode
    encoded, err := Encode(idx)
    if err != nil {
        t.Fatalf("encode failed: %v", err)
    }

    // Decode
    decoded, err := Decode(encoded)
    if err != nil {
        t.Fatalf("decode failed: %v", err)
    }

    // Verify structural equality
    if decoded.Header.NumDocs != idx.Header.NumDocs {
        t.Errorf("NumDocs mismatch")
    }

    // Verify functional equality (queries produce same results)
    result := decoded.Evaluate([]Predicate{EQ("$.name", "alice")})
    if !result.IsSet(0) {
        t.Error("query on decoded index failed")
    }
}
```

**Transformer testing pattern:**
```go
func TestISODateToEpochMs(t *testing.T) {
    tests := []struct {
        name    string
        input   any
        wantOk  bool
        wantVal float64
    }{
        {"valid RFC3339", "2024-01-15T10:30:00Z", true, float64(...)},
        {"invalid date string", "not-a-date", false, 0},
        {"non-string input", 12345, false, 0},
        {"nil input", nil, false, 0},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, ok := ISODateToEpochMs(tt.input)
            if ok != tt.wantOk {
                t.Errorf("ok = %v, want %v", ok, tt.wantOk)
                return
            }
            if ok && result.(float64) != tt.wantVal {
                t.Errorf("result = %v, want %v", result, tt.wantVal)
            }
        })
    }
}
```

---

*Testing analysis: 2026-03-24*
