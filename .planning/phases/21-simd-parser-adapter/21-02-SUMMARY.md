---
phase: 21-simd-parser-adapter
plan: 02
subsystem: documentation
tags: [simdjson, pure-simdjson, deployment, supply-chain, bigint]

requires:
  - phase: 19-simd-dependency-decision-integration-strategy
    provides: pinned pure-simdjson version, opt-in API, loading delegation, and fallback policy
provides:
  - Pinned MIT and bundled simdjson Apache-2.0/MIT attribution chain
  - Operator guide for SIMD activation, native loading, fallback, and integrity boundaries
  - Public activation example and accurate BIGINT limitation disclosure
affects: [21-03-tagged-simd-adapter, 22-simd-validation-benchmarks-ci]

tech-stack:
  added: []
  patterns: [three-gate optional parser activation, route-specific native artifact trust controls]

key-files:
  created:
    - NOTICE.md
    - docs/simd-deployment.md
  modified:
    - README.md
    - CHANGELOG.md

key-decisions:
  - "Document SIMD activation as three explicit gates: tagged build, fallible construction, and WithParser selection."
  - "Treat PURE_SIMDJSON_LIB_PATH integrity as operator-owned instead of extending go.sum or bootstrap checksum guarantees to it."
  - "Describe BIGINT divergence by failure layer, path, and governing mode while preserving atomic whole-document soft skips."

patterns-established:
  - "Optional native adapters document default behavior before opt-in steps and never hide construction failure behind fallback."
  - "Supply-chain guidance separates wrapper-module checks, automatic native-download checks, and explicit-path trust."

requirements-completed: [SIMD-04, SIMD-05, SIMD-06]

duration: 10min
completed: 2026-07-22
---

# Phase 21 Plan 02: SIMD Deployment and Attribution Summary

**Pinned license attribution and operator guidance now cover the complete opt-in SIMD path without changing or overstating stdlib defaults.**

## Performance

- **Duration:** 10 min
- **Started:** 2026-07-22T18:14:52Z
- **Completed:** 2026-07-22T18:24:40Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- Added a root notice sourced from the exact pure-simdjson v0.1.4 LICENSE and NOTICE, distinguishing its MIT license from bundled simdjson's Apache-2.0/MIT texts.
- Added an operations guide covering tagged compilation, fallible construction, explicit selection/fallback, all four bootstrap variables, and route-specific integrity ownership.
- Added a compiling README activation example and an Unreleased CHANGELOG entry that preserve stdlib defaults and accurately describe BIGINT failure routing.

## Task Commits

1. **Task 1: Source the NOTICE and write deployment guidance with correct failure and trust semantics** - `004b00c` (docs)
2. **Task 2: Add the concrete README activation section and accurate Unreleased entry** - `cfb8cb7` (docs)

## Files Created/Modified

- `NOTICE.md` - Traces pure-simdjson v0.1.4's MIT license and its bundled simdjson Apache-2.0/MIT attribution.
- `docs/simd-deployment.md` - Documents activation, bootstrap, air-gap and mirror loading, fallback, integrity, BIGINT behavior, and validation scope.
- `README.md` - Adds the optional SIMD parser installation and two-step construction example.
- `CHANGELOG.md` - Records the opt-in adapter, unchanged defaults, parser name, and corrected BIGINT limitation.

## Decisions Made

- Used the consumer-facing tag `v0.1.4` in installation guidance while treating the recorded tag commit only as repository audit evidence.
- Made fallback visibly caller-owned by branching after `NewSIMDParser` failure and logging the degraded stdlib selection.
- Corrected the older per-field BIGINT explanation: SIMD fails at the parser layer with no path, stdlib reaches the numeric layer with a path, and soft mode at either layer discards the document atomically.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - the SIMD adapter remains opt-in, and environment-specific native loading is documented for callers who choose it.

## Next Phase Readiness

- Plan 21-03 can implement the tagged adapter against the published constructor and deployment contract.
- Phase 22 retains ownership of parity evidence, benchmarks, five-platform runtime CI, and operational verification.

## Self-Check: PASSED

- `NOTICE.md`, `docs/simd-deployment.md`, `README.md`, and `CHANGELOG.md` exist with the planned content.
- Task commits `004b00c` and `cfb8cb7` are present on the assigned worktree branch.
- All task acceptance checks, the combined plan verification, `git diff --check`, and `go test ./...` passed.
- No known stubs or unplanned security-relevant surfaces were introduced.

---
*Phase: 21-simd-parser-adapter*
*Completed: 2026-07-22*
