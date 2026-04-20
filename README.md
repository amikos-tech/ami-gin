# GIN Index

[![CI](https://github.com/amikos-tech/ami-gin/actions/workflows/ci.yml/badge.svg?branch=main&event=push)](https://github.com/amikos-tech/ami-gin/actions/workflows/ci.yml) [![Go Reference](https://pkg.go.dev/badge/github.com/amikos-tech/ami-gin.svg)](https://pkg.go.dev/github.com/amikos-tech/ami-gin) [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

See [CONTRIBUTING.md](CONTRIBUTING.md) for local contributor workflows and [SECURITY.md](SECURITY.md) for disclosure guidance.

A Generalized Inverted Index (GIN) for JSON data, designed for row-group pruning in columnar storage formats like Parquet.

## Features

- **String indexing** - Exact match and IN queries on string fields
- **Numeric indexing** - Range queries (GT, GTE, LT, LTE) with per-row-group min/max stats
- **Field transformers** - Convert values (e.g., date strings to epoch) for efficient range queries
- **Trigram indexing** - Full-text CONTAINS queries using n-gram matching
- **Regex support** - Pattern matching with trigram-based candidate selection
- **Null tracking** - IS NULL / IS NOT NULL predicates
- **Bloom filter** - Fast-path rejection for non-existent values
- **HyperLogLog** - Efficient cardinality estimation
- **Compression** - zstd-compressed binary serialization
- **Parquet integration** - Build from Parquet, embed in metadata, sidecar files, S3 support
- **CLI tool** - Command-line interface for build, query, info, and extract operations

## Why GIN Index?

**A serverless pruning index for data lakes** - the GIN index is a compact, immutable index designed to answer one question: "Which row groups might contain my data?"

### The Problem

Querying large data lakes is expensive. When you search for `trace_id=abc123` across millions of Parquet files, traditional approaches either:
- **Full scan** - Read every row group (~TB of data, high latency, high cost)
- **Database approach** - Run PostgreSQL/Elasticsearch cluster (~ms latency, operational burden)
- **Parquet stats** - Use built-in min/max (useless for high-cardinality strings)

### The Solution

```
┌─────────────────────────────────────────────────────────────────────┐
│                    Serverless Row-Group Pruning                      │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   1. Cache index anywhere         2. Prune locally     3. Read only  │
│      (<1MB for millions of files)    (~1µs)              matching RGs│
│                                                                      │
│   ┌──────────────┐               ┌──────────────┐    ┌────────────┐ │
│   │  memcached   │  ─────────▶   │  GIN Index   │ ─▶ │ S3/GCS     │ │
│   │  nginx       │    decode     │  Evaluate()  │    │ [RG 5, 23] │ │
│   │  CDN edge    │               │              │    │            │ │
│   │  localStorage│               │ Result: 3    │    │ Skip 99%   │ │
│   └──────────────┘               │ row groups   │    │ of data    │ │
│                                  └──────────────┘    └────────────┘ │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### Key Advantages

| Challenge | PostgreSQL GIN | Elasticsearch | This GIN Index |
|-----------|---------------|---------------|----------------|
| **Deployment** | Database cluster | Search cluster | **Just bytes** - cache anywhere |
| **Query latency** | ~1ms | ~5-10ms | **~1µs** - client-side |
| **High cardinality** | Index bloat | Shard overhead | **adaptive-hybrid hot-value pruning** |
| **Index size** | MB-GB | GB | **~30KB per 1K row groups** |
| **Arbitrary JSON** | Schema required | Mapping required | **Auto-discovered paths** |

### Designed For

- **Log/observability platforms** - Query by `trace_id`, `request_id`, arbitrary labels
- **Vector databases** - Pre-filter segments before expensive ANN search
- **Data lake query engines** - Pruning index for DuckDB, Trino, Spark
- **Edge/serverless** - Cache index at CDN edge, query without backend

The index decouples **pruning** (which row groups to read) from **execution** (DuckDB, Trino, Spark). Your query engine handles the actual data reading - this index just tells it where to look.

## Installation

```bash
go get github.com/amikos-tech/ami-gin
```

## Quick Start

```go
package main

import (
    "fmt"
    gin "github.com/amikos-tech/ami-gin"
)

func main() {
    // Create builder for 3 row groups
    builder, err := gin.NewBuilder(gin.DefaultConfig(), 3)
    if err != nil {
        panic(err)
    }

    // Add documents to row groups
    builder.AddDocument(0, []byte(`{"name": "alice", "age": 30}`))
    builder.AddDocument(1, []byte(`{"name": "bob", "age": 25}`))
    builder.AddDocument(2, []byte(`{"name": "alice", "age": 40}`))

    // Build index
    idx := builder.Finalize()

    // Query: find row groups where name = "alice"
    result := idx.Evaluate([]gin.Predicate{
        gin.EQ("$.name", "alice"),
    })
    fmt.Println(result.ToSlice()) // [0, 2]
}
```

## Known limitations

GIN Index v0.2.0 expands the original predicate surface with adaptive high-cardinality pruning and derived representations, but it still intentionally excludes a few deferred capabilities.

- OR/AND composites are not part of the v0.2.0 query API yet.
- Index merge across multiple index files is intentionally deferred beyond v0.2.0.
- Query-time transformers are not supported in v0.2.0; transformations must happen at index-build time.

Serialized index compatibility remains strict: `Decode()` rejects older payload versions. Indexes built with `v0.1.0` (wire format `v3`) must be rebuilt with `v0.2.0` (wire format `v9`).

## Query Types

### Equality

```go
gin.EQ("$.status", "active")
gin.NE("$.status", "deleted")
gin.IN("$.status", "active", "pending", "review")
gin.NIN("$.status", "deleted", "archived")  // NOT IN
```

### Numeric Range

```go
gin.GT("$.price", 100.0)    // price > 100
gin.GTE("$.price", 100.0)   // price >= 100
gin.LT("$.price", 500.0)    // price < 500
gin.LTE("$.price", 500.0)   // price <= 500

// Combined range
idx.Evaluate([]gin.Predicate{
    gin.GTE("$.price", 100.0),
    gin.LTE("$.price", 500.0),
})
```

### Derived Representation Queries

Derived representations add companion indexes without dropping the raw source value. Raw-path queries stay raw by default; query a companion explicitly with `gin.As(alias, value)`. Hidden internal target paths are not part of the public query contract.

```go
config, _ := gin.NewConfig(
    gin.WithISODateTransformer("$.created_at", "epoch_ms"),
    gin.WithToLowerTransformer("$.email", "lower"),
    gin.WithEmailDomainTransformer("$.email", "domain"),
    gin.WithRegexExtractTransformer("$.message", "error_code", `ERROR\[(\w+)\]:`, 1),
)
builder, _ := gin.NewBuilder(config, numRGs)

builder.AddDocument(0, []byte(`{
    "created_at": "2024-07-10T09:00:00Z",
    "email": "Alice@Example.COM",
    "message": "ERROR[E1001]: Connection timeout"
}`))

idx := builder.Finalize()

// Raw source-path queries still use the original value.
raw := idx.Evaluate([]gin.Predicate{
    gin.EQ("$.created_at", "2024-07-10T09:00:00Z"),
})

// Alias queries opt into the derived companion explicitly.
july2024 := float64(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC).UnixMilli())
dateResult := idx.Evaluate([]gin.Predicate{
    gin.GTE("$.created_at", gin.As("epoch_ms", july2024)),
})
lowerResult := idx.Evaluate([]gin.Predicate{
    gin.EQ("$.email", gin.As("lower", "alice@example.com")),
})
domainResult := idx.Evaluate([]gin.Predicate{
    gin.EQ("$.email", gin.As("domain", "example.com")),
})
errorResult := idx.Evaluate([]gin.Predicate{
    gin.EQ("$.message", gin.As("error_code", "E1001")),
})
```

**Built-in additive helpers:**
- Date/time: `WithISODateTransformer(path, alias)`, `WithDateTransformer(path, alias)`, `WithCustomDateTransformer(path, alias, layout)`
- String normalization: `WithToLowerTransformer(path, alias)`, `WithEmailDomainTransformer(path, alias)`, `WithURLHostTransformer(path, alias)`
- Extracted subfields: `WithRegexExtractTransformer(path, alias, pattern, group)`, `WithRegexExtractIntTransformer(path, alias, pattern, group)`
- Numeric companions: `WithIPv4Transformer(path, alias)`, `WithSemVerTransformer(path, alias)`, `WithDurationTransformer(path, alias)`, `WithNumericBucketTransformer(path, alias, size)`, `WithBoolNormalizeTransformer(path, alias)`

**Custom companions:**
```go
myTransformer := func(v any) (any, bool) {
    s, ok := v.(string)
    if !ok {
        return nil, false
    }
    return strings.ToUpper(s), true
}

config, _ := gin.NewConfig(
    gin.WithCustomTransformer("$.my_field", "upper", myTransformer),
)
```

`WithCustomTransformer(...)` works for in-memory indexes, but opaque custom companions are not serializable. `Encode()` rejects them because the function cannot be reconstructed on `Decode()`.

**Example: IP subnet queries**
```go
config, _ := gin.NewConfig(
    gin.WithIPv4Transformer("$.client_ip", "ipv4_int"),
)

start, end, _ := gin.CIDRToRange("192.168.1.0/24")
result := idx.Evaluate([]gin.Predicate{
    gin.GTE("$.client_ip", gin.As("ipv4_int", start)),
    gin.LTE("$.client_ip", gin.As("ipv4_int", end)),
})
```

**Example: Version range queries**
```go
config, _ := gin.NewConfig(
    gin.WithSemVerTransformer("$.version", "semver_int"),
)

result := idx.Evaluate([]gin.Predicate{
    gin.GTE("$.version", gin.As("semver_int", float64(2000000))),
})
```

**Example: Case-insensitive email queries**
```go
config, _ := gin.NewConfig(
    gin.WithToLowerTransformer("$.email", "lower"),
)

result := idx.Evaluate([]gin.Predicate{
    gin.EQ("$.email", gin.As("lower", "alice@example.com")),
})
```

**Example: Extract error codes from log messages**
```go
config, _ := gin.NewConfig(
    gin.WithRegexExtractTransformer("$.message", "error_code", `ERROR\[(\w+)\]:`, 1),
)

result := idx.Evaluate([]gin.Predicate{
    gin.EQ("$.message", gin.As("error_code", "E1234")),
})
```

### Full-Text Search (CONTAINS)

```go
// Uses trigram index for substring matching
gin.Contains("$.description", "hello")
gin.Contains("$.title", "database")  // matches "database", "databases", etc.
```

### Regex Matching

```go
// Uses trigram index for regex candidate selection
gin.Regex("$.message", "ERROR|WARNING")        // Alternation
gin.Regex("$.brand", "Toyota|Tesla|Ford")      // Multiple literals
gin.Regex("$.log", "error.*timeout")           // Prefix + wildcard + suffix
gin.Regex("$.code", "[A-Z]{3}_[0-9]+")         // Pattern with literals
```

The Regex operator extracts literal strings from regex patterns and uses the trigram index for candidate row-group selection. This enables efficient pruning before actual regex matching.

**How it works:**
1. Parse regex pattern and extract literal substrings
2. For alternations like `(error|warn)_message`, extracts combined literals: `["error_message", "warn_message"]`
3. Query trigram index for each literal
4. Union results (OR semantics for alternation)
5. Row groups not containing any literal are pruned

**Limitations:**
- Requires trigram index enabled (`EnableTrigrams: true`)
- Literals shorter than trigram length (default: 3) cannot prune
- Pure wildcard patterns (`.*`) return all row groups
- This is **candidate selection**, not regex execution - actual matching happens at query time

### Null Handling

```go
gin.IsNull("$.optional_field")
gin.IsNotNull("$.required_field")
```

### Nested Fields and Arrays

```go
// Nested objects
gin.EQ("$.user.address.city", "New York")

// Array elements (wildcard)
gin.EQ("$.tags[*]", "important")
gin.IN("$.roles[*]", "admin", "editor")
```

## JSONPath Support

Supported path syntax:
- `$` - root
- `$.field` - dot notation
- `$['field']` - bracket notation
- `$.items[*]` - array wildcard

Not supported (will error):
- `$.items[0]` - array indices
- `$..field` - recursive descent
- `$.items[0:5]` - slices
- `$[?(@.price > 10)]` - filters

Validate paths before use:

```go
if err := gin.ValidateJSONPath("$.user.name"); err != nil {
    log.Fatal(err)
}
```

## Serialization

```go
// Encode to bytes (zstd compressed)
data, err := gin.Encode(idx)

// Save to file
os.WriteFile("index.gin", data, 0644)

// Load and decode
data, _ := os.ReadFile("index.gin")
idx, err := gin.Decode(data)
```

## Parquet Integration

The GIN index integrates directly with Parquet files, supporting three storage strategies:

1. **Sidecar file** - Index stored as `data.parquet.gin` alongside the Parquet file
2. **Embedded metadata** - Index stored in Parquet file's key-value metadata
3. **Build-time embedding** - Index built and embedded during Parquet file creation

### Build Index from Parquet

```go
// Build index from a Parquet file's JSON column
idx, err := gin.BuildFromParquet("data.parquet", "attributes", gin.DefaultConfig())
```

### Sidecar Workflow

```go
// Write index as sidecar file (data.parquet.gin)
err := gin.WriteSidecar("data.parquet", idx)

// Read sidecar
idx, err := gin.ReadSidecar("data.parquet")

// Check if sidecar exists
if gin.HasSidecar("data.parquet") {
    // ...
}
```

### Embedded Metadata Workflow

```go
cfg := gin.DefaultParquetConfig() // MetadataKey: "gin.index"

// Rebuild existing Parquet file with embedded index
err := gin.RebuildWithIndex("data.parquet", idx, cfg)

// Check if Parquet has embedded index
hasIdx, err := gin.HasGINIndex("data.parquet", cfg)

// Read embedded index
idx, err := gin.ReadFromParquetMetadata("data.parquet", cfg)
```

### Auto-Loading (Embedded First, Then Sidecar)

```go
// Tries embedded metadata first, falls back to sidecar
idx, err := gin.LoadIndex("data.parquet", gin.DefaultParquetConfig())
```

### Encode for Parquet Metadata (Build-Time Embedding)

When creating a new Parquet file, you can embed the index during creation:

```go
// Get key-value pair for Parquet metadata
key, value, err := gin.EncodeToMetadata(idx, gin.DefaultParquetConfig())
// key = "gin.index", value = base64-encoded compressed index

// Use with parquet-go writer
writer := parquet.NewGenericWriter[Record](f,
    parquet.KeyValueMetadata(key, value),
)
```

### Batch Processing (Programmatic)

Helper functions for working with multiple files:

```go
// Local filesystem
if gin.IsDirectory("./data") {
    // List all .parquet files in directory
    parquetFiles, err := gin.ListParquetFiles("./data")

    // List all .gin files in directory
    ginFiles, err := gin.ListGINFiles("./data")

    // Process each file
    for _, f := range parquetFiles {
        idx, _ := gin.BuildFromParquet(f, "attributes", gin.DefaultConfig())
        gin.WriteSidecar(f, idx)
    }
}

// S3
s3Client, _ := gin.NewS3ClientFromEnv()

// List all .parquet files under prefix
parquetKeys, err := s3Client.ListParquetFiles("bucket", "data/")

// List all .gin files under prefix
ginKeys, err := s3Client.ListGINFiles("bucket", "data/")
```

### S3 Support

All operations support S3 paths via AWS SDK v2:

```go
// Configure from environment variables:
// AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION
// AWS_ENDPOINT_URL (for MinIO, LocalStack), AWS_S3_PATH_STYLE=true
s3Client, err := gin.NewS3ClientFromEnv()

// Build from S3
idx, err := s3Client.BuildFromParquet("bucket", "path/to/data.parquet", "attributes", gin.DefaultConfig())

// Write sidecar to S3
err := s3Client.WriteSidecar("bucket", "path/to/data.parquet", idx)

// Read sidecar from S3
idx, err := s3Client.ReadSidecar("bucket", "path/to/data.parquet")

// Load index (tries embedded, then sidecar)
idx, err := s3Client.LoadIndex("bucket", "path/to/data.parquet", gin.DefaultParquetConfig())
```

### CLI Tool

A command-line tool is provided for common operations:

```bash
# Install
go install github.com/amikos-tech/ami-gin/cmd/gin-index@latest

# Build sidecar index
gin-index build -c attributes data.parquet
gin-index build -c attributes -o custom.gin data.parquet

# Build and embed into Parquet file
gin-index build -c attributes -embed data.parquet

# Query index
gin-index query data.parquet.gin '$.status = "error"'
gin-index query data.parquet.gin '$.count > 100'
gin-index query data.parquet.gin '$.name IN ("alice", "bob")'

# Show index info
gin-index info data.parquet.gin

# Extract embedded index to sidecar
gin-index extract -o data.parquet.gin data.parquet

# S3 paths (uses AWS env vars)
gin-index build -c attributes s3://bucket/data.parquet
gin-index query s3://bucket/data.parquet.gin '$.status = "ok"'
```

**Batch Processing (Directory/S3 Prefix):**

Process multiple files at once by passing a directory or S3 prefix:

```bash
# Build index for all .parquet files in a directory
gin-index build -c attributes ./data/
gin-index build -c attributes -embed ./data/

# Query all .gin files in a directory
gin-index query ./data/ '$.status = "error"'

# Show info for all .gin files
gin-index info ./data/

# S3 prefix - processes all .parquet files under the prefix
gin-index build -c attributes s3://bucket/data/
gin-index query s3://bucket/data/ '$.status = "error"'
gin-index info s3://bucket/data/

# Glob patterns work too
gin-index build -c attributes './data/*.parquet'
gin-index query './data/*.gin' '$.level = "error"'
```

**CLI Query Syntax:**
- Equality: `$.field = "value"`, `$.field != "value"`
- Numeric: `$.field > 100`, `$.field >= 100`, `$.field < 100`, `$.field <= 100`
- IN/NOT IN: `$.field IN ("a", "b")`, `$.field NOT IN (1, 2, 3)`
- Null: `$.field IS NULL`, `$.field IS NOT NULL`
- Contains: `$.field CONTAINS "substring"`
- Regex: `$.field REGEX "pattern"` (e.g., `$.brand REGEX "Toyota|Tesla"`)

## Configuration

```go
config := gin.GINConfig{
    CardinalityThreshold:    10000, // Exact below threshold, adaptive above it
    BloomFilterSize:         65536,
    BloomFilterHashes:       5,
    EnableTrigrams:          true,  // Enable CONTAINS queries
    TrigramMinLength:        3,
    HLLPrecision:            12,    // HyperLogLog precision (4-16)
    PrefixBlockSize:         16,
    AdaptiveMinRGCoverage:   2,     // Promote values seen in at least 2 row groups
    AdaptivePromotedTermCap: 64,    // Keep at most 64 exact hot terms per path
    AdaptiveCoverageCeiling: 0.80,  // Skip terms that cover more than 80% of row groups
    AdaptiveBucketCount:     128,   // Fixed bucket count for long-tail fallback
}

builder, err := gin.NewBuilder(config, numRowGroups)
if err != nil {
    panic(err)
}
```

### High-Cardinality String Modes

GIN Index uses three string-path modes:

- `exact` - path cardinality stays under `CardinalityThreshold`, so every observed value keeps an exact row-group bitmap.
- `adaptive-hybrid` - path exceeds `CardinalityThreshold`, but hot values still retain exact row-group pruning while the long tail falls back to fixed hash buckets.
- `bloom-only` - adaptive promotion is disabled, so high-cardinality paths keep only the bloom filter fast-path.

The additive adaptive knobs above control when a hot value is promoted (`AdaptiveMinRGCoverage`), how many promoted values are kept (`AdaptivePromotedTermCap`), how broad a promoted value is allowed to be (`AdaptiveCoverageCeiling`), and how much compact fallback space is reserved for the long tail (`AdaptiveBucketCount`). This means hot values on a high-cardinality path can still retain exact row-group pruning instead of degrading immediately to bloom-only behavior.

## Examples

See the [examples](./examples) directory:

```bash
go run ./examples/basic/main.go        # Equality queries
go run ./examples/range/main.go        # Numeric ranges
go run ./examples/transformers/main.go # Date field transformers
go run ./examples/transformers-advanced/main.go # IP, SemVer, email, regex transformers
go run ./examples/fulltext/main.go     # CONTAINS queries
go run ./examples/regex/main.go        # Regex pattern matching
go run ./examples/null/main.go         # NULL handling
go run ./examples/nested/main.go       # Nested JSON and arrays
go run ./examples/serialize/main.go    # Persistence
go run ./examples/full/main.go         # All types and operators
go run ./examples/parquet/main.go      # Parquet integration (sidecar, embedded, queries)
```

## Benchmarks

Run benchmarks with:

```bash
go test -bench=. -benchmem -benchtime=1s
```

### Phase 11 Real-Corpus Workflow

The default Phase 11 path stays smoke-only:

```bash
go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=smoke' -benchtime=1x -count=1 -benchmem
```

Opt-in external tiers use the pinned `common-pile/github_archive` snapshot root and the exact env vars `GIN_PHASE11_GITHUB_ARCHIVE_ROOT`, `GIN_PHASE11_ENABLE_SUBSET`, and `GIN_PHASE11_ENABLE_LARGE`.

Supported acquisition example:

```python
from huggingface_hub import snapshot_download

snapshot_download(
    repo_id="common-pile/github_archive",
    repo_type="dataset",
    revision="93d90fbdbc8f06c1fab72e74d5270dc897e1a090",
    allow_patterns="gharchive/v0/documents/*.jsonl.gz",
)
```

With the snapshot root exported once:

```bash
export GIN_PHASE11_GITHUB_ARCHIVE_ROOT=/path/to/common-pile-github-archive
GIN_PHASE11_ENABLE_SUBSET=1 go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=subset' -benchtime=1x -count=1 -benchmem
GIN_PHASE11_ENABLE_LARGE=1 go test ./... -run '^$' -bench 'BenchmarkPhase11RealCorpus/tier=large' -benchtime=1x -count=1 -benchmem
```

Tier notes:
- Smoke: checked-in 640-row synthesized fixture; default path for lightweight local verification.
- Subset: opt-in only; first 4 shards from `gharchive/v0/documents/*.jsonl.gz`; moderate local disk, RAM, and runtime cost.
- Large: opt-in only; first 32 shards from `gharchive/v0/documents/*.jsonl.gz`; materially higher local disk, RAM, and runtime cost.

Pinned provenance and interpretation live in [`11-BENCHMARK-RESULTS.md`](./.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-BENCHMARK-RESULTS.md) and [`11-REAL-CORPUS-REPORT.md`](./.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-REAL-CORPUS-REPORT.md).

### Performance Summary (Apple M3 Max)

| Operation | Latency | Notes |
|-----------|---------|-------|
| EQ query | ~1µs | Bloom filter + sorted term lookup |
| Range query (GT/LT) | 4-24µs | Min/max stats scan |
| IN query (10 values) | ~8µs | Union of EQ results |
| CONTAINS query | 2-17µs | Trigram intersection |
| IsNull/IsNotNull | 2-4µs | Bitmap lookup |
| Bloom lookup | ~100ns | Fast path rejection |
| AddDocument | ~43µs | JSON parsing + indexing |
| Encode (1K RGs) | ~4ms | zstd compression |
| Decode (1K RGs) | ~2ms | zstd decompression |

### Index Size

| Row Groups | Encoded Size | Per RG |
|------------|--------------|--------|
| 100 | 6.7 KB | 67 bytes |
| 500 | 18 KB | 36 bytes |
| 1,000 | 30 KB | 30 bytes |
| 2,000 | 51 KB | 26 bytes |

### Scaling Characteristics

**Query time scales well with index size:**
- 10 RGs: ~340ns
- 100 RGs: ~530ns
- 1,000 RGs: ~680ns
- 5,000 RGs: ~800ns

**Build time is linear with document count and complexity:**
- 100 docs (7 fields): ~1.2ms
- 1,000 docs: ~6.7ms
- High cardinality (10K unique values): ~3.3ms per 1K docs

### Component Performance

| Component | Operation | Latency |
|-----------|-----------|---------|
| Bloom Filter | Add | ~100ns |
| Bloom Filter | Lookup | ~100ns |
| RGSet (10K) | Intersect | ~12µs |
| RGSet (10K) | Union | ~10µs |
| Trigram | Add (50 chars) | ~16µs |
| Trigram | Search | 1-6µs |
| HyperLogLog | Add | ~70ns |
| HyperLogLog | Estimate | 7-410µs (precision dependent) |
| Prefix Compress | 1K terms | ~60µs |

### Real-World Scenario: 1M Docs / 50K Row Groups

Simulating a log storage scenario:
- **1M documents** across **50K row groups** (~20 docs/RG)
- **10 labels**: 2 integers (`status_code`, `duration_ms`) + 8 strings
- Mix of cardinalities: `trace_id` (high), `service` (low), `host` (medium)
- **Trigrams disabled** (no FTS)

| Metric | Value |
|--------|-------|
| **Index Size** | **289 KB** (0.28 MB) |
| Bytes per RG | 5.9 bytes |
| Bytes per doc | 0.3 bytes |
| Build time | 464ms |
| Encode | 41ms |
| Decode | 41ms |

**Query Performance:**

| Query | Latency | Notes |
|-------|---------|-------|
| `trace_id=X` (high cardinality) | **950ns** | adaptive-hybrid hot-term prune or compact tail fallback |
| `service=api` (low cardinality) | 6.5µs | ~10K RGs match |
| `trace_id=X AND level=error` | 6µs | High card + low card |
| `duration_ms > 5000` | 244µs | Range scan over 50K RGs |
| `service=api AND env=prod AND status>=400` | 285µs | 3 predicates combined |

**Key takeaway:** High-cardinality lookups (trace ID, request ID) are **sub-microsecond**. The entire index for 1M documents fits in **289 KB** - easily cacheable in memory, localStorage, or CDN edge.

### Benchmark Categories

The benchmark suite (`benchmark_test.go`) covers:

1. **Builder Performance** - Document ingestion, batch loading, finalization
2. **Query Performance** - All operators, parallel queries, multiple predicates
3. **Serialization** - Encode/decode latency, compression ratios
4. **Components** - Bloom filter, RGSet, trigram, HLL, prefix compression
5. **Scaling** - Row group count, document size, cardinality, nesting depth

## Comparison with Other Solutions

### vs PostgreSQL GIN/JSONB

| Aspect | This GIN Index | PostgreSQL GIN |
|--------|----------------|----------------|
| **Query Latency** | ~1µs (EQ) | ~0.7-1.2ms per predicate |
| **Deployment** | Embedded bytes, no server | Requires PostgreSQL server |
| **Cacheability** | Cache anywhere (nginx, memcached, CDN) | Tied to database buffer cache |
| **Index Size** | 26-67 bytes/row-group | Larger, includes posting lists |
| **Range Queries** | Native min/max stats | Poor (GIN doesn't support ranges) |
| **Full-Text** | Trigram-based CONTAINS | Full-featured tsvector/tsquery |
| **ACID** | No (read-only after build) | Full transaction support |

PostgreSQL GIN uses [Bitmap Index Scans](https://pganalyze.com/blog/gin-index) which cost ~0.7-1.2ms each when cached. This index achieves ~1µs queries by being purpose-built for row-group pruning with simpler data structures.

### vs Parquet Built-in Statistics

| Aspect | This GIN Index | Parquet Min/Max Stats | Parquet Bloom Filters |
|--------|----------------|----------------------|----------------------|
| **String Equality** | Exact term → RG bitmap | Only min/max (poor for strings) | Yes, but per-column only |
| **CONTAINS/FTS** | Trigram index | No | No |
| **Multi-path Queries** | Single index file | Scattered in column chunks | Scattered in column chunks |
| **Cardinality** | HyperLogLog estimates | No | No |
| **Null Tracking** | Explicit null/present bitmaps | Null count only | No |
| **Index Location** | Footer or sidecar file | Column chunk metadata | Column chunk metadata |

Parquet's [built-in bloom filters](https://parquet.apache.org/docs/file-format/bloomfilter/) are effective for single-column equality but require reading multiple column chunks for multi-field queries. This GIN index consolidates all paths into one structure.

### vs Delta Lake / Iceberg Data Skipping

| Aspect | This GIN Index | Delta Lake | Apache Iceberg |
|--------|----------------|------------|----------------|
| **Statistics** | Per-path term index + min/max | First 32 columns min/max | Partition-level + column stats |
| **High Cardinality** | adaptive-hybrid + bloom tail fallback | Requires Z-ordering | Requires sorting |
| **JSON Support** | Native path extraction | Requires schema | Requires schema |
| **Query Planning** | Client-side, cacheable | Spark/engine dependent | Engine dependent |
| **Deployment** | Standalone bytes | Delta transaction log | Metadata tables |

Delta Lake's [data skipping](https://docs.databricks.com/en/delta/data-skipping.html) relies on Z-ordering for effectiveness with high-cardinality columns. This GIN index handles high-cardinality paths natively with adaptive-hybrid hot-value recovery plus compact fallback for the tail.

### vs Elasticsearch

| Aspect | This GIN Index | Elasticsearch |
|--------|----------------|---------------|
| **Query Latency** | ~1µs | ~1-10ms (network + processing) |
| **Deployment** | Embedded, no server | Cluster required |
| **Index Size** | ~30KB for 1K row-groups | GB+ for equivalent data |
| **Use Case** | Row-group pruning | Full search engine |
| **Updates** | Rebuild required | Near real-time |

Elasticsearch provides [millisecond-level latency](https://www.datadoghq.com/blog/monitor-elasticsearch-performance-metrics/) for searches but requires cluster infrastructure. This index is designed for embedding in data lake metadata.

### Key Advantage: High-Cardinality Arbitrary JSON

This index was born from **log storage** needs - indexing arbitrary attributes/labels where:

- **High cardinality is the norm** - trace IDs, request IDs, user IDs, session tokens
- **Schema is unknown** - arbitrary key-value labels attached at runtime
- **Queries are selective** - "find logs where `trace_id=abc123`" should be instant

Traditional solutions struggle here:

| Challenge | PostgreSQL GIN | Parquet Stats | This GIN Index |
|-----------|---------------|---------------|----------------|
| `trace_id` (millions unique) | Index bloat, slow writes | Min/max useless | adaptive-hybrid exact hot values + compact tail fallback |
| `user.email` (arbitrary path) | Requires schema | Column must exist | Auto-discovered paths |
| `labels["env"]` (dynamic keys) | JSONB @> operator (~1ms) | Not supported | Native path indexing (~1µs) |
| Mixed types per path | Type coercion issues | Single type per column | Tracks observed types |

**Log/observability example:**
```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "error",
  "trace_id": "abc123def456",
  "user": {"id": "user_98765", "email": "alice@example.com"},
  "labels": {"env": "prod", "region": "us-east-1", "version": "2.1.0"},
  "message": "Connection timeout to downstream service"
}
```

Query: `trace_id=abc123def456 AND labels.env=prod AND level=error`
- Bloom filter rejects non-matching row groups instantly
- High-cardinality `trace_id` can retain exact row-group pruning for hot values via `adaptive-hybrid`
- Arbitrary `labels.*` paths indexed automatically

**Vector database metadata filtering:**

Vector databases need efficient pre-filtering before similarity search. Without good metadata indexing, you either:
1. Scan all vectors then filter (slow)
2. Filter first with poor index (still slow)
3. Build separate metadata infrastructure (complex)

```json
{
  "id": "doc_12345",
  "embedding": [0.1, 0.2, ...],
  "metadata": {
    "source": "arxiv",
    "year": 2024,
    "authors": ["Alice", "Bob"],
    "topics": ["machine-learning", "transformers"],
    "cited_by": 142,
    "full_text": "We present a novel approach to..."
  }
}
```

Query: Find similar vectors WHERE `metadata.source=arxiv AND metadata.year>=2023 AND metadata.topics[*]=transformers`

```
┌─────────────────────────────────────────────────────────────────┐
│                   Vector DB Hybrid Search                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   1. Metadata Filter (GIN Index)         2. Vector Search       │
│   ┌─────────────────────────────┐       ┌──────────────────┐   │
│   │ source=arxiv                │       │                  │   │
│   │ year>=2023        ──────────┼──────▶│  ANN Search      │   │
│   │ topics[*]=transformers      │       │  (only on        │   │
│   │                             │       │   segments 2,5)  │   │
│   │ Result: segments [2, 5]     │       │                  │   │
│   └─────────────────────────────┘       └──────────────────┘   │
│           ~1µs                              search scope        │
│                                             reduced 80%         │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

This enables:
- **Pre-filtering** - Prune segments before expensive ANN search
- **Flexible schemas** - Each document can have different metadata fields
- **High cardinality** - Filter by `doc_id`, `user_id`, `session_id`
- **Range + equality** - `year>=2023 AND source=arxiv`
- **Array membership** - `topics[*]=machine-learning`
- **Full-text on metadata** - `CONTAINS(full_text, "transformer")`

### Cacheable Pruning Index

The second differentiator is **deployment flexibility**:

```
┌─────────────────────────────────────────────────────────────────┐
│                     Data Lake Query Flow                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   Client                                                        │
│     │                                                           │
│     │ 1. Fetch GIN index (cached)                              │
│     ▼                                                           │
│   ┌─────────────────────────────────────────────────────────┐  │
│   │  nginx/memcached/CDN/local cache                        │  │
│   │  ┌─────────────┐                                        │  │
│   │  │ index.gin   │  ← 30KB, serves in <1ms               │  │
│   │  │ (cached)    │                                        │  │
│   │  └─────────────┘                                        │  │
│   └─────────────────────────────────────────────────────────┘  │
│     │                                                           │
│     │ 2. Evaluate predicates locally (~1µs)                    │
│     │    Result: [RG 5, RG 23, RG 47]                          │
│     │                                                           │
│     │ 3. Read only matching row groups from object storage     │
│     ▼                                                           │
│   ┌─────────────────────────────────────────────────────────┐  │
│   │  S3 / GCS / Azure Blob                                  │  │
│   │  ┌─────────────────────────────────────────────────┐    │  │
│   │  │ data.parquet                                    │    │  │
│   │  │  RG 0 ──────── skipped                         │    │  │
│   │  │  RG 5 ◀─────── read                            │    │  │
│   │  │  RG 10 ─────── skipped                         │    │  │
│   │  │  RG 23 ◀─────── read                           │    │  │
│   │  │  RG 47 ◀─────── read                           │    │  │
│   │  │  ...                                            │    │  │
│   │  └─────────────────────────────────────────────────┘    │  │
│   └─────────────────────────────────────────────────────────┘  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Benefits:**
- **No database server** - Index is just bytes, evaluate anywhere
- **Cache at edge** - nginx, memcached, CDN, browser localStorage
- **Cross-language** - Binary format works in any language
- **Offline capable** - Cache index locally for disconnected queries
- **Cost efficient** - Avoid scanning TB of Parquet data

This architecture is ideal for:
- **Log/observability platforms** - Index arbitrary labels, query by trace ID
- **Vector databases** - Pre-filter segments before ANN search
- **Serverless query engines** - No database to manage
- **Browser-based data explorers** - Cache index in localStorage
- **Edge computing / IoT analytics** - Offline-capable querying
- **Cost-sensitive data lake queries** - Minimize S3/GCS egress

## Architecture

### Index Structure

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              GINIndex                                       │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ Header                                                               │   │
│  │  • Version, NumRowGroups, NumDocs, NumPaths                         │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ Path Directory                                                       │   │
│  │  pathID → { PathName, ObservedTypes, Cardinality, Flags }           │   │
│  │                                                                      │   │
│  │  Example:                                                            │   │
│  │    0 → { "$.name",   String,  150,   0x00 }                         │   │
│  │    1 → { "$.age",    Int,     80,    0x00 }                         │   │
│  │    2 → { "$.tags[*]", String, 50000, FlagBloomOnly }                │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ Global Bloom Filter                                                  │   │
│  │  Fast rejection for path=value pairs                                 │   │
│  │  Contains: "$.name=alice", "$.name=bob", "$.age=30", ...            │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
│  ┌───────────────────────┐  ┌───────────────────────┐                      │
│  │ StringIndex           │  │ NumericIndex          │                      │
│  │ (per pathID)          │  │ (per pathID)          │                      │
│  │                       │  │                       │                      │
│  │ pathID: 0 ($.name)    │  │ pathID: 1 ($.age)     │                      │
│  │ ┌─────────┬────────┐  │  │ ┌─────┬──────┬──────┐│                      │
│  │ │  Term   │ RGSet  │  │  │ │ RG  │ Min  │ Max  ││                      │
│  │ ├─────────┼────────┤  │  │ ├─────┼──────┼──────┤│                      │
│  │ │ "alice" │ {0,2}  │  │  │ │  0  │  25  │  35  ││                      │
│  │ │ "bob"   │ {1}    │  │  │ │  1  │  20  │  45  ││                      │
│  │ │ "carol" │ {2,3}  │  │  │ │  2  │  30  │  30  ││                      │
│  │ └─────────┴────────┘  │  │ └─────┴──────┴──────┘│                      │
│  └───────────────────────┘  └───────────────────────┘                      │
│                                                                             │
│  ┌───────────────────────┐  ┌───────────────────────┐                      │
│  │ NullIndex             │  │ TrigramIndex          │                      │
│  │ (per pathID)          │  │ (per pathID)          │                      │
│  │                       │  │                       │                      │
│  │ pathID: 1 ($.age)     │  │ pathID: 3 ($.desc)    │                      │
│  │ ┌──────────┬────────┐ │  │ ┌─────────┬────────┐ │                      │
│  │ │ NullRGs  │ {4,7}  │ │  │ │ Trigram │ RGSet  │ │                      │
│  │ │ Present  │ {0-9}  │ │  │ ├─────────┼────────┤ │                      │
│  │ └──────────┴────────┘ │  │ │ "hel"   │ {0,2}  │ │                      │
│  └───────────────────────┘  │ │ "ell"   │ {0,2}  │ │                      │
│                             │ │ "llo"   │ {0,2,5}│ │                      │
│  ┌────────────────────────┐ │ │ "wor"   │ {1,3}  │ │                      │
│  │ DocID Mapping          │ │ └─────────┴────────┘ │                      │
│  │ (optional)             │ └───────────────────────┘                      │
│  │                        │                                                 │
│  │ pos → DocID            │  ┌───────────────────────┐                     │
│  │  0  → 1000             │  │ PathCardinality (HLL) │                     │
│  │  1  → 1001             │  │ (per pathID)          │                     │
│  │  2  → 1020             │  │                       │                     │
│  │  3  → 1021             │  │ Estimates unique vals │                     │
│  └────────────────────────┘  └───────────────────────┘                     │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

Note: All RGSet bitmaps use Roaring Bitmaps for efficient compression
```

### Data Flow

```
                                BUILD PHASE
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│   JSON Documents                                                            │
│        │                                                                    │
│        ▼                                                                    │
│   ┌─────────┐     ┌──────────────────────────────────────────────────┐     │
│   │ DocID 0 │────▶│                  GINBuilder                       │     │
│   │ RG: 0   │     │                                                   │     │
│   └─────────┘     │  AddDocument(docID, json)                        │     │
│   ┌─────────┐     │       │                                          │     │
│   │ DocID 1 │────▶│       ▼                                          │     │
│   │ RG: 0   │     │  ┌─────────────┐                                 │     │
│   └─────────┘     │  │ Walk JSON   │                                 │     │
│   ┌─────────┐     │  │ Extract:    │                                 │     │
│   │ DocID 2 │────▶│  │  • paths    │                                 │     │
│   │ RG: 1   │     │  │  • values   │                                 │     │
│   └─────────┘     │  │  • types    │                                 │     │
│        ⋮          │  └──────┬──────┘                                 │     │
│                   │         │                                         │     │
│                   │         ▼                                         │     │
│                   │  ┌─────────────────────────────────────────┐     │     │
│                   │  │ Update per-path structures:             │     │     │
│                   │  │  • stringTerms[term] → RGSet.Set(pos)   │     │     │
│                   │  │  • numericStats[pos].Min/Max            │     │     │
│                   │  │  • nullRGs.Set(pos) if null             │     │     │
│                   │  │  • trigrams.Add(term, pos)              │     │     │
│                   │  │  • bloom.Add(path=value)                │     │     │
│                   │  │  • hll.Add(value)                       │     │     │
│                   │  └─────────────────────────────────────────┘     │     │
│                   │                                                   │     │
│                   └───────────────────────┬──────────────────────────┘     │
│                                           │                                 │
│                                           ▼                                 │
│                                    Finalize()                               │
│                                           │                                 │
│                                           ▼                                 │
│                                    ┌─────────────┐                          │
│                                    │  GINIndex   │                          │
│                                    └─────────────┘                          │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘

                                QUERY PHASE
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                             │
│   Predicates: [EQ("$.name", "alice"), GT("$.age", 25)]                     │
│        │                                                                    │
│        ▼                                                                    │
│   ┌─────────────────────────────────────────────────────────────────┐      │
│   │                     idx.Evaluate(predicates)                     │      │
│   └─────────────────────────────────────────────────────────────────┘      │
│        │                                                                    │
│        ├──────────────────────┬──────────────────────┐                     │
│        ▼                      ▼                      ▼                     │
│   ┌──────────┐          ┌──────────┐          ┌──────────┐                 │
│   │ Predicate│          │ Predicate│          │   ...    │                 │
│   │    1     │          │    2     │          │          │                 │
│   └────┬─────┘          └────┬─────┘          └──────────┘                 │
│        │                     │                                              │
│        ▼                     ▼                                              │
│   ┌──────────────┐     ┌──────────────┐                                    │
│   │ Bloom Check  │     │ Bloom Check  │   ◀── Fast rejection path          │
│   │ path=value?  │     │   (skip for  │                                    │
│   └──────┬───────┘     │    ranges)   │                                    │
│          │             └──────┬───────┘                                    │
│          ▼                    ▼                                             │
│   ┌──────────────┐     ┌──────────────┐                                    │
│   │ StringIndex  │     │ NumericIndex │                                    │
│   │ lookup term  │     │ scan min/max │                                    │
│   │ → RGSet      │     │ → RGSet      │                                    │
│   └──────┬───────┘     └──────┬───────┘                                    │
│          │                    │                                             │
│          │    RGSet{0,2}      │    RGSet{0,1,2}                            │
│          │                    │                                             │
│          └─────────┬──────────┘                                             │
│                    │                                                        │
│                    ▼                                                        │
│             ┌─────────────┐                                                 │
│             │  Intersect  │                                                 │
│             │  (AND all)  │                                                 │
│             └──────┬──────┘                                                 │
│                    │                                                        │
│                    ▼                                                        │
│             ┌─────────────┐                                                 │
│             │ RGSet{0,2}  │  ◀── Matching row groups                       │
│             └──────┬──────┘                                                 │
│                    │                                                        │
│                    ▼                                                        │
│             ┌─────────────┐                                                 │
│             │ ToSlice()   │  → [0, 2]                                      │
│             │     or      │                                                 │
│             │ MatchingDocIDs() → [DocID...]                                │
│             └─────────────┘                                                 │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### DocID Codec (Optional)

For composite document identifiers (e.g., file + row group):

```go
// Encode file index and row group into single DocID
codec := gin.NewRowGroupCodec(20)  // 20 RGs per file
builder, err := gin.NewBuilder(config, totalRGs, gin.WithCodec(codec))
if err != nil {
    panic(err)
}

docID := codec.Encode(fileIndex, rgIndex)  // e.g., file=3, rg=15 → DocID=75
builder.AddDocument(docID, jsonDoc)

// Query and decode results
result := idx.Evaluate(predicates)
for _, docID := range idx.MatchingDocIDs(result) {
    decoded := codec.Decode(docID)  // [3, 15]
    fileIdx, rgIdx := decoded[0], decoded[1]
}
```

## How It Works

The GIN index maintains several data structures:

1. **Path Directory** - Maps JSON paths to their metadata (types, cardinality)
2. **String Index** - For each path, maps terms to row-group bitmaps (Roaring)
3. **Numeric Index** - Per-row-group min/max values for range pruning
4. **Null Index** - Bitmaps tracking which row groups have null/present values
5. **Trigram Index** - Maps 3-character sequences to row-group bitmaps
6. **Global Bloom Filter** - Fast rejection of non-existent path=value pairs
7. **DocID Mapping** - Optional external DocID to internal position mapping

Query evaluation intersects the matching row-group bitmaps from each predicate.

## Design Notes

### Why numRGs Must Be Known Upfront

The `NewBuilder(config, numRGs)` requires the total number of row groups at construction time. This is intentional:

1. **Complement operations require universe size** - Operations like `AllRGs()` and `Invert()` need to know the total number of row groups to compute complements. When a query cannot prune (e.g., unknown path, graceful degradation), the index returns "all row groups" - which requires knowing what "all" means.

2. **Parquet metadata provides this** - The index is designed for Parquet row-group pruning. In this context, the number of row groups is always available from Parquet file metadata before indexing begins.

3. **Bounds checking** - The builder validates that document positions don't exceed the declared row group count, catching configuration errors early.

## License

MIT
