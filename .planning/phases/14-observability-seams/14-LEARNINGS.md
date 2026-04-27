---
phase: 14
phase_name: "Observability Seams"
project: "GIN Index"
generated: "2026-04-22"
counts:
  decisions: 7
  lessons: 5
  patterns: 7
  surprises: 5
missing_artifacts: []
---

# Phase 14 Learnings: Observability Seams

## Decisions

### Freeze INFO-level metadata behind a typed `Attr` surface
The phase introduced a repo-owned `logging.Attr{Key, Value}` contract plus a single frozen allowlist for INFO-level keys instead of using variadic `...any` logging payloads.

**Rationale:** The typed surface enforces the frozen vocabulary at the API boundary and makes it much harder for future callers to leak raw predicate values, path names, doc IDs, or row-group IDs into logs.
**Source:** 14-01-SUMMARY.md, 14-04-SUMMARY.md, 14-VERIFICATION.md

---

### Give `Signals.Enabled()` explicit semantics
`telemetry.Signals.Enabled()` is backed by an explicit `enabled` flag that is set by `NewSignals(...)`; the zero value and `Disabled()` both report false.

**Rationale:** The phase deliberately avoided guessing enabled/disabled state from provider types or nil checks so the runtime semantics stay deterministic across default, decoded, and explicitly constructed paths.
**Source:** 14-01-SUMMARY.md

---

### Normalize observability on both fresh and decoded configs
The same `normalizeObservability` helper is called from `DefaultConfig()` and from `readConfig(...)` after decode reconstruction.

**Rationale:** Logger and signals are runtime-only fields, not serialized payload data. Fresh configs and decoded configs therefore need the same silent-default restoration step before any boundary code reads them.
**Source:** 14-01-SUMMARY.md, 14-04-SUMMARY.md, 14-VERIFICATION.md

---

### Add context-aware APIs as siblings, not replacements
`EvaluateContext`, `BuildFromParquetContext`, `BuildFromParquetReaderContext`, `EncodeContext`, `EncodeWithLevelContext`, and `DecodeContext` were added while the legacy entry points stayed as wrappers over `context.Background()`.

**Rationale:** The phase needed cancellation and observability hooks without breaking existing callers or widening the public migration surface more than necessary.
**Source:** 14-02-SUMMARY.md, 14-03-SUMMARY.md, STATE.md, 14-UAT.md

---

### Instrument query evaluation with one direct coarse span
`EvaluateContext` uses a single coarse tracing boundary around the full query evaluation and manages the span lifecycle directly.

**Rationale:** Query evaluation is fail-open and returns `*RGSet`, not `error`, so `RunBoundaryOperation` was the wrong abstraction. Direct `Start`/`End` kept the instrumentation boundary-only without forcing the query API into an error-shaped helper.
**Source:** 14-02-PLAN.md, 14-02-SUMMARY.md

---

### Route adaptive invariant violations through the new logger seam as warnings
The old `adaptiveInvariantLogger *log.Logger` state was removed and invariant failures now emit through the config-carried repo-owned logger seam with `logging.Warn(...)`.

**Rationale:** The invariant condition is a fail-open fallback, not a hard failure. Warning-level emission preserves the existing safety behavior (`AllRGs()`) while collapsing the codebase to one logging convention.
**Source:** 14-02-SUMMARY.md, 14-VERIFICATION.md

---

### Use runtime option shims for raw serialization observability
Raw encode/decode gained `EncodeOption` and `DecodeOption` shims instead of positional logger/provider arguments or new package-global hooks.

**Rationale:** Serialization needed context-aware observability injection even though decode has no config receiver. Runtime option shims preserve the additive boundary shape and avoid reintroducing a second observability convention.
**Source:** 14-03-SUMMARY.md

---

## Lessons

### Sub-percent perf gates need normalized measurement, not small-sample timing
The strict tracer budget test was too noisy with only 7 samples and produced a misleading ~10% overhead signal even though the compared paths were semantically the same disabled/noop case.

**Context:** The phase had to move to 51 samples, add warmup, and force `GOMAXPROCS(1)` before the 0.5% gate became decision-quality. Tiny wall-clock budgets are not trustworthy on ad-hoc runs.
**Source:** 14-02-SUMMARY.md, 14-UAT.md

---

### Cross-boundary vocabularies should be frozen before downstream plans consume them
The query migration needed `telemetry.OperationEvaluate`, but the foundation plan had not yet created a shared operation-name source file.

**Context:** That omission forced Plan 14-02 to create `telemetry/attrs.go` mid-phase. The lesson is to freeze reusable vocab sources early when multiple later plans will consume them.
**Source:** 14-02-SUMMARY.md

---

### Invariant-path tests must model the real lookup preconditions
The first version of the adaptive-invariant test helper patched the root `$` path instead of `$.field` and also indexed the wrong term for the bloom/string-length gates.

**Context:** The result was a false negative test setup that never reached the intended adaptive lookup path. For hot-path tests, getting into the target branch matters as much as the final assertion.
**Source:** 14-02-SUMMARY.md

---

### Decoded indexes need observability defaults reinstalled, not merely preserved in theory
Phase 14's end-to-end tests proved that finalized and decoded indexes stay safe only because decode explicitly restores a non-nil logger and disabled signals.

**Context:** Runtime observability implementations are intentionally omitted from serialized config payloads. Without post-decode restoration, decode would have reopened nil/silent-safety bugs.
**Source:** 14-01-SUMMARY.md, 14-04-SUMMARY.md, 14-VERIFICATION.md

---

### Regression-guard plans can be "green on arrival" and still be essential
The final policy plan landed with tests that already passed because Plans 14-01 through 14-03 had implemented the right behavior.

**Context:** For regression guards, the RED condition is future policy drift, not today's absence of code. A guard-only plan can therefore add durable value without needing any production diff.
**Source:** 14-04-SUMMARY.md

---

## Patterns

### Additive context-aware sibling pattern
Add a `...Context` sibling for a public boundary, keep the old API as a wrapper over `context.Background()`, and prove parity with explicit compatibility tests.

**When to use:** Any mature public API that needs cancellation, tracing, or logging context without forcing a breaking change across all callers at once.
**Source:** 14-02-SUMMARY.md, 14-03-SUMMARY.md, 14-UAT.md

---

### Shared normalization for runtime-only config fields
Use one normalization helper to install safe defaults for non-serialized runtime fields both at object construction time and after decode.

**When to use:** Any config that carries runtime objects such as loggers, providers, clients, or hooks that must not be serialized but still must be valid after reconstruction.
**Source:** 14-01-SUMMARY.md, 14-VERIFICATION.md

---

### Boundary-only observability on hot paths
Instrument only the coarse operation boundary and keep spans/logging out of predicate loops, row-group loops, page loops, and similar inner hot paths.

**When to use:** Performance-sensitive library code where visibility matters, but loop-level observability would distort the very costs being measured.
**Source:** 14-02-PLAN.md, 14-03-PLAN.md, 14-VERIFICATION.md

---

### Scoped context-in-struct exception for private readers
Allow a private, short-lived adapter struct to carry parent context when that is the narrowest way to compose caller cancellation into an interface like `io.ReaderAt`.

**When to use:** Private boundary adapters that exist for one operation only, are never reused across requests, and would otherwise have to drop caller cancellation back to `context.Background()`.
**Source:** 14-03-SUMMARY.md

---

### Runtime option-shim pattern for encode/decode
Use small runtime option functions such as `EncodeOption` and `DecodeOption` to seed observability or execution behavior into context-aware helpers.

**When to use:** Stateless top-level helpers that need optional runtime behavior but should not grow positional backend parameters or new global state.
**Source:** 14-03-SUMMARY.md

---

### Executable policy-guard pattern
Enforce observability rules with real emitted-attr capture, AST/API inspection, and dependency-file checks instead of relying on docs or manual review.

**When to use:** Constraints like frozen vocabularies, "no legacy surface," or "no backend-specific dependency leakage" that must keep holding after future refactors.
**Source:** 14-04-SUMMARY.md, 14-VERIFICATION.md

---

### Nil-collapse adapters to noop
Make adapter constructors accept nil safely and collapse to noop behavior rather than panicking or forcing callers to pre-guard every wiring site.

**When to use:** Optional observability adapters or pluggable backends where the default behavior should remain silent and safe when the caller does not supply a concrete implementation.
**Source:** 14-01-SUMMARY.md, 14-VERIFICATION.md

---

## Surprises

### A 7-sample strict perf gate exaggerated tracer overhead
The first strict perf run reported roughly 10% overhead on the tracer path even though both compared paths were effectively disabled/noop.

**Impact:** The phase had to harden the methodology before trusting the budget result. Small-sample nanosecond timings were not stable enough for a 0.5% gate.
**Source:** 14-02-SUMMARY.md

---

### The first adaptive-invariant test helper was wrong in two separate ways
It modified the root path instead of the field path and indexed a term that got filtered before the adaptive lookup branch.

**Impact:** All invariant tests initially returned `count=0`, which looked like a production regression until the fixture setup was corrected.
**Source:** 14-02-SUMMARY.md

---

### Frozen operation names were missing from the foundation plan
The need for shared operation constants only became obvious once the query boundary tried to consume them.

**Impact:** Plan 14-02 had to introduce `telemetry/attrs.go` as an unplanned but necessary vocabulary file, and that file then became the reusable source for later encode/decode/build boundaries.
**Source:** 14-02-SUMMARY.md

---

### The roadmap named the adapter subpackages under `telemetry`, but the correct home was `logging`
Verification had to explicitly call out that the roadmap SC path names were off even though the architectural intent was satisfied.

**Impact:** The phase avoided a false audit failure by documenting that `logging/slogadapter` and `logging/stdadapter` are the right locations for adapters over the `logging.Logger` contract.
**Source:** 14-VERIFICATION.md

---

### The final guard-rail plan needed no production fixes
Plan 14-04 added the policy and regression test surface in a single commit without changing library behavior.

**Impact:** That is a strong signal that the first three plans had already converged on the intended design; the last plan hardened the boundary rather than repairing it.
**Source:** 14-04-SUMMARY.md
