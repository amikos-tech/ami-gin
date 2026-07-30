---
phase: quick-260729-mg8
plan: 01
subsystem: parser
tags: [simd, pure-simdjson, numeric-policy, ci]

# Dependency graph
requires:
  - phase: 21-simd-parser-adapter
    provides: simdParser adapter (walkElement/materializeElement type switches) and IngestFailureMode numeric policy routing
provides:
  - TypeBigInt handling in the pure-simdjson adapter, matching stdlib numeric-failure-policy parity for integers outside int64/uint64 range
  - SIMD CI job native-library bootstrap pin corrected to v0.1.7 (matching the go.mod-pinned Go bindings version)
affects: [phase-22-simd-validation-benchmarks-ci]

# Tech tracking
tech-stack:
  added: []
  patterns: []

key-files:
  created: []
  modified: [parser_simd.go, .github/workflows/ci.yml]

key-decisions:
  - "TypeBigInt in walkElement routes through sink.StageJSONNumber (same numeric literal staging path as stdlib), not a new ad hoc big-integer code path"
  - "TypeBigInt in materializeElement wraps element.GetBigInt() as json.Number, mirroring the existing TypeInt64/TypeUint64/TypeFloat64 cases for the transform-buffering path"

patterns-established: []

requirements-completed: [SIMD-04, SIMD-05, SIMD-06, SIMD-07]

# Metrics
duration: 15min
completed: 2026-07-29
---

# Quick Task 260729-mg8: Fix SIMD Parser Adapter TypeBigInt Handling Summary

**Added `TypeBigInt` cases to both `parser_simd.go` element switches and bumped the SIMD CI native-library bootstrap pin from v0.1.4 to v0.1.7 to match the already-bumped Go bindings, restoring numeric-failure-policy parity between the SIMD and stdlib parsers for out-of-range integers.**

## Performance

- **Duration:** ~15 min
- **Completed:** 2026-07-29
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- `walkElement` now has a `case purejson.TypeBigInt:` that reads the raw decimal text via `element.GetBigInt()` and routes it through `sink.StageJSONNumber`, so out-of-range integers hit the same numeric failure policy as stdlib instead of the fail-closed `default:` arm.
- `materializeElement` now has a matching `case purejson.TypeBigInt:` that wraps `element.GetBigInt()` as `json.Number`, mirroring the existing `TypeInt64`/`TypeUint64`/`TypeFloat64` cases for the transform-buffering path.
- `.github/workflows/ci.yml` SIMD job now bootstraps `pure-simdjson-bootstrap@v0.1.7`, matching the `github.com/amikos-tech/pure-simdjson v0.1.7` entry in `go.mod` (bumped by dependabot PR #48).
- Confirmed locally (not just via `go build`): bootstrapped the real native library and ran `TestSIMDParserNativeNumericFailuresUseNumericPolicy` — all subtests pass, including all 4 sub-modes (`hard-numeric-soft-parser`, `soft-numeric-hard-parser`, `hard-numeric-hard-parser`, `soft-numeric-soft-parser`) of the `larger-than-uint64` case that exercises the new `TypeBigInt` path.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add TypeBigInt handling to walkElement and materializeElement** - `52ded55` (fix)
2. **Task 2: Bump SIMD CI native-library bootstrap pin to v0.1.7** - `8a48b15` (ci)

_No plan-metadata commit yet — orchestrator handles the docs commit for STATE.md/SUMMARY.md per constraints._

## Files Created/Modified
- `parser_simd.go` - Added `case purejson.TypeBigInt:` in `walkElement` (routes to `StageJSONNumber`) and in `materializeElement` (wraps as `json.Number`), each immediately before the existing `case purejson.TypeInvalid:` line
- `.github/workflows/ci.yml` - Bumped SIMD job's native-library bootstrap invocation from `@v0.1.4` to `@v0.1.7` (single line changed)

## Decisions Made
- Followed the plan's interface guidance exactly: `StageJSONNumber(state, canonicalPath, raw)` for the walk path, `json.Number(raw)` for the materialize path — no new abstractions introduced.
- Left both `default:` arms untouched, preserving the fail-closed catch-all for any future unhandled `pure-simdjson` element kinds.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None. Local verification went further than the plan's "may not be runnable locally" fallback: the native pure-simdjson v0.1.7 binary bootstrapped successfully in this environment (via `PURE_SIMDJSON_BINARY_MIRROR`/`PURE_SIMDJSON_CACHE_DIR` against the scratchpad cache dir), so `TestSIMDParserNativeNumericFailuresUseNumericPolicy` was run directly rather than deferred to CI.

## Verification Performed

1. `go build -tags simdjson ./...` — passed (proves `purejson.TypeBigInt` and `element.GetBigInt()` resolve against v0.1.7).
2. `make simd-isolation-check` — passed.
3. Native library bootstrap: `go run github.com/amikos-tech/pure-simdjson/cmd/pure-simdjson-bootstrap@v0.1.7 fetch` — succeeded, fetched the current-platform (darwin/arm64) artifact into the scratchpad cache dir.
4. `go test -tags simdjson -v -run TestSIMDParserNativeNumericFailuresUseNumericPolicy .` — all subtests PASS, including all 4 sub-modes of `larger-than-uint64`.
5. `go test -tags simdjson ./...` — full suite passed with the native library present.
6. `go build ./...` (no simdjson tag) — passed, confirming the change doesn't affect the default (non-SIMD) build.
7. `grep -n "pure-simdjson-bootstrap@v0.1.7" .github/workflows/ci.yml` — confirms the CI pin bump.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- PR #48 (dependabot gomod-minor-and-patch bump, including pure-simdjson v0.1.4→v0.1.7) should now pass its "SIMD parser" CI job once pushed, since both the compile-time gap (`TypeBigInt`) and the CI native-library version pin are fixed.
- No blockers for Phase 22 (SIMD Validation, Benchmarks & CI) — this fix removes a regression introduced by the dependency bump rather than adding new scope.

---
*Quick task: 260729-mg8*
*Completed: 2026-07-29*

## Self-Check: PASSED

- FOUND: parser_simd.go
- FOUND: .github/workflows/ci.yml
- FOUND: SUMMARY.md
- FOUND: commit 52ded55
- FOUND: commit 8a48b15
- FOUND: 2x `case purejson.TypeBigInt:` in parser_simd.go (walkElement + materializeElement)
- FOUND: `pure-simdjson-bootstrap@v0.1.7` pin in .github/workflows/ci.yml
