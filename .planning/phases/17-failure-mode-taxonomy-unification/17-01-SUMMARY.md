---
phase: 17-failure-mode-taxonomy-unification
plan: 01
subsystem: api
tags: [api, config, failure-modes, go]

requires:
  - phase: 16-adddocument-atomicity-lucene-contract
    provides: validator-backed AddDocument staging and non-tragic public failure isolation
provides:
  - Unified public IngestFailureMode API with hard/soft constants
  - Parser and numeric failure-mode config fields and options
  - Separate public-mode and transformer-metadata validators for legacy token compatibility
  - Focused tests for defaults, validation, and old-symbol removal
affects: [17-02, 17-03, 17-04, 18-structured-ingest-error-cli-integration]

tech-stack:
  added: []
  patterns:
    - Functional options with fail-fast enum validation
    - Public ingest-mode validator kept distinct from transformer metadata validator

key-files:
  created:
    - failure_modes_test.go
  modified:
    - gin.go
    - transformer_registry.go
    - builder.go
    - phase09_review_test.go
    - transformers_test.go
    - serialize_security_test.go

key-decisions:
  - "Public failure modes are IngestFailureHard=\"hard\" and IngestFailureSoft=\"soft\" with no old public aliases."
  - "ParserFailureMode and NumericFailureMode are builder-time config knobs and are not serialized."
  - "validateIngestFailureMode rejects legacy transformer wire tokens, while validateTransformerFailureMode accepts them for metadata compatibility."

patterns-established:
  - "Unified ingest failure mode: parser, transformer, and numeric config use IngestFailureMode."
  - "Validator asymmetry: public API validation is stricter than transformer metadata validation."

requirements-completed: [FAIL-01, FAIL-02]

duration: 7min
completed: 2026-04-23
---

# Phase 17 Plan 01: Unified Public Failure-Mode API Summary

**Unified IngestFailureMode API with hard parser/numeric defaults and validation coverage for public and legacy transformer metadata tokens**

## Performance

- **Duration:** 7 min
- **Started:** 2026-04-23T16:03:42Z
- **Completed:** 2026-04-23T16:10:18Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments

- Replaced the public transformer-only failure-mode taxonomy with `IngestFailureMode`, `IngestFailureHard`, and `IngestFailureSoft`.
- Added `GINConfig.ParserFailureMode`, `GINConfig.NumericFailureMode`, `WithParserFailureMode`, and `WithNumericFailureMode` with hard defaults and validation.
- Retargeted transformer registration metadata to `IngestFailureMode` while preserving private legacy token normalization for later v9 decode compatibility work.
- Added focused validation tests for defaults, struct-literal normalization, invalid parser/numeric/transformer modes, and legacy token validator asymmetry.

## Task Commits

Each task was committed atomically:

1. **Task 1 RED: new symbol expectations** - `4577162` (test)
2. **Task 1 GREEN: unified API/config implementation** - `69b3125` (feat)
3. **Task 2: validation coverage** - `7b53bd8` (test)

## Files Created/Modified

- `gin.go` - Defines `IngestFailureMode`, parser/numeric config knobs, normalization, and validation helpers.
- `transformer_registry.go` - Retargets `TransformerSpec.FailureMode` to `IngestFailureMode`.
- `builder.go` - Normalizes empty parser/numeric struct-literal config modes to hard before storing builder config.
- `failure_modes_test.go` - Adds unified failure-mode default and validation coverage.
- `phase09_review_test.go` - Mechanically renames old failure-mode constants in Phase 09 regression coverage.
- `transformers_test.go` - Mechanically renames the transformer soft-fail test to use `IngestFailureSoft`.
- `serialize_security_test.go` - Mechanically renames representation failure-mode round-trip coverage to use `IngestFailureSoft`.

## Decisions Made

- Public callers use `hard` / `soft`; private transformer metadata normalization still understands `strict` / `soft_fail`.
- `WithTransformerFailureMode` remains the public transformer option name but now accepts `IngestFailureMode`.
- Parser and numeric failure modes stay out of `SerializedConfig` because they affect builder ingest routing only.

## Deviations from Plan

None - implementation scope executed as written.

## TDD Gate Compliance

- Task 1 followed RED/GREEN: `4577162` failed to compile on undefined `IngestFailureHard` / `IngestFailureSoft`, then `69b3125` made the focused tests pass.
- Task 2 was a test-only coverage task after Task 1. Its new tests passed immediately because Task 1 already implemented the behavior they pin; no separate GREEN code commit was needed.

## Verification

Passed:

```bash
go test ./... -run 'Test(IngestFailureModeDefaultsAndValidation|NewBuilderAllowsLegacyConfigLiteralWhenAdaptiveDisabled|RepresentationFailureModeRoundTrip|BuilderSoftFailSkipsCompanionWhenConfigured)$' -count=1
! rg -n '\bTransformerFailureStrict\b|\bTransformerFailureSoft\b|\btype TransformerFailureMode\b' --glob '*.go'
! sh -c "rg -n '\bTransformerFailureMode\b' --glob '*.go' | rg -v '\bWithTransformerFailureMode\b'"
```

## Stub Scan

None - no placeholder, TODO, FIXME, or intentionally empty UI/data stubs were introduced.

## Threat Flags

None - no new network endpoint, auth path, file access pattern, or trust-boundary schema change was introduced beyond the planned caller-config validation surface.

## Issues Encountered

- The local `gsd-sdk` binary does not expose the documented `query` subcommand, so task and summary commits were made with direct `git` commands.
- No implementation blockers remained after the focused verification gates.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Plan 17-02 can build on the new `IngestFailureMode` metadata type to preserve v9 transformer wire tokens, and Plan 17-03 can consume the parser/numeric config fields for runtime soft-skip routing.

## Self-Check: PASSED

- Found `.planning/phases/17-failure-mode-taxonomy-unification/17-01-SUMMARY.md`.
- Found `failure_modes_test.go`.
- Verified task commits exist: `4577162`, `69b3125`, `7b53bd8`.
- Re-ran the plan-level verification command successfully.

---
*Phase: 17-failure-mode-taxonomy-unification*
*Completed: 2026-04-23*
