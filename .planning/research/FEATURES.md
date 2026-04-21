# Feature Landscape: v1.1 Performance, Observability & Experimentation

**Domain:** Go library (inverted index for JSON row-group pruning) — three additive feature themes
**Researched:** 2026-04-21
**Milestone scope:** SIMD JSON parser integration, observability/logging primitives, experimentation CLI
**Overall confidence:** MEDIUM-HIGH (go-wand evidence is concrete and current; simdjson ecosystem is well-documented; CLI patterns well-established)

---

## Theme 1 — SIMD JSON Parsing Integration

### User Stories

- **US-SIMD-01** — "As a builder-heavy user ingesting GBs of JSONL, I want to opt in to a SIMD parser so the build step stops being the bottleneck, without rewriting my code."
- **US-SIMD-02** — "As an existing v1.0 user, I want my current `AddDocument([]byte)` calls to keep working unchanged and with identical numeric semantics, regardless of which parser is configured."
- **US-SIMD-03** — "As a constrained-environment user (no AVX2, no CGo tolerance, ARM in some contexts), I want the default parser to stay pure-Go `encoding/json` so I'm never accidentally broken."

### Table Stakes (must-have)

| # | Feature | Complexity | Depends on | User story |
|---|---------|-----------|------------|------------|
| TS-SIMD-1 | Parser interface + default `encoding/json.UseNumber` adapter | S (1–2 days) | `builder.go:322 parseAndStageDocument`, `walkJSON` | US-SIMD-02 |
| TS-SIMD-2 | Opt-in config option (`WithJSONParser(parser)`) via existing `ConfigOption` pattern | S | `gin.go` functional options | US-SIMD-01, US-SIMD-03 |
| TS-SIMD-3 | `pure-simdjson` adapter that preserves exact-int semantics (no float coercion of integers that fit int64) | M (3–5 days) | `pure-simdjson` `doc.Root().GetInt64()` / `GetFloat64()` path-type dispatch | US-SIMD-02 |
| TS-SIMD-4 | Parser-choice-invariant behavior: identical path set, identical stored types, identical null/present bitmaps for any valid input | M | Decode-parity test harness (v1.0 Phase 07 already establishes pattern) | US-SIMD-02 |
| TS-SIMD-5 | Error reporting: a SIMD parse failure surfaces the same error type as `encoding/json` failure (caller should not need to type-switch by parser) | S | Error-wrapping via `pkg/errors` | US-SIMD-02 |
| TS-SIMD-6 | Benchmarks comparing `encoding/json.UseNumber` baseline vs pure-simdjson on a realistic corpus (SEED-001 datasets) | M | `benchmark_test.go`, SEED-001 simdjson example datasets | US-SIMD-01 |

**Hard invariants** (carried over from v1.0 Phase 07):

- Integers that round-trip as exact `int64` stay `int64` end-to-end; they must not become `float64`.
- `json.Number` semantics are the contract — the SIMD adapter must map its view types onto `int64` / `float64` / `string` deterministically.
- Mixed-mode promotion (int → float when a path later sees a real float) must behave identically across parsers.

### Differentiators (high-value)

| # | Feature | Complexity | Depends on | User story |
|---|---------|-----------|------------|------------|
| D-SIMD-1 | Streaming/batched SIMD path for JSONL ingest (parse N docs in one buffer pass) — explicit `AddDocuments(rgID, []byte)` or `AddDocumentsJSONL` | M–L (1 week) | `pure-simdjson` doc pooling, builder's per-RG staging | US-SIMD-01 |
| D-SIMD-2 | Runtime capability detection with graceful fallback: "parser X requested, CPU lacks AVX2, fell back to encoding/json" surfaced as a single `logging.Warn` event | S (sits on Theme 2) | Theme 2 logger | US-SIMD-03 |
| D-SIMD-3 | Per-document parser failure isolation: one malformed doc in a JSONL batch does not poison the rest of the batch | M | Builder transaction semantics (v1.0 Phase 07 already transactional) | US-SIMD-01 |
| D-SIMD-4 | A second-parser shim (`simdjson-go` from minio/dgraph) behind the same interface — proves the interface is not accidentally `pure-simdjson`-shaped | S once TS-SIMD-1 is in place | Parser interface | architectural hygiene |

### Anti-Features (explicit NO)

| Anti-Feature | Why avoid | What to do instead |
|--------------|-----------|--------------------|
| Make SIMD the default | Introduces a runtime/CGo/capability precondition for every new user; breaks `go get` + `go build` on minimal environments | Keep `encoding/json.UseNumber` default; document opt-in |
| Expose raw simdjson `Element`/`Iterator` in public API | Leaks the third-party type — flipping parsers later becomes a breaking change | Internal-only adapter, public API stays `[]byte` + the abstract parser interface |
| Hot-loop telemetry spans per document | At AddDocument rates (~43µs each per README), one span per doc drowns exporters | Coarse phase-level spans only (see Theme 2) |
| Custom-AST exposure through the parser interface | Coupling queries to an AST freezes the abstraction | Parser returns the same shape builder already walks (paths + values + types); don't leak parser internals |
| Parallel builder shards behind the parser seam | Builder currently single-threaded; concurrency is a separate, larger change | Keep parallelism out of scope for v1.1 |

### Parser Choice Matrix

| Parser | Pure Go? | CGo | Hardware req | Streaming JSONL? | Exact-int semantics | License | Notes |
|--------|----------|-----|--------------|------------------|---------------------|---------|-------|
| `encoding/json` + `UseNumber()` | Yes | No | none | Decoder (one doc at a time) | Yes (via `json.Number`) | BSD-3 | v1.0 baseline, default |
| `amikos-tech/pure-simdjson` | Yes (purego FFI wrapper around native simdjson) | No (uses purego) | AVX2 ideally, graceful fallback | Doc-at-a-time via `parser.Parse([]byte)` | Yes — `GetInt64` / `GetFloat64` distinguish | (check repo) | First-class in-house option; actively developed (last push 2026-04-20) |
| `minio/simdjson-go` | Yes (pure Go SIMD) | No | AVX2 + CLMUL (Haswell+/Ryzen+) | Yes — tape format | Needs explicit type checks per node | Apache-2.0 | 2,021 stars, last push 2025-08-26; ~10x `encoding/json` per minio's own benchmarks |
| `bytedance/sonic` | Mixed (JIT) | No but amd64-focused | amd64 | Standard JSON | Depends on mode | Apache-2.0 | Mentioned frequently in Go JSON benchmarks; amd64-only JIT |

**Recommendation for v1.1**: wire the interface + `encoding/json` default + `pure-simdjson` adapter as the reference SIMD implementation. Skip `simdjson-go` and `sonic` adapters in-milestone; prove they're possible by keeping the interface honest.

---

## Theme 2 — Observability & Logging Primitives

**Primary evidence source**: `github.com/amikos-tech/go-wand` Phases 07 (PR #114, merged 2026-04-16) and 08 (PR #115, merged 2026-04-17). These are fresh, well-reviewed in-house patterns we should adopt near-verbatim where they fit.

### User Stories

- **US-OBS-01** — "As a caller embedding the library, I want to plug in my own `slog` or `zap` logger and see structured events from builder/query/serialize boundaries without the library depending on my logger choice."
- **US-OBS-02** — "As a library consumer who does nothing, I want zero cost and zero noise — no log lines, no allocations, no OTel SDK pulled in."
- **US-OBS-03** — "As an operator debugging a slow query on real data, I want duration/failure/work-size metrics on build and query boundaries, and coarse traces that parent into my app's spans."
- **US-OBS-04** — "As a library author I want to name and freeze the vocabulary once so downstream dashboards keep working across versions."

### Table Stakes (must-have) — directly mirrors go-wand

| # | Feature | Complexity | Depends on | Evidence in go-wand |
|---|---------|-----------|------------|---------------------|
| TS-OBS-1 | Backend-neutral `Logger` interface with `Enabled(Level) bool` + `Log(Level, msg, kv...any)` | S | New `pkg/logging` (or `gin/logging`) package | `pkg/logging/logger.go` — 4-level interface: Debug/Info/Warn/Error |
| TS-OBS-2 | Noop default logger — if caller does nothing, library is silent | S | Above | `pkg/logging/noop.go`; "boundary options default to noop behavior" (docs/logging.md) |
| TS-OBS-3 | `Enabled(LevelDebug)` gate so expensive debug payload assembly is skipped | S | Above | `pkg/logging/logger.go:25-31` Debug helper |
| TS-OBS-4 | Structured KV normalization (odd trailing value saved under `missing` key, stringified keys) | S | Internal helper | `pkg/logging/internal/kv/kv.go` |
| TS-OBS-5 | Official `slog` adapter in its own sub-package so core doesn't pull transitive deps | S | `log/slog` stdlib | `pkg/logging/slogadapter/slog.go` |
| TS-OBS-6 | Official `zap` adapter in its own sub-package (opt-in import pulls `go.uber.org/zap`) | S | `go.uber.org/zap` | `pkg/logging/zapadapter/zap.go` |
| TS-OBS-7 | Boundary-level `WithXxxLogger` option carriers (not a global logger) | S | `builder.go`, `query.go`, `serialize.go`, `parquet.go`, S3 client, CLI | go-wand has `index.WithReadLogger`, `parquet.WithOpenLogger`, etc. |
| TS-OBS-8 | Canonical structured-key vocabulary documented once (`docs/logging.md`) — keys like `input`, `output`, `cache_path`, `source_path` so downstream consumers have stable log fields | S | docs/ | go-wand's `docs/logging.md` publishes the full vocabulary table |
| TS-OBS-9 | Safe-metadata policy — never log query text, raw user content, or credential-bearing URLs; summarize secrets before emission | S | Review pass at all boundary emit sites | go-wand docs/logging.md "Safe metadata" section |
| TS-OBS-10 | Logging contract stays context-free (Logger takes `msg + kv`, not `ctx`) to keep the library decoupled from context propagation policy | S | Design decision | go-wand: "The repo-owned Logger is intentionally fire-and-forget" (doc.go) |

### Differentiators (high-value) — telemetry seam, mirrors go-wand Phase 08

| # | Feature | Complexity | Depends on | Evidence |
|---|---------|-----------|------------|----------|
| D-OBS-1 | Local `Signals` container carrying `TracerProvider` + `MeterProvider` + optional `shutdown(ctx) error` — never mutates global `otel.SetTracerProvider` | S | `go.opentelemetry.io/otel/trace`, `go.opentelemetry.io/otel/metric` | `pkg/telemetry/telemetry.go` lines 1-80 |
| D-OBS-2 | `telemetry.Disabled()` zero-value path — zero-cost when caller wires nothing | S | Above | go-wand: "Disabled() returns the zero-value, fully disabled signal container" |
| D-OBS-3 | `FromEnv(ctx, serviceName) (Signals, error)` OTLP/HTTP bootstrap for `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES` — CLI-only convenience, not library-injected | M | `go.opentelemetry.io/otel/exporters/otlp/otlptracehttp` + `otlpmetrichttp` | `pkg/telemetry/env.go`; 5s init, 10s export timeout |
| D-OBS-4 | Frozen operation vocabulary: `index.build`, `index.finalize`, `index.encode`, `index.decode`, `query.evaluate`, `parquet.build`, `parquet.open`, `s3.load`, `cli.experiment`, etc. — low-cardinality, no per-doc spans | S | `pkg/telemetry/attrs.go` pattern | go-wand `pkg/telemetry/attrs.go` defines e.g. `OperationIndexRead`, `OperationParquetEmbed` |
| D-OBS-5 | Frozen metric vocabulary: `gin.operation.duration` (ms), `gin.operation.failures` (count), `gin.operation.documents`, `gin.operation.rows`, `gin.operation.bytes` | S | Above | go-wand: `MetricOperationDuration`, `MetricOperationFailures`, etc. |
| D-OBS-6 | Context-aware public API variants (`EvaluateContext(ctx, predicates)`, `BuildFromParquetContext`) so caller spans parent correctly — existing non-ctx APIs remain wrappers over `context.Background()` | M | `query.go`, `parquet.go` | go-wand added `EmbedContext`, `OpenContext`, `SearchContext` as additive variants |
| D-OBS-7 | Error-type attribute normalization (`error.type` with a fixed allowlist: `config`, `io`, `invalid_format`, `deserialization`, `integrity`, `not_found`, `other`) | S | Per-boundary error classifier | `pkg/telemetry/attrs.go:normalizeErrorType` |
| D-OBS-8 | Attribute allowlist policy: no file paths, no query text, no column/path names, no doc IDs, no row-group IDs, no term IDs in span/metric attributes | S | Review pass | go-wand docs/telemetry.md "Command Coverage" closing paragraph |

### Anti-Features (explicit NO) — most of these come straight from go-wand's stated non-goals

| Anti-Feature | Why avoid | What to do instead |
|--------------|-----------|--------------------|
| Force OTel SDK import on every user | Pulls ~50+ transitive deps onto users who never enable telemetry | Keep OTel imports guarded behind `gin/telemetry/…` sub-packages; core stays `otel/trace` + `otel/metric` noop-provider only |
| Instrument the query hot loop | Query latency is 1µs; a span adds micros-to-tens-of-micros | Boundary-only spans: one span per `Evaluate()` call, not per predicate / per RG |
| Call `otel.SetTracerProvider` globally | Hijacks process state, conflicts with caller's provider | Local Signals container only; document this as an explicit non-goal (go-wand docs/telemetry.md says so literally) |
| Merge logging and telemetry | Forces all callers to buy both; couples two lifecycles | Two independent packages, two independent wirings; go-wand keeps them distinct even though Phase 08 follows Phase 07 |
| Inject trace IDs into logs | Requires context propagation in Logger, which breaks the fire-and-forget contract | Leave trace-log correlation to the application boundary (go-wand docs/telemetry.md "Correlation" section) |
| Emit `debug` logs by default | Noise by default is a bug | Noop default + explicit opt-in adapter required to see anything |
| Use `fmt.Errorf` for instrumentation-relevant errors | Loses stack traces | Continue project convention: `errors.Wrap` / `errors.Errorf` (per CLAUDE.md) |

### Dependencies On Existing Systems (concrete wiring targets)

- `builder.go:299` `AddDocument` → one `index.build.add_document` span is still too hot; instead wrap `Finalize()` → `index.build.finalize` span + duration + doc count + RG count metrics.
- `builder.go:322` `parseAndStageDocument` → a log warning on transactional rollback (already the failure point in v1.0 Phase 07).
- `query.go` `Evaluate` → one span per public call, attributes: operator mix (distinct ops), predicate count — **no path names, no values**.
- `serialize.go` `Encode`, `Decode` → duration + bytes metric, failure type attribute.
- `parquet.go` `BuildFromParquet`, `LoadIndex`, `WriteSidecar`, `RebuildWithIndex` → already boundary-shaped; one span each.
- `s3.go` `S3Client.*` → already boundary-shaped; one span each.
- `cmd/gin-index/main.go` → root bootstrap via `telemetry.FromEnv(ctx, "gin-index")`, deferred `signals.Shutdown` with a short timeout before `os.Exit`, mirroring go-wand `cmd/go-wand/main.go` ownership.

---

## Theme 3 — Experimentation CLI

### User Stories

- **US-CLI-01** — "As a new evaluator, I want to point the CLI at a JSONL file and see 'here's what the index thinks about your data' in under one command, without needing Parquet."
- **US-CLI-02** — "As a teacher/demo author, I want a readable per-path summary (types, cardinality, adaptive mode, number of promoted hot terms, bloom occupancy) so I can explain what the index does."
- **US-CLI-03** — "As a user tuning config, I want to try a predicate and see which row groups match plus *why* each one was kept or pruned."

### Current CLI Surface (v1.0, from `cmd/gin-index/main.go:30-45`)

Existing subcommands: `build`, `query`, `info`, `extract`, `help`. All are Parquet-centric (`build -c column data.parquet`). There is no JSONL ingest path today.

### Table Stakes (must-have)

| # | Feature | Complexity | Depends on | User story |
|---|---------|-----------|------------|------------|
| TS-CLI-1 | New subcommand (proposed: `experiment` or `inspect`) accepting a `.jsonl` or `.jsonl.gz` file plus an explicit `-rg-size N` docs-per-row-group flag | S | Existing `NewBuilder` + `AddDocument` loop | US-CLI-01 |
| TS-CLI-2 | Synthetic row-group assignment: `rgID = docIndex / rgSize` — makes pruning observable without requiring Parquet | S | Builder API | US-CLI-01 |
| TS-CLI-3 | Summary report: per-path table of `{path, observed_types, cardinality_estimate, mode, bloom_occupancy, promoted_hot_terms_count, null_rg_count}` | S | `gin-index info` already renders most of this for `.gin` files — refactor that renderer for reuse | US-CLI-02 |
| TS-CLI-4 | Build-time stats: total docs, total row groups, unique paths, build duration, encoded-size (+ zstd-compressed size) | S | `Encode()` already returns `[]byte` | US-CLI-02 |
| TS-CLI-5 | Optional output: `-o out.gin` writes the index so the caller can then run `gin-index query` on it | S | Existing serialization | US-CLI-01, US-CLI-03 |
| TS-CLI-6 | JSONL streaming with bounded memory — read line-by-line, don't slurp the whole file | M | `bufio.Scanner` with a tunable buffer, or `encoding/json.Decoder` token stream | US-CLI-01 (scales beyond tiny files) |
| TS-CLI-7 | Error-tolerant mode: malformed JSONL lines are counted and reported, not fatal (default: first error wins, `-skip-errors` opts into tolerant mode) | S | Error counter | US-CLI-01 |

### Differentiators (high-value)

| # | Feature | Complexity | Depends on | User story |
|---|---------|-----------|------------|------------|
| D-CLI-1 | Inline predicate tester: `gin-index experiment data.jsonl --test '$.status = "error"'` ingests, builds, then evaluates the predicate and shows `{matching_rgs, candidate_rgs, pruned_rgs, pruning_ratio}` | S | Existing CLI predicate parser (`query.go` subcommand) | US-CLI-03 |
| D-CLI-2 | Pruning-attribution per predicate: show *which index structure* decided (`bloom_miss`, `string_exact_hit`, `numeric_range_miss`, `trigram_candidate`) | M | Requires an explainer path into `query.go`; currently opaque | US-CLI-03 |
| D-CLI-3 | Two-file diff (`gin-index experiment a.jsonl b.jsonl --diff`) — shows changes in per-path cardinality, types, newly-appearing paths | M | Two builds + diff renderer | expansion of US-CLI-02 |
| D-CLI-4 | Sample-mode flag `--sample 10000` to cap ingest when the user only wants a quick look at a big file | S | Counter + early exit | US-CLI-01 |
| D-CLI-5 | Multiple projections: `--projection '$.a,$.b,$.c'` builds only the listed paths to see the impact on index size | M | Requires projection filter in `builder.go` — larger change | advanced US-CLI-02 |
| D-CLI-6 | JSON output mode (`-format json`) so the summary can be piped into `jq` or a CI dashboard | S | Second renderer | automation use case |

### Anti-Features (explicit NO)

| Anti-Feature | Why avoid | What to do instead |
|--------------|-----------|--------------------|
| Full interactive REPL with readline, tab-completion, history | Large scope, UX maintenance burden, duplicates what `duckdb` and `jq` already do well | Subcommand with `-test` flag + JSON output for piping; defer REPL to a later milestone or let users wrap the CLI in a shell loop |
| Reimplement a query DSL beyond current operators | Scope creep; v1.0 `out of scope` explicitly excludes new DSL | Reuse existing CLI predicate parser (`$.status = "error"`, `$.count > 100`, `IN (…)`, etc.) |
| Switch from flat `flag` package to `cobra`/`urfave/cli` | Adds dependencies; CLI currently uses stdlib flag per `cmd/gin-index/main.go:91`; the bar for dependency addition is high on a lean library | Stay on `flag.NewFlagSet` unless multi-level subcommands become the norm |
| Embed a data-transform language (jq/gron) | Explicitly different tool category | Point users to `jq file.jsonl` for reshaping input before piping into `gin-index experiment -` (stdin) |
| Auto-download sample datasets | Implicit network I/O in a dev tool is unfriendly | Accept `-` for stdin; document `curl …/twitter.json \| gin-index experiment -` idiom |
| TUI (full-screen terminal UI) | Huge scope; k9s-style interaction is a separate product | Keep output plain text + optional JSON |

### Evidence from Adjacent Tools (informs UX choices, doesn't dictate)

- **DuckDB CLI** (referenced in the WebSearch sources): reads JSONL directly, auto-detects types, single-command exploration. Demonstrates that the table-stakes UX bar is "one command, typed summary, no config." We should match that simplicity for `experiment`.
- **gron** / **fx** / **jq**: interactive JSON exploration; confirm that *no one* expects a pruning index CLI to replicate their UX. Our job is "what would my index do with this data?", not "let me rummage through this JSON."
- **`gh` CLI** (stdlib-flag in early versions, now cobra): natural progression — start with `flag`, graduate to cobra only if subcommand count grows beyond ~8–10. v1.1 sits at 5 subcommands after adding `experiment`; stay on `flag`.

---

## Feature Dependencies (cross-theme)

```
Theme 1 (SIMD)  ─┐
                 ├─▶ Theme 3 CLI `experiment` can accept `-parser simdjson` flag
Theme 2 (OBS)   ─┤    (differentiator, not table stakes)
                 └─▶ Theme 3 CLI emits structured logs via Theme 2 slog adapter
                       when `-log-level debug` is set; default is silent
```

- SIMD has **no runtime dependency** on observability — parser selection is a config option, not an instrumented seam.
- CLI can ship independently of SIMD (it just uses the default `encoding/json` path).
- CLI can ship independently of observability (it has its own stdout/stderr UX; logging is opt-in via flag).
- Observability **should land before or alongside** SIMD so parser-selection events (e.g., capability fallback) have a place to go.

---

## MVP Recommendation for v1.1

**Land in this order:**

1. **Theme 2 Table Stakes (TS-OBS-1 through TS-OBS-10)** — establishes the logging seam, noop default, slog/zap adapters, structured-key vocabulary. Small, foundational, unlocks the rest. *Evidence base:* go-wand PR #114 gives a working blueprint.
2. **Theme 1 Table Stakes (TS-SIMD-1 through TS-SIMD-6)** — parser interface + default adapter + `pure-simdjson` adapter + benchmark corpus. Theme 2 is useful here (log parser-choice + fallback events).
3. **Theme 3 Table Stakes (TS-CLI-1 through TS-CLI-7)** — ships the `experiment` subcommand. Both earlier themes are visible to the CLI user as flags.
4. **Theme 2 Differentiators (D-OBS-1 through D-OBS-8)** — OTel `Signals`, FromEnv bootstrap, context-aware API variants, frozen vocab. Directly portable from go-wand Phase 08.
5. **Theme 3 Differentiators (D-CLI-1, D-CLI-2, D-CLI-4)** — predicate tester, pruning attribution, sample mode.
6. **Theme 1 Differentiators (D-SIMD-1, D-SIMD-3)** — JSONL batched SIMD path, per-doc failure isolation.

**Defer past v1.1:**
- D-CLI-3 (two-file diff), D-CLI-5 (projection filter) — larger scope, niche.
- D-SIMD-4 (second-parser shim) — interface hygiene only; validate later.
- Interactive REPL — out of scope.
- Parallel build under SIMD — out of scope.

---

## Confidence & Open Questions

| Area | Confidence | Notes |
|------|-----------|-------|
| go-wand observability blueprint | HIGH | Two recently-merged in-house PRs (#114, #115) with full source + docs; direct evidence. |
| SIMD parser landscape | HIGH | Well-surveyed via minio/simdjson-go (2k stars) and owned `pure-simdjson`; exact-int invariants understood from v1.0 Phase 07. |
| CLI UX patterns | MEDIUM-HIGH | duckdb/jq/gron/fx are reference points; the specific "experiment subcommand" shape is a design choice, not an industry template. |
| Benchmark corpus realism | MEDIUM | SEED-001 identifies simdjson example datasets; those need to be vendored or pinned before benchmarks are reproducible. |
| Pruning-attribution feasibility (D-CLI-2) | MEDIUM | `query.go` does not currently expose per-predicate decision provenance; adding it is a non-trivial internal API change. Flag as "needs phase-specific research." |

---

## Sources

- go-wand Phase 07 logging PR: `gh pr view 114 --repo amikos-tech/go-wand`
- go-wand Phase 08 telemetry PR: `gh pr view 115 --repo amikos-tech/go-wand`
- go-wand `pkg/logging/logger.go` (4-level interface)
- go-wand `pkg/logging/doc.go` (noop-default, context-free, adapter sub-packages)
- go-wand `pkg/telemetry/telemetry.go` (Signals container, Disabled() zero-value)
- go-wand `pkg/telemetry/attrs.go` (frozen operation + metric + error-type vocabulary)
- go-wand `pkg/telemetry/env.go` (OTEL env bootstrap, 5s init / 10s export timeout)
- go-wand `docs/logging.md` (safe-metadata policy, canonical key vocabulary)
- go-wand `docs/telemetry.md` (CLI ownership, correlation stance)
- `amikos-tech/pure-simdjson` repo (last pushed 2026-04-20, active) — `doc.go`, `example_test.go`
- `minio/simdjson-go` — 2,021 stars, Apache-2.0, last pushed 2025-08-26, AVX2+CLMUL required, ~10x encoding/json on minio's own benchmarks ([blog.min.io](https://blog.min.io/simdjson-go-parsing-gigabyes-of-json-per-second-in-go/))
- `simdjson-go` Go Package docs ([pkg.go.dev/github.com/minio/simdjson-go](https://pkg.go.dev/github.com/minio/simdjson-go))
- v1.0 `builder.go:322 parseAndStageDocument` — the integration point for the SIMD parser seam
- v1.0 `cmd/gin-index/main.go:30-45` — existing subcommand skeleton the `experiment` command extends
- SEED-001: `/Users/tazarov/experiments/amikos/custom-gin/.planning/seeds/SEED-001-simdjson-test-datasets.md` — benchmark corpus plan
- DuckDB-as-jq perspective ([pgrs.net](https://www.pgrs.net/2024/03/21/duckdb-as-the-new-jq/), [stephan-rayner.github.io](https://stephan-rayner.github.io/posts/jq2duckdb/))
- fx terminal viewer ([terminal.guide](https://www.terminal.guide/tools/dev-tool/fx/))
- gron (JSON-to-grep) — referenced in the DuckDB and jq articles above
