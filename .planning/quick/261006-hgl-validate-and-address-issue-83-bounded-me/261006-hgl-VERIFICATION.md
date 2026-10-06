---
phase: 261006-hgl
verified: 2026-10-06
status: passed
score: 13/14 expectations verified; E14 open until the maintainer approves the draft follow-up issue. Status was human_needed after the first pass; the E3 warning (W1) is fixed in dbb9c71 and 6df7fe9, see the last section. E14 does not gate the code.
gaps: []
human_verification:
  - test: "Maintainer reviews the draft GitHub issue in 261006-hgl-DEFERRED.md before the orchestrator creates it"
    expected: "Maintainer approves or edits the draft; orchestrator then creates the issue"
    why_human: "E14 requires maintainer sight of the draft before creation; only the maintainer can give it"
  - test: "Decide whether to fix the E3 heap test (WARNING W1 below)"
    expected: "Either accept the weakness (E2 tests cover cache retention) or tighten TestEncoderProfileUncachedDoesNotRetainEncoder"
    why_human: "Test-quality decision; behavior itself is correct"
---

# Quick task 261006-hgl Verification Report

**Goal:** Opt-in `EncoderProfileBoundedMemoryUncached`: a one-worker, low-memory zstd encoder built per call and never stored in the shared encoder cache (issue #83).
**Branch:** issue-83-bounded-memory-no-retention, 4 commits over main. **Go:** go1.27.1 local.

## Implementation (non-test code, `git diff main`)

`serialize.go` only (31 lines): new constant `EncoderProfileBoundedMemoryUncached` (value 2), `String()` returns `"bounded-memory-uncached"`, `validate()` accepts it and the error text names all three constants, `sharedZstdEncoder` returns `newZstdEncoder(...)` before taking `zstdEncoderMu` or writing `zstdEncoders`, and `newZstdEncoder` gives it the one-worker and low-memory settings. `CompressionNone` returns before `sharedZstdEncoder` is called (`serialize.go` ~line 526). All encode paths reach the encoder through that one call site (`serialize.go:530`).

## Test runs (run by verifier)

- `go test -race -run 'Uncached|EncoderProfile' -count=1 .` : ok.
- `go test -run TestEncoderProfileUncachedDoesNotRetainEncoder -count=5 -v .` : 5/5 PASS.
- `go test -race -run TestEncodeUncachedConcurrentMatchesDefault -count=3 .` : ok.
- `go test -count=1 .` (full package) : ok (34.5 s). `go test ./...` : all ok.
- Host has 12 CPUs (`hw.ncpu`).

Mutation checks in a scratch worktree (removed afterwards):
- M1: disable the cache bypass (`if false`): `TestEncodeUncachedConcurrentMatchesDefault`, `TestEncoderProfileUncachedLeavesCacheEmpty`, `TestEncoderProfileUncachedConfigReachesAllEncodePaths` FAIL. The heap test `TestEncoderProfileUncachedDoesNotRetainEncoder` still PASSES (see W1).
- M2: drop the uncached profile from the one-worker/low-memory branch: `TestEncoderProfileUncachedUsesSingleWorker` FAILS. Good.

## Expectations E1-E14

Test line numbers are in `serialize_profile_test.go` unless stated.

| # | Verdict | Test (file:line) and evidence |
|---|---------|-------------------------------|
| E1 | VERIFIED | `TestEncoderProfileUncachedOutputIdenticalToDefault` :482. Single-block and multi-block fixtures, every `encoderProfileLevels` level, `bytes.Equal` against default, plus `Decode`. Asserts the full Then clause. |
| E2 | VERIFIED | `TestEncoderProfileUncachedLeavesCacheEmpty` :512 with `assertNoCachedEncoder` :471, which checks all three profiles per level. Covers `EncodeWithLevelContext` at every level and `EncodeContext`. Fails under mutation M1. |
| E3 | VERIFIED (behavior), WARNING on test | `TestEncoderProfileUncachedDoesNotRetainEncoder` :601. Asserts growth under 8 MiB after one level-15 encode and GC, with a control (cached bounded grows over 20 MiB). Passes 5/5. W1: the warm-up encode runs before the baseline, so if the profile were cached the baseline would already include the retained encoder and the test would still pass (confirmed by M1). The cache case is caught by E2/E6/E11 tests, and the control proves the heap method sees a 42 MB encoder, so the behavior is proven only through those tests. The benchmark doc also records 0.001 MB retained. |
| E4 | VERIFIED | `TestEncoderProfileUncachedUsesSingleWorker` :624. It calls `newZstdEncoder` (the function `sharedZstdEncoder` delegates to for this profile), forces init, and reflects on worker-channel capacity and `lowMem`; asserts `workers == 1 && lowMem`. The asserted count is the absolute value 1, not derived from GOMAXPROCS. On this 12-CPU host the test is discriminating (M2 fails it). The test does not sweep GOMAXPROCS, so the skipped GOMAXPROCS(4) run is not a gap: the klauspost option fixes the worker count at 1 whatever GOMAXPROCS is, and the worker count with GOMAXPROCS=1 would be 1 either way. Residual: only the explicit sweep is absent. |
| E5 | VERIFIED | Same test as E2 :512. Per-call uncached over default config via `EncodeWithLevelContext` and `EncodeContext`: cache empty. Uncached config plus per-call `EncoderProfileBoundedMemory`: bounded entry present (`sharedZstdEncoderCached`). Both directions asserted. |
| E6 | VERIFIED (with note) | `TestEncoderProfileUncachedConfigReachesAllEncodePaths` :547: `Encode`, `WriteSidecar`, `EncodeToMetadata` each leave the cache empty (fails under M1). The existing `TestEncoderProfileConfigReachesAllEncodePaths` :178 covers four paths (Encode, EncodeContext, WriteSidecar, EncodeToMetadata). No test calls `RebuildWithIndex`, in the old or new test, so the "rebuild" path named in E6 is not exercised on either profile. `RebuildWithIndex` (`parquet.go:350`) reaches the encoder through the same `encodeWithLevel` call site, so risk is low. The new test omits `EncodeContext`, which `Encode` wraps. |
| E7 | VERIFIED | `TestEncoderProfileConstantValuesAndCachedProfiles` :638. Asserts values 0, 1, 2 and that default and bounded each populate the cache at `CompressionBalanced`. Older tests (:134, :178) are unchanged: `git diff` shows no deleted test lines. |
| E8 | VERIFIED | `TestEncoderProfileUncachedStringAndValidation` :655: `String()` equals `"bounded-memory-uncached"`, `WithEncoderProfile` accepts the new value, and `validate()` error for 200 contains all three constant names. `TestEncoderProfileUnknownValueRejected` :417 checks the `NewConfig` and encode paths return errors; they check only "unknown encoder profile 200", not the three names, but all paths use the single `validate()` message. |
| E9 | VERIFIED | `TestEncoderProfileUncachedWireFormat` :675. `Version == 11` (main's `gin.go:36` is also 11, and `gin.go` is unchanged), `Decode` succeeds, decoded `Config.EncoderProfile` is default. |
| E10 | VERIFIED | `TestEncoderProfileUncachedCompressionNone` :697. Output has the `GINu` prefix, equals default output, cache empty, under 1 MiB allocated. "Builds no encoder" is proven by this proxy plus code: `level == CompressionNone` returns before `sharedZstdEncoder`. |
| E11 | VERIFIED | `TestEncodeUncachedConcurrentMatchesDefault` (`serialize_concurrency_test.go`, new). 8 goroutines, 2 levels, `bytes.Equal` to default golden, run under `-race` (ok x3). It checks the uncached cache key only; default and bounded entries cannot be asserted empty there because the golden encode fills the default entry. Acceptable. Fails under M1. |
| E12 | VERIFIED | Benchmark doc: `git diff main` has 0 removed lines; new section "Uncached bounded-memory profile (issue #83)" has a Host line (darwin/arm64, M2 Max, GOMAXPROCS=4, go1.27.1, klauspost v1.20.0) and rows for bounded-memory and bounded-memory-uncached with allocated per call, retained after the call, time per call. It states "When to use it ... use `EncoderProfileBoundedMemory`" and "N x 44 MB". Godoc on the constant says "Do not use it in a process that encodes often" and "N x 44 MB". README and CHANGELOG say the same. The only matches for `close`/`free` in added non-test text say "does not close it" / "The profile does not close it"; no text claims `Close()` frees memory. CHANGELOG has an "Unreleased" entry for #83. |
| E13 | VERIFIED | `git diff main --stat`: CHANGELOG.md, README.md, benchmark_test.go, docs/..., serialize.go, 2 test files. No change in `cmd/`, `gin.go`, `builder.go`, `parquet.go`. Grep of the added Go lines: no new `With*`/`New*` function, no flag, limiter or semaphore, no `GINConfig` reference. `serialize.go` hunks are only the constant, `String`, `validate`, comments, the bypass in `sharedZstdEncoder` and the one condition in `newZstdEncoder`; no decoder code changed. Test-only helpers (`evictSharedZstdEncoder`, `sharedZstdEncoderCached`) are in `_test.go` files. |
| E14 | PARTIAL-BY-DESIGN | `261006-hgl-DEFERRED.md` exists, records the unmeasured shared decoder retention (D7 out of scope), and holds a draft issue (title, body, acceptance). No GitHub issue was created by the executor. Maintainer sight of the draft and issue creation remain for the orchestrator. |

## Lint (`make lint`)

- Default toolchain (go1.27.1): fails with `typecheck: 1 issues` on BOTH `main` (checked in a `git worktree`) and this branch, with the same output. It is a pre-existing environment problem (golangci-lint v2.11.4 built with an older Go than the local toolchain), not caused by this branch.
- `GOTOOLCHAIN=go1.25.5 make lint`: `0 issues.` on BOTH `main` and this branch. The `check-*` prerequisites and `TestSIMDDocumentationContract` also pass.
- Worktrees removed; `git worktree list` shows only the main checkout; `git status` is clean.

## Warnings

- W1 (E3 test weakness): in `TestEncoderProfileUncachedDoesNotRetainEncoder`, take the baseline after an `evictAllProfiles`, or evict after the warm-up and before `base := settledHeap()`, so a cached encoder would show as growth. Behavior is correct today; this only closes a vacuous-pass path. Non-blocking.
- W2 (E6): `RebuildWithIndex` is not exercised by any profile test. Optional.
- W3 (E4): no explicit GOMAXPROCS sweep; judged not a gap.

## Status

`human_needed`: no expectation failed. Two human items: maintainer approval of the draft issue (E14), and a decision on W1. Task goal is achieved in the code.

_Verified: 2026-10-06 by Claude (gsd-verifier)_

## After verification: simplify and review rounds (orchestrator, 2026-10-06)

Commits `dbb9c71`, `2ddebf7` and `6df7fe9` came after the table above. Changes that touch the expectations:

- E3. The heap test now warms up with the cached bounded-memory profile and evicts it before the baseline. Two mutations fail it: the uncached profile sent through the cache (retained 44215344 bytes), and the uncached encoder kept in a package variable (retained 44225872 bytes). This closes finding W1.
- E2, E6, E11. The uncached profile runs through the existing bounded-memory tests (byte identity, encode paths, one worker). The concurrent test also checks the bounded-memory key.
- E13. `encodeWithLevel` chooses the uncached encoder. `sharedZstdEncoder` returns an error for the uncached profile. The diff still has no new exported name other than the constant, no config field, no CLI flag, no limiter and no decoder change. `Makefile` help text and `CLAUDE.md` gained one clause each.
- E12. The benchmark reports a signed heap difference. The new doc section has numbers from a new run. `git diff main -- docs/encoder-profile-benchmarks.md` removes 0 lines.

Final checks: `make test` 1244 tests, 1 skipped (missing `testdata/test.parquet`, same on `main`). Race run of the profile tests passed. `GOTOOLCHAIN=go1.25.5 make lint` 0 issues. `make lint` with the local go1.27.1 fails with the same typecheck error on `main`.
