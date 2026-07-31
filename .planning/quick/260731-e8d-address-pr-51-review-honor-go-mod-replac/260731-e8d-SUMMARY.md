---
phase: quick-260731-e8d
plan: 01
subsystem: testing
tags: [ci, go-modules, notice-guard, simd, go-list]

# Dependency graph
requires:
  - phase: quick-260730-kny
    provides: "NOTICE version guard (make check-notice-version) that check-notice-version and this test suite exercise"
provides:
  - "CI simd-parser job that honors go.mod replace directives when picking which pure-simdjson release to fetch"
  - "notice_version_guard_test.go with zero hardcoded dependency-version literals"
affects: [ci, notice-version-guard, pure-simdjson-dependency-bumps]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Replace-aware go list template `{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}` reused identically across Makefile and CI"
    - "Tests derive dependency version once via go list -m instead of hardcoding it"

key-files:
  created: []
  modified:
    - .github/workflows/ci.yml
    - notice_version_guard_test.go

key-decisions:
  - "Sentinel replacement version changed from the plan's v9.9.9 to v0.9.9 — go.mod parsing rejects major version 9 for a module path with no /vN suffix (pure-simdjson only permits major 0 or 1); v0.9.9 preserves the sentinel's purpose (synthetic, non-published) while satisfying Go's module compatibility rule."

requirements-completed: [PR51-01, PR51-02]

# Metrics
duration: 15min
completed: 2026-07-31
---

# Quick Task 260731-e8d: Address PR #51 review — honor go.mod replace + drop hardcoded NOTICE test version

**CI's simd-parser job now resolves the same replace-aware `go list` version as `make check-notice-version`, and `notice_version_guard_test.go` derives its version fixtures at runtime instead of hardcoding `v0.1.7`.**

## Performance

- **Duration:** ~15 min
- **Tasks:** 2 completed
- **Files modified:** 2

## Accomplishments
- `ci.yml`'s simd-parser job derives `pure_simdjson_version` with the replace-aware template `{{if .Replace}}{{.Replace.Version}}{{else}}{{.Version}}{{end}}`, matching `Makefile:97`, so a `replace` directive in go.mod now changes what CI fetches — closing the drift PR #51 flagged.
- `notice_version_guard_test.go` no longer hardcodes `v0.1.7`. All five subtests that need a version now call `copyNoticeGuardInputs(t)` which returns `(dir, version)`, with `version` derived once via `go list -m` against the real repository root.
- Added `sentinelReplacementVersion` constant for the "different version" fixture, replacing the previous `v0.1.8` (a real, if unpublished, tag guess) with an explicitly synthetic value.

## Task Commits

1. **Task 1: Make CI's simd-parser job honor go.mod replace directives** - `367faa7` (fix)
2. **Task 2: Derive the effective version in notice_version_guard_test.go instead of hardcoding it** - `48ac6d8` (test)

_Docs commit (SUMMARY.md/STATE.md) handled by orchestrator, not included above._

## Files Created/Modified
- `.github/workflows/ci.yml` - simd-parser job's version-derivation line now mirrors the Makefile's replace-aware `go list` template (one-line change)
- `notice_version_guard_test.go` - added `sentinelReplacementVersion` constant and `effectivePureSIMDJSONVersion` helper; `copyNoticeGuardInputs` now returns `(dir, version)`; all 5 version-dependent subtests use the derived version instead of hardcoded `v0.1.7`/`v0.1.8`

## Decisions Made
- Used `v0.9.9` instead of the plan's specified `v9.9.9` for `sentinelReplacementVersion`. Go's module compatibility rule rejects any major version other than 0 or 1 for a module path without an explicit `/vN` suffix (pure-simdjson has no such suffix); `go mod` parsing fails go.mod outright with `version "v9.9.9" invalid: should be v0 or v1, not v9`. `v0.9.9` satisfies the same intent — a synthetic, clearly-non-published sentinel distinct from the real `v0.1.x` line — while being valid go.mod syntax.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Plan's sentinel version `v9.9.9` is invalid Go module syntax**
- **Found during:** Task 2, first verification run of `go test -run TestNoticeVersionGuard`
- **Issue:** `replace github.com/amikos-tech/pure-simdjson ... => github.com/amikos-tech/pure-simdjson v9.9.9` fails go.mod parsing: `version "v9.9.9" invalid: should be v0 or v1, not v9`. Go enforces that a module's major version in a `replace`/`require` directive must match the module path's major version suffix (none here, so only v0/v1 allowed).
- **Fix:** Changed `sentinelReplacementVersion` from `v9.9.9` to `v0.9.9` — same synthetic-sentinel intent, valid module version.
- **Files modified:** `notice_version_guard_test.go`
- **Verification:** Re-ran `go test -run TestNoticeVersionGuard -count=1 -v .` — all 6 subtests pass, including `replacement_version_is_effective`.
- **Committed in:** `48ac6d8` (part of Task 2 commit)

## Verification Results

All mandatory verification commands were run and passed:

```
$ go test -run TestNoticeVersionGuard -count=1 .
ok  	github.com/amikos-tech/ami-gin	1.326s
(all 6 subtests PASS, verified individually with -v)

$ go build ./...
(exit 0)

$ make check-notice-version
(exit 0)

$ make simd-isolation-check
go build ./...
go vet ./...
(exit 0)

$ yq eval '.' .github/workflows/ci.yml > /dev/null
(exit 0)

$ grep -n 'v0\.1\.7' notice_version_guard_test.go
(no output, exit 1 — confirms zero hardcoded v0.1.7 literals remain)

$ go vet ./...
(exit 0)

$ go test -short -count=1 ./...
ok all packages
```

## Known Stubs

None.

## Threat Flags

None — this change only tightens existing version-derivation logic to match already-reviewed Makefile behavior (verified_facts 1, 3) and makes the test suite read-only introspective (`go list -m` against the checked-out repo, no network calls, no writes to the real go.mod). No new trust boundary was introduced beyond what the plan's threat model already covered.

## Self-Check: PASSED

All modified files exist on disk and both task commits (`367faa7`, `48ac6d8`) are present in `git log`.
