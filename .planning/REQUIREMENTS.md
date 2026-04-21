# Requirements: GIN Index v1.1 Performance, Observability & Experimentation

**Defined:** 2026-04-21
**Core Value:** Make index internals observable, experimentation frictionless, and prepare the builder for a future SIMD JSON path — without forcing new dependencies on consumers who don't opt in.

## Scope Note

v1.1 introduces the **parser seam** (a pure refactor that extracts the JSON-parse boundary from the builder) but defers the SIMD parser implementation to v1.2. Rationale:

- `github.com/amikos-tech/pure-simdjson` has no LICENSE file and no version tag as of 2026-04-21 — cannot be declared a stable dependency.
- Shared-library distribution mechanism (vendored vs. cargo-build vs. user-provided path) needs a design decision before the SIMD phase can commit.
- Landing the seam now means v1.2 can add the SIMD adapter as an isolated, build-tag-gated phase without touching any builder internals.

## Requirements

### Parser Seam

- [ ] **PARSER-01**: Builder exposes a pluggable `Parser` interface with a narrow `ParserSink` write-side, defaulting to a `stdlibParser` that wraps the current `json.Decoder.UseNumber()` path with zero behavior change. Validated via a parity test harness on the existing test corpus.

### Observability

- [ ] **OBS-01**: Library exposes a minimal local `Logger` interface (`Enabled(Level) bool`, `Log(Level, msg, ...Attr)`); noop default so the library stays silent by default.
- [ ] **OBS-02**: Observability is zero-cost when disabled — `BenchmarkEvaluateDisabledLogging` asserts 0 allocs/op and `BenchmarkEvaluateWithTracer` stays within 0.5% of the no-tracer baseline.
- [ ] **OBS-03**: `slog` and `stdlib log` adapters shipped as separate sub-packages; public API never exposes `*slog.Logger` directly.
- [ ] **OBS-04**: Frozen structured-attribute vocabulary (`operation`, `predicate_op`, `path_mode`, `status`, `error.type`); PII allowlist bans predicate values, path field names, doc/RG/term IDs from INFO level.
- [ ] **OBS-05**: `Telemetry`/`Signals` container carries OTel `TracerProvider` and `MeterProvider`; the library never mutates global OTel state (no `otel.SetTracerProvider`).
- [ ] **OBS-06**: Boundary-only instrumentation — coarse spans on `Evaluate`, `Encode`, `Decode`, `BuildFromParquet`; per-predicate decisions emit span events on the parent span, not nested spans.
- [ ] **OBS-07**: Context-aware API variants `EvaluateContext` and `BuildFromParquetContext` added as additive siblings; existing methods wrap with `context.Background()` — no breaking change.
- [ ] **OBS-08**: Existing `adaptiveInvariantLogger *log.Logger` at `query.go:17` migrated to the new `Logger` interface in the same phase (single convention, no dual logger state).

### Experimentation CLI

- [ ] **CLI-01**: New `experiment` subcommand in `cmd/gin-index/main.go` accepts JSONL from a file path or `-` (stdin).
- [ ] **CLI-02**: Emits a per-path summary table (types, cardinality estimate, mode, bloom occupancy, promoted hot terms) reusing `writeIndexInfo` / `formatPathInfo`.
- [ ] **CLI-03**: Streaming JSONL ingest with bounded memory — uses `bufio.Reader.ReadBytes` (or `Scanner` with an explicit buffer size) to avoid the 64KB default-line truncation.
- [ ] **CLI-04**: Optional `-o out.gin` flag writes the built index sidecar.
- [ ] **CLI-05**: `--json` output mode emits the summary in a stable schema for CI/piping into jq.
- [ ] **CLI-06**: Inline predicate tester `--test '<predicate>'` shows matched/pruned row-group counts and pruning ratio.
- [ ] **CLI-07**: Error-tolerant mode `--on-error continue|abort` configurable; default `abort`.
- [ ] **CLI-08**: Sample mode `--sample N` limits ingested documents for quick inspection of large JSONL files.

## Out of Scope (deferred to v1.2 or later)

| Feature | Reason |
|---------|--------|
| `pure-simdjson` parser implementation | Upstream blockers: no LICENSE file, no version tag, shared-library distribution undecided |
| SIMD parser benchmarks vs stdlib | Blocked on SIMD implementation |
| CI matrix for `-tags simdjson` builds | Blocked on SIMD implementation |
| Vendoring SEED-001 simdjson example datasets | Blocked on SIMD implementation |
| Experimentation CLI REPL / TUI mode | Scope creep — v1.1 ships a CLI subcommand, not an interactive tool |
| Two-file index diff | Low value vs complexity; wait for user demand |
| `zap` adapter | Ship `slog`/`stdlib` only; add `zap` on explicit user request |
| Auto-color output in the CLI | Explicit opt-out from colour libraries per research recommendation |
| Distributed tracing integration examples | Ship the API; ship examples when a consumer has a concrete integration |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| PARSER-01 | Phase 13 | Planned |
| OBS-01 | Phase 14 | Planned |
| OBS-02 | Phase 14 | Planned |
| OBS-03 | Phase 14 | Planned |
| OBS-04 | Phase 14 | Planned |
| OBS-05 | Phase 14 | Planned |
| OBS-06 | Phase 14 | Planned |
| OBS-07 | Phase 14 | Planned |
| OBS-08 | Phase 14 | Planned |
| CLI-01 | Phase 15 | Planned |
| CLI-02 | Phase 15 | Planned |
| CLI-03 | Phase 15 | Planned |
| CLI-04 | Phase 15 | Planned |
| CLI-05 | Phase 15 | Planned |
| CLI-06 | Phase 15 | Planned |
| CLI-07 | Phase 15 | Planned |
| CLI-08 | Phase 15 | Planned |

**Coverage:**
- Requirements total: 17
- Checked off: 0
- Mapped to phases: 17
- Unmapped: 0

---
*Requirements defined: 2026-04-21 for milestone v1.1 Performance, Observability & Experimentation*
*SIMD impl (original PARSER-02..05) deferred to v1.2 pending upstream pure-simdjson resolution*
