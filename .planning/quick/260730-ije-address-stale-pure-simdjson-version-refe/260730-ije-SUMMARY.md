---
phase: quick-260730-ije
plan: 01
status: complete
subsystem: tooling
tags: [make, go-modules, pure-simdjson, documentation]

requires:
  - phase: quick-260730-ftv
    provides: go.mod-derived SIMD bootstrap version and adapter-default depth guidance
provides:
  - Version-independent SIMD adapter source and changelog wording
  - Lint-integrated NOTICE pin alignment with the selected Go module version
  - Explicit upstream-versus-adapter maximum-depth configuration guidance
affects: [phase-22-simd-validation-benchmarks-ci, release-maintenance]

tech-stack:
  added: []
  patterns:
    - Derive legal-notice dependency pins from the selected Go module graph
    - Validate exact NOTICE line shapes and complete Go semantic-version tokens

key-files:
  created: []
  modified:
    - parser_simd.go
    - CHANGELOG.md
    - Makefile
    - docs/simd-deployment.md

key-decisions:
  - "Keep go.mod authoritative by resolving pure-simdjson with go list instead of adding another literal version."
  - "Require the exact dependency, heading, LICENSE, and NOTICE line shapes, including their 2/1/1/1 version-token distribution."
  - "Document upstream WithMaxDepth without adding options to the adapter's existing NewSIMDParser constructor."

patterns-established:
  - "NOTICE alignment guard: compare complete semantic-version tokens and exact required-line shapes against module-derived expectations."

requirements-completed: [IJE-01, IJE-02, IJE-03, IJE-04, IJE-05]

metrics:
  duration: 17min
  completed: 2026-07-30
---

# Quick Task 260730-ije: NOTICE Alignment and SIMD Depth Guidance Summary

**The SIMD adapter avoids duplicated release claims, lint fail-closes every intentional NOTICE pin and line shape against the Go module graph, and deployment guidance states the adapter's exact depth-configuration boundary.**

## Performance

- **Duration:** 17 min total (10 min initial execution + 7 min verifier-gap fix)
- **Started:** 2026-07-30T10:36:31Z
- **Completed:** 2026-07-30T10:46:02Z
- **Re-verified:** 2026-07-30T11:24:57Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- Removed the stale `pure-simdjson v0.1.4` source and Unreleased changelog attribution without replacing it with another duplicated version.
- Added `check-notice-version`, which derives the selected version with `go list`, requires the exact dependency/tree, heading, LICENSE, and NOTICE line shapes, and rejects missing or drifted complete version tokens.
- Wired the NOTICE guard into `lint` and documented it in `make help`.
- Clarified that upstream offers `WithMaxDepth`, while the optionless `NewSIMDParser()` uses upstream defaults and does not expose maximum-depth configuration.
- Preserved parser code, public API, dependencies, NOTICE content, tests, and the tested 1,023/1,024 depth boundary.

## Task Commits

Each task was committed atomically:

1. **Task 1: Remove stale attribution and document the adapter depth boundary** - `ab606b7` (docs)
2. **Task 2: Enforce NOTICE version alignment through lint** - `f1b8538` (chore)
3. **Verifier gap fix: Fail closed on missing tokens, line substitutions, and suffix drift** - `36125d7` (fix)

_Planning artifacts remain uncommitted for the orchestrator._

## Files Created/Modified

- `parser_simd.go` - Attributes the nesting boundary to the default parser configuration without a release literal; executable Go is unchanged.
- `CHANGELOG.md` - Describes the opt-in same-package adapter without a stale version claim.
- `Makefile` - Adds the module-derived, exact-shape and complete-token NOTICE alignment guard, lint prerequisite, and help entry.
- `docs/simd-deployment.md` - Distinguishes upstream `WithMaxDepth` support from the adapter's optionless constructor.

## Decisions Made

- Used `go list -m -f '{{.Version}}'` as the only operational version source; `go.mod` and `go.sum` remain unchanged.
- Enforced four exact pin-line identities, five complete Go semantic-version tokens in a 2/1/1/1 distribution, and equality with the selected module version.
- Kept the adapter API and construction behavior unchanged rather than exposing upstream parser options.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Verification wording] Kept the required source phrase on one comment line**
- **Found during:** Task 1 verification
- **Issue:** The first edit split `default pure-simdjson parser configuration` across comment lines, so the plan's literal guard could not match it.
- **Fix:** Rewrapped only the comment; executable Go remained unchanged.
- **Files modified:** `parser_simd.go`
- **Verification:** The complete Task 1 check and focused tagged nesting test passed.
- **Committed in:** `ab606b7`

**2. [Rule 3 - Verification environment] Ran the mutation loop with Bash word splitting**
- **Found during:** Task 2 verification
- **Issue:** The plan's unquoted newline-separated `required_lines` loop assumes POSIX/Bash word splitting, while the executor shell was zsh and treated all four line numbers as one value.
- **Fix:** Re-ran the unchanged mutation suite under `/bin/bash`.
- **Files modified:** None
- **Verification:** All five token corruptions and all four required-line deletions were rejected.
- **Committed in:** Not applicable

**3. [Rule 1 - Guard bug] Closed verifier-discovered fail-open mutations**
- **Found during:** Independent quick-task verification after Task 2
- **Issue:** The aggregate line count and base `vX.Y.Z` extraction accepted deletion of either dependency-line token, substitution of a required line with another version-bearing line, and suffix drift such as `v0.1.7-wrong`.
- **Fix:** Added complete Go semantic-version extraction, an exact five-token count, exact expected shapes for all four required lines, and one-occurrence checks for each shape.
- **Files modified:** `Makefile`
- **Verification:** The committed guard rejected all five token deletions, four line-shape substitutions, and prerelease/build/incompatible suffix drift, alongside the original mutation matrix.
- **Committed in:** `36125d7`

---

**Total deviations:** 3 auto-fixed (1 wording layout, 1 verification-environment mismatch, 1 verifier-discovered guard bug)
**Impact on plan:** No behavior, API, dependency, NOTICE, or test scope changed.

## Issues Encountered

### Pre-existing repository-wide lint debt

`make lint` ran the new `check-notice-version` prerequisite successfully, then
the exact underlying command `golangci-lint run` exited 1 with 50 existing
`goconst` findings. The 11 reported files were:

- `atomicity_test.go`
- `benchmark_test.go`
- `generators_test.go`
- `gin_test.go`
- `integration_property_test.go`
- `observability_internal_test.go`
- `parquet.go`
- `phase09_review_test.go`
- `serialize_security_test.go`
- `transformer_registry_test.go`
- `transformers_test.go`

Every reported file was byte-unchanged from the pre-edit baseline
`c316c70aadcb473f7f23d0eede3be2ac66dfec97`. Per the executor scope boundary,
none was modified and lint was not weakened. Non-regression was proven with:

- `golangci-lint run --new-from-rev=c316c70aadcb473f7f23d0eede3be2ac66dfec97 ./...` - 0 issues.
- `golangci-lint run --build-tags simdjson --new-from-rev=c316c70aadcb473f7f23d0eede3be2ac66dfec97 ./...` - 0 issues.
- `golangci-lint run --new-from-rev=refs/gsd/baselines/quick-260730-ije-fix ./...` - 0 issues against the Makefile-only fix baseline at `f1b8538`.

The temporary fix baseline ref was deleted only after these checks, the
Makefile-only fix scope gate, and the complete four-file task scope gate passed.

## Verification Gap Closure

- **Dependency-line token deletion:** Both the visible version and tree-link version are now required; deletion of either fails.
- **Required-line substitution:** Exact shapes are required for the dependency/tree line, version heading, LICENSE permalink, and NOTICE permalink; all four substitutions fail.
- **Complete version comparison:** The token pattern consumes prerelease, build, and `+incompatible` suffixes before exact comparison with the module-derived version.
- **Adversarial coverage:** The committed guard rejected 21 mutations: 5 token substitutions, 5 token deletions, 4 whole-line deletions, 4 line-shape substitutions, and 3 suffix drifts.

## Verification Performed

1. `go test -tags simdjson -run '^TestSIMDParserNestingDepthBoundaryContract$' -count=1 .` - passed.
2. Task 1 stale-version, constructor-signature, nesting wording, `gofmt`, and diff checks - passed.
3. `make check-validator-markers` - passed.
4. `make check-notice-version` - passed against the aligned current NOTICE.
5. Expanded NOTICE mutation suite - rejected 5 token substitutions, 5 token deletions, 4 whole-line deletions, 4 required-line substitutions, and 3 suffix drifts.
6. Makefile module-command, no-literal-pin, lint-prerequisite, help-output, and whitespace checks - passed.
7. `make test` re-run after `36125d7` - passed 1,047 tests with one expected missing-fixture skip.
8. Full `make lint` - NOTICE and validator guards passed; repository-wide lint then exposed the 50 pre-existing findings documented above.
9. Fix-baseline, original-baseline default, and original-baseline `simdjson` changed-file lint - 0 issues in all three checks.
10. Exact non-planning scope gates - the verifier fix changed only `Makefile`; the complete quick-task diff contains only `CHANGELOG.md`, `Makefile`, `docs/simd-deployment.md`, and `parser_simd.go`; whitespace checks passed.

## Known Stubs

None. Placeholder and hardcoded-empty assignment scans found no stubs in the
four modified files.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Future `pure-simdjson` version changes in the module graph will fail the normal lint path until all four exact NOTICE lines and all five complete version tokens are aligned.
- Phase 22 documentation can rely on the explicit distinction between upstream depth options and the current adapter API.
- The unrelated repository-wide `goconst` baseline remains deferred; this task introduces no new lint findings.

---
*Quick task: 260730-ije*
*Completed: 2026-07-30*

## Self-Check: PASSED

- Found all four modified implementation files and this summary.
- Found task commits `ab606b7`, `f1b8538`, and `36125d7`; each contains only its task-owned files.
- Confirmed `36125d7` changes only `Makefile`.
- Confirmed the complete non-planning diff from `c316c70` contains exactly the four authorized implementation files.
- Confirmed no files are staged and `.planning/STATE.md` was not updated.
