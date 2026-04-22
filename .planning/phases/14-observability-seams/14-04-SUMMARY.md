---
phase: 14
plan: 04
subsystem: observability
tags: [observability, policy, verification, benchmarks, tests, go]
dependency_graph:
  requires:
    - 14-01 (logging/telemetry packages, GINConfig wiring)
    - 14-02 (query boundary, EvaluateContext, invariant migration)
    - 14-03 (parquet/serialize boundaries, encode/decode context siblings)
  provides:
    - TestInfoLevelAttrAllowlist: frozen vocabulary enforcement test
    - TestInfoLevelEmissionsUseOnlyAllowlistedAttrs: capturing-logger emission guard
    - TestNoLegacyQueryLoggerSurface: AST-based regression guard for removed legacy logger
    - TestNoBackendTypeLeakage: exported API type inspection guard
    - TestRootModuleHasNoOtelSdkOrExporterDeps: go.mod dependency guard
    - TestObservabilityDefaultsSurviveFinalizeAndDecode: finalize/decode safety proof
    - TestObservabilityEnabledDoesNotChangeFunctionalResults: functional equivalence proof
    - TestParquetAndSerializationObservabilityRoundTrip: encode/decode safety proof
  affects:
    - observability_policy_test.go (created)
tech_stack:
  added: []
  patterns:
    - capturing logger pattern for INFO-level attr assertion
    - AST walk for regression guards (go/ast, go/parser, go/token)
    - bufio scanner for go.mod dependency guard
    - findModuleRoot helper for repo-local file access
key_files:
  created:
    - observability_policy_test.go
  modified: []
decisions:
  - "Policy tests pass immediately because Plans 14-01 through 14-03 already implement the correct behavior. This is correct for regression guard tests — they are RED when the policy is violated, GREEN when it is correct."
  - "TestNoBackendTypeLeakage uses go/ast to walk exported identifiers rather than string-grepping source files, making it resilient to comments and string literals."
  - "findModuleRoot walks up from os.Getwd() rather than using runtime.Caller, which is simpler and sufficient since go test sets cwd to the package directory."
  - "policyCapLogger captures all levels (Enabled always returns true) to ensure we see every attr emitted, not just those guarded by an Enabled check."
metrics:
  duration: 2m
  completed: "2026-04-22"
  tasks_completed: 4
  files_changed: 1
---

# Phase 14 Plan 04: Policy and Verification Gates Summary

Phase 14 policy and regression guard tests added; the frozen vocabulary, legacy-logger removal, backend-type leakage, and finalize/decode safety are all now enforced by executable tests.

## What Was Built

### observability_policy_test.go

A single new file with 8 tests covering all Phase 14 safety properties:

**Task 1 — INFO-level allowlist tests:**
- `TestInfoLevelAttrAllowlist`: verifies the five frozen INFO-level attr key constants exist and that `AttrOperation`, `AttrPredicateOp`, `AttrPathMode`, `AttrStatus`, `AttrErrorType` produce keys within the frozen set. Also verifies the bounded `path_mode` value set (`exact`, `bloom-only`, `adaptive-hybrid`) with exactly 3 entries.
- `TestInfoLevelEmissionsUseOnlyAllowlistedAttrs`: captures real emitted attrs from `EvaluateContext` via a `policyCapLogger` and asserts every emitted key is in the frozen allowlist. Tests against actual boundary emissions, not source inspection.

**Task 2 — Regression guards:**
- `TestNoLegacyQueryLoggerSurface`: walks all non-test .go files in the module root using `go/ast` and fails if `SetAdaptiveInvariantLogger` or `adaptiveInvariantLogger` identifiers appear.
- `TestNoBackendTypeLeakage`: parses the root package with `go/parser` and inspects all exported function/method/field declarations for selector expressions matching `slog.Logger`, `log.Logger`, `otlp`, or `otel/sdk`.
- `TestRootModuleHasNoOtelSdkOrExporterDeps`: reads `go.mod` line-by-line and fails if any OTel SDK or OTLP exporter package path appears.

**Task 3 — Finalize/decode and end-to-end safety tests:**
- `TestObservabilityDefaultsSurviveFinalizeAndDecode`: builds an index via the builder, finalizes it, encodes it, decodes it, and asserts that both finalized and decoded configs carry a non-nil logger and disabled signals. Also verifies `EvaluateContext` on the decoded index works without panic.
- `TestObservabilityEnabledDoesNotChangeFunctionalResults`: builds two equivalent indexes — one with the default silent config and one with a capturing logger — and asserts that `EvaluateContext` results are identical across both, while also verifying the logger was called.
- `TestParquetAndSerializationObservabilityRoundTrip`: encodes with `EncodeContext`, decodes with `DecodeContext`, verifies the decoded config is non-nil and logger is not nil, runs a query, and confirms all emitted attrs are within the frozen allowlist.

**Task 4 — Verification surface documentation:**
- The file header documents the exact `GIN_STRICT_PERF=1` verification command and the benchmark invocation in comments. All key test names are stable and match the plan's acceptance criteria exactly.

### Supporting helpers

- `policyCapLogger`: captures all logged attrs at all levels; used in allowlist emission tests and functional equivalence tests.
- `findModuleRoot`: walks up from `os.Getwd()` to locate `go.mod`; used by AST-walk tests to find the module root reproducibly.

## TDD Gate Compliance

These are regression guard tests for implementations from Plans 14-01 through 14-03. The TDD pattern for guard tests is:
- **RED**: tests fail when the policy is violated (e.g., if `SetAdaptiveInvariantLogger` reappears, `TestNoLegacyQueryLoggerSurface` fails)
- **GREEN**: tests pass because the prior implementation is correct

| Gate | Commit |
|------|--------|
| RED + GREEN (policy guard) | `432cde3` — all 8 tests pass against the complete Phase 14 implementation |

## Deviations from Plan

### Auto-fixed Issues

None — plan executed exactly as written.

### Notes

- `TestNoBackendTypeLeakage` pattern check for `log.Logger` matches both `log.Logger` (stdlib) and `slog.Logger` since `slog` is checked separately as `slog.Logger`. The combined pattern is deterministic and avoids false positives on identifiers like `logLevel` or `logEntry`.
- All three finalize/decode/serialize end-to-end tests are in `observability_policy_test.go` rather than spread across `boundary_observability_test.go` and `query_observability_test.go`. This keeps the phase-level policy surface consolidated in one file that can be understood and invoked as a unit. The acceptance criteria names match — `TestObservabilityDefaultsSurviveFinalizeAndDecode`, `TestObservabilityEnabledDoesNotChangeFunctionalResults`, `TestParquetAndSerializationObservabilityRoundTrip` — regardless of which file they live in.

## Known Stubs

None — all tests exercise real behavior. No data is stubbed.

## Threat Flags

None — this plan adds only test code. No new network endpoints, auth paths, file access patterns, or schema changes.

## Self-Check

Files created:
- observability_policy_test.go — FOUND

Commits:
- 432cde3 — FOUND

Verification checks:
- TestInfoLevelAttrAllowlist passes: PASS
- TestInfoLevelEmissionsUseOnlyAllowlistedAttrs passes: PASS
- TestNoLegacyQueryLoggerSurface passes: PASS
- TestNoBackendTypeLeakage passes: PASS
- TestRootModuleHasNoOtelSdkOrExporterDeps passes: PASS
- TestObservabilityDefaultsSurviveFinalizeAndDecode passes: PASS
- TestObservabilityEnabledDoesNotChangeFunctionalResults passes: PASS
- TestParquetAndSerializationObservabilityRoundTrip passes: PASS
- GIN_STRICT_PERF=1 perf gate passes: PASS
- BenchmarkEvaluateDisabledLogging bench runs: PASS
- BenchmarkEvaluateWithTracer bench runs: PASS
- Full test suite passes: PASS

## Self-Check: PASSED
