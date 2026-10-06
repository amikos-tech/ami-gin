---
phase: quick-261006-hgl
plan: 01
subsystem: serialize
tags: [zstd, encoder-profile, memory, issue-83]
key-files:
  modified:
    - serialize.go
    - serialize_profile_test.go
    - serialize_concurrency_test.go
    - benchmark_test.go
    - docs/encoder-profile-benchmarks.md
    - README.md
    - CHANGELOG.md
  created:
    - .planning/quick/261006-hgl-validate-and-address-issue-83-bounded-me/261006-hgl-DEFERRED.md
decisions:
  - "New EncoderProfileBoundedMemoryUncached = 2; sharedZstdEncoder builds a fresh encoder and skips the cache and its mutex"
  - "No new option, GINConfig field, CLI flag, limiter or decoder change (E13)"
completed: 2026-10-06
---

# Quick 261006-hgl: EncoderProfileBoundedMemoryUncached (issue #83)

Third encoder profile that builds a one-worker, low-memory zstd encoder per call and drops it, so level-15 retained heap after the call is near zero (0.001 MB measured, against 42.2 to 42.5 MB for the cached bounded profile).

## Commits

- 3980527 feat(serialize): add EncoderProfileBoundedMemoryUncached encoder profile (#83), includes tests E1-E11
- 8f67e4b docs: uncached benchmarks, benchmark_test.go change, new doc section
- 45042c6 docs: README and CHANGELOG ("Unreleased")
- dfe83d2 test: lint fix for the heap test helper

## Deviations

- [Rule 3] The plan's helper returned the byte length. `unparam` in golangci-lint rejected it, so the helper returns nothing. Still `//go:noinline`.
- Godoc, README and CHANGELOG say "about 2 ms" for construction, not the "about 4 ms" in the plan. The measured extra time at level 15 on the small fixture was about 1.8 ms (2.72 ms uncached minus 0.92 ms cached). Addendum 5 asked for this correction. The 44 MB figure holds (44.7 MB measured).
- TDD: tests and code were written in one pass, and the RED run was not done separately. The E3 test has a control half (cached bounded profile must grow the heap by over 20 MiB), which shows the test sees a retained encoder.
- The E4 extra run at GOMAXPROCS(4) was skipped (optional in the plan).
- DEFERRED.md does not cite decoder concurrency. `decoder_options.go` does not exist; the decoder concurrency default was not verified (addendum 1).

## Verification (real runs)

- `go build ./...`: ok.
- `make test`: PASS, 1230 tests, 1 skipped (missing testdata/test.parquet, pre-existing), 38 s.
- `go test -race -run 'Uncached|EncoderProfile' ./...`: ok.
- `go test -count=5 -run TestEncoderProfileUncachedDoesNotRetainEncoder .`: ok, stable.
- `make lint`: the `make` target fails in this environment with a typecheck error in `logging/attrs.go` (`export data version 4 is greater than maximum supported version 2`). The cause is golangci-lint v2.11.4 built on an older Go, run under the Go 1.27.1 toolchain. It is not from this change. With `GOTOOLCHAIN=go1.25.5` (the go.mod version) the same linter version reports `0 issues`. The first run under that toolchain found one real `unparam` issue, which was fixed.
- E13 check: `git diff --stat main -- cmd gin.go parquet.go s3.go` is empty. The old doc tables have 0 removed lines (`+31 -0`).

## Benchmark (darwin/arm64, Apple M2 Max, GOMAXPROCS=4, go1.27.1, klauspost v1.20.0, 10x)

Raw output saved in the scratchpad as `bench.txt`. Numbers are in the new doc section. Retained heap at L15: cached 42.23 MB (small), 42.50 MB (highcard); uncached 0.0013 MB for both.

## Known Stubs

None.

## Self-Check: PASSED

Files exist and the four commits are in `git log`.
