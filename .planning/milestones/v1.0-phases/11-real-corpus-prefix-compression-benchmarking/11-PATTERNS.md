# Phase 11: Real-Corpus Prefix Compression Benchmarking - Pattern Map

**Mapped:** 2026-04-20
**Files analyzed:** 6
**Analogs found:** 5 / 6 direct, with 1 partial fixture analog

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `benchmark_test.go` | test | batch, file-I/O | `benchmark_test.go` | exact |
| `testdata/phase11/github_archive_smoke.jsonl` | test | file-I/O | `benchmark_test.go` | partial |
| `testdata/phase11/README.md` | config | transform | `README.md` | role-match |
| `README.md` | config | transform | `README.md` | exact |
| `.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-BENCHMARK-RESULTS.md` | test | transform | `.planning/phases/10-serialization-compaction/10-03-SUMMARY.md` | role-match |
| `.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-REAL-CORPUS-REPORT.md` | config | transform | `.planning/phases/10-serialization-compaction/10-VERIFICATION.md` | role-match |

## Pattern Assignments

### `benchmark_test.go` (test, batch + file-I/O)

**Primary analog:** `benchmark_test.go`

**Imports pattern** (`benchmark_test.go:1-12`):
```go
package gin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"sync"
	"testing"
)
```

Use the existing stdlib-only import ordering and keep the Phase 11 helpers in this file rather than creating a separate benchmark harness package.

**Fixture builder pattern** (`benchmark_test.go:1225-1328`):
```go
func mustBuildBenchmarkIndex(config GINConfig, docs []string) *GINIndex {
	builder, err := NewBuilder(config, len(docs))
	if err != nil {
		panic(err)
	}
	for i, doc := range docs {
		if err := builder.AddDocument(DocID(i), []byte(doc)); err != nil {
			panic(err)
		}
	}
	return builder.Finalize()
}

func buildPhase10MixedFixture() phase10BenchmarkFixture {
	config, err := NewConfig(
		WithToLowerTransformer("$.email", "lower"),
		WithEmailDomainTransformer("$.email", "domain"),
	)
	if err != nil {
		panic(err)
	}

	builder, err := NewBuilder(config, 8)
	if err != nil {
		panic(err)
	}
	rawProbe := ""
	for i := 0; i < 64; i++ {
		email := fmt.Sprintf("customer.%03d@example.com", i)
		if i%5 == 0 {
			email = fmt.Sprintf("platform.%03d@other.dev", i)
		}
		if i == 0 {
			rawProbe = email
		}
		doc := fmt.Sprintf(
			`{"email":"%s","team":"team-%02d","city":"city-%02d","profile_id":"acct-eu-prod-%03d"}`,
			email,
			i%4,
			i%6,
			i,
		)
		if err := builder.AddDocument(DocID(i/8), []byte(doc)); err != nil {
			panic(err)
		}
	}
	idx := builder.Finalize()

	return phase10BenchmarkFixture{
		idx: idx,
		queries: []phase10BenchmarkQuery{
			{name: "RawPath", predicate: EQ("$.email", rawProbe)},
			{name: "Alias", predicate: EQ("$.email", As("domain", "example.com"))},
		},
	}
}
```

Copy the pattern of deterministic fixture builders that also define concrete post-decode probe predicates. Phase 11 should keep that same "fixture + probes together" shape even when the docs come from file loaders instead of inline strings.

**Metric reporting pattern** (`benchmark_test.go:1380-1410`):
```go
func phase10BenchmarkMetricsForFixture(fixture phase10BenchmarkFixture) phase10BenchmarkMetrics {
	compactRaw, err := EncodeWithLevel(fixture.idx, CompressionNone)
	if err != nil {
		panic(err)
	}
	defaultZstd, err := Encode(fixture.idx)
	if err != nil {
		panic(err)
	}

	legacyPayloadBytes := phase10LegacyStringPayloadBytes(fixture.idx)
	compactPayloadBytes := phase10CompactStringPayloadBytes(fixture.idx)
	legacyRawBytes := len(compactRaw) - compactPayloadBytes + legacyPayloadBytes
	bytesSavedPct := 0.0
	if legacyRawBytes > 0 {
		bytesSavedPct = (float64(legacyRawBytes-len(compactRaw)) / float64(legacyRawBytes)) * 100
	}

	return phase10BenchmarkMetrics{
		legacyRawBytes:   legacyRawBytes,
		compactRawBytes:  len(compactRaw),
		defaultZstdBytes: len(defaultZstd),
		bytesSavedPct:    bytesSavedPct,
	}
}

func phase10ReportMetrics(b *testing.B, metrics phase10BenchmarkMetrics) {
	b.ReportMetric(float64(metrics.legacyRawBytes), "legacy_raw_bytes")
	b.ReportMetric(float64(metrics.compactRawBytes), "compact_raw_bytes")
	b.ReportMetric(float64(metrics.defaultZstdBytes), "default_zstd_bytes")
	b.ReportMetric(metrics.bytesSavedPct, "bytes_saved_pct")
}
```

Copy these metric names exactly. Phase 11 should extend this accounting model, not rename it.

**Benchmark family / leaf pattern** (`benchmark_test.go:1413-1485`):
```go
func BenchmarkPhase10SerializationCompaction(b *testing.B) {
	fixtures := []struct {
		name  string
		build func() phase10BenchmarkFixture
	}{
		{name: "Mixed", build: buildPhase10MixedFixture},
		{name: "HighPrefix", build: buildPhase10HighPrefixFixture},
		{name: "RandomLike", build: buildPhase10RandomLikeFixture},
	}

	for _, fixtureDef := range fixtures {
		fixtureDef := fixtureDef
		b.Run(fixtureDef.name, func(b *testing.B) {
			fixture := fixtureDef.build()
			metrics := phase10BenchmarkMetricsForFixture(fixture)
			compactRaw, err := EncodeWithLevel(fixture.idx, CompressionNone)
			if err != nil {
				b.Fatalf("EncodeWithLevel() error = %v", err)
			}
			defaultZstd, err := Encode(fixture.idx)
			if err != nil {
				b.Fatalf("Encode() error = %v", err)
			}

			b.Run("Size", func(b *testing.B) {
				phase10ReportMetrics(b, metrics)
				for i := 0; i < b.N; i++ {
				}
			})

			b.Run("Encode", func(b *testing.B) {
				phase10ReportMetrics(b, metrics)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err := Encode(fixture.idx); err != nil {
						b.Fatalf("Encode() error = %v", err)
					}
				}
			})

			b.Run("Decode", func(b *testing.B) {
				phase10ReportMetrics(b, metrics)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err := Decode(defaultZstd); err != nil {
						b.Fatalf("Decode() error = %v", err)
					}
				}
			})

			b.Run("QueryAfterDecode", func(b *testing.B) {
				decoded, err := Decode(compactRaw)
				if err != nil {
					b.Fatalf("Decode(compactRaw) error = %v", err)
				}
				for _, query := range fixture.queries {
					query := query
					b.Run(query.name, func(b *testing.B) {
						phase10ReportMetrics(b, metrics)
						if got := decoded.Evaluate([]Predicate{query.predicate}).Count(); got == 0 {
							b.Fatalf("query %s returned 0 matches", query.name)
						}
						b.ResetTimer()
						for i := 0; i < b.N; i++ {
							_ = decoded.Evaluate([]Predicate{query.predicate})
						}
					})
				}
			})
		})
	}
}
```

Copy the same benchmark leaf layout: `Size`, `Encode`, `Decode`, `QueryAfterDecode`. Phase 11 should layer `tier=` and `projection=` labels around this structure rather than inventing a new benchmark shape.

**Multi-axis naming pattern** (`benchmark_test.go:2048-2062`):
```go
for _, preparedMode := range preparedModes {
	preparedMode := preparedMode
	for _, probe := range probes {
		probe := probe
		name := fmt.Sprintf("%s/shape=%s/%s", preparedMode.mode.name, phase08AdaptiveBenchmarkShape, probe.name)
		b.Run(name, func(b *testing.B) {
			pred := probe.pred(fixture)
			candidate := preparedMode.idx.Evaluate([]Predicate{pred}).Count()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				preparedMode.idx.Evaluate([]Predicate{pred})
			}
			b.ReportMetric(float64(candidate), "candidate_rgs")
			b.ReportMetric(float64(preparedMode.encodedBytes), "encoded_bytes")
		})
	}
}
```

Use slash-delimited dimension labels such as `tier=smoke/projection=structured/leaf=Size` or equivalent. This repo already uses multi-axis benchmark names in that form.

**Secondary analogs for env/path helpers:** `s3.go:28-45`, `cmd/gin-index/main.go:643-714`, `cmd/gin-index/main_test.go:47-74`

Use those for:
- env-var lookup and defaulting
- local path validation with `os.Stat`
- shard discovery with `filepath.Glob`
- fail-fast benchmark errors with `b.Fatalf`

There is no existing `b.Skip` precedent in the repo, so Phase 11 should adapt the env-lookup shape to benchmark semantics itself.

---

### `testdata/phase11/github_archive_smoke.jsonl` (test fixture, file-I/O)

**Closest analog:** `benchmark_test.go` (partial only)

**Deterministic fixture-content pattern** (`benchmark_test.go:1251-1269`, `benchmark_test.go:1290-1299`, `benchmark_test.go:1311-1319`):
```go
for i := 0; i < 64; i++ {
	email := fmt.Sprintf("customer.%03d@example.com", i)
	if i%5 == 0 {
		email = fmt.Sprintf("platform.%03d@other.dev", i)
	}
	doc := fmt.Sprintf(
		`{"email":"%s","team":"team-%02d","city":"city-%02d","profile_id":"acct-eu-prod-%03d"}`,
		email,
		i%4,
		i%6,
		i,
	)
	...
}

for i := 0; i < 16; i++ {
	userID := fmt.Sprintf("tenant-eu-prod-user-tail-%03d", i)
	...
	docs = append(docs, fmt.Sprintf(`{"user_id":"%s","cluster":"tenant-eu-prod-cluster-%02d","service":"tenant-eu-prod-service-%02d"}`, userID, i%4, i%3))
}

for i := 0; i < 32; i++ {
	token := fmt.Sprintf("%08x%08x", rng.Uint32(), rng.Uint32())
	docs = append(docs, fmt.Sprintf(`{"token":"%s","bucket":"r%02d"}`, token, i%8))
}
```

Copy the determinism discipline, not the inline-string storage. The checked-in smoke corpus should use concrete, repeated field families and stable ordering just like these builders.

**Direct-gap note:**

The repo currently has no `testdata/` directory and no checked-in corpus fixture precedent. Planner should treat this as a new file-layout pattern, while borrowing the deterministic fixture style from `benchmark_test.go`.

---

### `testdata/phase11/README.md` (fixture metadata/provenance doc, transform)

**Analog:** `README.md`

**Short setup-note pattern** (`README.md:444-452`):
````markdown
### S3 Support

All operations support S3 paths via AWS SDK v2:

```go
// Configure from environment variables:
// AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION
// AWS_ENDPOINT_URL (for MinIO, LocalStack), AWS_S3_PATH_STYLE=true
```
````

**Command block pattern** (`README.md:584-588`):
````markdown
Run benchmarks with:

```bash
go test -bench=. -benchmem -benchtime=1s
```
````

Copy the README style of:
- short heading
- one-sentence purpose statement
- explicit env/path callouts
- one concrete command block

For this file, keep it narrow: upstream source name, expected external snapshot layout, and the fact that the checked-in file is smoke-only.

---

### `README.md` (user-facing benchmark docs, transform)

**Analog:** `README.md`

**Benchmark section pattern** (`README.md:584-671`):
````markdown
## Benchmarks

Run benchmarks with:

```bash
go test -bench=. -benchmem -benchtime=1s
```

### Performance Summary (Apple M3 Max)

| Operation | Latency | Notes |
|-----------|---------|-------|
...

### Index Size

| Row Groups | Encoded Size | Per RG |
|------------|--------------|--------|
...

### Benchmark Categories

The benchmark suite (`benchmark_test.go`) covers:
````

Use this same layout for Phase 11 docs:
- keep commands in fenced `bash` blocks
- use short explanatory paragraphs
- use tables for metrics/results
- keep benchmark scope tied to `benchmark_test.go`

**Env-var documentation pattern** (`README.md:444-452`):
```markdown
// Configure from environment variables:
// AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_REGION
// AWS_ENDPOINT_URL (for MinIO, LocalStack), AWS_S3_PATH_STYLE=true
```

For Phase 11, document the opt-in env vars in this terse list style rather than with long prose.

---

### `.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-BENCHMARK-RESULTS.md` (benchmark evidence artifact, transform)

**Analog:** `.planning/phases/10-serialization-compaction/10-03-SUMMARY.md`

**Results write-up structure** (`10-03-SUMMARY.md:44-82`):
```markdown
## Accomplishments

- Added `buildPhase10MixedFixture`, `buildPhase10HighPrefixFixture`, and `buildPhase10RandomLikeFixture` plus a single `BenchmarkPhase10SerializationCompaction` family with fixture-local `Size`, `Encode`, `Decode`, and `QueryAfterDecode` leaves.
- Reported exact `legacy_raw_bytes`, `compact_raw_bytes`, `default_zstd_bytes`, and `bytes_saved_pct` metrics by reconstructing only the old raw-string layout for the sections changed in Phase 10.
- Included concrete post-decode probes for raw-path equality on every fixture, a representation-aware alias probe on the mixed fixture, and an adaptive/high-cardinality probe on the high-prefix fixture.

## Verification Evidence

- Benchmark harness smoke passed:
  `go test ./... -run '^$' -bench 'BenchmarkPhase10SerializationCompaction' -benchtime=1x -count=1`
- Benchmark timing evidence passed:
  `go test ./... -run '^$' -bench 'BenchmarkPhase10SerializationCompaction' -benchtime=1s -count=3 -benchmem`

Representative output from the timing run:

- **Mixed**
  `legacy_raw_bytes=65294`, `compact_raw_bytes=63500`, `default_zstd_bytes=3848`, `bytes_saved_pct=2.748`
...
```

Copy this artifact pattern directly:
- state what the benchmark added
- record the exact commands
- include representative output grouped by scenario
- keep raw metrics and timing evidence in the same file

Phase 11 should change the grouping keys from `Mixed/HighPrefix/RandomLike` to the real-corpus tiers and projections, but keep the command/result cadence.

---

### `.planning/phases/11-real-corpus-prefix-compression-benchmarking/11-REAL-CORPUS-REPORT.md` (narrative recommendation report, transform)

**Primary analog:** `.planning/phases/10-serialization-compaction/10-VERIFICATION.md`
**Secondary analog:** `.planning/phases/10-serialization-compaction/10-03-SUMMARY.md`

**Evidence-backed report structure** (`10-VERIFICATION.md:16-53`):
```markdown
## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | ... | ✓ VERIFIED | ... |
...

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Benchmark harness smoke | `go test ./... -run '^$' -bench 'BenchmarkPhase10SerializationCompaction' -benchtime=1x -count=1` | Passed and reported stable size metrics ... | ✓ PASS |
| Benchmark timing evidence | `go test ./... -run '^$' -bench 'BenchmarkPhase10SerializationCompaction' -benchtime=1s -count=3 -benchmem` | Passed on HEAD. Fresh size metrics remained ... | ✓ PASS |
```

Use this as the backbone for a report that ties claims to commands and measured evidence.

**Interpretation / no-win reporting pattern** (`10-03-SUMMARY.md:61-63`):
```markdown
- Used `EncodeWithLevel(..., CompressionNone)` for exact raw-wire accounting and `Encode(...)` for default compressed reporting so both raw and zstd views remain visible.
- Kept the benchmark repo-local and single-package by deriving legacy raw-wire bytes from current in-memory structures rather than maintaining a parallel legacy serializer implementation.
- Let the benchmark output show that high-prefix data wins slightly on raw bytes while mixed/random-like fixtures stay effectively flat-to-negative, matching the phase research rather than papering over it.
```

Phase 11 should preserve this reporting behavior explicitly. The report needs narrative sections such as `Helps`, `Flat / No-Win`, and `Recommendation`, but every claim should still be grounded in the evidence-table style above.

## Shared Patterns

### Environment gating and defaults

**Sources:** `s3.go:28-45`, `cmd/gin-index/main_test.go:47-50`

```go
func S3ConfigFromEnv() S3Config {
	cfg := S3Config{
		Endpoint:  os.Getenv("AWS_ENDPOINT_URL"),
		Region:    os.Getenv("AWS_REGION"),
		AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		PathStyle: os.Getenv("AWS_S3_PATH_STYLE") == "true",
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = os.Getenv("AWS_S3_ENDPOINT")
	}
	if cfg.Region == "" {
		cfg.Region = os.Getenv("AWS_DEFAULT_REGION")
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	return cfg
}

helper := os.Getenv("GIN_INDEX_PARSE_HELPER")
if helper == "" {
	return
}
```

Apply to all Phase 11 opt-in surfaces. Use env lookup with explicit fallback/default behavior. Adapt the final control flow to benchmarks with `b.Skip` when absent and `b.Fatalf` when explicitly enabled but misconfigured.

### Local path validation and globbing

**Sources:** `parquet.go:36-47`, `cmd/gin-index/main.go:643-714`

```go
func parquetFileMode(parquetFile string) (os.FileMode, error) {
	info, err := os.Stat(parquetFile)
	if err != nil {
		return 0, errors.Wrap(err, "stat parquet file")
	}
	return artifactFileMode(info.Mode()), nil
}

func localFileMode(path string) (os.FileMode, error) {
	cleanedPath := filepath.Clean(path)
	info, err := os.Stat(cleanedPath)
	if err != nil {
		return 0, errors.Wrap(err, "stat local file")
	}
	return artifactFileMode(info.Mode()), nil
}

matches, err := filepath.Glob(path)
if err != nil {
	return nil, err
}
```

Apply to the external corpus root helper. Clean the path, `os.Stat` the expected root, and use `filepath.Glob` for shard discovery instead of hand-rolled directory walking.

### Benchmark metric names and reporting

**Source:** `benchmark_test.go:1406-1410`

```go
b.ReportMetric(float64(metrics.legacyRawBytes), "legacy_raw_bytes")
b.ReportMetric(float64(metrics.compactRawBytes), "compact_raw_bytes")
b.ReportMetric(float64(metrics.defaultZstdBytes), "default_zstd_bytes")
b.ReportMetric(metrics.bytesSavedPct, "bytes_saved_pct")
```

Apply to all Phase 11 benchmark leaves, including smoke and opt-in tiers.

### Multi-dimensional benchmark names

**Source:** `benchmark_test.go:2052-2062`

```go
name := fmt.Sprintf("%s/shape=%s/%s", preparedMode.mode.name, phase08AdaptiveBenchmarkShape, probe.name)
b.Run(name, func(b *testing.B) {
	...
})
```

Apply to Phase 11 tier and projection labels. Prefer stable slash-delimited names over ad hoc string concatenation.

### Evidence-backed phase reporting

**Sources:** `10-03-SUMMARY.md:65-82`, `10-VERIFICATION.md:20-53`

```markdown
- Benchmark harness smoke passed:
  `go test ./... -run '^$' -bench 'BenchmarkPhase10SerializationCompaction' -benchtime=1x -count=1`
- Benchmark timing evidence passed:
  `go test ./... -run '^$' -bench 'BenchmarkPhase10SerializationCompaction' -benchtime=1s -count=3 -benchmem`

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 8 | Phase 10 benchmark evidence covers mixed, high-prefix, and random-like fixtures with raw-wire, zstd, encode/decode, and post-decode query reporting. | ✓ VERIFIED | `benchmark_test.go:1238-1485` ... |
```

Apply to both `11-BENCHMARK-RESULTS.md` and `11-REAL-CORPUS-REPORT.md`. Keep raw commands, representative output, and explicit evidence references in the checked-in artifacts.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| None with zero guidance | - | - | The only gap is that the repo has no existing `testdata/` benchmark corpus asset; use the partial fixture-style analog from `benchmark_test.go` plus the doc style from `README.md`. |

## Metadata

**Analog search scope:** `benchmark_test.go`, `s3.go`, `parquet.go`, `cmd/gin-index/main.go`, `cmd/gin-index/main_test.go`, `README.md`, `.planning/phases/10-serialization-compaction/10-03-SUMMARY.md`, `.planning/phases/10-serialization-compaction/10-VERIFICATION.md`
**Files scanned:** 8
**Pattern extraction date:** 2026-04-20
