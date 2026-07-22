---
phase: 21-simd-parser-adapter
plan: 03
subsystem: parser
tags: [go, simdjson, build-tags, numeric-fidelity, dependency-isolation]

requires:
  - phase: 21-01
    provides: Typed parser sink contract and stdlib parser parity baseline
  - phase: 19
    provides: Validated pure-simdjson integration and deployment strategy
provides:
  - Build-tagged pure-simdjson parser adapter with deterministic native document cleanup
  - Exact json.Number materialization for transformer subtrees
  - Source-tag and default dependency-graph isolation guard
affects: [22-simd-validation-benchmarks-ci]

tech-stack:
  added:
    - github.com/amikos-tech/pure-simdjson v0.1.4
    - github.com/ebitengine/purego v0.10.0
  patterns:
    - Optional native parser implementations live behind exact first-line build tags
    - Native document lifetime is contained within one Parse call
    - Transformed subtrees materialize numbers as json.Number before sink staging

key-files:
  created:
    - parser_simd.go
  modified:
    - go.mod
    - go.sum
    - Makefile

key-decisions:
  - Keep pure-simdjson imports entirely inside parser_simd.go under the simdjson build tag
  - Preserve walker context when document cleanup also fails, while returning cleanup failure as primary
  - Enforce SIMD isolation through both source inspection and the default Go dependency graph

patterns-established:
  - "Transform-first traversal: each raw, indexed, and wildcard path checks field transformers before typed staging"
  - "Duplicate object keys: retain the last value and sort surviving keys for deterministic traversal"

requirements-completed: [SIMD-04, SIMD-05, SIMD-06, SIMD-07]

duration: 10min
completed: 2026-07-22
---

# Phase 21 Plan 03: SIMD Parser Adapter Summary

**Build-tagged pure-simdjson adapter with deterministic cleanup, exact numeric subtree materialization, last-key-wins traversal, and default graph isolation.**

## Performance

- **Duration:** 10 min
- **Started:** 2026-07-22T18:32:55Z
- **Completed:** 2026-07-22T18:42:51Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- Added the optional `pure-simdjson` parser constructor and recursive typed-tape walker behind the exact `simdjson` build tag.
- Confined native document ownership to each parse call, including explicit close-error propagation without losing an earlier traversal error.
- Preserved parser parity for field transforms, indexed and wildcard paths, duplicate keys, deterministic object order, and numeric lexemes.
- Added a standalone Makefile guard that rejects untagged SIMD imports and proves the default build and test graph excludes the native dependency.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add lifecycle-safe parity-preserving SIMD adapter** - `dcd8fdf` (feat)
2. **Task 2: Prove source-tag and default dependency-graph isolation** - `a532b0d` (chore)

## Files Created/Modified

- `parser_simd.go` - Build-tagged parser adapter, typed traversal, subtree materialization, and lifecycle handling.
- `go.mod` - Pins `github.com/amikos-tech/pure-simdjson` at exactly `v0.1.4`.
- `go.sum` - Records checksums for the pinned SIMD module and its declared transitive dependencies.
- `Makefile` - Adds `simd-isolation-check` and its help entry.

## Verification

- `go test -run '^TestTypedSink|^TestParserParity_AuthoredFixtures' -count=1 .`
- `go build ./...`
- `go vet ./...`
- `go build -tags simdjson ./...`
- `go vet -tags simdjson ./...`
- `go test -tags simdjson -run '^$' ./...`
- `make simd-isolation-check`
- `make test` — completed 1,038 tests with one pre-existing fixture-dependent test skipped.

## Decisions Made

- Used `json.Number` for every numeric value materialized for a field transformer, retaining integer width and stable float spelling.
- Applied field transformers before direct typed staging at every recursive path so raw, indexed, and wildcard semantics remain aligned.
- Combined a strict importer build-tag check with `go list -deps -test ./...` so isolation failures are caught at both source and graph levels.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - the SIMD backend remains opt-in through the `simdjson` build tag.

## Next Phase Readiness

- Phase 22 can exercise runtime parity, benchmarks, and CI using the tagged constructor.
- No blockers remain.

## Self-Check: PASSED

- All four implementation files exist and contain no known stubs.
- Task commits `dcd8fdf` and `a532b0d` are present.
- Default and tagged verification passed, and the worktree was clean before summary creation.
- `.planning/STATE.md`, `.planning/ROADMAP.md`, and `.planning/config.json` were not modified.

---
*Phase: 21-simd-parser-adapter*
*Completed: 2026-07-22*
