# Phase 18: Structured IngestError + CLI integration - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in `18-CONTEXT.md`; this log preserves the alternatives considered.

**Date:** 2026-04-24
**Phase:** 18-structured-ingesterror-cli-integration
**Areas discussed:** Public IngestError API shape, Layer/path/value semantics, CLI failure reporting, Enforcement and test matrix, Scope boundary with soft mode

---

## Public IngestError API Shape

| Option | Description | Selected |
|--------|-------------|----------|
| Concrete struct with `Err` + `Unwrap()` + `Cause()` | Public fields `Path`, `Layer`, `Value`, `Err`; supports `errors.As`, `errors.Unwrap`, and `pkg/errors.Cause`. | Yes |
| Concrete struct with `Cause error`, no `Cause()` method | Exact roadmap field name, but weaker compatibility with existing repo convention. |  |
| Concrete struct with private fields and getters | More API control, less convenient for callers and tests. |  |
| Struct plus layer sentinel errors | Adds `errors.Is` classification but expands public API and chain complexity. |  |
| Other | Free-form alternative. |  |

**User's choice:** Option 1: concrete `*IngestError` with `Path`, `Layer`, `Value`, `Err`, plus `Error()`, `Unwrap()`, and `Cause()`.
**Notes:** The `Err` field is documented as the underlying cause because Go cannot have both a `Cause` field and `Cause()` method on the same type.

---

## Layer/Path/Value Semantics

| Option | Description | Selected |
|--------|-------------|----------|
| Best-known user location | Canonical path when known, empty when unknown, `$` only for known root failures; parser contract bugs stay non-`IngestError`. | Yes |
| Always-populated path | Use `$` when unknown; simpler but can imply false precision. |  |
| Internal operation path | Expose derived/internal target paths and parser internals. |  |
| Other | Free-form alternative. |  |

**User's choice:** Option 1: best-known user location.
**Notes:** This keeps parser unknown-location failures honest and avoids leaking representation internals for transformer failures.

---

## CLI Failure Reporting

| Option | Description | Selected |
|--------|-------------|----------|
| Layer+code groups with 3 samples per group | Strong diagnostics, but introduces a stable failure-code taxonomy. |  |
| Layer-only groups with 5 samples per group | Simpler grouping, but initially proposed with a larger sample cap. |  |
| Flat `errors[]` plus count map by layer | Preserves chronological order but splits grouping across fields. |  |
| NDJSON event stream | Useful for huge streams but changes the output contract. |  |
| Layer-only groups with 3 structured samples per layer | Balanced choice: no failure-code enum, structured samples, bounded output. | Yes |

**User's choice:** Lock the balanced hybrid: layer-only failure groups with 3 structured samples per layer, no failure-code enum.
**Notes:** User explicitly wanted good debug value with minimal API surface.

---

## Enforcement And Test Matrix

| Option | Description | Selected |
|--------|-------------|----------|
| Scoped guard + table-driven tests | Guard ingest surfaces only; add per-layer `errors.As` and CLI grouped-count tests. | Yes |
| AST-based Go test guard + table-driven tests | More precise but more implementation code. |  |
| Behavior-only tests, no static guard | Minimal tooling but can miss untested plain error sites. |  |
| Broad repo grep | Aggressive and likely noisy because other subsystems legitimately use plain errors. |  |
| Other | Free-form alternative. |  |

**User's choice:** Option 1: scoped guard plus table-driven tests.
**Notes:** The guard should allow tragic/internal/parser-contract errors to remain outside `IngestError`.

---

## Scope Boundary With Soft Mode

| Option | Description | Selected |
|--------|-------------|----------|
| Hard-error-only | `IngestError` is for returned hard failures; soft mode keeps counters/logs and nil return behavior. | Yes |
| Add soft-skip counts to CLI summary only | More visible but adds CLI surface. |  |
| Add a library soft-skip observer/sampled event API | More auditability, larger public API. |  |
| Return `IngestError` for soft skips too | Unified pipeline but breaks Phase 17 nil-on-soft semantics. |  |
| Other | Free-form alternative. |  |

**User's choice:** Option 1: hard-error-only.
**Notes:** This preserves Phase 17 behavior and keeps the API focused on returned hard document failures.

## the agent's Discretion

- Exact `IngestError.Error()` wording.
- Exact helper names and file organization.
- Exact JSON nesting for CLI failure groups within the locked layer-only, 3-sample-cap contract.
- Exact scoped guard implementation mechanism.

## Deferred Ideas

- Failure-code enum.
- Layer sentinel errors for `errors.Is`.
- Soft-skip observer API or CLI soft-skip samples.
- `ValidateDocument` dry-run API.
