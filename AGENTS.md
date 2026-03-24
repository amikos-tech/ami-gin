# AGENTS.md

This file provides guidance to Codex (Codex.ai/code) when working with code in this repository.

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
- `builder.go:147` - Transformer application in `walkJSON` before type switch

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
