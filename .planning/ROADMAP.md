# Roadmap: GIN Index

## Milestones

- ✅ **v0.1.0 OSS Launch** — Phases 01-05 (shipped pre-v1.0)
- ✅ **v1.0 Query & Index Quality** — Phases 06-12 (shipped 2026-04-21) — see [`milestones/v1.0-ROADMAP.md`](./milestones/v1.0-ROADMAP.md)
- 🚧 **v1.1 Performance, Observability & Experimentation** — Phases 13-15 (started 2026-04-21)

## Phases

<details>
<summary>✅ v1.0 Query & Index Quality (Phases 06-12) — SHIPPED 2026-04-21</summary>

- [x] Phase 06: Query Path Hot Path (2/2 plans) — completed 2026-04-14
- [x] Phase 07: Builder Parsing & Numeric Fidelity (2/2 plans) — completed 2026-04-15
- [x] Phase 08: Adaptive High-Cardinality Indexing (3/3 plans) — completed 2026-04-15
- [x] Phase 09: Derived Representations (3/3 plans) — completed 2026-04-16
- [x] Phase 10: Serialization Compaction (3/3 plans) — completed 2026-04-17
- [x] Phase 11: Real-Corpus Prefix Compression Benchmarking (3/3 plans) — completed 2026-04-20
- [x] Phase 12: Milestone Evidence Reconciliation (3/3 plans) — completed 2026-04-21

Full details: [`milestones/v1.0-ROADMAP.md`](./milestones/v1.0-ROADMAP.md)

</details>

### 🚧 v1.1 Performance, Observability & Experimentation (Phases 13-15) — ACTIVE

- [ ] **Phase 13: Parser Seam Extraction** — Pure refactor: extract the JSON-parse boundary from the builder into a pluggable `Parser` interface with a `stdlibParser` default. Parity harness is the merge gate.
- [ ] **Phase 14: Observability Seams** — `Logger` + `Telemetry` + `Signals` (OTel providers, never global), boundary-only spans, frozen attribute vocabulary, `slog`/`stdlib` adapters, context-aware API variants, `adaptiveInvariantLogger` migration.
- [ ] **Phase 15: Experimentation CLI** — New `experiment` subcommand: JSONL in (file or stdin) → index → per-path summary + optional sidecar write, predicate tester, JSON mode, sample/error-tolerant modes.

## Phase Details

### Phase 13: Parser Seam Extraction
**Goal**: Extract the JSON-parse boundary from the builder behind a pluggable `Parser` interface, with zero behavior change, so a SIMD parser can land in v1.2 without touching builder internals.
**Depends on**: Nothing (first phase of v1.1)
**Requirements**: PARSER-01
**Success Criteria** (what must be TRUE):
  1. Consumers can pass `WithParser(p)` to `NewBuilder`; omitting it yields the v1.0 `json.Decoder.UseNumber()` behavior via a `stdlibParser` default (Name() == "stdlib").
  2. A `parser_parity_test.go` harness runs the existing builder corpus through the legacy direct-call path and through `stdlibParser`, asserting byte-identical encoded index output and identical `Evaluate` results for every representative predicate.
  3. Public surface adds only the `Parser` interface, the narrow `ParserSink` write-side, `WithParser`, and `stdlibParser` — no existing method signature changes, no breaking rename, `go test ./...` and the v1.0 benchmark suite remain green.
  4. `Parser.Name()` is reachable from telemetry attribute sites (consumed by Phase 14) — verified by a unit test that asserts the default builder reports `"stdlib"`.
**Plans**: TBD

### Phase 14: Observability Seams
**Goal**: Make index build, query evaluation, and serialization observable through a backend-neutral logger and a `Signals`-style OTel container — zero-cost when disabled, no global OTel mutation, one logging convention across the codebase.
**Depends on**: Phase 13 (consumes `Parser.Name()` as a telemetry attribute; `adaptiveInvariantLogger` migrates in the same phase)
**Requirements**: OBS-01, OBS-02, OBS-03, OBS-04, OBS-05, OBS-06, OBS-07, OBS-08
**Success Criteria** (what must be TRUE):
  1. Library is silent by default: `NoopLogger` is wired in `DefaultConfig`, and `BenchmarkEvaluateDisabledLogging` asserts **0 allocs/op** against the v1.0 baseline.
  2. `BenchmarkEvaluateWithTracer` (tracer supplied but disabled) stays within **0.5% wall-clock** of the no-tracer baseline, enforced as a merge-gate assertion in the benchmark harness.
  3. Public API never exposes `*slog.Logger` or any OTel SDK type directly — `slog` and `stdlib log` adapters ship as separate sub-packages (`telemetry/slogadapter`, `telemetry/stdadapter`) and the core `go.mod` does not pull OTel SDK/exporters.
  4. A single grep for `log.Logger` field declarations in the library returns zero hits after migration: the `adaptiveInvariantLogger *log.Logger` at `query.go:17` is routed through the new `Logger` interface with no dual-logger state.
  5. `EvaluateContext` and `BuildFromParquetContext` are exported as additive siblings; existing `Evaluate` / `BuildFromParquet` delegate with `context.Background()` and a compatibility test proves the old entry points are untouched.
  6. Attribute vocabulary is frozen in a single source file (event names + `Attr` keys) and a test asserts emitted INFO-level attributes come only from the allowlist (`operation`, `predicate_op`, `path_mode`, `status`, `error.type`) — predicate values, path field names, doc/RG/term IDs are rejected.
**Plans**: TBD

### Phase 15: Experimentation CLI
**Goal**: A new `gin-index experiment` subcommand that turns a JSONL file (or stdin stream) into a built index plus a human- or JSON-readable per-path summary, with an inline predicate tester — so a new evaluator can measure pruning quality on their own data in one command.
**Depends on**: Phase 13 (`--parser` flag selects the parser seam) and Phase 14 (`--log-level` flag wires the telemetry adapter)
**Requirements**: CLI-01, CLI-02, CLI-03, CLI-04, CLI-05, CLI-06, CLI-07, CLI-08
**Success Criteria** (what must be TRUE):
  1. `gin-index experiment path/to/docs.jsonl` and `cat docs.jsonl | gin-index experiment -` both produce a per-path summary table (types, cardinality estimate, mode, bloom occupancy, promoted hot terms) reusing `writeIndexInfo` / `formatPathInfo`.
  2. Streaming JSONL ingest is verified on a fixture with lines longer than 64 KiB (exercises `bufio.Reader.ReadBytes` / sized `Scanner`) without truncation or OOM, with memory bounded independent of file size.
  3. `--test '<predicate>'` reports `matched`, `pruned`, and `pruning_ratio` row-group counts using `parsePredicate`; `--json` emits the same summary in a stable schema that is `jq`-parseable and asserted by a golden test.
  4. `-o out.gin` writes a loadable sidecar (round-trip test: load the sidecar with `gin.ReadSidecar` and verify the same pruning ratio for a canonical predicate).
  5. `--sample N` caps ingested documents at N; `--on-error continue|abort` toggles line-level error tolerance (default `abort`); both flags are covered by CLI end-to-end tests.
  6. The CLI ships with no new dependencies (stdlib `flag`, `text/tabwriter`, `bufio`, `encoding/json` only) and contains no REPL / TUI / color-auto-detection code — charter compliance asserted by a linter-or-grep check in CI.
**Plans**: TBD

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 06. Query Path Hot Path | v1.0 | 2/2 | Complete | 2026-04-14 |
| 07. Builder Parsing & Numeric Fidelity | v1.0 | 2/2 | Complete | 2026-04-15 |
| 08. Adaptive High-Cardinality Indexing | v1.0 | 3/3 | Complete | 2026-04-15 |
| 09. Derived Representations | v1.0 | 3/3 | Complete | 2026-04-16 |
| 10. Serialization Compaction | v1.0 | 3/3 | Complete | 2026-04-17 |
| 11. Real-Corpus Prefix Compression Benchmarking | v1.0 | 3/3 | Complete | 2026-04-20 |
| 12. Milestone Evidence Reconciliation | v1.0 | 3/3 | Complete | 2026-04-21 |
| 13. Parser Seam Extraction | v1.1 | 0/- | Not started | - |
| 14. Observability Seams | v1.1 | 0/- | Not started | - |
| 15. Experimentation CLI | v1.1 | 0/- | Not started | - |

---
*v1.1 started 2026-04-21 with 17 requirements across 3 phases. v1.0 shipped 2026-04-21. Prior milestone v0.1.0 completed the OSS launch (phases 01-05).*

## Backlog

### Phase 999.1: Lefthook Pre-Push Quality Gates (BACKLOG)

**Goal:** Add `lefthook`-based pre-push quality gates, modeled on `/Users/tazarov/experiments/telia/tclr/tclr-v2/lefthook.yml`, to block pushes when required local validation fails or required tools are missing. Scope the future implementation around this repo's native checks such as `make lint` and `make test`, with room for selective changed-package execution if that keeps hook latency reasonable.
**Requirements:** TBD
**Plans:** 0 plans

Plans:
- [ ] TBD (promote with /gsd-review-backlog when ready)

### Phase 999.2: walkJSON NormalizePath Fast-Path (BACKLOG)

**Goal:** Add a fast-path in `walkJSON` to skip `NormalizePath` when the path is already in canonical form (builder-generated paths never contain bracket-quoted fields). This avoids `jp.ParseString` overhead on every recursive call during ingestion.
**Requirements:** Profile ingestion to confirm `NormalizePath` is a measurable hotspot before implementing.
**Plans:** 0 plans

### Phase 999.3: Minor Code Clarity in Phase 06 (BACKLOG)

**Goal:** Address non-blocking observations from Phase 06 review: (a) add comment on `findPath` bounds check explaining it guards against corruption; (b) reorder or comment `validatePathReferences` to clarify it reads the original directory; (c) make benchmark fixture path count assertion less brittle.
**Requirements:** None — cosmetic improvements only.
**Plans:** 0 plans

### Phase 999.4: WithEncodeStrategy Config Option (BACKLOG)

**Goal:** Expose `WithEncodeStrategy(Auto|RawOnly|FrontCodedOnly)` ConfigOption for ordered string sections so callers can declare data shape per-index and skip the dual-encode pass on known-random paths (UUIDs, hashes). Implementation sketch: new `EncodeStrategy` uint8 iota, field on `GINConfig`, option in `gin.go`, threaded through to `writeOrderedStrings` in `serialize.go:440`. Follows RocksDB's `block_restart_interval` precedent — industry-standard "knob, not heuristic" pattern confirmed by 2026-04-20 research sweep across Lucene BlockTree, Tantivy SSTable, PostgreSQL GIN, RocksDB/LevelDB, Roaring Bitmaps, Bleve/Vellum, Badger, and Lasch VLDB 2020. Zero libraries use upfront sampling to choose front-coded vs raw.
**Requirements:** Blocked on profiling data from Phase 999.5 justifying the API surface. Originates from PR #23 review feedback (non-blocking).
**Plans:** 0 plans

Plans:
- [ ] TBD (promote with /gsd-review-backlog when ready)

### Phase 999.5: Profile Encode CPU on Real Workloads (BACKLOG)

**Goal:** Measure encode CPU and allocations for `writeOrderedStrings` on representative workloads (UUID-heavy paths, log-style timestamp paths, mixed JSON corpora) before committing to the Phase 999.4 API surface. Per Roaring Bitmaps' measure-first philosophy and the PR #23 reviewer's own framing — "if encode performance ever shows up in profiling" — we want profiling data to justify whether the dual-encode cost in `serialize.go:456-474` is a real bottleneck or a speculative optimization. Deliverable: a benchmark + flamegraph report showing encode CPU share on realistic fixtures, decision record on whether 999.4 is worth shipping.
**Requirements:** TBD — define workload fixtures (UUID, timestamp, mixed) during planning.
**Plans:** 0 plans

Plans:
- [ ] TBD (promote with /gsd-review-backlog when ready)
