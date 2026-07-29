---
phase: quick-260729-eb3
plan: 01
subsystem: simd-parser-adapter
tags: [simd, parser, parity, lifecycle]
status: complete
dependency-graph:
  requires: [64bb3fa]
  provides: [SIMD-REVIEW-FALLBACK, SIMD-REVIEW-LIFECYCLE, SIMD-REVIEW-PARITY]
  affects: [builder.go, parser.go, parser_sink.go, parser_simd.go, parser parity tests, docs/simd-deployment.md]
tech-stack:
  added: []
  patterns: [validated stdlib compatibility staging, tragic parser-panic recovery, authored binary parity fixtures]
key-files:
  created:
    - testdata/parity-golden/mixed-float-int.bin
    - testdata/parity-golden/single-rg-array-siblings.bin
    - testdata/parity-golden/transformer-buffered-container-numerics.bin
  modified:
    - builder.go
    - parser.go
    - parser_sink.go
    - parser_simd.go
    - parser_lifecycle_test.go
    - parser_simd_lifecycle_test.go
    - parser_simd_integration_test.go
    - parser_parity_fixtures_test.go
    - parser_parity_test.go
    - docs/simd-deployment.md
decisions:
  - "A well-formed native invalid-JSON result below the SIMD depth limit reuses stdlibParser.Parse for staging, eliminating a second transformer/numeric policy implementation."
  - "Any panic escaping Parser.Parse poisons GINBuilder before the same panic is re-raised."
  - "When walk panic and document Close failure coincide, parserLifecycleError becomes the panic value so both causes remain observable without a configured logger."
  - "The documented pure-simdjson v0.1.4 boundary is exact: 1,023 nested containers accepted, depth 1,024 rejected."
metrics:
  duration: ~30m
  completed: 2026-07-29
---

# Quick Task 260729-eb3: SIMD Parser Review Findings Summary

Resolved the numeric-rejection fallback bugs, reconciled parser-panic lifecycle
behavior, expanded authored SIMD byte-parity coverage, and documented the
intentional nesting and malformed-input limits.

## What Changed

**Fallback parity**
- Replaced the single-leaf `StageJSONNumber` fallback with validated
  `stdlibParser.Parse` staging.
- Hard transformer policy now wins consistently over soft numeric policy.
- Duplicate object keys preserve `encoding/json` last-key-wins behavior,
  including a shadowed overflow followed by a valid value.
- Removed the redundant recursive bad-number search helper.

**Lifecycle integrity**
- `AddDocument` records a tragic builder error for any panic escaping
  `Parser.Parse`, then re-raises the panic.
- A concurrent SIMD walk panic and `doc.Close()` failure now re-raises a
  `parserLifecycleError` containing both causes; structured logging remains an
  additional signal rather than the only signal.
- Tests prove recovered panics cannot leave a reusable builder.

**Parity contract and coverage**
- Promoted `mixed-float-int` and `single-rg-array-siblings` into the authored
  golden suite.
- Added a transformer-buffered object/array fixture containing integers, whole
  floats, fractions, and nested numeric values.
- Added exact boundary coverage: byte parity at depth 1,023, stdlib acceptance
  at 1,024, and SIMD `ErrDepthLimitExceeded` at 1,024.
- Pinned the known malformed `1e400 garbage` hard-parser/soft-numeric
  classification asymmetry.
- Corrected the `StageUint64` and successful-parse lifecycle comments.
- Updated `docs/simd-deployment.md` and the parity-golden inventory.

## Verification

- `go test -count=1 ./...` - passed.
- `go build ./...` - passed.
- `make simd-isolation-check` - passed (`go build` and `go vet` included).
- `go test -count=1 -tags simdjson -race ./...` - passed on the clean rerun.
- The first tagged race run hit the unrelated probabilistic
  `TestPropertyHLLEstimateWithinBounds` after 70 cases. The exact test then
  passed three consecutive race runs with 250 cases each before the clean full
  rerun.
- `golangci-lint run --new-from-rev=64bb3fa ./...` - 0 issues.
- `golangci-lint run --build-tags simdjson --new-from-rev=64bb3fa ./...` -
  0 issues.
- `make lint` remains red on 50 pre-existing `goconst` findings; no finding is
  introduced by this quick task.

## Commits

| Commit | Message |
| --- | --- |
| `6fd8fc4` | `fix(21): preserve SIMD rejected-document staging parity` |
| `316c933` | `fix(21): make recovered parser panics terminal` |
| `760c489` | `test(21): pin SIMD parity limits and materialization` |

## Deviations

- The fallback fix removed `findUnstageableJSONNumber` instead of extending it;
  normal stdlib staging is smaller and preserves all existing policy by
  construction.
- The depth guard was renamed and aligned to the upstream first-rejected depth
  (`>= 1024`) after checking pure-simdjson v0.1.4's boundary tests.

## Self-Check

- All three implementation commits are present in `git log`.
- The three new golden files exist and all twelve authored fixtures pass under
  the real SIMD parser.
- No planned source, test, or deployment-documentation work remains.
