---
quick_id: 260422-lv1
description: Address PR #29 review feedback items 1, 2, 3, 4, 7
mode: quick
status: complete
created: 2026-04-22
completed: 2026-04-22
commits:
  - b0ea5f1
  - 778efcb
  - ac47988
  - 8679f64
---

# Quick Task 260422-lv1 Summary

## Outcome

All five targeted PR #29 review findings landed as four atomic commits on the
`gsd/phase-14-observability-seams` branch. Full test suite green, lint clean,
`go mod tidy -diff` empty.

## What shipped

### 1. Predicate-op vocabulary fix (commit `b0ea5f1`)

`query.go:243-249` — `adaptiveInvariantAllRGs` now emits
`AttrOperation(telemetry.OperationEvaluate)` + `AttrPredicateOp(op)` instead
of overloading the `operation` key with a predicate operator string. This
aligns the warn-level emission with the frozen-vocabulary contract (INFO-level
`operation` reserved for coarse boundary names, `predicate_op` reserved for
operator kinds).

### 2. ErrorTypeOther promotion + go mod tidy (commit `778efcb`)

- New exported constant `telemetry.ErrorTypeOther = "other"` in
  `telemetry/attrs.go`, alongside the existing `Operation*` constants.
- Removed duplicate `normalizedErrorTypeOther` declarations in `parquet.go:19`
  and `telemetry/boundary.go:13`.
- Updated references in `parquet.go`, `serialize.go`, and `telemetry/boundary.go`
  to use the exported constant.
- `go mod tidy` moved the three OTel API modules to the direct `require`
  block and dropped `go-logr/logr`, `go-logr/stdr`, and
  `go.opentelemetry.io/auto/sdk` as unused indirects.

Drift test `TestErrorTypeNormalizersStayInSync` continues to pass, so the
existing guard against vocabulary drift still protects the single-source-of-truth
invariant.

### 3. Allocs test rename (commit `ac47988`)

- `TestEvaluateDisabledLoggingAllocsZero` → `TestEvaluateDisabledLoggingAllocsAtMostOne`.
- Docstring rewritten to describe the `≤1 alloc` tolerance and the reason
  (scheduler/jitter noise on some Go runtimes).
- Failure message updated to match.
- Cross-file references in `observability_policy_test.go` (two comments)
  and `benchmark_test.go` (one comment on `BenchmarkEvaluateDisabledLogging`)
  renamed in lockstep.

### 4. Parser-name info-leak test wiring (commit `8679f64`)

- `TestBuildFromParquetContextEmitsParserNameWithoutInfoLeak` now wires an
  `infoAttrRecorder` (level-aware capturing logger) via `WithLogger` and runs
  `BuildFromParquetContext` against the same nonexistent file.
- Assertion changed from "function doesn't panic" to the negative constraint
  implied by the test name: any captured INFO-level attrs must stay inside
  the frozen allowlist and must not leak parser identity in key or value.
- Empty captures remain acceptable — the parquet boundary currently emits
  observability via OTel spans/metrics rather than INFO logs, so the guard
  only fires if a future instrumentation change adds INFO emissions that
  leak parser info.

## Items deliberately deferred

- **Item 5 (Claude review):** `Info/Warn/Error` helpers skipping `Enabled()` —
  design choice, documented-behavior nit only. Not a bug.
- **Item 6 (Claude review):** `classifyParquetError` string heuristics — already
  sentinel-first for the most common case (`os.ErrNotExist`), no real failure
  observed. Revisit if parquet-go error strings change or misclassification
  is seen in the wild.
- **Items 8, 9 (Claude review):** `slogadapter.Enabled()` allocation (Go
  singleton, no allocation) and `NewSignals(nil,nil,nil)` returning
  `Enabled()=true` (intentional, documented in `telemetry/telemetry.go:32-35`).

## Verification

- `go build ./...` — clean.
- `go vet ./...` — clean.
- `golangci-lint run` — 0 issues.
- `go test ./... -count=1` — all packages pass.
- `go mod tidy -diff` — empty.
- Targeted regression tests pass:
  - `TestInfoLevelEmissionsUseOnlyAllowlistedAttrs`
  - `TestErrorTypeNormalizersStayInSync`
  - `TestClassifyParquetError`
  - `TestClassifySerializeError`
  - `TestEvaluateDisabledLoggingAllocsAtMostOne`
  - `TestBuildFromParquetContextEmitsParserNameWithoutInfoLeak`

## Commits

| SHA | Scope | Title |
|-----|-------|-------|
| `b0ea5f1` | observability | fix(observability): emit predicate_op attr for adaptive invariant violation |
| `778efcb` | telemetry | refactor(telemetry): promote ErrorTypeOther and tidy module graph |
| `ac47988` | test | test(observability): rename allocs test to match +1 tolerance |
| `8679f64` | test | test(observability): verify parser-name info-leak constraint with capturing logger |

## Key links

- PR: https://github.com/amikos-tech/ami-gin/pull/29
- Original review: https://github.com/amikos-tech/ami-gin/pull/29#issuecomment-4296220687
- Plan: `.planning/quick/260422-lv1-address-pr-29-feedback-fix-attr-vocab-ti/260422-lv1-PLAN.md`
