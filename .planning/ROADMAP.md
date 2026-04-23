# Roadmap: GIN Index

## Milestones

- ✅ **v0.1.0 OSS Launch** — Phases 01-05 (shipped pre-v1.0)
- ✅ **v1.0 Query & Index Quality** — Phases 06-12 (shipped 2026-04-21) — see [`milestones/v1.0-ROADMAP.md`](./milestones/v1.0-ROADMAP.md)
- ✅ **v1.1 Performance, Observability & Experimentation** — Phases 13-15 (functionally complete 2026-04-22; PRs #29 and #30 merged)
- 🚧 **v1.2 Ingest Correctness & Per-Document Isolation** — Phases 16-18 (started 2026-04-23)
- ⏸️ **v1.3 SIMD JSON Path** — Phases 19-20 (preview only; deferred 2026-04-21 pending `pure-simdjson` license/tag/distribution resolution; renumbered from v1.2 on 2026-04-23)

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

<details>
<summary>✅ v1.1 Performance, Observability & Experimentation (Phases 13-15) — FUNCTIONALLY COMPLETE 2026-04-22</summary>

- [x] **Phase 13: Parser Seam Extraction** — Pure refactor: extract the JSON-parse boundary from the builder into a pluggable `Parser` interface with a `stdlibParser` default. Completed 2026-04-21; residual benchmark noise accepted in `13-SECURITY.md`.
- [x] **Phase 14: Observability Seams** — `Logger` + `Telemetry` + `Signals` (OTel providers, never global), boundary-only spans, frozen attribute vocabulary, `slog`/`stdlib` adapters, context-aware API variants, `adaptiveInvariantLogger` migration. Completed 2026-04-22.
- [x] **Phase 15: Experimentation CLI** — New `experiment` subcommand: JSONL in (file or stdin) → index → per-path summary + optional sidecar write, predicate tester, JSON mode, sample/error-tolerant modes. Completed 2026-04-22.

</details>

### 🚧 v1.2 Ingest Correctness & Per-Document Isolation (Phases 16-18) — ACTIVE

- [x] **Phase 16: AddDocument Atomicity (Lucene contract)** — Extend `validateStagedPaths` to cover every reason `mergeStagedPaths` / `mergeNumericObservation` could fail; make the merge step infallible by construction; rename `poisonErr` → `tragicErr` and narrow it to internal-invariant violations; `recover()`-in-merge belt-and-suspenders. Atomicity property test as merge gate. Completed 2026-04-23.
- [ ] **Phase 17: Failure-Mode Taxonomy Unification** — Unified `IngestFailureMode` type (`Hard`/`Soft`) replaces `TransformerFailureMode` (deliberate breaking rename); extends to parser and numeric-promotion layers; new `WithParserFailureMode` / `WithNumericFailureMode` config knobs.
- [ ] **Phase 18: Structured `IngestError` + CLI integration** — Exported `IngestError` carrying `Path`, `Layer`, `Cause`, `Value` (caller redacts); `errors.As`-friendly; `gin-index experiment --on-error continue` summary reports per-layer grouped failures with structured samples in text and `--json` modes.

### ⏸️ v1.3 SIMD JSON Path (Phases 19-20) — PREVIEW / DEFERRED

- [ ] **Phase 19: SIMD Parser Adapter** — Same-package `simdjson` parser behind `//go:build simdjson`, explicit opt-in via `WithParser(...)`, preserve exact-int semantics, keep `stdlib` as the default path.
- [ ] **Phase 20: SIMD Validation, Datasets & CI** — Benchmark stdlib vs SIMD on SEED-001-backed fixtures, vendor the required simdjson example corpus + NOTICE metadata, add `-tags simdjson` CI coverage, and lock the shared-library distribution/loading contract.

## Phase Details

### Phase 13: Parser Seam Extraction
**Goal**: Extract the JSON-parse boundary from the builder behind a pluggable `Parser` interface, with zero behavior change, so a SIMD parser can land in v1.3 without touching builder internals.
**Depends on**: Nothing (first phase of v1.1)
**Requirements**: PARSER-01
**Success Criteria** (what must be TRUE):
  1. Consumers can pass `WithParser(p)` to `NewBuilder`; omitting it yields the v1.0 `json.Decoder.UseNumber()` behavior via a `stdlibParser` default (Name() == "stdlib").
  2. A `parser_parity_test.go` harness runs the existing builder corpus through the legacy direct-call path and through `stdlibParser`, asserting byte-identical encoded index output and identical `Evaluate` results for every representative predicate.
  3. Public surface adds the exported `Parser` interface and `WithParser` entry point while keeping `parserSink` and `stdlibParser` package-private by design — no existing method signature changes, no breaking rename, `go test ./...` remains green, and the residual benchmark-noise exception is documented in `13-SECURITY.md`.
  4. `Parser.Name()` is reachable from telemetry attribute sites (consumed by Phase 14) — verified by a unit test that asserts the default builder reports `"stdlib"`.
**Plans**: 3 plans

Plans:
- [x] 13-01-PLAN.md — Parser interface + stdlibParser + sink adapters (additive; no AddDocument wiring yet)
- [x] 13-02-PLAN.md — Wire AddDocument through b.parser.Parse + NewBuilder default + delete dead walkers
- [x] 13-03-PLAN.md — Parity harness (authored goldens + gopter determinism + 12-operator Evaluate matrix) — merge gate

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
**Plans**: 4 plans

Plans:
- [x] 14-01-PLAN.md — core logging/telemetry packages, GINConfig wiring, and default observability behavior
- [x] 14-02-PLAN.md — query boundary instrumentation, EvaluateContext, invariant logger migration, and perf gates
- [x] 14-03-PLAN.md — parquet/build + encode/decode context siblings and boundary coverage
- [x] 14-04-PLAN.md — policy and phase-level verification gates

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
**Plans**: 3 plans

Plans:
- [x] 15-01-PLAN.md — experiment subcommand foundation: dispatcher wiring, stdin-aware runner, two-pass streaming ingest, base text summary, foundation tests
- [x] 15-02-PLAN.md — shared report model, JSON mode, predicate test, sidecar write, and log-level wiring
- [x] 15-03-PLAN.md — sample/on-error semantics, counter reporting, and executable policy guards

### Phase 16: AddDocument Atomicity (Lucene contract)
**Goal**: `AddDocument` returning a non-tragic error leaves the builder in a state indistinguishable from never having received the failed call. Bring the builder in line with the Lucene per-document contract (*"if an Exception is hit, the index will be consistent, but this document may not have been added"*) without changing the builder's external API surface.
**Depends on**: Nothing (first phase of v1.2)
**Requirements**: ATOMIC-01, ATOMIC-02, ATOMIC-03
**Success Criteria** (what must be TRUE):
  1. `mergeStagedPaths` (`builder.go:743`) and `mergeNumericObservation` (`builder.go:799`) return no error in their new signatures; a compile-time check enforces the change.
  2. `validateStagedPaths` (`builder.go:724`) is extended to fully simulate every reason the merge functions could fail, against the *real* `pathData` state (not a fresh preview). Per-failure-mode tests cover numeric promotion, transformer rejection, parser errors, and any other identified mode.
  3. `tragicErr` (renamed from `poisonErr` at `builder.go:34`) is set in zero code paths reachable from user input. A unit test exhausts the public failure-mode catalog and asserts `tragicErr` stays nil throughout.
  4. A `recover()`-in-merge belt-and-suspenders converts any reachable panic in `mergeStagedPaths` to `tragicErr` rather than letting it escape; an explicit test invokes the recovery path.
  5. An atomicity property test (`atomicity_test.go`) ingests a `gopter`-generated corpus of ≥1000 documents with ≥10% guaranteed-failing documents (mixed parser/numeric/transformer failures) and asserts the encoded index is byte-identical to the same corpus without the failures.
  6. A `// MUST_BE_CHECKED_BY_VALIDATOR` marker convention is established on merge-layer functions, with a CI grep that flags any reintroduction of merge-layer error returns.
**Plans**: 4 plans

Plans:
- [x] 16-01-PLAN.md — Validator-complete numeric promotion hoisting and no-error merge signatures
- [x] 16-02-PLAN.md — Tragic terminal state rename, merge recovery, and safe recovery logging
- [x] 16-03-PLAN.md — Public failure catalog and AddDocument atomicity property tests
- [x] 16-04-PLAN.md — Local lint and CI marker/signature enforcement

### Phase 17: Failure-Mode Taxonomy Unification
**Goal**: Provide one mental model for "what can go wrong with a document and how do I configure the response." Unify the existing transformer-only failure-mode concept into a single `IngestFailureMode` type that applies uniformly to parser, transformer, and numeric-promotion layers.
**Depends on**: Phase 16 (per-document failure must be a first-class concept before the taxonomy can route it cleanly)
**Requirements**: FAIL-01, FAIL-02
**Success Criteria** (what must be TRUE):
  1. New `IngestFailureMode` type (`Hard`/`Soft`) replaces the existing `TransformerFailureMode` constants; the rename is a deliberate breaking change documented in CHANGELOG with a one-line migration note.
  2. New config knobs `WithParserFailureMode(mode)` and `WithNumericFailureMode(mode)` thread through to the parser and numeric layers; default `Hard` for both, preserving current behavior.
  3. `Soft` mode at any layer skips the failing document silently and returns no error to the caller; per-layer test coverage matches the existing transformer-failure-mode tests.
  4. An `examples/failure-modes/main.go` demonstrates one config that rejects on any failure and one that skips on any failure, with predictable output for both.
**Plans**: TBD (planned during `/gsd-discuss-phase 17` and `/gsd-plan-phase 17`)

### Phase 18: Structured `IngestError` + CLI integration
**Goal**: Make per-document failures actionable by callers. Replace the current opaque wrapped-string error with a structured type that carries enough context to identify, classify, and act on failures programmatically; surface the same structure in the existing CLI summary.
**Depends on**: Phase 16 (atomicity contract), Phase 17 (failure-mode taxonomy)
**Requirements**: IERR-01, IERR-02, IERR-03
**Success Criteria** (what must be TRUE):
  1. Exported `IngestError` type carries `Path` (JSONPath), `Layer` (parser / transformer / numeric / schema), `Cause` (wrapped underlying error), and `Value` (verbatim string repr — caller redacts as needed; the library does not redact).
  2. `errors.As`-friendly: callers can extract `IngestError` from anywhere in the wrap chain. Per-layer test matrix asserts round-trip extraction for every error site.
  3. All ingest-error sites identified in Phases 16 and 17 wrap their underlying error in `IngestError` with the four fields populated; a grep+test enforces no plain `errors.New` / `errors.Wrap` returns from ingest sites.
  4. `gin-index experiment --on-error continue` reports per-document failures grouped by `Layer` (with counts) plus a sample of the first N `IngestError`s with structured fields, in both text and `--json` output modes; both modes are golden-tested.
  5. A test feeds 100 docs with 10 known failures (3 parser, 4 transformer, 3 numeric) through `gin-index experiment --on-error continue --json` and asserts the JSON output contains the correct grouped counts and a non-empty sample array with structured fields.
**Plans**: TBD (planned during `/gsd-discuss-phase 18` and `/gsd-plan-phase 18`)

### Phase 19: SIMD Parser Adapter
**Goal**: Land an opt-in same-package SIMD parser implementation behind the Phase 13 seam, without changing the default `encoding/json` path or weakening the Phase 07 numeric-fidelity guarantees.
**Depends on**: Phase 13
**Blocked on**: upstream `pure-simdjson` LICENSE file, version tag, and a settled shared-library distribution/loading decision
**Requirements**: Deferred SIMD scope from original PARSER-02..05 (to be restated when v1.3 formally opens)
**Success Criteria** (what must be TRUE):
  1. `parser_simd.go` behind `//go:build simdjson` adds a same-package SIMD parser constructor and `WithParser(...)` can select it without any builder-internal changes beyond the already-landed Phase 13 seam.
  2. The SIMD path preserves exact-int semantics for the Phase 07 numeric corpus, routing overflow-sensitive numbers through the existing builder classifier rather than silently coercing them to `float64`.
  3. Default builds remain stdlib-only: no simd dependency or runtime shared-library requirement unless the build tag is enabled and the parser is explicitly selected.
  4. Parity tests prove `Evaluate` results match the stdlib parser across the authored Phase 13 fixtures and targeted numeric edge cases.
**Plans**: TBD

### Phase 20: SIMD Validation, Datasets & CI
**Goal**: Validate and operationalize the SIMD path with reproducible corpora, distribution guidance, and CI coverage so the opt-in parser is shippable rather than experimental.
**Depends on**: Phase 19
**Blocked on**: final parser dependency choice and shared-library distribution contract
**Requirements**: Deferred SIMD scope from original PARSER-02..05 (to be restated when v1.3 formally opens)
**Success Criteria** (what must be TRUE):
  1. A benchmark suite compares stdlib vs SIMD ingest on SEED-001-backed fixtures and reports reproducible CPU and allocation deltas.
  2. Required simdjson example fixtures are vendored under `testdata/` with preserved license/NOTICE metadata and documented size limits.
  3. CI runs both the default and `-tags simdjson` test paths on the supported platform set, with explicit skip/fail behavior when the shared library is unavailable.
  4. Runtime loading and release/distribution guidance for the shared library is documented and tested so consumers can enable the SIMD path without guesswork.
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
| 13. Parser Seam Extraction | v1.1 | 3/3 | Complete | 2026-04-21 |
| 14. Observability Seams | v1.1 | 4/4 | Complete | 2026-04-22 |
| 15. Experimentation CLI | v1.1 | 3/3 | Complete | 2026-04-22 |
| 16. AddDocument Atomicity (Lucene contract) | v1.2 | 4/4 | Complete | 2026-04-23 |
| 17. Failure-Mode Taxonomy Unification | v1.2 | 0/- | Planned | - |
| 18. Structured IngestError + CLI integration | v1.2 | 0/- | Planned | - |
| 19. SIMD Parser Adapter | v1.3 preview | 0/- | Deferred | - |
| 20. SIMD Validation, Datasets & CI | v1.3 preview | 0/- | Deferred | - |

---
*v1.2 started 2026-04-23 with 8 requirements across 3 phases (ATOMIC-01..03, FAIL-01..02, IERR-01..03). Architectural strategy: validate-before-mutate with Lucene's per-document contract as the target. v1.1 functionally complete 2026-04-22 (PRs #29, #30). v1.0 shipped 2026-04-21. Prior milestone v0.1.0 completed the OSS launch (phases 01-05).*
*v1.3 (was v1.2) preview carries forward the deferred SIMD scope from original PARSER-02..05; exact requirement IDs will be restated when that milestone formally opens. Renumbered from v1.2 → v1.3 on 2026-04-23 to preserve chronological phase numbering.*

## Backlog

### Phase 999.1: Lefthook Pre-Push Quality Gates (BACKLOG)

**Goal:** Add `lefthook`-based pre-push quality gates, modeled on `/Users/tazarov/experiments/telia/tclr/tclr-v2/lefthook.yml`, to block pushes when required local validation fails or required tools are missing. Scope the future implementation around this repo's native checks such as `make lint` and `make test`, with room for selective changed-package execution if that keeps hook latency reasonable.
**Requirements:** TBD
**Plans:** 0 plans

Plans:
- [ ] TBD (promote with /gsd-review-backlog when ready)

### Phase 999.6: Row-Level Pruning Messaging And Positioning (BACKLOG)

**Goal:** Clarify across the README, CLI/docs, and experimentation guidance that GIN Index is not limited to Parquet-style row-group pruning. The library should be presented as supporting both grouped pruning and row-level pruning when callers choose `rg=1`, without changing the existing pruning-first model or forcing an API rename. Deliverable: messaging, examples, and terminology guidance that explain when row groups are a storage optimization versus when single-row groups are the right mental model.
**Requirements:** TBD — likely includes README/product positioning updates, `gin-index experiment` wording, and terminology consistency review.
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

### Phase 999.7: Bloom AddString Allocation Cleanup (BACKLOG, considered+deferred during v1.2 brainstorming)

**Goal:** Eliminate the per-insert allocations in `bloom.AddString` (`bloom.go:55`): the `path + "=" + term` concat and the `append(data, 0xFF)` in the hash function (`bloom.go:47`). Sketches: precompute and cache hashed `path=` prefix per builder; use a salt or pre-sized buffer instead of `append(data, 0xFF)` for the second hash. Per project precedent (999.5), profile first to confirm the bloom hot path is meaningful in real ingestion before implementing.
**Requirements:** Profile ingestion on representative JSONL workloads first; quantify allocation share attributable to `bloom.AddString`.
**Plans:** 0 plans

Plans:
- [ ] TBD (promote with /gsd-review-backlog when ready)

### Phase 999.8: Per-Path `[*]` Wildcard Opt-Out (BACKLOG, considered+deferred during v1.2 brainstorming)

**Goal:** Allow callers to opt out of the implicit array-element double-staging at `path[i]` and `path[*]` (`parser_stdlib.go:95-100`, `builder.go:498-503`). Today every array element is staged twice — once at the indexed path and once at the wildcard — which doubles per-element work for any path the caller doesn't query with `[*]`. Sketches: per-path config (`WithoutWildcard("$.tags")`) or a builder-wide default toggled per path. Mostly a perf win; surface design needs care to avoid violating the principle of least surprise for existing wildcard users.
**Requirements:** TBD — define API surface (per-path vs builder-wide vs config map) during planning; benchmark wildcard cost on long-array fixtures.
**Plans:** 0 plans

Plans:
- [ ] TBD (promote with /gsd-review-backlog when ready)
