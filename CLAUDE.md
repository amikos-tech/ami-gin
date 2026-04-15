# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GIN Index is a Generalized Inverted Index for JSON data, designed for row-group pruning in columnar storage (Parquet). It enables fast predicate evaluation to determine which row groups may contain matching documents.

## Build and Test Commands

```bash
# Build
go build ./...

# Run all tests
go test -v

# Run specific test
go test -v -run TestQueryEQ

# Run examples
go run ./examples/basic/main.go
```

## Architecture

### Core Data Flow

1. **Builder** (`builder.go`) - Ingests JSON documents via `AddDocument(rgID, jsonDoc)`, walks JSON structure, extracts paths/values
2. **Index** (`gin.go`) - Final immutable index created by `Finalize()`, contains all index structures
3. **Query** (`query.go`) - Evaluates predicates against index, returns `RGSet` bitmap of matching row groups
4. **Serialize** (`serialize.go`) - Binary encoding with zstd compression via `Encode()`/`Decode()`

### Index Structures (all keyed by pathID)

- **StringIndex** - Sorted terms with parallel RG bitmaps for exact match
- **NumericIndex** - Per-RG min/max stats for range query pruning
- **NullIndex** - Two bitmaps per path: null RGs and present RGs
- **TrigramIndex** - N-gram to RG bitmap mapping for CONTAINS queries
- **GlobalBloom** - Bloom filter for fast path=value rejection
- **PathCardinality** - HyperLogLog per path for cardinality estimation

### Key Types

- `RGSet` (`bitmap.go`) - Row group bitmap with Set/Intersect/Union operations
- `Predicate` - Query condition: Path + Operator + Value
- `GINConfig` - Builder configuration (bloom size, trigram settings, HLL precision)

### JSONPath Validation

Uses `ojg/jp` library. Only supports: `$`, `$.field`, `$['field']`, `$[*]`. Rejects array indices, recursive descent, slices, filters - see `jsonpath.go`.

### Supported Operators

`EQ`, `NE`, `GT`, `GTE`, `LT`, `LTE`, `IN`, `NIN`, `IsNull`, `IsNotNull`, `Contains`, `Regex`

### Regex Query Support

The `Regex` operator uses trigram index for candidate row-group selection before pattern matching.

**Files:**
- `regex.go` - `ExtractLiterals()`, `AnalyzeRegex()`, literal extraction from regex patterns
- `query.go:289` - `evaluateRegex()` implementation

**How it works:**
1. Parse regex using `regexp/syntax` with Perl mode
2. Apply `Simplify()` (factors common prefixes: `Toyota|Tesla` → `T(oyota|esla)`)
3. Extract combined literals via Cartesian product (e.g., `(error|warn)_msg` → `["error_msg", "warn_msg"]`)
4. Query trigram index for each literal, union results
5. Row groups not containing any literal are pruned

**Key functions:**
- `extractCombinedLiterals(re)` - Recursive literal extraction with Cartesian product for concatenation
- `extractConcatLiterals(subs)` - Handles `OpConcat` by building combined strings
- `hasUnboundedWildcard(re)` - Detects `.*` or `.+` patterns

### Field Transformers

Transform values before indexing via `GINConfig.fieldTransformers`. Use cases: date range queries, IP subnet filtering, version comparisons, case-insensitive search.

**Types:**
- `FieldTransformer` - `func(value any) (any, bool)` - returns transformed value and success flag

**Built-in transformers:**

| Category | Transformer | Description | Example |
|----------|-------------|-------------|---------|
| Date | `ISODateToEpochMs` | RFC3339/ISO8601 to epoch ms | `2024-01-15T10:30:00Z` → `1705315800000` |
| Date | `DateToEpochMs` | YYYY-MM-DD to epoch ms | `2024-01-15` → `1705276800000` |
| Date | `CustomDateToEpochMs(layout)` | Custom format to epoch ms | Layout: `2006/01/02 15:04` |
| String | `ToLower` | Lowercase normalization | `Alice@Example.COM` → `alice@example.com` |
| String | `EmailDomain` | Extract domain from email | `alice@example.com` → `example.com` |
| String | `URLHost` | Extract host from URL | `https://api.example.com/v1` → `api.example.com` |
| String | `RegexExtract(pattern, group)` | Extract via regex capture | Pattern: `ERROR\[(\w+)\]:`, group 1 |
| Numeric | `RegexExtractInt(pattern, group)` | Extract + convert to float64 | `order-12345` → `12345` |
| Numeric | `IPv4ToInt` | IPv4 to uint32 for ranges | `192.168.1.1` → `3232235777` |
| Helper | `CIDRToRange(cidr)` | Parse CIDR to start/end float64 | `192.168.1.0/24` → `(start, end)` |
| Helper | `InSubnet(path, cidr)` | Returns []Predicate for subnet check | `InSubnet("$.ip", "10.0.0.0/8")` |
| Numeric | `SemVerToInt` | Semver to int (major*1M+minor*1K+patch) | `v2.1.3` → `2001003` |
| Numeric | `DurationToMs` | Go duration to ms | `1h30m` → `5400000` |
| Numeric | `NumericBucket(size)` | Bucket values | `150` with size `100` → `100` |
| Boolean | `BoolNormalize` | Normalize boolean-like values | `"yes"`, `"1"`, `"on"` → `true` |

**Files:**
- `gin.go` - `FieldTransformer` type, `GINConfig.fieldTransformers`, `WithFieldTransformer` option
- `transformers.go` - All built-in transformers
- `transformers_test.go` - Unit and integration tests
- `builder.go` - Transformer application in `decodeTransformedValue()` and `stageMaterializedValue()` before type switch

## Go Conventions

**Constructors**: Use functional options pattern with two-phase validation

```go
type FooOption func(*Foo) error                    // Options return errors

func WithBar(bar string) FooOption {               // Option-level validation
    return func(f *Foo) error {
        if bar == "" { return errors.New("bar required") }
        f.bar = bar
        return nil
    }
}

func NewFoo(opts ...FooOption) (*Foo, error) {
    f := &Foo{}
    for _, opt := range opts {                     // Apply options, fail fast
        if err := opt(f); err != nil { return nil, err }
    }
    if err := validator.New().Struct(f); err != nil {  // Struct validation
        return nil, err
    }
    return f, nil
}
```

Reference: `pkg/catalog/pg_catalog.go:86`. Note: `validator.New()` in constructors is fine; cache validators for hot paths.

**Validation**: Use `github.com/go-playground/validator/v10` for all struct validation

- Register custom validators via `Validator.RegisterValidation()`
- Use struct tags: `validate:"required,at_least_one_host"`

**Defaults**: Use `github.com/creasty/defaults` for struct default values

- See `pkg/types/logservice_defaults.go` for examples
- Use struct tags: `default:"value"`
- Call `defaults.Set(&struct)` to apply

**Error Handling**: Use `github.com/pkg/errors` for all error creation and propagation

- `errors.New("message")` for new errors (captures stack trace)
- `errors.Errorf("format %s", val)` for formatted new errors (captures stack trace)
- `errors.Wrap(err, "context")` to wrap existing errors (captures stack trace at wrap point)
- `errors.Wrapf(err, "context %s", val)` for formatted wrap (captures stack trace at wrap point)
- **DEPRECATED**: `fmt.Errorf` with `%w` - migrate to `errors.Wrap`/`errors.Wrapf` (see #1670)
- Use `errors.Cause(err)` to get root cause, `errors.Is()`/`errors.As()` for comparison
- Reference: `pkg/catalog/pg_catalog.go` for usage patterns

---

## Makefile Conventions

**Required targets**: `test`, `integration-test`, `lint`, `lint-fix`, `security-scan`, `clean`, `help`

**Learn more**: Use `tclr-makefile` skill for target specifications, templates, and examples.

<!-- GSD:project-start source:PROJECT.md -->
## Project

**GIN Index — Open Source Readiness**

GIN Index is a Generalized Inverted Index library for JSON data, designed for row-group pruning in columnar storage (Parquet). It enables fast predicate evaluation to determine which row groups may contain matching documents — filling a gap between full-scan and standing up a database. This project tracks the work needed to take it from a private repo to a credible public open-source release.

**Core Value:** A credible first impression: anyone who finds the repo can immediately understand, build, test, and contribute — with no internal artifacts leaking through.

### Constraints

- **License**: MIT — simple, permissive, compatible with all dependency licenses
- **Module path**: Must be `github.com/amikos-tech/ami-gin` to match the GitHub repo URL
- **Go version**: 1.25.5 (already current)
- **No breaking API changes**: Existing API surface is clean — preserve it through the OSS transition
<!-- GSD:project-end -->

<!-- GSD:stack-start source:codebase/STACK.md -->
## Technology Stack

## Languages
- Go 1.25.5 - Entire codebase (library + CLI tool)
- None
## Runtime
- Go toolchain 1.25.5 (specified in `go.mod`)
- No `.go-version` or `.tool-versions` file detected
- Go modules (`go.mod` + `go.sum`)
- Lockfile: `go.sum` present (67KB)
- Module path: `github.com/amikos-tech/ami-gin`
## Frameworks
- No web or application framework. This is a standalone Go library with a CLI tool.
- `testing` (stdlib) - Unit and benchmark tests
- `github.com/leanovate/gopter` v0.2.11 - Property-based testing (generators, properties, shrinking)
- `gotest.tools/gotestsum` (installed at test time via Makefile) - Test runner with JUnit output
- `go build` - Standard Go build
- `golangci-lint` - Linting (config: `.golangci.yml`)
- `make` - Build automation (`Makefile`)
- Linters enabled: `dupword`, `gocritic`, `mirror`
- Formatter: `gci` (import ordering)
- Import order: standard -> third-party -> `github.com/amikos-tech/ami-gin` -> blank -> dot
- `errcheck` suppressed for `_test.go` and `examples/`
- Timeout: 5m, concurrency: 4
## Key Dependencies
- `github.com/cespare/xxhash/v2` v2.3.0 - Fast non-cryptographic hash function; used by BloomFilter (`bloom.go`) and HyperLogLog (`hyperloglog.go`) for hashing values
- `github.com/klauspost/compress` v1.18.3 - zstd compression/decompression for index serialization (`serialize.go`); supports configurable compression levels 0-19
- `github.com/RoaringBitmap/roaring/v2` v2.14.4 - Compressed bitmap data structure; underlies `RGSet` (`bitmap.go`) for row-group tracking with set operations (And/Or/AndNot)
- `github.com/ohler55/ojg` v1.28.0 - JSONPath parsing library; used in `jsonpath.go` for path validation via `ojg/jp` subpackage
- `github.com/parquet-go/parquet-go` v0.27.0 - Apache Parquet file reading/writing; used in `parquet.go` for building indexes from Parquet files and embedding indexes in Parquet metadata
- `github.com/aws/aws-sdk-go-v2` v1.41.1 + service packages - AWS S3 integration for remote Parquet file access (`s3.go`); includes `config`, `credentials`, `service/s3` subpackages
- `github.com/pkg/errors` v0.9.1 - Error wrapping with stack traces; used throughout for `errors.New()`, `errors.Wrap()`, `errors.Errorf()`
- `github.com/leanovate/gopter` v0.2.11 - Property-based testing framework; used in `generators_test.go`, `property_test.go`, `integration_property_test.go`
- `github.com/bits-and-blooms/bitset` v1.24.2 - Dependency of roaring bitmaps
- `github.com/mschoch/smat` v0.2.0 - Dependency of roaring bitmaps (state machine testing)
- `github.com/andybalholm/brotli` v1.1.1 - Compression codec for parquet-go
- `github.com/pierrec/lz4/v4` v4.1.21 - Compression codec for parquet-go
- `github.com/google/uuid` v1.6.0 - Dependency of parquet-go
- `github.com/parquet-go/bitpack` v1.0.0 - Bit-packing for parquet-go
- `github.com/parquet-go/jsonlite` v1.0.0 - JSON handling for parquet-go
- `github.com/twpayne/go-geom` v1.6.1 - Geometry types for parquet-go
- `golang.org/x/sys` v0.38.0 - System calls (dependency of compress, parquet)
- `google.golang.org/protobuf` v1.34.2 - Protocol buffers (dependency of parquet-go)
## Configuration
- `AWS_ENDPOINT_URL` or `AWS_S3_ENDPOINT` - Custom S3 endpoint (e.g., MinIO)
- `AWS_REGION` or `AWS_DEFAULT_REGION` - AWS region (defaults to `us-east-1`)
- `AWS_ACCESS_KEY_ID` - S3 access key
- `AWS_SECRET_ACCESS_KEY` - S3 secret key
- `AWS_S3_PATH_STYLE` - Set to `"true"` for path-style S3 access
- `GINConfig` struct in `gin.go` with functional options pattern (`ConfigOption`)
- Defaults: bloom 65536 bits / 5 hashes, trigrams enabled, HLL precision 12, prefix block 16, cardinality threshold 10000
- `go.mod` - Module definition and dependencies
- `.golangci.yml` - Linter configuration
- `Makefile` - Build targets: `build`, `test`, `lint`, `lint-fix`, `clean`, `help`
## Platform Requirements
- Go 1.25.5+
- `golangci-lint` for linting
- `gotestsum` (auto-installed by `make test`)
- No CGo dependencies - pure Go
- Standalone Go binary (no runtime dependencies)
- AWS credentials needed only for S3 operations
- File system access for local Parquet/sidecar operations
## CLI Tool
- `build` - Build GIN index from Parquet file(s) (local or S3)
- `query` - Evaluate predicates against an index
- `info` - Display index metadata and path directory
- `extract` - Extract embedded index to sidecar file
## CI/CD
- `.github/workflows/claude-code-review.yml` - Claude-based code review
- `.github/workflows/claude.yml` - Claude workflow
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

## Package Organization
- `cmd/gin-index/main.go` - CLI entry point
- `examples/*/main.go` - Runnable examples (basic, full, fulltext, nested, null, parquet, range, regex, serialize, transformers, transformers-advanced)
## Naming Patterns
- Single lowercase word per file: `bitmap.go`, `bloom.go`, `builder.go`, `query.go`
- Compound names use underscores: `transformer_registry.go`
- Test files co-located with matching `_test.go` suffix: `gin_test.go`, `regex_test.go`
- Specialized test files named by type: `property_test.go`, `benchmark_test.go`, `generators_test.go`, `integration_property_test.go`
- PascalCase: `GINIndex`, `GINBuilder`, `GINConfig`, `RGSet`, `BloomFilter`, `HyperLogLog`, `TrigramIndex`
- Acronyms stay uppercase: `GIN`, `RG`, `HLL`, `FTS`, `IP`
- Internal build types use camelCase: `pathBuildData`
- Type aliases use PascalCase: `DocID`, `Operator`, `FieldTransformer`, `TransformerID`
- Constructors: `NewXxx(...)` returning `(*Xxx, error)` - e.g., `NewBuilder()`, `NewBloomFilter()`, `NewRGSet()`
- Must-constructors: `MustNewXxx(...)` wrapping `NewXxx` with `panic` on error - e.g., `MustNewRGSet()`, `MustNewBloomFilter()`
- Predicate builders are top-level functions returning `Predicate`: `EQ()`, `GT()`, `IN()`, `Contains()`, `Regex()`, `IsNull()`
- Transformer functions are top-level: `ISODateToEpochMs()`, `ToLower()`, `IPv4ToInt()`, `SemVerToInt()`
- Helper functions that return closures: `CustomDateToEpochMs(layout)`, `RegexExtract(pattern, group)`, `NumericBucket(size)`
- camelCase for locals and unexported fields
- PascalCase for exported fields: `NumRGs`, `Trigrams`, `GlobalMin`
- Constants: PascalCase for exported (`MagicBytes`, `Version`), camelCase for unexported (`maxLiteralExpansion`, `maxConfigSize`)
- Iota enums: `Op`-prefixed for operators (`OpEQ`, `OpNE`, `OpGT`), `Type`-prefixed for types (`TypeString`, `TypeInt`), `Flag`-prefixed for flags (`FlagBloomOnly`, `FlagTrigramIndex`), `Transformer`-prefixed for IDs (`TransformerISODateToEpochMs`)
- Name by behavior: `DocIDCodec` (encodes/decodes DocIDs)
- Methods: `Encode()`, `Decode()`, `Name()`
## Functional Options Pattern
- `WithISODateTransformer(path)`, `WithToLowerTransformer(path)`, `WithIPv4Transformer(path)` etc.
- These wrap `WithRegisteredTransformer()` which handles both runtime and serialization config.
## Error Handling
## Import Organization
## Code Style
- Linters enabled: `dupword`, `gocritic`, `mirror`
- `staticcheck` with all checks except `ST1000` (package comments), `ST1003` (naming), `ST1020` (exported comments)
- `errcheck` excluded for `_test.go` files and `examples/` directory
- Import ordering enforced by `gci` formatter: standard, default, then project prefix
## Type Switch Pattern
## Must* Pattern
## Comment and Documentation Style
## Module Structure
- `bitmap.go` - RGSet type and operations
- `bloom.go` - BloomFilter type
- `builder.go` - GINBuilder (index construction)
- `gin.go` - GINIndex, GINConfig, types, options
- `query.go` - Query evaluation and predicate constructors
- `serialize.go` - Binary encoding/decoding with compression
- `regex.go` - Regex literal extraction for trigram optimization
- `transformers.go` - Built-in field transformer functions
- `transformer_registry.go` - Transformer serialization/reconstruction
- `parquet.go` - Parquet file integration
- `s3.go` - S3 storage integration
- `docid.go` - DocID codec abstraction
- `jsonpath.go` - JSONPath validation
- `hyperloglog.go` - HyperLogLog cardinality estimator
- `trigram.go` - N-gram index for CONTAINS queries
- `prefix.go` - Prefix compression for sorted strings
## Constant Groups
## Default Configuration
## Immutable Results
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

## Pattern Overview
- Single flat Go package (`package gin`) -- all core types and logic live at the module root
- Builder pattern for index construction (mutable) producing an immutable index for querying
- Binary serialization with zstd compression for compact storage
- Multiple index structures (string, numeric, null, trigram, bloom, HLL) keyed by path ID
- Row-group-level granularity -- the index answers "which row groups MAY contain matching documents"
- Functional options pattern for configuration (`ConfigOption`, `BuilderOption`, `NGramOption`, etc.)
## Layers
- Purpose: Define index build parameters and field transformers
- Location: `gin.go` (types + options), `transformers.go` (built-in transformers), `transformer_registry.go` (serializable transformer registry)
- Contains: `GINConfig`, `ConfigOption` functions (`WithFieldTransformer`, `WithISODateTransformer`, etc.), `FieldTransformer` type, `TransformerSpec` for serialization
- Depends on: Nothing (leaf layer)
- Used by: Builder, Serialize
- Purpose: Ingest JSON documents, walk their structure, populate index data structures
- Location: `builder.go`
- Contains: `GINBuilder`, `pathBuildData`, `AddDocument()`, `walkJSON()`, `Finalize()`
- Depends on: Configuration, Data Structures (RGSet, BloomFilter, TrigramIndex, HyperLogLog)
- Used by: User code, Parquet integration, CLI
- Purpose: Hold the immutable, finalized index and its component structures
- Location: `gin.go` (types: `GINIndex`, `Header`, `PathEntry`, `StringIndex`, `NumericIndex`, `NullIndex`, `StringLengthIndex`)
- Contains: All index struct definitions, type constants, operator enum
- Depends on: Data Structures (RGSet)
- Used by: Query, Serialize, Parquet integration
- Purpose: Evaluate predicates against the index, return matching row groups
- Location: `query.go`
- Contains: `Evaluate()`, per-operator evaluation functions, predicate constructors (`EQ()`, `GT()`, `Contains()`, etc.)
- Depends on: Index Layer, Data Structures, Regex analysis
- Used by: User code, CLI
- Purpose: Binary encode/decode the full index with optional zstd compression
- Location: `serialize.go`
- Contains: `Encode()`, `EncodeWithLevel()`, `Decode()`, per-structure read/write functions
- Depends on: Index Layer, Data Structures, zstd, roaring bitmap serialization
- Used by: Parquet integration, CLI, S3 client
- Purpose: Probabilistic and bitmap data structures used across the index
- Location: `bitmap.go` (RGSet), `bloom.go` (BloomFilter), `trigram.go` (TrigramIndex), `hyperloglog.go` (HyperLogLog), `prefix.go` (PrefixCompressor)
- Contains: Self-contained data structure implementations
- Depends on: `roaring/v2` (RGSet), `xxhash/v2` (BloomFilter, HLL)
- Used by: Builder, Index, Query, Serialize
- Purpose: Read/write indexes from/to Parquet files (embedded metadata or sidecar `.gin` files)
- Location: `parquet.go`
- Contains: `BuildFromParquet()`, `WriteSidecar()`, `ReadSidecar()`, `EncodeToMetadata()`, `DecodeFromMetadata()`, `LoadIndex()`, `RebuildWithIndex()`, `ParquetIndexWriter`
- Depends on: Builder, Serialize, `parquet-go`
- Used by: CLI, S3 client
- Purpose: Read/write Parquet files and GIN indexes from AWS S3
- Location: `s3.go`
- Contains: `S3Client`, `S3Config`, `s3ReaderAt` (implements `io.ReaderAt` over S3 range requests)
- Depends on: Parquet integration, Serialize, AWS SDK v2
- Used by: CLI
- Purpose: JSONPath validation, regex literal extraction
- Location: `jsonpath.go` (path validation via `ojg/jp`), `regex.go` (regex analysis for trigram pruning), `docid.go` (document ID codec abstraction)
- Depends on: `ojg/jp`, `regexp/syntax`
- Used by: Query (regex), user code (path validation, DocID codec)
- Purpose: Command-line interface for index operations
- Location: `cmd/gin-index/main.go`
- Contains: `build`, `query`, `info`, `extract` subcommands, predicate parser
- Depends on: All other layers
- Used by: End users
## Data Flow
- `GINBuilder` is mutable during construction, holding all intermediate data in maps
- `GINIndex` is effectively immutable after `Finalize()` -- all fields are populated and not modified
- No shared state or concurrency primitives -- single-threaded build and query
## Key Abstractions
- Purpose: Bitmap representing which row groups match a condition
- File: `bitmap.go`
- Pattern: Wraps `roaring.Bitmap` with bounds checking and set operations
- Operations: `Set()`, `IsSet()`, `Intersect()`, `Union()`, `Invert()`, `Clone()`, `IsEmpty()`, `Count()`, `ToSlice()`
- Factory functions: `AllRGs(n)` (all bits set), `NoRGs(n)` (empty), `MustNewRGSet(n)`
- Purpose: Represents a single query condition
- File: `gin.go` (type), `query.go` (constructors + evaluation)
- Pattern: Simple value object with Path + Operator + Value
- Constructor functions: `EQ()`, `NE()`, `GT()`, `GTE()`, `LT()`, `LTE()`, `IN()`, `NIN()`, `IsNull()`, `IsNotNull()`, `Contains()`, `Regex()`
- Purpose: Transform field values before indexing (e.g., date strings to epoch milliseconds)
- File: `gin.go` (type definition), `transformers.go` (implementations), `transformer_registry.go` (serialization support)
- Pattern: `func(value any) (any, bool)` -- returns transformed value and success flag
- Registry pattern enables serialization: each transformer has a `TransformerID` and can be reconstructed from ID + params
- Purpose: Encode/decode composite document identifiers
- File: `docid.go`
- Pattern: Interface with `Encode(indices ...int) DocID` and `Decode(docID DocID) []int`
- Implementations: `IdentityCodec` (1:1 mapping), `RowGroupCodec` (fileIndex * rowGroupsPerFile + rgIndex)
- Purpose: Configurable index parameters with sensible defaults
- File: `gin.go`
- Pattern: Functional options returning `error` -- `type ConfigOption func(*GINConfig) error`
- `DefaultConfig()` provides production-ready defaults (bloom=65536, trigrams=enabled, HLL precision=12)
## Entry Points
- Location: `builder.go:NewBuilder()`, `builder.go:AddDocument()`, `builder.go:Finalize()`
- Triggers: User code importing `github.com/amikos-tech/ami-gin`
- Responsibilities: Full build-query-serialize lifecycle
- Location: `cmd/gin-index/main.go`
- Triggers: `gin-index build|query|info|extract` commands
- Responsibilities: Parquet file operations (build indexes, query, inspect, extract)
- Location: `parquet.go:BuildFromParquet()`, `parquet.go:LoadIndex()`
- Triggers: When working with Parquet files directly
- Responsibilities: Read JSON column from Parquet, build index, store as sidecar or embedded metadata
- Location: `s3.go:NewS3Client()`, `s3.go:S3Client.BuildFromParquet()`
- Triggers: When files are on S3
- Responsibilities: Remote file access via range requests, S3 sidecar management
## Error Handling
- All constructors return `(*T, error)` -- validation happens at construction time
- `Must*` variants (e.g., `MustNewRGSet`, `MustNewBloomFilter`) panic on error -- used only in builder internals where errors indicate programming bugs
- Errors are wrapped with context at each layer: `errors.Wrap(err, "create bloom filter")`
- `errors.Errorf()` for new errors with formatting
- No sentinel errors -- all errors are string-based
- The query layer does NOT return errors -- unknown paths or unsupported operations return `AllRGs()` (safe fallback = no pruning)
## Cross-Cutting Concerns
- JSONPath validation via `jsonpath.go:ValidateJSONPath()` using `ojg/jp` parser
- Config validation at construction time via functional options returning errors
- Input validation in constructors (`numRGs > 0`, `precision 4-16`, etc.)
- Regex compile timeout (100ms) in transformer registry to prevent ReDoS
<!-- GSD:architecture-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd:quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd:debug` for investigation and bug fixing
- `/gsd:execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->

<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd:profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
