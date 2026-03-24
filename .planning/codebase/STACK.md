# Technology Stack

**Analysis Date:** 2026-03-24

## Languages

**Primary:**
- Go 1.25.5 - Entire codebase (library + CLI tool)

**Secondary:**
- None

## Runtime

**Environment:**
- Go toolchain 1.25.5 (specified in `go.mod`)
- No `.go-version` or `.tool-versions` file detected

**Package Manager:**
- Go modules (`go.mod` + `go.sum`)
- Lockfile: `go.sum` present (67KB)
- Module path: `github.com/amikos-tech/gin-index`

## Frameworks

**Core:**
- No web or application framework. This is a standalone Go library with a CLI tool.

**Testing:**
- `testing` (stdlib) - Unit and benchmark tests
- `github.com/leanovate/gopter` v0.2.11 - Property-based testing (generators, properties, shrinking)
- `gotest.tools/gotestsum` (installed at test time via Makefile) - Test runner with JUnit output

**Build/Dev:**
- `go build` - Standard Go build
- `golangci-lint` - Linting (config: `.golangci.yml`)
- `make` - Build automation (`Makefile`)

**Linting Configuration (`.golangci.yml`):**
- Linters enabled: `dupword`, `gocritic`, `mirror`
- Formatter: `gci` (import ordering)
- Import order: standard -> third-party -> `github.com/amikos-tech/gin-index` -> blank -> dot
- `errcheck` suppressed for `_test.go` and `examples/`
- Timeout: 5m, concurrency: 4

## Key Dependencies

**Critical (Direct):**
- `github.com/cespare/xxhash/v2` v2.3.0 - Fast non-cryptographic hash function; used by BloomFilter (`bloom.go`) and HyperLogLog (`hyperloglog.go`) for hashing values
- `github.com/klauspost/compress` v1.18.3 - zstd compression/decompression for index serialization (`serialize.go`); supports configurable compression levels 0-19
- `github.com/RoaringBitmap/roaring/v2` v2.14.4 - Compressed bitmap data structure; underlies `RGSet` (`bitmap.go`) for row-group tracking with set operations (And/Or/AndNot)
- `github.com/ohler55/ojg` v1.28.0 - JSONPath parsing library; used in `jsonpath.go` for path validation via `ojg/jp` subpackage

**Infrastructure (Direct):**
- `github.com/parquet-go/parquet-go` v0.27.0 - Apache Parquet file reading/writing; used in `parquet.go` for building indexes from Parquet files and embedding indexes in Parquet metadata
- `github.com/aws/aws-sdk-go-v2` v1.41.1 + service packages - AWS S3 integration for remote Parquet file access (`s3.go`); includes `config`, `credentials`, `service/s3` subpackages
- `github.com/pkg/errors` v0.9.1 - Error wrapping with stack traces; used throughout for `errors.New()`, `errors.Wrap()`, `errors.Errorf()`

**Test-Only (Indirect):**
- `github.com/leanovate/gopter` v0.2.11 - Property-based testing framework; used in `generators_test.go`, `property_test.go`, `integration_property_test.go`

**Transitive (Indirect):**
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

**Environment Variables (for S3 integration only):**
- `AWS_ENDPOINT_URL` or `AWS_S3_ENDPOINT` - Custom S3 endpoint (e.g., MinIO)
- `AWS_REGION` or `AWS_DEFAULT_REGION` - AWS region (defaults to `us-east-1`)
- `AWS_ACCESS_KEY_ID` - S3 access key
- `AWS_SECRET_ACCESS_KEY` - S3 secret key
- `AWS_S3_PATH_STYLE` - Set to `"true"` for path-style S3 access

**Library Configuration:**
- `GINConfig` struct in `gin.go` with functional options pattern (`ConfigOption`)
- Defaults: bloom 65536 bits / 5 hashes, trigrams enabled, HLL precision 12, prefix block 16, cardinality threshold 10000

**Build:**
- `go.mod` - Module definition and dependencies
- `.golangci.yml` - Linter configuration
- `Makefile` - Build targets: `build`, `test`, `lint`, `lint-fix`, `clean`, `help`

## Build Commands

```bash
# Build library
go build ./...

# Build CLI tool
go build ./cmd/gin-index/

# Run all tests with coverage (via gotestsum)
make test

# Run tests directly
go test -v ./...

# Run specific test
go test -v -run TestQueryEQ

# Lint
make lint

# Lint with auto-fix
make lint-fix

# Clean artifacts
make clean
```

## Platform Requirements

**Development:**
- Go 1.25.5+
- `golangci-lint` for linting
- `gotestsum` (auto-installed by `make test`)
- No CGo dependencies - pure Go

**Production:**
- Standalone Go binary (no runtime dependencies)
- AWS credentials needed only for S3 operations
- File system access for local Parquet/sidecar operations

## CLI Tool

**Location:** `cmd/gin-index/main.go`
**Binary:** `gin-index`

**Commands:**
- `build` - Build GIN index from Parquet file(s) (local or S3)
- `query` - Evaluate predicates against an index
- `info` - Display index metadata and path directory
- `extract` - Extract embedded index to sidecar file

**Supports:** Single file, directory batch, and S3 prefix batch operations.

## CI/CD

**GitHub Actions:**
- `.github/workflows/claude-code-review.yml` - Claude-based code review
- `.github/workflows/claude.yml` - Claude workflow

No dedicated Go CI pipeline (test/lint/build) detected in GitHub Actions.

---

*Stack analysis: 2026-03-24*
