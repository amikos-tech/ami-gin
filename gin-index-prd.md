# GIN Index for Parquet JSON Columns

## Product Requirements Document (PRD)

**Version:** 1.0  
**Status:** Draft  
**Author:** Kiba Team  
**Last Updated:** January 2025

---

## 1. Executive Summary

This document specifies the design for a Generalized Inverted Index (GIN) that enables efficient querying of JSON data stored in Parquet columns. The index is embedded in the Parquet footer's key-value metadata section and supports exact match, range, null, and substring queries with row-group-level pruning granularity.

### 1.1 Problem Statement

Parquet files in Kiba contain a JSON column (`log_data`) with structured log entries. Parquet treats this column as opaque bytes, providing no native statistics or pruning capabilities. Users need to query specific JSON paths with various predicates:

- Exact match: `user_id = "tom"`
- Range queries: `latency_ms > 100 AND latency_ms < 500`
- Null checks: `error_code IS NULL`
- Substring search: `message CONTAINS "timeout"`

Without secondary indexing, every query must scan all row groups, resulting in unnecessary S3 bandwidth and compute costs.

### 1.2 Goals

1. **RG-level pruning**: Identify which row groups to fetch, minimizing S3 reads
2. **Schemaless operation**: Index arbitrary JSON paths without upfront schema declaration
3. **Type-aware indexing**: Automatically detect and optimally index different value types
4. **Compact footprint**: Index size should be <5% of data size after compression
5. **Fast index reads**: Index lookup should complete in <10ms for typical queries

### 1.3 Non-Goals

1. Row-level indexing (RG is the minimum S3 fetch unit)
2. Full-text search with relevance ranking (BMW handles this separately)
3. Cross-file indexing (each file is independent)
4. Real-time index updates (index is built at compaction time)

---

## 2. Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           GIN Index Architecture                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Parquet File                                                                │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  Row Group 0    [50,000 rows]                                       │    │
│  │  Row Group 1    [50,000 rows]                                       │    │
│  │  ...                                                                 │    │
│  │  Row Group N    [50,000 rows]                                       │    │
│  │                                                                      │    │
│  │  Footer                                                              │    │
│  │  ├── Schema                                                          │    │
│  │  ├── Row Group Metadata                                              │    │
│  │  └── Key-Value Metadata                                              │    │
│  │      └── "gin:json:v1" → [GIN Index Blob]                           │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│  GIN Index Blob (compressed, optionally encrypted)                          │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  Header                                                              │    │
│  │  Path Directory                                                      │    │
│  │  Global Bloom Filter                                                 │    │
│  │  Term → RG Bitmaps (low cardinality paths)                          │    │
│  │  Numeric Stats (per-RG min/max)                                     │    │
│  │  Null Bitmaps                                                        │    │
│  │  Trigram Index (opt-in text paths)                                  │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Index Structure Specification

### 3.1 Header

```
┌────────────────────────────────────────────────────────────────────────────┐
│ Field              │ Type      │ Description                               │
├────────────────────┼───────────┼───────────────────────────────────────────┤
│ magic              │ [4]byte   │ "GIN\x01" - format identifier             │
│ version            │ uint16    │ Format version (currently 1)              │
│ flags              │ uint16    │ Feature flags (encryption, compression)   │
│ num_row_groups     │ uint32    │ Number of row groups in file              │
│ num_docs           │ uint64    │ Total document count                      │
│ num_paths          │ uint32    │ Number of indexed paths                   │
│ cardinality_thresh │ uint32    │ Threshold for bloom-only (default 10000)  │
│ rg_doc_offsets     │ []uint64  │ Cumulative doc count per RG               │
│ section_offsets    │ []uint32  │ Byte offsets to each section              │
└────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 Path Directory

Each observed JSON path has an entry describing its characteristics and index locations.

```
┌────────────────────────────────────────────────────────────────────────────┐
│ Field              │ Type      │ Description                               │
├────────────────────┼───────────┼───────────────────────────────────────────┤
│ path_id            │ uint16    │ Internal path identifier                  │
│ path_name          │ string    │ JSON path (e.g., "user_id", "meta.host") │
│ observed_types     │ uint8     │ Bitmap: [string|int|float|bool|null]     │
│ cardinality        │ uint32    │ Unique value count (capped at threshold) │
│ flags              │ uint8     │ text_indexed, bloom_only, etc.           │
│ string_offset      │ uint32    │ Offset to string index (0 if none)        │
│ numeric_offset     │ uint32    │ Offset to numeric stats (0 if none)       │
│ null_offset        │ uint32    │ Offset to null bitmap (0 if none)         │
│ text_offset        │ uint32    │ Offset to trigram index (0 if none)       │
└────────────────────────────────────────────────────────────────────────────┘
```

**Path Name Encoding:**
- Nested paths use dot notation: `metadata.request.trace_id`
- Array indices use brackets: `tags[0]`, `tags[*]` for any element
- Maximum path depth: 16 levels
- Maximum path length: 256 bytes

### 3.3 Global Bloom Filter

A single bloom filter covering all (path_id, value) pairs for fast negative lookups.

```
┌────────────────────────────────────────────────────────────────────────────┐
│ Field              │ Type      │ Description                               │
├────────────────────┼───────────┼───────────────────────────────────────────┤
│ num_bits           │ uint32    │ Bloom filter size in bits                 │
│ num_hashes         │ uint8     │ Number of hash functions (k)              │
│ bits               │ []byte    │ Bit array                                 │
└────────────────────────────────────────────────────────────────────────────┘
```

**Bloom Key Construction:**
- For strings: `hash(path_id || ":" || value)`
- For numerics: `hash(path_id || ":" || canonical_numeric_string(value))`
- For bools: `hash(path_id || ":true")` or `hash(path_id || ":false")`

**Sizing:**
- Target false positive rate: 1%
- Bits per element: ~10
- Hash functions: 7 (optimal for 1% FPR)

### 3.4 String Index Section

For paths with string values, provides exact match capability.

#### 3.4.1 Low Cardinality (term→RG bitmap)

When `cardinality < cardinality_threshold`:

```
┌────────────────────────────────────────────────────────────────────────────┐
│ Field              │ Type      │ Description                               │
├────────────────────┼───────────┼───────────────────────────────────────────┤
│ num_terms          │ uint32    │ Number of unique terms                    │
│ terms              │ []string  │ Sorted, prefix-compressed terms           │
│ rg_bitmaps         │ []uint64  │ One bitmap per term (parallel array)      │
└────────────────────────────────────────────────────────────────────────────┘
```

**Term Encoding (Prefix Compression):**
```
For terms: ["error", "error_code", "error_msg", "info", "warn"]

Encoded as:
  [0, "error"]      - 0 shared prefix bytes, full term
  [5, "_code"]      - 5 shared bytes with previous, suffix only
  [5, "_msg"]       - 5 shared bytes
  [0, "info"]       - 0 shared (new prefix)
  [0, "warn"]       - 0 shared
```

**RG Bitmap:**
- For ≤64 RGs: single `uint64` (bit N = 1 if term appears in RG N)
- For >64 RGs: `[]uint64` with length `ceil(num_rgs / 64)`

#### 3.4.2 High Cardinality (bloom only)

When `cardinality >= cardinality_threshold`, no term→RG section is stored. The global bloom filter handles exact match queries with file-level pruning only.

### 3.5 Numeric Index Section

For paths with numeric values (int64 or float64).

```
┌────────────────────────────────────────────────────────────────────────────┐
│ Field              │ Type      │ Description                               │
├────────────────────┼───────────┼───────────────────────────────────────────┤
│ value_type         │ uint8     │ 0=int64, 1=float64                        │
│ global_min         │ int64/f64 │ Minimum across all RGs                    │
│ global_max         │ int64/f64 │ Maximum across all RGs                    │
│ rg_stats           │ []RGStat  │ Per-RG min/max                            │
└────────────────────────────────────────────────────────────────────────────┘

RGStat:
┌────────────────────────────────────────────────────────────────────────────┐
│ min                │ int64/f64 │ Minimum value in this RG                  │
│ max                │ int64/f64 │ Maximum value in this RG                  │
│ has_value          │ bool      │ False if path absent in entire RG         │
└────────────────────────────────────────────────────────────────────────────┘
```

**Mixed Integer/Float Handling:**
- If path has both int and float values, promote all to float64
- Store `value_type = 1` (float64)
- Integer precision preserved up to 2^53

### 3.6 Null Index Section

For paths that have null values or are absent in some documents.

```
┌────────────────────────────────────────────────────────────────────────────┐
│ Field              │ Type      │ Description                               │
├────────────────────┼───────────┼───────────────────────────────────────────┤
│ null_rg_bitmap     │ uint64    │ Bit N = 1 if RG N has null/missing        │
│ present_rg_bitmap  │ uint64    │ Bit N = 1 if RG N has non-null values     │
└────────────────────────────────────────────────────────────────────────────┘
```

**Null vs Missing:**
- `null`: JSON explicitly contains `"field": null`
- `missing`: JSON document doesn't contain the path at all
- Both are treated equivalently for `IS NULL` queries
- `present_rg_bitmap` enables `IS NOT NULL` without full scan

### 3.7 Text Index Section (Opt-In)

For paths explicitly configured for substring search.

```
┌────────────────────────────────────────────────────────────────────────────┐
│ Field              │ Type      │ Description                               │
├────────────────────┼───────────┼───────────────────────────────────────────┤
│ num_trigrams       │ uint32    │ Number of unique trigrams                 │
│ trigrams           │ []string  │ Sorted 3-byte sequences                   │
│ rg_bitmaps         │ []uint64  │ One bitmap per trigram                    │
└────────────────────────────────────────────────────────────────────────────┘
```

**Trigram Generation:**
```
Input: "connection timeout"

Trigrams (with padding):
  "$$c", "$co", "con", "onn", "nne", "nec", "ect", "cti", 
  "tio", "ion", "on ", "n t", " ti", "tim", "ime", "meo", 
  "eou", "out", "ut$", "t$$"

Where $ is a boundary marker (handles start/end matching)
```

**Substring Query Execution:**
```
Query: message CONTAINS "time"

1. Generate query trigrams: ["tim", "ime"]
2. Look up each trigram's RG bitmap
3. Intersect bitmaps: only RGs containing ALL trigrams
4. DuckDB scans remaining RGs with LIKE '%time%'
```

---

## 4. Index Construction

### 4.1 Builder Configuration

```go
type GINConfig struct {
    // Cardinality threshold for switching from term→RG to bloom-only
    // Default: 10000
    CardinalityThreshold uint32
    
    // Paths that should have trigram indexing for substring search
    // Empty means no text indexing
    TextIndexedPaths []string
    
    // Bloom filter target false positive rate
    // Default: 0.01 (1%)
    BloomFPR float64
    
    // Maximum number of paths to index (memory bound)
    // Default: 1000
    MaxPaths int
    
    // Maximum trigrams per text path (size bound)
    // Default: 100000
    MaxTrigramsPerPath int
}
```

### 4.2 Build Process

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Index Build Flow                                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Input: Stream of (row_group_id, json_document) pairs                       │
│                                                                              │
│  Phase 1: Collection (streaming, per-document)                              │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  For each document:                                                  │    │
│  │    1. Walk JSON, extract (path, value, type) tuples                 │    │
│  │    2. For each tuple:                                                │    │
│  │       - Register path if new                                         │    │
│  │       - Update path's observed_types bitmap                          │    │
│  │       - Add (path_id, value) to bloom builder                       │    │
│  │       - If string:                                                   │    │
│  │           - Add value to path's term set                            │    │
│  │           - Mark current RG in term's bitmap                         │    │
│  │           - If text_indexed: generate and collect trigrams          │    │
│  │       - If numeric:                                                  │    │
│  │           - Update current RG's min/max                             │    │
│  │       - If null/missing:                                             │    │
│  │           - Mark current RG in null bitmap                          │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│  Phase 2: Finalization (after all documents)                                │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  For each path:                                                      │    │
│  │    - Calculate final cardinality                                     │    │
│  │    - If cardinality >= threshold:                                    │    │
│  │        - Discard term→RG data                                       │    │
│  │        - Mark as bloom_only                                          │    │
│  │    - Sort terms (for binary search)                                  │    │
│  │    - Apply prefix compression                                        │    │
│  │                                                                      │    │
│  │  Build final bloom filter                                            │    │
│  │  Serialize all sections                                              │    │
│  │  Apply zstd compression                                              │    │
│  │  Optionally apply encryption                                         │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                              │
│  Output: Compressed blob for Parquet KV metadata                            │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 4.3 Memory Management During Build

For large files (1M+ docs), memory must be bounded:

```
┌────────────────────────────────────────────────────────────────────────────┐
│ Component              │ Memory Strategy                                   │
├────────────────────────┼───────────────────────────────────────────────────┤
│ Path registry          │ Fixed cap (MaxPaths), LRU eviction               │
│ Term sets              │ HyperLogLog for cardinality, switch to bloom     │
│                        │ when exceeding threshold during build             │
│ Bloom builder          │ Streaming add, fixed size based on estimate      │
│ Numeric stats          │ O(num_rgs) per path, negligible                   │
│ Trigram collection     │ Cap per path (MaxTrigramsPerPath)                │
└────────────────────────────────────────────────────────────────────────────┘
```

**Cardinality Tracking:**
- Use HyperLogLog (HLL) with 12-bit precision (~1.6% error)
- HLL size: ~4KB per path
- When HLL estimate exceeds threshold * 0.9, stop collecting terms

---

## 5. Query Execution

### 5.1 Supported Query Predicates

```
┌────────────────────────────────────────────────────────────────────────────┐
│ Predicate              │ Index Used           │ Pruning Level             │
├────────────────────────┼──────────────────────┼───────────────────────────┤
│ field = "value"        │ Bloom + Term→RG      │ File + RG                 │
│ field IN ("a", "b")    │ Bloom + Term→RG      │ File + RG (union)         │
│ field != "value"       │ None (scan)          │ None                      │
│ field = 123            │ Bloom + Numeric      │ File + RG                 │
│ field > 100            │ Numeric min/max      │ RG                        │
│ field < 100            │ Numeric min/max      │ RG                        │
│ field BETWEEN 10 AND 20│ Numeric min/max      │ RG                        │
│ field IS NULL          │ Null bitmap          │ RG                        │
│ field IS NOT NULL      │ Present bitmap       │ RG                        │
│ field CONTAINS "xyz"   │ Trigram (if indexed) │ RG                        │
│ field LIKE "%xyz%"     │ Trigram (if indexed) │ RG                        │
│ field LIKE "xyz%"      │ Term prefix (future) │ RG                        │
└────────────────────────────────────────────────────────────────────────────┘
```

### 5.2 Query Execution Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│            Query: user_id = "tom" AND latency > 100 AND msg CONTAINS "err" │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Step 1: Parse query, extract predicates by path                            │
│    predicates = [                                                            │
│      {path: "user_id", op: EQ, value: "tom", type: string},                │
│      {path: "latency", op: GT, value: 100, type: numeric},                 │
│      {path: "msg", op: CONTAINS, value: "err", type: text}                 │
│    ]                                                                         │
│                                                                              │
│  Step 2: Bloom filter fast-path (file-level pruning)                        │
│    if !bloom.MayContain("user_id:tom"):                                     │
│      return EMPTY  // File definitely doesn't have user_id=tom             │
│                                                                              │
│  Step 3: Per-predicate RG pruning                                           │
│                                                                              │
│    user_id = "tom":                                                          │
│      path_info = directory.Lookup("user_id")                                │
│      if path_info.has_term_index:                                           │
│        rg_set_1 = term_index.Lookup("tom")  // e.g., {0, 2, 5}             │
│      else:                                                                   │
│        rg_set_1 = ALL_RGS  // bloom passed, but no RG info                 │
│                                                                              │
│    latency > 100:                                                            │
│      path_info = directory.Lookup("latency")                                │
│      rg_set_2 = {}                                                           │
│      for rg in 0..num_rgs:                                                  │
│        if numeric_stats[rg].max > 100:                                      │
│          rg_set_2.add(rg)  // e.g., {0, 1, 2, 5, 7}                        │
│                                                                              │
│    msg CONTAINS "err":                                                       │
│      if "msg" in text_indexed_paths:                                        │
│        trigrams = generate_trigrams("err")  // ["err"]                     │
│        rg_set_3 = trigram_index.Lookup("err")  // e.g., {0, 2, 3, 5}       │
│      else:                                                                   │
│        rg_set_3 = ALL_RGS  // must scan                                    │
│                                                                              │
│  Step 4: Intersect RG sets                                                   │
│    final_rgs = rg_set_1 ∩ rg_set_2 ∩ rg_set_3                              │
│              = {0, 2, 5} ∩ {0, 1, 2, 5, 7} ∩ {0, 2, 3, 5}                  │
│              = {0, 2, 5}                                                     │
│                                                                              │
│  Step 5: Return RG list for fetching                                        │
│    return [0, 2, 5]  // Fetch these RGs from S3, scan with DuckDB          │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 5.3 Query API

```go
// GINQuery represents a parsed query against the GIN index
type GINQuery struct {
    Predicates []Predicate
}

type Predicate struct {
    Path     string
    Operator Operator  // EQ, NE, GT, LT, GTE, LTE, IN, IS_NULL, IS_NOT_NULL, CONTAINS
    Value    any       // string, int64, float64, []string (for IN), nil (for IS_NULL)
}

type Operator uint8
const (
    OpEQ Operator = iota
    OpNE
    OpGT
    OpLT
    OpGTE
    OpLTE
    OpIN
    OpIsNull
    OpIsNotNull
    OpContains
)

// GINIndex is the main query interface
type GINIndex interface {
    // Evaluate returns the set of row groups that MAY contain matching documents
    // An empty result means the file definitely has no matches
    // A full result (all RGs) means no pruning was possible
    Evaluate(query GINQuery) (RGSet, error)
    
    // Stats returns index metadata for debugging/monitoring
    Stats() GINStats
}

type RGSet struct {
    // Bitmap representation for efficient intersection
    bitmap uint64      // For ≤64 RGs
    large  []uint64    // For >64 RGs (nil if not needed)
    numRGs int
}

type GINStats struct {
    NumPaths          int
    NumTerms          int
    NumTrigrams       int
    BloomSizeBits     int
    CompressedSizeBytes int
    UncompressedSizeBytes int
}
```

---

## 6. Serialization Format

### 6.1 Wire Format

All multi-byte integers are little-endian.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Serialized Layout                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Offset 0x00:   Header (fixed size)                                         │
│  Offset 0x40:   Path Directory (variable)                                   │
│  Offset X:      Global Bloom Filter                                         │
│  Offset Y:      String Index Sections (concatenated)                        │
│  Offset Z:      Numeric Index Sections (concatenated)                       │
│  Offset W:      Null Index Sections (concatenated)                          │
│  Offset V:      Text Index Sections (concatenated)                          │
│                                                                              │
│  Section offsets stored in header for O(1) access                           │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 6.2 Compression

- Algorithm: zstd level 3 (balance speed/ratio)
- Applied to entire blob after serialization
- Typical compression ratio: 3-4x

### 6.3 Encryption (Optional)

- Algorithm: AES-256-GCM
- Key derivation: PBKDF2 with user passphrase + random salt
- Salt stored in blob header (unencrypted)
- Nonce: random per-file

```
Encrypted blob layout:
  [salt: 32 bytes][nonce: 12 bytes][encrypted_data][auth_tag: 16 bytes]
```

---

## 7. Size Estimates

### 7.1 Component Sizes

```
┌────────────────────────────────────────────────────────────────────────────┐
│ Component                       │ Formula                    │ Example    │
├─────────────────────────────────┼────────────────────────────┼────────────┤
│ Header                          │ ~100 bytes fixed           │ 100 B      │
│ Path directory (P paths)        │ P × 50 bytes               │ 2.5 KB     │
│ Global bloom (N items, 1% FPR)  │ N × 10 bits                │ 1.2 MB     │
│ Term→RG (T terms, R RGs)        │ T × (avg_len + 8)          │ 200 KB     │
│ Numeric stats (P paths, R RGs)  │ P × R × 16 bytes           │ 8 KB       │
│ Null bitmaps (P paths)          │ P × 16 bytes               │ 800 B      │
│ Trigram index (G trigrams)      │ G × 11 bytes               │ 500 KB     │
└────────────────────────────────────────────────────────────────────────────┘
```

### 7.2 Realistic Scenario

```
Assumptions:
  - 1M documents, 20 row groups
  - 50 JSON paths
  - 10 low-cardinality string paths (avg 500 unique values each)
  - 15 medium-cardinality paths (avg 5K unique values each)
  - 10 high-cardinality paths (>10K unique, bloom only)
  - 10 numeric paths
  - 5 paths with nulls
  - 2 text-indexed paths (avg 50K unique trigrams each)

Calculation:
  Header + Path directory:           ~2.5 KB
  Global bloom (100K items):         ~125 KB
  Low-card term→RG (5K terms):       ~60 KB
  Medium-card term→RG (75K terms):   ~900 KB
  Numeric stats (10 × 20 × 16):      ~3.2 KB
  Null bitmaps (5 × 16):             ~80 B
  Trigram index (100K trigrams):     ~1.1 MB
  
  Total uncompressed:                ~2.2 MB
  Total compressed (3x):             ~750 KB
```

### 7.3 Size Bounds

| Data Size | Docs | Target Index Size | Notes |
|-----------|------|-------------------|-------|
| 100 MB    | 100K | <5 MB             | Comfortable |
| 1 GB      | 1M   | <20 MB            | ~2% overhead |
| 10 GB     | 10M  | <100 MB           | May need path limits |

---

## 8. Configuration Reference

### 8.1 Build-Time Configuration

```go
type GINConfig struct {
    // CardinalityThreshold: paths with more unique values than this
    // use bloom-only (no term→RG mapping)
    // Range: 1000-100000, Default: 10000
    CardinalityThreshold uint32

    // TextIndexedPaths: paths that should have trigram indexing
    // for CONTAINS/LIKE queries. Empty = no text indexing.
    // Example: ["message", "error.description"]
    TextIndexedPaths []string

    // BloomFPR: target false positive rate for bloom filter
    // Lower = larger bloom, fewer false positives
    // Range: 0.001-0.1, Default: 0.01
    BloomFPR float64

    // MaxPaths: maximum paths to index (prevents memory explosion
    // on documents with thousands of paths)
    // Range: 100-10000, Default: 1000
    MaxPaths int

    // MaxTrigramsPerPath: cap on trigrams per text path
    // Prevents explosion on paths with huge text values
    // Range: 10000-1000000, Default: 100000
    MaxTrigramsPerPath int

    // MaxTermsPerPath: cap on terms before forcing bloom-only
    // Even if cardinality is below threshold, cap terms for memory
    // Range: 10000-1000000, Default: 100000
    MaxTermsPerPath int
}
```

### 8.2 Query-Time Configuration

```go
type GINQueryConfig struct {
    // SkipBloomCheck: bypass bloom filter (for benchmarking)
    // Default: false
    SkipBloomCheck bool

    // MaxRGsToReturn: if more RGs match, return ALL_RGS
    // (signals that pruning wasn't effective enough)
    // Range: 1-1000, Default: 0 (no limit)
    MaxRGsToReturn int
}
```

---

## 9. Error Handling

### 9.1 Build Errors

| Error | Cause | Recovery |
|-------|-------|----------|
| `ErrMaxPathsExceeded` | Document has >MaxPaths unique paths | Skip excess paths, log warning |
| `ErrInvalidJSON` | Document is not valid JSON | Skip document, increment error counter |
| `ErrPathTooDeep` | Path exceeds 16 levels | Truncate path, log warning |
| `ErrValueTooLarge` | String value >64KB | Skip value, use null instead |

### 9.2 Query Errors

| Error | Cause | Recovery |
|-------|-------|----------|
| `ErrPathNotIndexed` | Query references non-existent path | Return ALL_RGS (no pruning) |
| `ErrTypeMismatch` | Numeric query on string-only path | Return ALL_RGS |
| `ErrCorruptIndex` | Index fails integrity check | Return ALL_RGS, log error |
| `ErrTextNotIndexed` | CONTAINS on non-text path | Return ALL_RGS |

---

## 10. Integration Points

### 10.1 Compaction Pipeline Integration

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                     Compaction with GIN Index                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Existing Pipeline:                                                          │
│    WAL Segments → DuckDB Compaction → Parquet File → S3                    │
│                                                                              │
│  With GIN:                                                                   │
│    WAL Segments → DuckDB Compaction → Parquet File                         │
│                                           ↓                                  │
│                                    GIN Index Build                          │
│                                           ↓                                  │
│                                    Append to Footer KV                      │
│                                           ↓                                  │
│                                         S3                                   │
│                                                                              │
│  GIN build happens AFTER DuckDB writes Parquet but BEFORE S3 upload        │
│  Uses streaming read of the Parquet file (no double memory)                │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 10.2 Query Pipeline Integration

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      Query with GIN Pruning                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Query: SELECT * FROM logs                                                   │
│         WHERE user_id = 'tom' AND latency > 100                             │
│                                                                              │
│  Step 1: File Selection (existing timestamp/partition pruning)              │
│    → Candidate files: [file_001.parquet, file_002.parquet, ...]            │
│                                                                              │
│  Step 2: GIN File Pruning (NEW)                                             │
│    For each candidate file:                                                  │
│      a. Fetch footer (cached in L1/L2 index cache)                          │
│      b. Extract GIN blob from KV metadata                                   │
│      c. Check global bloom for "user_id:tom"                                │
│      d. If bloom says NO → skip entire file                                │
│                                                                              │
│  Step 3: GIN RG Pruning (NEW)                                               │
│    For files passing bloom check:                                            │
│      a. Evaluate predicates against GIN index                               │
│      b. Get candidate RG set                                                 │
│                                                                              │
│  Step 4: S3 Fetch                                                            │
│    Fetch only candidate RGs (not whole file)                                │
│    → Bandwidth reduction: often 80-95%                                      │
│                                                                              │
│  Step 5: DuckDB Scan                                                         │
│    Full predicate evaluation on fetched RGs                                 │
│    → Fast: 50K rows per RG × few RGs                                       │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 11. Testing Strategy

### 11.1 Unit Tests

- **Path extraction**: JSON walking, nested paths, arrays
- **Type detection**: Mixed types, null handling, type promotion
- **Bloom filter**: FPR validation, hash distribution
- **Term index**: Prefix compression, binary search, bitmap operations
- **Numeric stats**: Min/max correctness, range query logic
- **Trigram generation**: Boundary handling, Unicode support

### 11.2 Integration Tests

- **Round-trip**: Build index → serialize → deserialize → query
- **Large scale**: 1M+ documents, verify memory bounds
- **Edge cases**: Empty documents, single-RG files, all-null paths
- **Correctness**: Query results match brute-force scan

### 11.3 Benchmark Tests

- **Build throughput**: Documents per second
- **Query latency**: P50, P99 for various query patterns
- **Memory usage**: Peak during build, steady-state for queries
- **Compression ratio**: Various data distributions

---

## 12. Future Considerations

### 12.1 Potential Enhancements

1. **Prefix queries**: `field LIKE "abc%"` using sorted term index
2. **Regex support**: `field ~ "^err.*timeout$"` via trigram filtering
3. **Numeric histograms**: Better pruning for skewed distributions
4. **Cross-file bloom**: Manifest-level bloom for multi-file pruning
5. **Incremental updates**: Append-only index updates for streaming

### 12.2 Known Limitations

1. **No row-level pruning**: RG is the granularity floor
2. **No NOT queries**: `field != "x"` requires full scan
3. **Substring requires opt-in**: CONTAINS without text index = full scan
4. **Memory during build**: Large files need careful memory management

---

## 13. Glossary

| Term | Definition |
|------|------------|
| **RG** | Row Group - Parquet's horizontal partition, typically 50K rows |
| **GIN** | Generalized Inverted Index - index mapping values to locations |
| **Bloom Filter** | Probabilistic data structure for set membership |
| **Trigram** | 3-character subsequence used for substring matching |
| **Cardinality** | Number of unique values for a path |
| **Term→RG** | Mapping from a value to the row groups containing it |

---

## Appendix A: Example Index Dump

```
GIN Index v1
============
File: logs_2025-01-15_001.parquet
Row Groups: 20
Documents: 1,000,000
Compressed Size: 742 KB

Path Directory (47 paths):
  [0] user_id          string   card=8423   term→RG
  [1] request_id       string   card=982341 bloom-only
  [2] hostname         string   card=156    term→RG
  [3] latency_ms       int64    -           numeric
  [4] error_rate       float64  -           numeric
  [5] status_code      int64    -           numeric
  [6] message          string   card=45123  text-indexed
  ...

Global Bloom:
  Size: 1,245,184 bits (152 KB)
  Hash functions: 7
  Estimated FPR: 0.98%

String Index (user_id, 8423 terms):
  "alice"     → RGs {0, 1, 5, 12}
  "bob"       → RGs {2, 3, 7, 8, 15}
  "charlie"   → RGs {0, 4, 9, 11, 18, 19}
  ...

Numeric Index (latency_ms):
  RG 0:  min=12,    max=4521
  RG 1:  min=8,     max=3892
  RG 2:  min=45,    max=12893
  ...

Text Index (message, 52341 trigrams):
  "err" → RGs {0, 2, 5, 7, 8, 11, 15, 18}
  "tim" → RGs {1, 3, 4, 6, 9, 12, 14, 17}
  "out" → RGs {0, 1, 2, 5, 8, 11, 15, 16, 19}
  ...
```

---

## Appendix B: Query Examples

```sql
-- Example 1: Simple exact match
-- GIN: Bloom check + term→RG lookup
-- Expected pruning: 85-95%
SELECT * FROM logs WHERE user_id = 'alice'

-- Example 2: Numeric range
-- GIN: Per-RG min/max check
-- Expected pruning: 60-80%
SELECT * FROM logs WHERE latency_ms > 1000

-- Example 3: Combined predicates
-- GIN: Intersect RG sets from each predicate
-- Expected pruning: 95-99%
SELECT * FROM logs 
WHERE user_id = 'alice' 
  AND latency_ms > 1000 
  AND status_code = 500

-- Example 4: Substring search (text-indexed path)
-- GIN: Trigram lookup + intersection
-- Expected pruning: 70-90%
SELECT * FROM logs WHERE message CONTAINS 'connection timeout'

-- Example 5: Null check
-- GIN: Null bitmap lookup
-- Expected pruning: 80-95% (depends on null distribution)
SELECT * FROM logs WHERE error_code IS NOT NULL

-- Example 6: High-cardinality exact match
-- GIN: Bloom only (file-level pruning)
-- Expected pruning: 0% within file, but fast negative across files
SELECT * FROM logs WHERE request_id = 'abc-123-def-456'
```

---

*End of Document*
