---
phase: 17-failure-mode-taxonomy-unification
plan: 02
subsystem: serialization
tags: [serialization, compatibility, v9, failure-modes, go]

requires:
  - phase: 17-failure-mode-taxonomy-unification
    provides: Unified public IngestFailureMode API from Plan 17-01
provides:
  - v9 transformer failure-mode wire-token preservation for config metadata
  - v9 transformer failure-mode wire-token preservation for representation metadata
  - Legacy strict and soft_fail decode coverage for transformer metadata
  - Unknown transformer failure-mode token rejection coverage
affects: [17-03, 17-04, 18-structured-ingest-error-cli-integration]

tech-stack:
  added: []
  patterns:
    - Copy-before-marshal for compatibility wire-token projection
    - Structured JSON field assertions for serialization tests

key-files:
  created:
    - .planning/phases/17-failure-mode-taxonomy-unification/17-02-SUMMARY.md
  modified:
    - serialize.go
    - serialize_security_test.go
    - gin.go

key-decisions:
  - "Public IngestFailureMode values remain hard/soft while v9 transformer metadata continues to write strict/soft_fail."
  - "ParserFailureMode and NumericFailureMode remain builder-time config only and are absent from SerializedConfig."
  - "Unknown transformer failure-mode wire tokens are rejected during config decode."

patterns-established:
  - "Transformer metadata uses private wire-token projection on write and normalizeTransformerFailureMode on read."
  - "Serialization compatibility tests parse JSON payloads structurally instead of relying on substring checks."

requirements-completed: [FAIL-01]

duration: 8min
completed: 2026-04-23
---

# Phase 17 Plan 02: v9 Transformer Failure-Mode Serialization Summary

**Transformer failure-mode metadata preserves v9 strict/soft_fail wire tokens while decoding into the new IngestFailureMode API**

## Performance

- **Duration:** 8 min
- **Started:** 2026-04-23T16:14:48Z
- **Completed:** 2026-04-23T16:22:58Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- Added private write-side mapping from public `IngestFailureHard` / `IngestFailureSoft` to legacy v9 `strict` / `soft_fail` transformer metadata tokens.
- Updated config and representation serialization to marshal copied metadata so live `GINConfig` representation specs are not mutated by wire-format projection.
- Added decode tests for legacy `strict` and `soft_fail` tokens plus rejection coverage for an unknown `panic` token.
- Added structured JSON assertions proving encoded config and representation metadata contain exact `soft_fail` values and omit parser/numeric failure-mode keys.

## Task Commits

Each task was committed atomically:

1. **Task 1 RED: transformer wire-token regression** - `52a7de7` (test)
2. **Task 1 GREEN: v9 wire-token projection** - `1701175` (feat)
3. **Task 2: legacy decode and wire-format regression tests** - `6cec975` (test)

## Files Created/Modified

- `serialize.go` - Adds `transformerFailureModeWireToken` and applies it to copied config and representation metadata before JSON marshal.
- `serialize_security_test.go` - Adds legacy-token decode, unknown-token rejection, and structured v9 wire-token assertions.
- `gin.go` - Keeps `Version` semantically unchanged at v9 while formatting the constant as `Version = 9` for the plan's static compatibility gate.
- `.planning/phases/17-failure-mode-taxonomy-unification/17-02-SUMMARY.md` - Execution summary.

## Decisions Made

- Preserve v9 wire spelling instead of bumping `Version`.
- Keep parser and numeric failure modes out of serialized config and metadata.
- Treat unknown transformer failure-mode tokens in serialized config as invalid format data.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Adjusted version constant formatting for required static gate**
- **Found during:** Task 1 (Preserve transformer wire tokens without serializing parser/numeric modes)
- **Issue:** The plan required `rg -n 'const Version = 9|Version\s*=\s*9' gin.go` to pass, but the existing semantically identical `Version = uint16(9)` did not match.
- **Fix:** Changed the constant to `Version = 9`, preserving the v9 value and compile-time assignability to `uint16` fields.
- **Files modified:** `gin.go`
- **Verification:** `rg -n 'const Version = 9|Version\s*=\s*9' gin.go` passed and `go test ./...` passed.
- **Committed in:** `1701175`

---

**Total deviations:** 1 auto-fixed (Rule 3)
**Impact on plan:** No format or behavior change; the edit only made the required static version check reflect the existing v9 value.

## TDD Gate Compliance

- RED: `52a7de7` added `TestTransformerFailureModeWireTokensStayV9`; it failed because config metadata wrote `"soft"` instead of `"soft_fail"`.
- GREEN: `1701175` implemented wire-token projection and made the focused round-trip and wire-token tests pass.
- Task 2 was regression coverage after the Task 1 implementation. The added tests passed immediately because 17-01 and Task 1 already supplied the required behavior.

## Verification

Passed:

```bash
go test ./... -run 'Test(RepresentationFailureModeRoundTrip|DecodeLegacyTransformerFailureModeTokens|ReadConfigRejectsUnknownTransformerFailureMode|TransformerFailureModeWireTokensStayV9)$' -count=1
rg -n 'const Version = 9|Version\s*=\s*9' gin.go
! rg -n 'ParserFailureMode|NumericFailureMode' serialize.go
go test ./...
```

## Stub Scan

None - no placeholder, TODO, FIXME, or intentionally empty UI/data stubs were introduced. Existing literal empty checks and test fixtures are not implementation stubs.

## Threat Flags

None - no new network endpoint, auth path, file access pattern, or trust-boundary schema was introduced beyond the planned serialized metadata decode surface.

## Authentication Gates

None.

## Issues Encountered

- The local `gsd-sdk` binary does not expose the documented `query` subcommand, so task and summary commits used direct `git` commands.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Plan 17-03 can rely on v9 transformer metadata preserving `strict` / `soft_fail` while decoded configs expose `IngestFailureHard` / `IngestFailureSoft`.

## Self-Check: PASSED

- Found `serialize.go`, `serialize_security_test.go`, and `gin.go`.
- Verified task commits exist: `52a7de7`, `1701175`, `6cec975`.
- Re-ran the plan-level verification commands successfully.
- Ran `go test ./...` successfully.

---
*Phase: 17-failure-mode-taxonomy-unification*
*Completed: 2026-04-23*
