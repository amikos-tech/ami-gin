# External Integrations

**Analysis Date:** 2026-03-24

## APIs & External Services

**AWS S3:**
- Used for remote Parquet file access and sidecar index storage
- SDK: `github.com/aws/aws-sdk-go-v2` v1.41.1
- Service packages: `service/s3`, `config`, `credentials`
- Auth: Static credentials via `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` env vars, or default AWS credential chain
- Integration file: `s3.go`
- Client wrapper: `S3Client` struct with methods for reading/writing Parquet files, sidecar indexes, and listing objects
- Custom `s3ReaderAt` implements `io.ReaderAt` for range-based S3 reads (Parquet random access)
- Timeouts: 10s for HEAD, 30s for range GET, 60s for full GET/PUT/LIST
- Path-style access: Configurable via `AWS_S3_PATH_STYLE=true` (for MinIO/custom endpoints)
- Factory: `NewS3ClientFromEnv()` reads all config from environment
- Configuration struct: `S3Config` in `s3.go`

**No other external APIs or services.**

## Data Storage

**File Formats:**

**Apache Parquet (read/write):**
- Library: `github.com/parquet-go/parquet-go` v0.27.0
- Integration file: `parquet.go`
- Reads: Parquet files to build GIN indexes from JSON columns; reads row groups, column chunks, pages
- Writes: Rebuilds Parquet files with embedded GIN index metadata via `RebuildWithIndex()`
- Metadata: Stores/retrieves GIN index as base64-encoded key-value metadata (default key: `gin.index`)
- Reader abstraction: Supports both `*os.File` and `io.ReaderAt` (for S3 streaming)
- Helper: `ParquetIndexWriter` struct for streaming index construction

**GIN Binary Format (custom, read/write):**
- Integration file: `serialize.go`
- Custom binary wire format with little-endian encoding
- Magic bytes: `GINu` (uncompressed) or `GINc` (compressed) prefix, legacy format also supported
- Version: 3 (current)
- Compression: zstd via `github.com/klauspost/compress/zstd` with configurable levels (0=none, 1-19)
- Default compression: level 15 (`CompressionBest`)
- Sidecar files: `.gin` extension appended to Parquet filename
- File structure (in order): Header, PathDirectory, BloomFilter, StringIndexes, StringLengthIndexes, NumericIndexes, NullIndexes, TrigramIndexes, HyperLogLogs, DocIDMapping (optional), Config (JSON)

**Databases:**
- None

**File Storage:**
- Local filesystem for Parquet and sidecar `.gin` files
- S3-compatible object storage (see above)

**Caching:**
- None

## Core Library Integrations

### Roaring Bitmaps
- Package: `github.com/RoaringBitmap/roaring/v2` v2.14.4
- Integration file: `bitmap.go`
- Purpose: Compressed bitmap for tracking which row groups contain matching values
- Usage: `RGSet` wraps `roaring.Bitmap` with row-group-aware operations (Set/Intersect/Union/Invert/Clone)
- Serialization: `roaring.Bitmap.ToBytes()` / `roaring.Bitmap.UnmarshalBinary()` in `serialize.go`

### xxHash
- Package: `github.com/cespare/xxhash/v2` v2.3.0
- Integration files: `bloom.go`, `hyperloglog.go`
- Purpose: Fast 64-bit hash function for probabilistic data structures
- Bloom filter: Double hashing scheme using `xxhash.Sum64(data)` and `xxhash.Sum64(data + 0xFF)`
- HyperLogLog: Single hash `xxhash.Sum64(data)` split into register index + value

### OJG (Optimized JSON for Go)
- Package: `github.com/ohler55/ojg` v1.28.0
- Integration file: `jsonpath.go`
- Subpackage used: `ojg/jp` (JSONPath parsing)
- Purpose: Parse and validate JSONPath expressions
- Supported fragments: `Root ($)`, `Child (.field)`, `Wildcard ([*])`, `Bracket (['field'])`
- Rejected fragments: `Nth ([0])`, `Slice ([0:5])`, `Filter ([?(...)])`, `Descent (..)`, `Union ([a,b])`

### zstd Compression
- Package: `github.com/klauspost/compress` v1.18.3
- Integration file: `serialize.go`
- Subpackage used: `klauspost/compress/zstd`
- Purpose: Compress/decompress serialized GIN index data
- Encoder: Created per-call with configurable level via `zstd.WithEncoderLevel()`
- Decoder: Created per-call with `zstd.NewReader(nil)` + `DecodeAll()`
- Compression levels defined: `CompressionNone(0)`, `CompressionFastest(1)`, `CompressionBalanced(3)`, `CompressionBetter(9)`, `CompressionBest(15)`, `CompressionMax(19)`

### Standard Library Integrations

**`encoding/binary`** - Used extensively in `serialize.go` and `prefix.go` for little-endian binary I/O of index structures

**`encoding/json`** - Used in `builder.go` for JSON document parsing, `serialize.go` for config serialization, `transformer_registry.go` for transformer parameter serialization

**`encoding/base64`** - Used in `parquet.go` for encoding/decoding GIN index as Parquet metadata value

**`regexp/syntax`** - Used in `regex.go` for regex AST parsing and literal extraction (trigram-based regex candidate selection)

**`regexp`** - Used in `transformers.go` and `transformer_registry.go` for regex-based field transformers

**`net`** - Used in `transformers.go` for IPv4 address parsing (`net.ParseIP`) and CIDR range calculation (`net.ParseCIDR`)

**`net/url`** - Used in `transformers.go` for URL host extraction (`url.Parse`)

**`time`** - Used in `transformers.go` for date parsing (RFC3339, custom formats) and duration parsing

## Authentication & Identity

**Auth Provider:** None (library has no auth layer)
- S3 authentication delegated to AWS SDK default credential chain
- No user authentication, sessions, or identity management

## Monitoring & Observability

**Error Tracking:** None
**Logs:** None (library is silent; CLI uses `fmt.Printf`/`fmt.Fprintf` to stderr)
**Metrics:** None

## CI/CD & Deployment

**Hosting:** Not applicable (Go library, not a service)

**CI Pipeline:**
- GitHub Actions workflows exist but are Claude-related (code review), not Go CI
- No automated test/lint/build pipeline detected
- `Makefile` provides local `test`, `lint`, `build` targets

**Distribution:**
- Go module: `go get github.com/amikos-tech/gin-index`
- CLI binary: `go install github.com/amikos-tech/gin-index/cmd/gin-index@latest`

## Environment Configuration

**Required env vars:**
- None for library-only usage

**Required for S3 operations:**
- `AWS_ACCESS_KEY_ID` - S3 access key (or use default AWS credential chain)
- `AWS_SECRET_ACCESS_KEY` - S3 secret key (or use default AWS credential chain)

**Optional:**
- `AWS_REGION` / `AWS_DEFAULT_REGION` - Defaults to `us-east-1`
- `AWS_ENDPOINT_URL` / `AWS_S3_ENDPOINT` - Custom S3 endpoint
- `AWS_S3_PATH_STYLE` - Set to `"true"` for MinIO/path-style access

**Secrets location:** Environment variables only (read in `s3.go:S3ConfigFromEnv()`)

## Webhooks & Callbacks

**Incoming:** None
**Outgoing:** None

## Import Dependency Graph

**Core data flow imports (no external deps):**
- `gin.go` -> `encoding/json`
- `query.go` -> stdlib only (`fmt`, `sort`, `strconv`)
- `trigram.go` -> stdlib only (`strings`, `unicode`)
- `docid.go` -> no imports
- `prefix.go` -> `encoding/binary`, `io`, `sort`, `pkg/errors`

**External integration imports:**
- `bitmap.go` -> `roaring/v2`, `pkg/errors`
- `bloom.go` -> `xxhash/v2`, `pkg/errors`
- `hyperloglog.go` -> `xxhash/v2`, `pkg/errors`
- `builder.go` -> `encoding/json`, `pkg/errors`
- `serialize.go` -> `roaring/v2`, `klauspost/compress/zstd`, `pkg/errors`
- `jsonpath.go` -> `ohler55/ojg/jp`
- `parquet.go` -> `parquet-go/parquet-go`, `pkg/errors`
- `s3.go` -> `aws-sdk-go-v2/*`, `parquet-go/parquet-go`, `pkg/errors`
- `transformers.go` -> stdlib only (`math`, `net`, `net/url`, `regexp`, `strconv`, `strings`, `time`, `pkg/errors`)
- `transformer_registry.go` -> `encoding/json`, `regexp`, `time`, `pkg/errors`
- `regex.go` -> `regexp/syntax` only

**Test-only imports:**
- `generators_test.go` -> `gopter`, `gopter/gen`
- `integration_property_test.go` -> `gopter`, `gopter/gen`, `gopter/prop`
- `property_test.go` -> `gopter`, `gopter/gen`, `gopter/prop`

## Key Observations

- The library has a clean separation: core index logic uses only stdlib + 4 external packages (roaring, xxhash, zstd, ojg). S3 and Parquet integrations are additive.
- All external dependencies are well-established, maintained Go packages with no known deprecation risks.
- The `pkg/errors` package is used consistently throughout (not `fmt.Errorf` with `%w`), matching the project's CLAUDE.md convention.
- AWS SDK v2 is a heavy transitive dependency tree pulled in for S3 support; it would be the main candidate for optional build tags if dependency size becomes a concern.
- The parquet-go library brings several compression codecs (brotli, lz4) as transitive dependencies even though the GIN index itself only uses zstd.

---

*Integration audit: 2026-03-24*
