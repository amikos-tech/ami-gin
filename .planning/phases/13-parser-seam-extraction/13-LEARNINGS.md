---
phase: 13
phase_name: "parser-seam-extraction"
project: "GIN Index"
generated: "2026-04-22"
counts:
  decisions: 9
  lessons: 5
  patterns: 6
  surprises: 4
missing_artifacts:
  - "VERIFICATION.md (phase used VALIDATION.md instead)"
  - "STATE.md (phase-local — used .planning/STATE.md)"
---

# Phase 13 Learnings: parser-seam-extraction

## Decisions

### Narrow the exported parser-seam API surface (D-02)
Export only `Parser` and `WithParser`; keep `parserSink` and `stdlibParser` package-private even though the original ROADMAP success criterion listed all four as public.

**Rationale:** Every parser in the v1.1→v1.2 roadmap lives inside `package gin`, so exporting the sink is premature. Adding exports later is non-breaking; removing them is breaking. The narrower surface defers third-party-parser support without blocking it.
**Source:** 13-DISCUSSION-LOG.md, 13-parser-seam-extraction-01-SUMMARY.md, 13-parser-seam-extraction-03-SUMMARY.md

### ParserSink contract exposes `*documentBuildState`
Sink methods take `state` as their first argument rather than hiding state in a wrapper or returning it from `Parse`.

**Rationale:** Minimal diff from the existing call chain, zero new allocs/doc, and preserves the int64 classifier inside `builder.go` (Pitfall #1 from research). The two alternatives either added +1 alloc/AddDocument or surfaced `*documentBuildState` in two signatures instead of one.
**Source:** 13-DISCUSSION-LOG.md

### Parity-harness reference mechanism = golden bytes only
Generate `Encode()` output from v1.0 before the PR and commit it to `testdata/parity-golden/` as the source of truth.

**Rationale:** Zero dead code (vs. keeping a verbatim legacy walker) and permanent pinning of byte-level behavior. Hybrid (goldens + property equivalence) was achievable later as a separate determinism canary.
**Source:** 13-DISCUSSION-LOG.md, 13-parser-seam-extraction-02-SUMMARY.md

### Trusted stdlib parser uses a concrete fast path inside `AddDocument`
The default stdlibParser path uses a concrete helper inside `AddDocument`; only custom parsers go through `b.parser.Parse` with full runtime guards.

**Rationale:** The first generic seam wire-up introduced benchmark drift on the default `AddDocument` path. Special-casing the trusted path keeps the hot path benchmark-flat while still paying the contract-validation cost on extension implementations.
**Source:** 13-parser-seam-extraction-02-SUMMARY.md

### Cache `parserName` at `NewBuilder`, nil-check `WithParser`
`WithParser(nil)` errors at construction; `NewBuilder` calls `Name()` once and stores the result; empty `Name()` is also rejected.

**Rationale:** Matches the `WithCodec` precedent in the codebase. The on-demand variant defeats Phase 14's telemetry caching plans, and silent-fallthrough on nil is a known footgun.
**Source:** 13-DISCUSSION-LOG.md, 13-parser-seam-extraction-02-SUMMARY.md

### No error wrapping at `AddDocument` — parser owns its error context
`AddDocument` returns parser errors verbatim. Phase 14 will add parser name as a telemetry attribute, not as an error-string prefix.

**Rationale:** Wrapping at the seam would change v1.0 error text and create a parallel telemetry channel; the chosen approach keeps error strings stable and routes parser identity through telemetry instead.
**Source:** 13-DISCUSSION-LOG.md

### Transformer subtree buffering via `ShouldBufferForTransform(path) bool` sink method
The parser asks the sink before each path visit whether to buffer the subtree, mirroring today's `decodeTransformedValue` logic exactly.

**Rationale:** A `RepresentedPaths()` map upfront leaks shape and adds cache-invalidation risk; folding into `StageMaterialized` defeats streaming and adds an allocation regression. The chosen single-method approach preserves streaming with no per-doc allocations.
**Source:** 13-DISCUSSION-LOG.md, 13-parser-seam-extraction-01-SUMMARY.md

### Always-on parity assertions, no skip-on-missing-goldens path
The merge-gate parity test is non-skippable; missing goldens are a hard failure rather than a bootstrap shortcut.

**Rationale:** A skip path would silently mask regressions if the goldens were ever omitted in a future refactor. Authored goldens stay the byte-level source of truth at all times.
**Source:** 13-parser-seam-extraction-03-SUMMARY.md

### Accept transformer-heavy benchmark drift as residual risk in `13-SECURITY.md`
The transformer-heavy explicit-number probe stayed slightly above the `+2%` ns/op threshold (+2.14% / +2.19%); allocs/op stayed flat. The disposition was recorded as accepted residual risk rather than reopening the implementation.

**Rationale:** Allocation flatness and parity/correctness gates remained green; the threshold sits at the noise floor of the M3 Max for this workload. Reopening implementation work would not improve correctness and could destabilize the seam.
**Source:** 13-parser-seam-extraction-03-SUMMARY.md, 13-03-BENCH.md

---

## Lessons

### Generic seam wire-ups can quietly drift the hot-path benchmark
The first plan-02 wire-up was behaviorally correct but added measurable drift on the default `AddDocument` path. The trusted fast-path helper restored flatness.

**Context:** The benchmark gate caught drift that would have been invisible to behavioral tests. Without the focused probe, the seam refactor would have shipped with a real (if small) regression on every consumer using the default parser.
**Source:** 13-parser-seam-extraction-02-SUMMARY.md

### Multi-threaded broad benchmark runs are scheduler-noisy on this hardware
The default multi-threaded parent benchmark runs were dominated by scheduler/GC noise on the M3 Max — too unstable to act as a merge gate.

**Context:** Forced a methodology change mid-phase: focused `GOMAXPROCS=1`, `-count=3`, exact-anchored regex against named subbenchmarks. The new approach produced decision-quality numbers that survived re-runs.
**Source:** 13-parser-seam-extraction-02-SUMMARY.md, 13-03-BENCH.md

### Compressed `Encode()` output starts with `GINc`, not raw `GIN\x01`
The on-disk parity goldens are compressed `Encode()` outputs and begin with the transport wrapper magic (`GINc`); the inner index header (`GIN\x01`) is one layer down.

**Context:** The README initially documented the wrong magic. Surfaced because the parity goldens are committed binary blobs that contributors will inevitably inspect with `xxd`.
**Source:** 13-parser-seam-extraction-02-SUMMARY.md

### Wave-1 additive seam files are one compile unit, not four atomic commits
Splitting plan-01 tasks 1–4 into separate commits would leave intermediate commits uncompilable because the new parser files reference the new `GINBuilder` seam fields.

**Context:** The plan modeled tasks as independently committable; execution had to consolidate them into one green commit. Future seam-extraction plans should expect "one compile unit" granularity for additive surface work.
**Source:** 13-parser-seam-extraction-01-SUMMARY.md

### Even with `make lint` green, parser-name string literals duplicated across files trigger `dupword`-class warnings
Plan 03 had to introduce a package-local `stdlibParserName` constant (used by both `stdlibParser.Name()` and the parser tests) to keep parser-name assertions lint-clean.

**Context:** A small but real symptom of the dupword/gocritic linter set on a refactor that intentionally pins string identities in tests. The fix is a private constant, not a public API change.
**Source:** 13-parser-seam-extraction-03-SUMMARY.md

---

## Patterns

### Parser seam pattern (parser owns traversal, sink owns staging, builder owns classification)
`Parser` owns JSON traversal; `parserSink` owns staging into builder maps; the numeric int64 classifier stays in `builder.go`. Three responsibilities, three layers, one direction of dependency.

**When to use:** Any future seam extraction in this codebase where (a) a hot path needs an extension point, (b) the legacy walker is intertwined with classifier logic that should stay put, and (c) the new contract should not leak builder internals to extension authors.
**Source:** 13-parser-seam-extraction-01-SUMMARY.md

### Compatibility-move pattern: extract into a dedicated file before switching the hot path
Move the existing walker logic verbatim into `parser_stdlib.go` as wave 1; switch the hot path in wave 2; assert parity in wave 3. Each wave is independently green.

**When to use:** Whenever a refactor swaps a stable subsystem behind a new interface — splitting "move" from "switch" turns one risky diff into two reviewable ones and lets parity tests target the move boundary.
**Source:** 13-parser-seam-extraction-01-SUMMARY.md

### Guarded dispatch pattern: trusted fast path + custom slow path with full validation
The default trusted parser uses a concrete fast helper; custom parser implementations go through the full sink-contract path with `BeginDocument` mismatch guards and parser-error verbatim propagation.

**When to use:** Any extension point where the in-tree default is performance-critical but third-party implementations need defensive runtime checks. Pay the validation cost only on the extension path.
**Source:** 13-parser-seam-extraction-02-SUMMARY.md, 13-DISCUSSION-LOG.md

### Parity artifact pattern: shared untagged fixture file + build-tagged regenerator + committed goldens
Untagged `parser_parity_fixtures_test.go` defines the corpus shared between two consumers; `parity_goldens_test.go` (build-tagged `regenerate_goldens`) writes the committed `testdata/parity-golden/*.bin` blobs; the always-on harness asserts byte equality.

**When to use:** Whenever a refactor must prove byte-level parity against a frozen reference. The build-tag isolates the regenerator from normal `go test` runs while keeping it co-located and discoverable.
**Source:** 13-parser-seam-extraction-02-SUMMARY.md

### Merge-gate pattern: always-on golden parity + gopter determinism canary + Evaluate operator matrix
Three layers of always-on assertions: byte-level goldens (authored fixtures), property-based determinism across runs, and a 12-operator Evaluate matrix exercising query semantics through the seam.

**When to use:** When a behavior-preserving refactor needs both byte-level and semantic-level proof. The three layers catch different failure modes (encoding drift, nondeterminism, query-path skew) and stay cheap enough to run on every CI.
**Source:** 13-parser-seam-extraction-03-SUMMARY.md

### Benchmark risk-acceptance pattern: preserve the failing artifact, document the residual risk, reconcile state after human review
When a perf gate sits at the noise floor and reopening the implementation would not improve correctness, capture the failing artifact in `<phase>-BENCH.md`, record acceptance in `<phase>-SECURITY.md`, and reconcile `ROADMAP.md` / `STATE.md` after the human review checkpoint.

**When to use:** When (a) allocs/op stays flat, (b) parity/correctness gates are green, (c) the wall-clock threshold is within machine noise, and (d) implementation rework would not be load-bearing. Provides an auditable exception path that does not silently weaken the gate.
**Source:** 13-parser-seam-extraction-03-SUMMARY.md, 13-03-BENCH.md, 13-VALIDATION.md

---

## Surprises

### Plan-02 special case (trusted stdlib fast path) was not anticipated in PLAN.md
The initial plan had a single seam-dispatch path. The benchmark gate forced introducing a second concrete path mid-execution.

**Impact:** Auto-fixed deviation; no scope expansion but the seam now has two execution shapes (trusted and validated) instead of one — worth knowing for Phase 14 telemetry, which must instrument both.
**Source:** 13-parser-seam-extraction-02-SUMMARY.md

### Broad parent benchmarks were unusable as a merge gate on this machine
The default multi-threaded benchmark runs proved too noisy to gate on; the team had to switch to `GOMAXPROCS=1` focused subbenchmarks mid-phase.

**Impact:** Methodology shift recorded in `13-02-BENCH.md`. Phase 14 (and any future perf-gated phase) should default to focused single-threaded probes rather than broad runs.
**Source:** 13-parser-seam-extraction-02-SUMMARY.md, 13-03-BENCH.md

### Transformer-heavy probe stayed `+2.14%`/`+2.19%` over baseline despite allocations being flat
With identical `B/op` and `allocs/op`, two of five gated benchmarks still failed the `<= +2%` ns/op threshold.

**Impact:** Forced introducing a documented "accepted residual risk" disposition rather than auto-finalizing the phase. Establishes a precedent (and a pattern) for handling perf-gate noise floor cases on future phases.
**Source:** 13-03-BENCH.md, 13-parser-seam-extraction-03-SUMMARY.md

### A stale `.git/index.lock` from a parallel commit attempt
During plan-01 execution, an accidental parallel commit attempt left a `.git/index.lock` behind that had to be removed manually before further git operations.

**Impact:** No content lost, but a reminder that parallel commit flows in the GSD execution path can race the git index. The fix was manual lock removal; the underlying coordination issue may be worth addressing in tooling.
**Source:** 13-parser-seam-extraction-01-SUMMARY.md
