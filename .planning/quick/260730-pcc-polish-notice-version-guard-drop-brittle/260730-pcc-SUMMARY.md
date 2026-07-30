---
phase: quick-260730-pcc
plan: 01
subsystem: build-tooling
tags: [makefile, notice, licensing, guard, testing, diagnostics]

requires:
  - phase: quick-260730-ftv
    provides: Hardened NOTICE version guard with file-wide semver drift scan
provides:
  - Behavioral-only NOTICE guard test suite (no Makefile source-text assertion)
  - Documented single-attribution constraint on the NOTICE version scan
  - Filtered guard failure dump limited to pure-simdjson and semver-token lines
affects: []

tech-stack:
  added: []
  patterns:
    - Assert guard behavior by executing it, never by grepping the recipe's source text
    - Reuse the scan's own `$version_pattern` in the failure dump so filter and scan cannot drift
    - Record non-obvious scan assumptions as a short comment above the target

key-files:
  created: []
  modified:
    - notice_version_guard_test.go
    - Makefile

key-decisions:
  - Deleted the `uses a bytewise locale` subtest because it asserted exact Makefile source text rather than guard behavior
  - Filtered the failure dump with a `pure-simdjson` OR semver-token alternation, never a `pure-simdjson`-only filter
  - Left the pre-existing golangci-lint backlog untouched as out of scope

patterns-established:
  - "Dump narrowing must preserve the alternation: filtering to the dependency name alone re-hides version-only offenders"
  - "Mutation batteries run on scratch copies outside the repo; the source file is verified byte-identical afterward"

requirements-completed: [POLISH-01, POLISH-02, POLISH-03]

duration: 21 min
completed: 2026-07-30
---

# Quick 260730-pcc: NOTICE Version Guard Polish Summary

**The NOTICE guard keeps identical detection behavior while shedding a brittle source-text test, documenting its single-attribution assumption, and printing only the NOTICE.md lines that actually matter.**

## Performance

- **Duration:** ~21 min
- **Tasks:** 3/3 complete
- **Commits:** 2 (Task 3 is verification-only)

## What Changed

### Task 1 — Deleted the brittle bytewise-locale subtest (`e5afb1e`)

Removed the `uses a bytewise locale` subtest from `notice_version_guard_test.go` (7 lines).

It asserted an exact Makefile source substring:

```go
"check-notice-version:\n\t@set -eu; \\\n\texport LC_ALL=C; \\"
```

That is a false-failure trap: rewriting `export LC_ALL=C` as the behaviorally identical
`LC_ALL=C; export LC_ALL` still rejects the ZWSP mutation, yet breaks the assertion.
**Empirically confirmed** — see Verification below.

Six subtests remain, all untouched. `go vet ./...` confirms verified fact 1: no symbol or
import became unused (`readTestFile`, `repositoryRoot`, and `strings` are still referenced).

### Task 2 — Documented the constraint and narrowed the dump (`90368f4`)

**Fix 2** — two-line comment above `.PHONY: check-notice-version`:

```make
# The version scan treats pure-simdjson as the only versioned attribution allowed in NOTICE.md.
# Adding a second vendored dependency with its own version requires widening the scan.
```

**Fix 3** — narrowed the `fail_notice` dump from a whole-file dump to:

```sh
sed -n -E "/pure-simdjson|$$version_pattern/{=;l;}" NOTICE.md >&2
```

Single quotes became double quotes so `$$version_pattern` expands. Reusing the scan's own
pattern keeps the dump filter from drifting away from the scan. The `|| printf` fallback,
`>&2` redirect, and trailing continuation are unchanged.

The banner above the dump was retitled from `NOTICE.md content (...)` to
`NOTICE.md pure-simdjson and version lines (...)`, since it no longer shows the whole file.

Every edited recipe line retains its leading tab and trailing ` \` — verified programmatically.

## Verification

All gates from the plan:

| Gate | Result |
|------|--------|
| `LC_ALL=en_US.UTF-8 go test ./...` | PASS |
| `make check-notice-version` (clean tree) | PASS |
| `make lint` | **Pre-existing failure** — see Deviations |
| Mutation (a) ZWSP heading | REJECTED (exit 2), dump shows `\342\200\213` |
| Mutation (b) `Historical note: ... v0.1.2` | REJECTED (exit 2) |
| Mutation (c) bare `v0.0.9` | REJECTED (exit 2) |
| Mutation (d) `some-other-lib v2.3.4` | REJECTED (exit 2) |
| Case (c) dump still shows `v0.0.9` | **YES** — `26: v0.0.9$` |
| Control (unmutated scratch copy) | PASSES (exit 0) |
| `git diff --quiet NOTICE.md` | exit 0 — byte-identical |

**Dump narrowing measured on case (c):** 61 lines (old whole-file `sed -n '=;l'`) → 21 lines
(new filtered stderr, including 2 header lines and make's error line). The offending bare
`v0.0.9` at line 26 is still present — this was the specific regression risk of Fix 3, and it
is avoided because the filter keeps the `pure-simdjson` OR semver-token alternation.

**NOTICE.md integrity:** SHA-256 `f393f2bf...a4d6` before and after, identical.

**Mutation methodology:** all four cases ran on scratch copies in system temp, outside the
repository. The ZWSP payload was built from `chr(0x200b)` — never pasted as a literal
invisible character. All scratch directories were removed afterward.

**Empirical check of the Task 1 rationale:** on a scratch copy of `HEAD`, rewriting
`export LC_ALL=C` → `LC_ALL=C; export LC_ALL` now passes the full guard suite, confirming the
deleted assertion was the only thing standing in the way of a behaviorally identical rewrite.

## Deviations from Plan

### 1. `make lint` fails on a pre-existing backlog (not caused by this task)

`make lint` exits non-zero with 51 issues. I verified this is pre-existing by extracting the
tree at the pre-change commit (`e5afb1e^`) and running `golangci-lint` there: **identical**
51 issues (goconst 50, staticcheck 1).

The single `staticcheck QF1001` is in `notice_version_guard_test.go`, but inside the
`CI runs the dedicated guard before golangci-lint` subtest, which this task did not modify —
it merely renumbered from line 79 to 72 after the 7-line deletion. The plan explicitly said
"Do NOT touch any other subtest".

Per the SCOPE BOUNDARY rule these were **not** fixed. Logged to `deferred-items.md`.
Note that `make lint`'s own guard dependencies (`check-validator-markers`,
`check-notice-version`) both pass; only the `golangci-lint run` step fails.

### 2. Nuance: the plan's redundancy rationale is platform-dependent

The plan stated the `invisible version character is escaped` subtest "already catches a
missing `LC_ALL=C`". I tested this directly: on a scratch copy with `export LC_ALL=C`
**removed entirely**, the suite still passed on macOS.

Root cause — BSD `sed`'s `l` command octal-escapes multibyte characters regardless of locale:

```
LC_ALL=C          -> ## pure-simdjson v\342\200\2130.1.7$
LC_ALL=en_US.UTF-8 -> ## pure-simdjson v\342\200\2130.1.7$
```

GNU `sed` differs: under a UTF-8 locale it prints valid multibyte characters literally, so the
behavioral subtest **does** catch a missing `LC_ALL=C` on `ubuntu-latest`, which is where all
five CI jobs run and where the guard is enforced.

Net effect: the rationale holds at the CI enforcement point; it is simply not reproducible
locally on macOS. No behavior changed — the `export LC_ALL=C` line was left untouched. Flagged
because it slightly qualifies the plan's stated justification for the deletion. Logged to
`deferred-items.md`.

### 3. Uncommitted `.planning/config.json` left untouched

A pre-existing uncommitted `branching_strategy` deletion in `.planning/config.json` from
earlier unrelated work was deliberately **not** staged or committed, per instructions. It
remains in the working tree.

## Commits

| Hash | Message |
|------|---------|
| `e5afb1e` | `test(quick-260730-pcc-01): drop brittle bytewise-locale source assertion` |
| `90368f4` | `build(quick-260730-pcc-02): document scan constraint and narrow NOTICE dump` |

Diff vs. pre-task state: `Makefile` +4/-2, `notice_version_guard_test.go` -7.

## Known Stubs

None.

## Threat Flags

None. The change strictly reduces diagnostic output (T-pcc-01 accepted: NOTICE.md is a public
committed file with no secrets). T-pcc-02 was re-proved by the mutation battery; T-pcc-03 was
verified by confirming tabs and continuations survived and the target runs clean.

## Self-Check: PASSED

- `notice_version_guard_test.go` — FOUND, contains `invisible version character is escaped`,
  does not contain `uses a bytewise locale`
- `Makefile` — FOUND, contains `only versioned attribution` and `sed -n -E`, no
  `sed -n '=;l'` remains
- Commit `e5afb1e` — FOUND in `git log`
- Commit `90368f4` — FOUND in `git log`
- `NOTICE.md` — byte-identical to HEAD (`git diff --quiet` exit 0)
- All scratch directories removed
