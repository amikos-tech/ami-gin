---
phase: 21-simd-parser-adapter
plan: 05
subsystem: parser
tags: [go, simdjson, lifecycle, panic-recovery, atomicity]

requires:
  - phase: 21-04
    provides: Fatal parser-lifecycle routing, builder poisoning, and multi-cause cleanup errors
provides:
  - Panic-aware exactly-once SIMD document cleanup
  - Terminal failed-close routing for concurrent walk panics
  - Native-free caller-recovery and retry-dispatch regression coverage
affects: [22-simd-validation-benchmarks-ci]

tech-stack:
  added: []
  patterns:
    - Recover a walk panic before mandatory cleanup, then preserve or convert it according to cleanup outcome
    - Keep error-valued panic causes directly discoverable through multi-cause lifecycle errors
    - Prove terminal builder state by attempting a second AddDocument before assertions

key-files:
  created: []
  modified:
    - parser_simd.go
    - parser_simd_lifecycle_test.go

key-decisions:
  - Successful SIMD document cleanup resumes the identical walk panic, while failed cleanup returns a terminal lifecycle error
  - Error-valued panics remain peer causes and non-error panic values receive stable diagnostic context

patterns-established:
  - "Panic-aware cleanup: recover before close, close once, then either re-panic unchanged or return a lifecycle marker"
  - "Retry proof: execute the second AddDocument before assertions so old soft-skip behavior cannot hide"

requirements-completed: [SIMD-04]

duration: 7 min
completed: 2026-07-23
---

# Phase 21 Plan 05: Panic-Aware SIMD Cleanup Summary

**Failed SIMD document cleanup now poisons the builder even during a concurrent walk panic, while successful cleanup preserves the original panic value unchanged.**

## Performance

- **Duration:** 7 min
- **Started:** 2026-07-23T07:53:30Z
- **Completed:** 2026-07-23T08:01:10Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments

- Recovered walk panics before document close so a failed close can reach the existing fatal parser-lifecycle route.
- Preserved error-valued panic identity through `errors.Is` and converted non-error panic values with stable SIMD-walk context.
- Strengthened successful-close coverage to require identity-preserving re-panic after exactly one cleanup call.
- Added a native-free soft-mode regression that performs an actual second `AddDocument` and proves the stored tragic error blocks parser redispatch.

## Task Commits

The TDD task was committed as separate RED and GREEN outcomes:

1. **Task 1 RED: Add failing panic cleanup regression** - `7de2fc7` (test)
2. **Task 1 GREEN: Preserve failed cleanup during walk panic** - `91d4a7b` (fix)

## Files Created/Modified

- `parser_simd.go` - Recovers concurrent walk panics before exactly-once close and returns a lifecycle marker when cleanup fails.
- `parser_simd_lifecycle_test.go` - Covers panic identity, error and non-error panic provenance, caller recovery, terminal soft-mode behavior, and blocked retry dispatch.

## Verification

- `go test -tags simdjson -run '^TestSIMDDocumentLifecycle' -count=1 .` — passed without constructing `NewSIMDParser` or loading a native library.
- `go test -run '^TestParserLifecycle' -count=1 .` — passed.
- `go test -count=1 ./...` — passed.
- `go test -tags simdjson -count=1 ./...` — passed.
- `go build ./... && go vet ./...` — passed.
- `go build -tags simdjson ./... && go vet -tags simdjson ./...` — passed.
- `make simd-isolation-check` — passed.

## TDD Gate Compliance

- RED failed for the intended reason: panic-plus-close failure escaped to caller recovery instead of returning a lifecycle marker.
- GREEN passed after changing only `finishSIMDDocument`.
- The RED commit `7de2fc7` precedes the GREEN commit `91d4a7b`.

## Decisions Made

- Treat failed document cleanup as the terminal outcome when it coincides with a walk panic, allowing `AddDocument` to store the lifecycle failure.
- Preserve ordinary Go panic behavior when cleanup succeeds by re-panicking the identical recovered value.
- Preserve an error-valued panic directly as a peer cause; wrap only non-error panic values with stable diagnostic text.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

- The metadata commit helper could not stage the new summary because `.planning/` is ignored. The exact summary path was force-added explicitly, while tracked state files were staged normally.

## User Setup Required

None - the regression is native-free and the SIMD backend remains explicitly opt-in.

## Next Phase Readiness

- The final Phase 21 lifecycle gap is closed.
- Phase 22 can proceed with native parity, benchmarks, tagged CI, and platform validation.

## Self-Check: PASSED

- `parser_simd.go` and `parser_simd_lifecycle_test.go` exist and contain the planned lifecycle implementation and regression.
- Task commits `7de2fc7` and `91d4a7b` are present.
- Focused, full default, full tagged, build, vet, and isolation gates passed.
- Stub scan found no placeholder or unwired behavior in the modified files.
- Threat-surface scan found no new surface outside the plan's lifecycle threat model.

---
*Phase: 21-simd-parser-adapter*
*Completed: 2026-07-23*
