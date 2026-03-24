# Architecture

**Analysis Date:** 2026-03-24

## Pattern Overview

**Overall:** Single-package Go library with CLI companion

The GIN Index is a **Generalized Inverted Index** library for JSON data, designed for row-group pruning in columnar storage (Parquet). It is a Go library (package `gin`) consumed as a dependency, plus a CLI tool (`cmd/gin-index`) for direct operations on Parquet files.

**Key Characteristics:**
- Single flat Go package (`package gin`) -- all core types and logic live at the module root
- Builder pattern for index construction (mutable) producing an immutable index for querying
- Binary serialization with zstd compression for compact storage
- Multiple index structures (string, numeric, null, trigram, bloom, HLL) keyed by path ID
- Row-group-level granularity -- the index answers "which row groups MAY contain matching documents"
- Functional options pattern for configuration (`ConfigOption`, `BuilderOption`, `NGramOption`, etc.)

## Layers

**Configuration Layer:**
- Purpose: Define index build parameters and field transformers
- Location: `gin.go` (types + options), `transformers.go` (built-in transformers), `transformer_registry.go` (serializable transformer registry)
- Contains: `GINConfig`, `ConfigOption` functions (`WithFieldTransformer`, `WithISODateTransformer`, etc.), `FieldTransformer` type, `TransformerSpec` for serialization
- Depends on: Nothing (leaf layer)
- Used by: Builder, Serialize

**Builder Layer:**
- Purpose: Ingest JSON documents, walk their structure, populate index data structures
- Location: `builder.go`
- Contains: `GINBuilder`, `pathBuildData`, `AddDocument()`, `walkJSON()`, `Finalize()`
- Depends on: Configuration, Data Structures (RGSet, BloomFilter, TrigramIndex, HyperLogLog)
- Used by: User code, Parquet integration, CLI

**Index Layer (Core):**
- Purpose: Hold the immutable, finalized index and its component structures
- Location: `gin.go` (types: `GINIndex`, `Header`, `PathEntry`, `StringIndex`, `NumericIndex`, `NullIndex`, `StringLengthIndex`)
- Contains: All index struct definitions, type constants, operator enum
- Depends on: Data Structures (RGSet)
- Used by: Query, Serialize, Parquet integration

**Query Layer:**
- Purpose: Evaluate predicates against the index, return matching row groups
- Location: `query.go`
- Contains: `Evaluate()`, per-operator evaluation functions, predicate constructors (`EQ()`, `GT()`, `Contains()`, etc.)
- Depends on: Index Layer, Data Structures, Regex analysis
- Used by: User code, CLI

**Serialization Layer:**
- Purpose: Binary encode/decode the full index with optional zstd compression
- Location: `serialize.go`
- Contains: `Encode()`, `EncodeWithLevel()`, `Decode()`, per-structure read/write functions
- Depends on: Index Layer, Data Structures, zstd, roaring bitmap serialization
- Used by: Parquet integration, CLI, S3 client

**Data Structure Layer:**
- Purpose: Probabilistic and bitmap data structures used across the index
- Location: `bitmap.go` (RGSet), `bloom.go` (BloomFilter), `trigram.go` (TrigramIndex), `hyperloglog.go` (HyperLogLog), `prefix.go` (PrefixCompressor)
- Contains: Self-contained data structure implementations
- Depends on: `roaring/v2` (RGSet), `xxhash/v2` (BloomFilter, HLL)
- Used by: Builder, Index, Query, Serialize

**Parquet Integration Layer:**
- Purpose: Read/write indexes from/to Parquet files (embedded metadata or sidecar `.gin` files)
- Location: `parquet.go`
- Contains: `BuildFromParquet()`, `WriteSidecar()`, `ReadSidecar()`, `EncodeToMetadata()`, `DecodeFromMetadata()`, `LoadIndex()`, `RebuildWithIndex()`, `ParquetIndexWriter`
- Depends on: Builder, Serialize, `parquet-go`
- Used by: CLI, S3 client

**S3 Integration Layer:**
- Purpose: Read/write Parquet files and GIN indexes from AWS S3
- Location: `s3.go`
- Contains: `S3Client`, `S3Config`, `s3ReaderAt` (implements `io.ReaderAt` over S3 range requests)
- Depends on: Parquet integration, Serialize, AWS SDK v2
- Used by: CLI

**Utility Layer:**
- Purpose: JSONPath validation, regex literal extraction
- Location: `jsonpath.go` (path validation via `ojg/jp`), `regex.go` (regex analysis for trigram pruning), `docid.go` (document ID codec abstraction)
- Depends on: `ojg/jp`, `regexp/syntax`
- Used by: Query (regex), user code (path validation, DocID codec)

**CLI Layer:**
- Purpose: Command-line interface for index operations
- Location: `cmd/gin-index/main.go`
- Contains: `build`, `query`, `info`, `extract` subcommands, predicate parser
- Depends on: All other layers
- Used by: End users

## Data Flow

**Index Construction (Build):**

1. User creates `GINConfig` via `NewConfig()` with functional options (or uses `DefaultConfig()`)
2. User creates `GINBuilder` via `NewBuilder(config, numRGs)` -- allocates bloom filter, path data map
3. User calls `builder.AddDocument(docID, jsonBytes)` repeatedly:
   - JSON is unmarshalled via `encoding/json`
   - `walkJSON()` recursively traverses the document tree
   - At each leaf, field transformers are applied if configured for the path
   - Values are routed by type: strings -> string terms + bloom + trigrams; numbers -> min/max stats; nulls -> null bitmap; bools -> string terms
   - HyperLogLog tracks per-path cardinality
4. User calls `builder.Finalize()` to produce an immutable `*GINIndex`:
   - Paths are sorted alphabetically and assigned sequential IDs
   - High-cardinality paths (above threshold) are flagged as bloom-only (no string index)
   - All per-path build data is materialized into index structures

**Query Evaluation:**

1. User constructs predicates using constructors: `gin.EQ("$.field", "value")`
2. User calls `idx.Evaluate(predicates)`:
   - Starts with `AllRGs` (all row groups are candidates)
   - For each predicate, evaluates against the appropriate index structure
   - Results are intersected (AND semantics across predicates)
   - Short-circuits if result becomes empty
3. Per-predicate evaluation strategy depends on operator:
   - **EQ (string):** Bloom filter check -> string length check -> binary search in sorted terms
   - **EQ (numeric):** Global min/max check -> per-RG min/max scan
   - **GT/GTE/LT/LTE:** Global min/max bounds check -> per-RG stats scan
   - **IN:** Union of individual EQ evaluations
   - **NE/NIN:** Complement of EQ/IN intersected with present RGs
   - **Contains:** Trigram index search (intersect trigram bitmaps)
   - **Regex:** Extract literals from regex AST -> query trigram index per literal -> union results
   - **IsNull/IsNotNull:** Direct bitmap lookup from NullIndex

**Serialization:**

1. `Encode(idx)` serializes all structures to a byte buffer in fixed order:
   Header -> PathDirectory -> BloomFilter -> StringIndexes -> StringLengthIndexes -> NumericIndexes -> NullIndexes -> TrigramIndexes -> HyperLogLogs -> DocIDMapping -> Config
2. Outer wrapper prepends a 4-byte magic (`GINc` for compressed, `GINu` for uncompressed)
3. `Decode(data)` reads magic, decompresses if needed, then reads structures in the same order
4. Legacy format (no magic prefix) is supported for backward compatibility

**State Management:**
- `GINBuilder` is mutable during construction, holding all intermediate data in maps
- `GINIndex` is effectively immutable after `Finalize()` -- all fields are populated and not modified
- No shared state or concurrency primitives -- single-threaded build and query

## Key Abstractions

**RGSet (Row Group Set):**
- Purpose: Bitmap representing which row groups match a condition
- File: `bitmap.go`
- Pattern: Wraps `roaring.Bitmap` with bounds checking and set operations
- Operations: `Set()`, `IsSet()`, `Intersect()`, `Union()`, `Invert()`, `Clone()`, `IsEmpty()`, `Count()`, `ToSlice()`
- Factory functions: `AllRGs(n)` (all bits set), `NoRGs(n)` (empty), `MustNewRGSet(n)`

**Predicate:**
- Purpose: Represents a single query condition
- File: `gin.go` (type), `query.go` (constructors + evaluation)
- Pattern: Simple value object with Path + Operator + Value
- Constructor functions: `EQ()`, `NE()`, `GT()`, `GTE()`, `LT()`, `LTE()`, `IN()`, `NIN()`, `IsNull()`, `IsNotNull()`, `Contains()`, `Regex()`

**FieldTransformer:**
- Purpose: Transform field values before indexing (e.g., date strings to epoch milliseconds)
- File: `gin.go` (type definition), `transformers.go` (implementations), `transformer_registry.go` (serialization support)
- Pattern: `func(value any) (any, bool)` -- returns transformed value and success flag
- Registry pattern enables serialization: each transformer has a `TransformerID` and can be reconstructed from ID + params

**DocIDCodec:**
- Purpose: Encode/decode composite document identifiers
- File: `docid.go`
- Pattern: Interface with `Encode(indices ...int) DocID` and `Decode(docID DocID) []int`
- Implementations: `IdentityCodec` (1:1 mapping), `RowGroupCodec` (fileIndex * rowGroupsPerFile + rgIndex)

**GINConfig / ConfigOption:**
- Purpose: Configurable index parameters with sensible defaults
- File: `gin.go`
- Pattern: Functional options returning `error` -- `type ConfigOption func(*GINConfig) error`
- `DefaultConfig()` provides production-ready defaults (bloom=65536, trigrams=enabled, HLL precision=12)

## Entry Points

**Library API (primary):**
- Location: `builder.go:NewBuilder()`, `builder.go:AddDocument()`, `builder.go:Finalize()`
- Triggers: User code importing `github.com/amikos-tech/gin-index`
- Responsibilities: Full build-query-serialize lifecycle

**CLI Tool:**
- Location: `cmd/gin-index/main.go`
- Triggers: `gin-index build|query|info|extract` commands
- Responsibilities: Parquet file operations (build indexes, query, inspect, extract)

**Parquet Integration:**
- Location: `parquet.go:BuildFromParquet()`, `parquet.go:LoadIndex()`
- Triggers: When working with Parquet files directly
- Responsibilities: Read JSON column from Parquet, build index, store as sidecar or embedded metadata

**S3 Integration:**
- Location: `s3.go:NewS3Client()`, `s3.go:S3Client.BuildFromParquet()`
- Triggers: When files are on S3
- Responsibilities: Remote file access via range requests, S3 sidecar management

## Error Handling

**Strategy:** Explicit error returns using `github.com/pkg/errors` for wrapping with context

**Patterns:**
- All constructors return `(*T, error)` -- validation happens at construction time
- `Must*` variants (e.g., `MustNewRGSet`, `MustNewBloomFilter`) panic on error -- used only in builder internals where errors indicate programming bugs
- Errors are wrapped with context at each layer: `errors.Wrap(err, "create bloom filter")`
- `errors.Errorf()` for new errors with formatting
- No sentinel errors -- all errors are string-based
- The query layer does NOT return errors -- unknown paths or unsupported operations return `AllRGs()` (safe fallback = no pruning)

## Cross-Cutting Concerns

**Logging:** None. The library has zero logging -- it is a pure computation library. The CLI uses `fmt.Printf`/`fmt.Fprintf` directly.

**Validation:**
- JSONPath validation via `jsonpath.go:ValidateJSONPath()` using `ojg/jp` parser
- Config validation at construction time via functional options returning errors
- Input validation in constructors (`numRGs > 0`, `precision 4-16`, etc.)
- Regex compile timeout (100ms) in transformer registry to prevent ReDoS

**Authentication:** Not applicable to the library. S3 integration (`s3.go`) uses AWS SDK v2 with env-based credential configuration.

**Compression:** zstd compression (configurable levels 0-19) for serialized index data. Default is level 15 (`CompressionBest`). Supports uncompressed mode and legacy format (backward compatibility).

**Concurrency:** None. The library is single-threaded. No mutexes, no goroutines (except the regex compile timeout in `transformer_registry.go`).

---

*Architecture analysis: 2026-03-24*
