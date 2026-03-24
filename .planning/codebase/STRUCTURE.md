# Codebase Structure

**Analysis Date:** 2026-03-24

## Directory Layout

```
custom-gin/
├── .claude/                    # Claude Code settings
├── .github/workflows/          # CI: code review workflows
├── .planning/codebase/         # GSD codebase analysis docs (this file)
├── cmd/
│   └── gin-index/
│       └── main.go             # CLI tool for Parquet index operations
├── examples/
│   ├── basic/main.go           # EQ queries, builder basics
│   ├── full/main.go            # Full feature demo
│   ├── fulltext/main.go        # Contains/trigram queries
│   ├── nested/main.go          # Nested JSON path queries
│   ├── null/main.go            # Null/not-null queries
│   ├── parquet/main.go         # Parquet file integration
│   ├── range/main.go           # Numeric range queries
│   ├── regex/main.go           # Regex queries with trigram pruning
│   ├── serialize/main.go       # Encode/decode round-trip
│   ├── transformers/main.go    # Field transformer basics
│   └── transformers-advanced/main.go  # Advanced transformer patterns
├── bitmap.go                   # RGSet: row group bitmap (wraps roaring)
├── bloom.go                    # BloomFilter: probabilistic set membership
├── builder.go                  # GINBuilder: index construction
├── docid.go                    # DocID type and codec interface
├── gin.go                      # Core types, config, constants
├── hyperloglog.go              # HyperLogLog: cardinality estimation
├── jsonpath.go                 # JSONPath validation (ojg/jp)
├── parquet.go                  # Parquet file integration (build, read, write)
├── prefix.go                   # PrefixCompressor: front-coding for strings
├── query.go                    # Query evaluation and predicate constructors
├── regex.go                    # Regex literal extraction for trigram pruning
├── s3.go                       # AWS S3 client for remote Parquet/index files
├── serialize.go                # Binary serialization with zstd compression
├── transformer_registry.go     # Serializable transformer ID registry
├── transformers.go             # Built-in field transformer implementations
├── trigram.go                  # TrigramIndex: n-gram index for text search
├── benchmark_test.go           # Performance benchmarks
├── docid_test.go               # DocID codec tests
├── generators_test.go          # Test data generators (rapid/property)
├── gin_test.go                 # Core integration tests
├── integration_property_test.go # Property-based integration tests
├── parquet_test.go             # Parquet integration tests
├── property_test.go            # Property-based tests
├── regex_test.go               # Regex extraction tests
├── transformer_registry_test.go # Transformer registry tests
├── transformers_test.go        # Transformer unit + integration tests
├── go.mod                      # Module: github.com/amikos-tech/gin-index
├── go.sum                      # Dependency checksums
├── Makefile                    # Build, test, lint targets
├── CLAUDE.md                   # Project instructions for Claude Code
├── AGENTS.md                   # Agent guidelines
├── README.md                   # Project documentation
├── .golangci.yml               # Linter configuration
└── gin-index-prd.md            # Product requirements document
```

## Directory Purposes

**Root (`/`):**
- Purpose: All library source code lives here as a single Go package (`package gin`)
- Contains: `.go` source files, `_test.go` test files, config files
- Key files: `gin.go` (types), `builder.go` (construction), `query.go` (evaluation), `serialize.go` (persistence)

**`cmd/gin-index/`:**
- Purpose: CLI binary for index operations on Parquet files
- Contains: Single `main.go` with subcommands: `build`, `query`, `info`, `extract`
- Key files: `cmd/gin-index/main.go`

**`examples/`:**
- Purpose: Runnable examples demonstrating library features
- Contains: One directory per example, each with a `main.go`
- Key files: `examples/basic/main.go` (start here), `examples/parquet/main.go` (Parquet integration)

**`.github/workflows/`:**
- Purpose: GitHub Actions CI configuration
- Contains: Claude code review workflow files

**`.planning/codebase/`:**
- Purpose: GSD codebase analysis documents
- Generated: Yes (by codebase mapping)
- Committed: Yes

## Key File Locations

**Entry Points:**
- `cmd/gin-index/main.go`: CLI tool entry point
- `gin.go`: Library entry point -- core types, `GINConfig`, `NewConfig()`, `DefaultConfig()`
- `builder.go`: Index construction entry -- `NewBuilder()`, `AddDocument()`, `Finalize()`

**Configuration:**
- `go.mod`: Module definition and dependencies
- `Makefile`: Build, test, lint commands
- `.golangci.yml`: Linter rules
- `gin.go:121-247`: `GINConfig` struct and all `ConfigOption` functions

**Core Logic (Index Structures):**
- `gin.go:27-92`: `GINIndex` and all index struct types (`StringIndex`, `NumericIndex`, `NullIndex`, `StringLengthIndex`)
- `bitmap.go`: `RGSet` -- roaring bitmap wrapper for row group sets
- `bloom.go`: `BloomFilter` -- xxhash-based probabilistic filter
- `trigram.go`: `TrigramIndex` -- n-gram to RGSet mapping
- `hyperloglog.go`: `HyperLogLog` -- cardinality estimation
- `prefix.go`: `PrefixCompressor` -- front-coding compression for sorted strings

**Build Pipeline:**
- `builder.go:14-25`: `GINBuilder` struct with all build-time state
- `builder.go:52-75`: `NewBuilder()` constructor
- `builder.go:121-145`: `AddDocument()` -- JSON parse + recursive walk
- `builder.go:147-198`: `walkJSON()` -- recursive JSON traversal with transformer application
- `builder.go:254-379`: `Finalize()` -- materialize all index structures

**Query Engine:**
- `query.go:9-23`: `Evaluate()` -- top-level query entry point (AND semantics)
- `query.go:25-59`: `evaluatePredicate()` -- operator dispatch
- `query.go:70-124`: `evaluateEQ()` -- bloom + string length + string index + numeric index
- `query.go:269-341`: `evaluateContains()` and `evaluateRegex()` -- trigram-based text search
- `query.go:364-410`: Predicate constructor functions (`EQ()`, `GT()`, `Contains()`, etc.)

**Serialization:**
- `serialize.go:81-157`: `Encode()` / `EncodeWithLevel()` -- serialize with configurable zstd compression
- `serialize.go:159-251`: `Decode()` -- deserialize with magic byte detection and legacy format support
- `serialize.go:34-43`: `SerializedConfig` -- JSON-serializable config for round-tripping

**Transformers:**
- `transformers.go`: All 13 built-in `FieldTransformer` implementations
- `transformer_registry.go`: `TransformerID` enum, `TransformerSpec`, `ReconstructTransformer()`
- `gin.go:143-225`: `WithFieldTransformer()` and convenience options (`WithISODateTransformer()`, etc.)

**Parquet Integration:**
- `parquet.go:72-151`: `BuildFromParquet()` -- build index from Parquet file's JSON column
- `parquet.go:30-52`: Sidecar file operations (`WriteSidecar`, `ReadSidecar`, `HasSidecar`)
- `parquet.go:153-171`: Metadata embedding (`EncodeToMetadata`, `DecodeFromMetadata`)
- `parquet.go:245-307`: `RebuildWithIndex()` -- rewrite Parquet file with embedded index
- `parquet.go:309-330`: `LoadIndex()` -- try metadata first, fallback to sidecar

**S3 Integration:**
- `s3.go:20-46`: `S3Config`, `S3ConfigFromEnv()`
- `s3.go:53-82`: `NewS3Client()` -- create client from config
- `s3.go:88-119`: `s3ReaderAt` -- `io.ReaderAt` implementation over S3 range requests
- `s3.go:207-269`: `S3Client.BuildFromParquet()`, `S3Client.LoadIndex()`

**Utilities:**
- `jsonpath.go:21-93`: `ValidateJSONPath()` -- whitelist-based path validation
- `regex.go`: `ExtractLiterals()`, `AnalyzeRegex()` -- regex AST analysis for trigram pruning
- `docid.go`: `DocID` type, `DocIDCodec` interface, `IdentityCodec`, `RowGroupCodec`

**Testing:**
- `gin_test.go`: Main integration tests covering all operators and index types
- `benchmark_test.go`: Performance benchmarks
- `property_test.go`: Property-based tests (likely using `gopter`)
- `integration_property_test.go`: End-to-end property-based tests
- `generators_test.go`: Test data generators for property tests
- `regex_test.go`: Regex literal extraction tests
- `transformers_test.go`: Transformer unit + integration tests
- `transformer_registry_test.go`: Registry serialization round-trip tests
- `docid_test.go`: DocID codec tests
- `parquet_test.go`: Parquet integration tests

## Naming Conventions

**Files:**
- Lowercase, single-word or hyphenated: `bitmap.go`, `transformer_registry.go`
- Test files co-located with source: `transformers.go` / `transformers_test.go`
- One primary type or concept per file: `bloom.go` = `BloomFilter`, `hyperloglog.go` = `HyperLogLog`

**Directories:**
- `cmd/<binary-name>/`: CLI binaries
- `examples/<feature-name>/`: Example programs

**Types:**
- PascalCase exported types: `GINIndex`, `GINBuilder`, `RGSet`, `BloomFilter`
- Prefix `GIN` for core index types: `GINIndex`, `GINConfig`, `GINBuilder`
- `*Option` suffix for functional option types: `ConfigOption`, `BuilderOption`, `NGramOption`
- `*Index` suffix for index structures: `StringIndex`, `NumericIndex`, `TrigramIndex`

**Functions:**
- `New*()` constructors return `(*T, error)`: `NewBuilder()`, `NewBloomFilter()`, `NewRGSet()`
- `MustNew*()` panic variants for internal use: `MustNewRGSet()`, `MustNewBloomFilter()`
- `With*()` for functional options: `WithFieldTransformer()`, `WithCodec()`, `WithN()`
- Operator constructors are bare names: `EQ()`, `GT()`, `Contains()`, `Regex()`

**Constants:**
- PascalCase: `OpEQ`, `TypeString`, `FlagBloomOnly`, `CompressionBest`
- `Op` prefix for operators, `Type` prefix for value types, `Flag` prefix for bitflags
- `Transformer` prefix for transformer IDs: `TransformerISODateToEpochMs`

## Where to Add New Code

**New Query Operator:**
1. Add constant to operator enum in `gin.go` (e.g., `OpStartsWith`)
2. Add constructor function in `query.go` (e.g., `func StartsWith(path, prefix string) Predicate`)
3. Add `case` in `evaluatePredicate()` switch in `query.go`
4. Implement `evaluateStartsWith()` method on `*GINIndex` in `query.go`
5. Add `String()` case in `query.go`
6. Add tests in `gin_test.go`
7. Add CLI parser support in `cmd/gin-index/main.go:parsePredicate()`

**New Index Structure:**
1. Define struct type in `gin.go` (e.g., `type FooIndex struct { ... }`)
2. Add field to `GINIndex` struct in `gin.go`
3. Initialize in `NewGINIndex()` in `gin.go`
4. Populate in `builder.go:Finalize()`
5. Add `writeFooIndex()` / `readFooIndex()` in `serialize.go`
6. Call write/read in `EncodeWithLevel()` / `Decode()` (maintain order)
7. Use in query evaluation in `query.go`

**New Field Transformer:**
1. Implement transformer function in `transformers.go` matching `FieldTransformer` signature
2. Add `TransformerID` constant in `transformer_registry.go`
3. Add name mapping in `transformerNames` map in `transformer_registry.go`
4. Add reconstruction case in `ReconstructTransformer()` in `transformer_registry.go`
5. Add convenience `ConfigOption` in `gin.go` (e.g., `WithFooTransformer()`)
6. Add tests in `transformers_test.go`
7. Add registry round-trip test in `transformer_registry_test.go`

**New Example:**
1. Create `examples/<feature-name>/main.go`
2. Use `package main` with `func main()` wrapping a `func run() error`
3. Import as `gin "github.com/amikos-tech/gin-index"`

**New CLI Subcommand:**
1. Add case in `main()` switch in `cmd/gin-index/main.go`
2. Implement `cmd<Name>(args []string)` function
3. Update `printUsage()`

**New Data Structure:**
1. Create `<name>.go` at package root
2. Follow the pattern: unexported struct fields + exported constructor `New<Name>()` + `Must<Name>()` variant
3. Use functional options: `type <Name>Option func(*<Name>) error`
4. Add serialization support in `serialize.go` if needed

## Special Directories

**`.planning/codebase/`:**
- Purpose: GSD codebase analysis documentation
- Generated: Yes (by codebase mapping tool)
- Committed: Yes

**`cmd/`:**
- Purpose: Go binary entry points (follows standard Go project layout)
- Generated: No
- Committed: Yes

**`examples/`:**
- Purpose: Runnable example programs for documentation
- Generated: No
- Committed: Yes

## Module Boundaries

This project is a **single Go module** (`github.com/amikos-tech/gin-index`) with a **single package** (`package gin`). There are no internal packages or sub-packages. All library code is in the root package.

The only separate `package main` programs are:
- `cmd/gin-index/main.go` -- CLI tool
- `examples/*/main.go` -- Example programs

All test files are in `package gin` (not `package gin_test`), giving them access to unexported types and functions.

---

*Structure analysis: 2026-03-24*
