---
phase: 22-simd-validation-benchmarks-ci
plan: 04
subsystem: documentation
tags: [simdjson, documentation-contract, release-guidance, go-example, make]

# Dependency graph
requires:
  - phase: 22-simd-validation-benchmarks-ci
    plan: 02
    provides: Required-host constructor policy plus encoded-byte and query parity gates
  - phase: 22-simd-validation-benchmarks-ci
    plan: 03
    provides: Bounded deterministic fuzz corpus replay and malformed-attribution exclusion
  - phase: 21-simd-parser-adapter
    provides: Opt-in SIMD parser API, loading context, and caller-owned lifecycle
provides:
  - Compile-checked consumer enablement and cleanup guidance synchronized by markers
  - Effective-version bootstrap pin and tag-aware release guidance for optional native SIMD
  - One authoritative Go documentation contract exposed through check-simd-docs and lint
affects: [22-07, 22-08, SIMD-10, SIMD-11, release-process]

# Tech tracking
tech-stack:
  added: []
  patterns: [effective-module metadata via go list, named upstream constant extraction, marker-synchronized compile-only Example, mutation-negative documentation guard]

key-files:
  created:
    - simd_example_test.go
    - simd_docs_guard_test.go
  modified:
    - docs/simd-deployment.md
    - .goreleaser.yml
    - Makefile

key-decisions:
  - "The documentation contract derives both effective module version and directory through go list, honors versioned replacements, and rejects unversioned local replacements before forming a pinned link."
  - "The public SIMD Example has no output directive, so tagged tests compile the ownership path without executing native library loading."
  - "Five-platform language describes the configured two-required/three-advisory policy pending Plan 22-08 remote evidence, while parity remains qualified to documents without parser-layer errors."

patterns-established:
  - "Named-constant allowlist: inspect only libraryEnvPath, mirrorEnvVar, disableGHEnvVar, and cacheDirEnvVar instead of sweeping all PURE_SIMDJSON diagnostics."
  - "Executable guide snippets: compare marker-delimited bodies between Markdown and a compile-only external-package Example."

requirements-completed: [SIMD-10, SIMD-11]

# Metrics
duration: 9 min
completed: 2026-08-01
---

# Phase 22 Plan 04: Executable SIMD Deployment Guidance Summary

**Version-pinned SIMD deployment and release guidance backed by a compile-only public Example and mutation-tested Go contract on the canonical lint path**

## Performance

- **Duration:** 9 min
- **Started:** 2026-08-01T09:16:01Z
- **Completed:** 2026-08-01T09:25:57Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- Re-ran authored, realistic, query, malformed-attribution, and bounded fuzz parity gates before changing shipping guidance; all remained green.
- Replaced the moving upstream bootstrap link with the effective module tag, documented explicit-path provenance ownership, and qualified parity to documents that ingest without a parser-layer error.
- Documented the configured five-platform policy as two required and three advisory legs pending the final remote evidence gate, without claiming a SIMD speedup or completed remote matrix.
- Added a tagged external-package `ExampleNewSIMDParser` that compiles constructor, builder selection, ingestion, and cleanup while remaining non-runnable because it has no output directive.
- Added one Go contract that reads four named upstream loading constants from the effective module directory and rejects env, guide-path, cross-link, version-pin, release-header, or snippet drift through focused mutation tests.
- Exposed the contract through `make check-simd-docs`, wired it into `lint`, and documented it in `make help`.

## Task Commits

Each task was committed atomically:

1. **Task 1: Correct and compile-check consumer and release guidance** - `857eea8` (docs)
2. **Task 2 RED: Specify the SIMD documentation contract** - `0065ddb` (test)
3. **Task 2 GREEN: Enforce the SIMD documentation contract through Make and Go** - `03e97ff` (feat)

**Plan metadata:** committed with this summary.

## Files Created/Modified

- `docs/simd-deployment.md` - Effective-tag bootstrap link, synchronized enablement snippet, qualified parity statement, five-platform policy, and evidence restraint.
- `simd_example_test.go` - Tagged compile-only external-package Example with caller-owned parser cleanup.
- `.goreleaser.yml` - Generic release header signal and tag-aware absolute deployment-guide link; release assets remain disabled.
- `simd_docs_guard_test.go` - Effective module resolution, named constant parsing, aligned contract, and contract-specific mutation-negative tests.
- `Makefile` - Thin `check-simd-docs` target, lint dependency, and help entry.

## Decisions Made

- Used `go list -m -json` as the single effective module metadata source so both version and directory follow a versioned replacement together.
- Kept the pure-simdjson module path split in untagged test source, preserving the default dependency-isolation policy.
- Limited upstream inspection to four named Go constants. Unrelated tokens such as `PURE_SIMDJSON_WARN_LEAKS` cannot expand the public loading-variable allowlist.
- Kept release changes to the generic header only; `builds: skip: true` and all changelog behavior remain unchanged.

## Verification Evidence

- Pre-guidance parity gate passed with `AMI_GIN_SIMD_REQUIRED=1` across constructor policy, authored fixtures, four Phase 20 fixtures, the shared Evaluate matrix, the malformed-attribution oracle, and all committed `FuzzParserParity` seeds.
- Task 1 acceptance checks proved the effective bootstrap tag, absence of `/blob/main/`, exact qualified claim, all five policy pairs, exact Example shape, no output directive, preserved release skip, and tag-aware release link.
- RED gate failed on the intentionally missing documentation loader, validator, and effective-module functions before implementation.
- `make check-simd-docs` and `go test -run '^TestSIMDDocumentationContract$' .` passed with all mutation-negative subtests.
- `go test -tags simdjson -run '^ExampleNewSIMDParser$' .` compiled the Example and reported no runnable tests.
- `go test ./...`, `make simd-isolation-check`, and `make lint` passed.
- `git diff --exit-code -- README.md CHANGELOG.md parser_simd.go` passed; all three remain guard inputs rather than edit targets.

## TDD Gate Compliance

- Task 2 RED commit `0065ddb` preceded GREEN commit `03e97ff`.
- RED failed for the intended missing contract implementation; GREEN passed the focused, default, isolation, and lint gates.
- No refactor commit was needed after GREEN.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Targeted the CHANGELOG mutation at the link destination**
- **Found during:** Task 2 GREEN verification
- **Issue:** The initial negative test replaced the first visible guide-path token in CHANGELOG, which was link-label text; the valid link destination remained and the mutation correctly did not fail the contract.
- **Fix:** Changed README and CHANGELOG mutations to replace the Markdown link destination specifically.
- **Files modified:** `simd_docs_guard_test.go`
- **Verification:** The aligned contract and every mutation-negative subtest pass, including distinct README and CHANGELOG link diagnostics.
- **Committed in:** `03e97ff`

---

**Total deviations:** 1 auto-fixed (1 bug).
**Impact on plan:** The correction made the intended drift test precise without changing contract scope or production behavior.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Known Stubs

None.

## Next Phase Readiness

- Plan 22-07 can invoke `check-simd-docs` in workflow policy and build the five-leg SIMD matrix without duplicating documentation parsing.
- Plan 22-08 can evaluate actual remote outcomes against the guide's deliberately pending two-required/three-advisory policy.
- No public parser behavior, dependency, release asset, downloader, checksum verifier, cache installer, ABI validator, or CLI surface changed.

## Self-Check: PASSED

- All five created or modified implementation files exist, and task commits `857eea8`, `0065ddb`, and `03e97ff` are present in git history.
- Every task acceptance criterion and plan-level verification command passed after the final task commit.
- Stub and threat-surface scans found no incomplete goal behavior or new production trust boundary; T-22-02 and T-22-04 are enforced by the new contract.

---
*Phase: 22-simd-validation-benchmarks-ci*
*Completed: 2026-08-01*
