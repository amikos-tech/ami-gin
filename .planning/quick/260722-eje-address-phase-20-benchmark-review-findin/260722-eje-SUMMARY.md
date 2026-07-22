---
status: complete
phase: quick-260722-eje
plan: 01
subsystem: testing
tags: [benchmarks, external-corpus, json-depth, fixtures, planning-state]

requires:
  - phase: 20-realistic-benchmark-dataset-foundation
    provides: Phase 20 synthesized fixtures and realistic benchmark workflow
provides:
  - Bounded external-corpus loading with exact opt-in and full-tree depth validation
  - Query benchmark preflight against a real PathDirectory entry
  - Deterministic fixture generation with byte-for-byte drift coverage in make test
  - Correct Phase 21 planning position in STATE.md
affects: [21-simd-parser-adapter, 22-simd-validation-benchmarks-ci]

tech-stack:
  added: []
  patterns:
    - Validate every external JSON branch at the loading boundary before index construction
    - Resolve benchmark predicates against the finished index before timing
    - Share ordered fixture descriptors and exact-byte rendering between generation and tests

key-files:
  created:
    - .planning/quick/260722-eje-address-phase-20-benchmark-review-findin/260722-eje-SUMMARY.md
  modified:
    - benchmark_test.go
    - README.md
    - testdata/phase20/README.md
    - .planning/phases/20-realistic-benchmark-dataset-foundation/20-SECURITY.md
    - testdata/phase20/generate.go
    - testdata/phase20/generate_test.go
    - Makefile
    - .planning/STATE.md

key-decisions:
  - "Keep the public builder/query/path model unchanged; special-key candidates are skipped unless the finished index resolves them."
  - "Enforce depth across the complete decoded document during external loading, before Build, Encode, or Query setup."
  - "Keep STATE.md and this summary for the main orchestrator; create only the two authorized source/test commits during isolated execution."

patterns-established:
  - "Benchmark preflight: resolvePredicatePath must return a valid ID and nonnil entry before Evaluate can be accepted as evidence."
  - "Fixture drift: generatedFixtures and renderFixture are the single ordered source for command output and committed-byte tests."

requirements-completed:
  - P20-REVIEW-EXTERNAL-CORPUS
  - P20-REVIEW-GENERATOR-DISCOVERY
  - P20-REVIEW-PLANNING-STATE

duration: 17 min
completed: 2026-07-22
---

# Quick Task 260722-eje: Phase 20 Benchmark Review Summary

**External benchmark inputs are fully bounded and depth-validated, timed queries require a real indexed path, and fixture generation is deterministic and drift-tested by the normal test target.**

## Performance

- **Duration:** 17 min
- **Started:** 2026-07-22T08:10:12Z
- **Completed:** 2026-07-22T08:27:37Z
- **Tasks:** 3
- **Files modified/created:** 9

## Accomplishments

- Hardened the optional external corpus gate, regular-file discovery, 64-file/8-MiB-total/2-MiB-document budgets, LF/CRLF accounting, scanner overflow handling, and full-document depth-64 validation.
- Derived external Query predicates only after building the real index, skipped unresolved `user-name` and `a.b` candidates, and required successful `resolvePredicatePath` preflight before warm-up and timing.
- Added the required real-index regression for `{"a_null":null,"z_safe":true}`, proving `$.z_safe` resolves to a valid ID and entry before `Evaluate`.
- Replaced map-driven fixture generation with ordered descriptors, non-panicking error handling, shared exact rendering, committed-byte drift tests, and explicit `make test` discovery.
- Corrected the four inconsistent Phase 21 values in `STATE.md` while leaving the ROADMAP backlog entry for Phase 999.7 unchanged.

## Task Commits

1. **Task 1: Bound external loading and require a real indexed Query path** — `d32f2bf` (`fix(phase20): harden external benchmark corpus`)
2. **Task 2: Make fixture generation deterministic, drift-tested, and normally discovered** — `ff5fe7e` (`test(phase20): verify generated fixture drift`)
3. **Task 3: Reconcile project position and run the complete quick-task gate** — included in the final scoped planning-artifact commit.

## Verification

- `go test ./... -run '^TestPhase20' -count=1` — passed, including exact-gate, symlink, byte-budget, scanner overflow, full-tree depth, special-key, null-then-safe, and query-preflight regressions.
- `go test ./testdata/phase20 -count=1` — passed drift and symlink-destination coverage.
- `make -n test | rg --fixed-strings -- '--packages="./... ./testdata/phase20"'` — confirmed normal discovery of the generator package.
- `make test` — passed 1,015 tests; one existing optional-Parquet-data test was skipped because `testdata/test.parquet` is absent.
- `go run ./testdata/phase20/generate.go` followed by fixture `git diff --exit-code` — passed with zero drift across all four JSONL files.
- Offline smoke benchmark with both external environment variables unset — all Build, Encode, and Query leaves completed with `PASS`.
- `gsd-sdk query state.load` against an isolated copy plus exact STATE/ROADMAP assertions — passed; Phase 21 is current and Phase 999.7 remains backlog-only.
- `git diff --check`, cached-config check, and isolated-worktree config-status check — passed. The user's pre-existing main-worktree `.planning/config.json` edit remains untouched and unstaged.
- No actual external simdjson directory was supplied. The automated substitute accepted a synthesized 1,727,204-byte JSON document and exercised the documented external-path validation and limits.

## Deviations from Plan

### Runtime Clarifications Applied

- Added the mandatory null-then-safe real-index regression requested after the checker cap.
- Created only the two authorized source/test commits during isolated execution. The main orchestrator owns `STATE.md` and quick-task artifacts.
- Treated `.planning/config.json` as clean-only in the isolated worktree while preserving the main worktree's unrelated dirty state.

## Issues Encountered

- `gsd-sdk query state.load` successfully parsed the corrected state but also normalized `.planning/config.json` because it warned about two unknown top-level keys. The isolated executor immediately restored the file to its exact committed bytes and re-ran state parsing against an isolated copy so the check could not mutate repository configuration.

## User Setup Required

None.

## Next Phase Readiness

- Phase 20 benchmark infrastructure is hardened and ready to support Phase 21 adapter work and Phase 22 measurements.
- Phase 21 is the next active planning target; Phase 999.7 remains backlog-only.

## Self-Check: PASSED

- Both task commits exist on the main branch.
- All authorized committed files are present, and no tracked files were deleted.
- `.planning/STATE.md` and quick-task artifacts are committed in the final scoped orchestrator commit.
- The user's `.planning/config.json` edit remains intact and unstaged; `.planning/ROADMAP.md` is unchanged.

---
*Quick task: 260722-eje*
*Completed: 2026-07-22*
