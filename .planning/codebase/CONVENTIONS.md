# Coding Conventions

**Analysis Date:** 2026-03-24

## Package Organization

**Single package:** The entire library is in package `gin` at the root. All `.go` files share the package name. There are no sub-packages for the core library.

**Ancillary packages:**
- `cmd/gin-index/main.go` - CLI entry point
- `examples/*/main.go` - Runnable examples (basic, full, fulltext, nested, null, parquet, range, regex, serialize, transformers, transformers-advanced)

**Rule:** Place all core library code in the root package `gin`. Use `cmd/` for CLI tools and `examples/` for runnable demos.

## Naming Patterns

**Files:**
- Single lowercase word per file: `bitmap.go`, `bloom.go`, `builder.go`, `query.go`
- Compound names use underscores: `transformer_registry.go`
- Test files co-located with matching `_test.go` suffix: `gin_test.go`, `regex_test.go`
- Specialized test files named by type: `property_test.go`, `benchmark_test.go`, `generators_test.go`, `integration_property_test.go`

**Types:**
- PascalCase: `GINIndex`, `GINBuilder`, `GINConfig`, `RGSet`, `BloomFilter`, `HyperLogLog`, `TrigramIndex`
- Acronyms stay uppercase: `GIN`, `RG`, `HLL`, `FTS`, `IP`
- Internal build types use camelCase: `pathBuildData`
- Type aliases use PascalCase: `DocID`, `Operator`, `FieldTransformer`, `TransformerID`

**Functions:**
- Constructors: `NewXxx(...)` returning `(*Xxx, error)` - e.g., `NewBuilder()`, `NewBloomFilter()`, `NewRGSet()`
- Must-constructors: `MustNewXxx(...)` wrapping `NewXxx` with `panic` on error - e.g., `MustNewRGSet()`, `MustNewBloomFilter()`
- Predicate builders are top-level functions returning `Predicate`: `EQ()`, `GT()`, `IN()`, `Contains()`, `Regex()`, `IsNull()`
- Transformer functions are top-level: `ISODateToEpochMs()`, `ToLower()`, `IPv4ToInt()`, `SemVerToInt()`
- Helper functions that return closures: `CustomDateToEpochMs(layout)`, `RegexExtract(pattern, group)`, `NumericBucket(size)`

**Variables:**
- camelCase for locals and unexported fields
- PascalCase for exported fields: `NumRGs`, `Trigrams`, `GlobalMin`
- Constants: PascalCase for exported (`MagicBytes`, `Version`), camelCase for unexported (`maxLiteralExpansion`, `maxConfigSize`)
- Iota enums: `Op`-prefixed for operators (`OpEQ`, `OpNE`, `OpGT`), `Type`-prefixed for types (`TypeString`, `TypeInt`), `Flag`-prefixed for flags (`FlagBloomOnly`, `FlagTrigramIndex`), `Transformer`-prefixed for IDs (`TransformerISODateToEpochMs`)

**Interfaces:**
- Name by behavior: `DocIDCodec` (encodes/decodes DocIDs)
- Methods: `Encode()`, `Decode()`, `Name()`

## Functional Options Pattern

All constructors use the functional options pattern consistently:

```go
// Option type is a function that modifies config and returns error
type ConfigOption func(*GINConfig) error
type BuilderOption func(*GINBuilder) error
type RGSetOption func(*RGSet) error
type BloomFilterOption func(*BloomFilter) error
type HyperLogLogOption func(*HyperLogLog) error
type NGramOption func(*NGramConfig) error
type PrefixCompressorOption func(*PrefixCompressor) error

// Option constructors named With*
func WithCodec(codec DocIDCodec) BuilderOption {
    return func(b *GINBuilder) error {
        if codec == nil {
            return errors.New("codec cannot be nil")
        }
        b.codec = codec
        return nil
    }
}

// Constructor applies options and fails fast
func NewBuilder(config GINConfig, numRGs int, opts ...BuilderOption) (*GINBuilder, error) {
    // ... create struct ...
    for _, opt := range opts {
        if err := opt(b); err != nil {
            return nil, err
        }
    }
    return b, nil
}
```

**Rule:** Every type with configurable parameters uses this pattern. Option functions validate their inputs and return errors. Constructors apply options in order and fail fast.

**Convenience config options:** For common transformer configurations, use shorthand `ConfigOption` constructors:
- `WithISODateTransformer(path)`, `WithToLowerTransformer(path)`, `WithIPv4Transformer(path)` etc.
- These wrap `WithRegisteredTransformer()` which handles both runtime and serialization config.

## Error Handling

**Package:** Use `github.com/pkg/errors` for all error creation. Never use `fmt.Errorf` with `%w`.

**Patterns:**
```go
// New errors with context
errors.New("numRGs must be greater than 0")
errors.Errorf("precision must be between 4 and 16, got %d", precision)

// Wrapping existing errors
errors.Wrap(err, "create bloom filter")
errors.Wrap(err, "failed to parse JSON")
errors.Wrapf(err, "context %s", val)
```

**Validation:** Validate inputs at construction time. Use guard clauses at the top of constructors:
```go
func NewBuilder(config GINConfig, numRGs int, opts ...BuilderOption) (*GINBuilder, error) {
    if numRGs <= 0 {
        return nil, errors.New("numRGs must be greater than 0")
    }
    // ...
}
```

**Silent failures:** Some methods silently ignore invalid inputs instead of returning errors. This is intentional for hot-path operations:
```go
func (rs *RGSet) Set(rgID int) {
    if rgID < 0 || rgID >= rs.NumRGs {
        return  // silently ignored
    }
    rs.bitmap.Add(uint32(rgID))
}
```

**Custom error types:** Used sparingly. `JSONPathError` in `jsonpath.go` is the only custom error type:
```go
type JSONPathError struct {
    Path    string
    Message string
}
func (e *JSONPathError) Error() string {
    return fmt.Sprintf("invalid JSONPath %q: %s", e.Path, e.Message)
}
```

## Import Organization

**Order (enforced by `gci` via `.golangci.yml`):**
1. Standard library imports
2. Third-party imports
3. Internal imports (prefix `github.com/amikos-tech/gin-index`)
4. Blank imports
5. Dot imports

**Example from `builder.go`:**
```go
import (
    "encoding/json"
    "fmt"
    "math"
    "sort"
    "strconv"
    "strings"

    "github.com/pkg/errors"
)
```

**Path aliases:** None used. All imports are fully qualified.

## Code Style

**Formatting:** Standard `gofmt`. No custom formatter configuration.

**Linting:** `golangci-lint` v2 configured in `.golangci.yml`:
- Linters enabled: `dupword`, `gocritic`, `mirror`
- `staticcheck` with all checks except `ST1000` (package comments), `ST1003` (naming), `ST1020` (exported comments)
- `errcheck` excluded for `_test.go` files and `examples/` directory
- Import ordering enforced by `gci` formatter: standard, default, then project prefix

**Run linting:**
```bash
make lint       # Check
make lint-fix   # Auto-fix
```

## Type Switch Pattern

Heavily used for dynamic JSON type handling. This is the core pattern for processing JSON values:

```go
switch v := value.(type) {
case nil:
    // handle null
case bool:
    // handle boolean
case float64:
    // handle number (JSON numbers always float64)
case string:
    // handle string
case []any:
    // handle array
case map[string]any:
    // handle object
}
```

See `builder.go:159` (`walkJSON`), `query.go:73` (`evaluateEQ`), `query.go:343` (`toFloat64`), `transformers.go:206` (`BoolNormalize`).

**Rule:** When handling dynamic `any` values from JSON, always use type switches. JSON numbers are always `float64`. Add `int`/`int64` cases for programmatic callers.

## Must* Pattern

Used for constructors where error is considered a programming bug (e.g., invalid constants):

```go
func MustNewRGSet(numRGs int, opts ...RGSetOption) *RGSet {
    rs, err := NewRGSet(numRGs, opts...)
    if err != nil {
        panic(err)
    }
    return rs
}
```

Present in: `bitmap.go` (`MustNewRGSet`), `bloom.go` (`MustNewBloomFilter`), `hyperloglog.go` (`MustNewHyperLogLog`), `prefix.go` (`MustNewPrefixCompressor`), `jsonpath.go` (`MustValidateJSONPath`).

**Rule:** Use `Must*` variants only in initialization code or tests where invalid args are bugs. Never use in runtime paths handling user input.

## Comment and Documentation Style

**Minimal comments.** Code is expected to be self-documenting through naming. Comments used for:
1. Godoc on exported types/functions (brief, single-line)
2. Algorithm explanations (HyperLogLog, prefix compression)
3. Non-obvious behavior notes

```go
// DocID represents an external document identifier.
type DocID uint64

// FieldTransformer transforms a value before indexing.
// Returns (transformedValue, ok). If ok=false, original value is indexed.
type FieldTransformer func(value any) (any, bool)

// HyperLogLog implements the HyperLogLog algorithm for cardinality estimation.
// It uses 2^precision registers to estimate the number of distinct elements.
type HyperLogLog struct {
```

**Rule:** Keep comments minimal. Prefer descriptive names. Add Godoc to all exported types and functions. Do not add inline comments unless the logic is non-obvious.

## Module Structure

**Flat file organization.** Each file contains a single responsibility:
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

**Rule:** One primary type or concern per file. Keep files under ~500 lines. The largest file is `serialize.go` at 931 lines.

## Constant Groups

Use `iota` for typed enumerations with bit-flag semantics where appropriate:

```go
// Bit flags (use 1 << iota)
const (
    TypeString uint8 = 1 << iota
    TypeInt
    TypeFloat
    TypeBool
    TypeNull
)

// Sequential enums (use plain iota)
const (
    OpEQ Operator = iota
    OpNE
    OpGT
    // ...
)
```

## Default Configuration

Provide `DefaultXxx()` functions returning sensible defaults:

```go
func DefaultConfig() GINConfig {
    return GINConfig{
        CardinalityThreshold: 10000,
        BloomFilterSize:      65536,
        BloomFilterHashes:    5,
        EnableTrigrams:       true,
        TrigramMinLength:     3,
        HLLPrecision:         12,
        PrefixBlockSize:      16,
    }
}
```

## Immutable Results

Query results return new `*RGSet` instances. Set operations (Intersect, Union, Invert) create new sets rather than mutating in place:

```go
func (rs *RGSet) Intersect(other *RGSet) *RGSet {
    result := rs.bitmap.Clone()
    result.And(other.bitmap)
    return &RGSet{bitmap: result, NumRGs: rs.NumRGs}
}
```

**Rule:** All bitmap operations return new instances. Never mutate an RGSet that may be shared.

---

*Convention analysis: 2026-03-24*
