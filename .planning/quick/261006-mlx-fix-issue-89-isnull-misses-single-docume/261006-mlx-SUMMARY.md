---
phase: quick-261006-mlx
plan: 01
status: complete
requirements: [ISSUE-89]
commits: [a5c1534, 2666db9, 415add6]
---

# Quick 261006-mlx: IsNull selects one-document row groups lacking the path (#89)

`evaluateIsNull` now returns `NullRGBitmap ∪ AbsentRGs ∪ (root present − path present)`, so a row group whose only document lacks the path is selected. No wire format, builder or serialize change; `Version` stays 11.

## Commits

- a5c1534 `test(query)`: failing tests (RED) and updated oracle/pinned tests
- 2666db9 `fix(query)`: uniform IsNull in `query.go`, godoc, fail-open on missing root evidence
- 415add6 `test(query)`: property test superset vs v1.4.0 rule and exactness on all RGs; SIMD root-presence test
- b43bcf8 `docs`: README rule, `gin.go` AggregateIndex/AbsentRGs comments, CHANGELOG Unreleased

## Verification

- RED confirmed before the fix: repro gave `IsNull($.env)=[2]`, golden `IsNull($.b)=[3]`, parity case `[2]`.
- `make test` (GOTOOLCHAIN=go1.25.5): DONE 1259 tests, 1 skipped (missing `testdata/test.parquet`, pre-existing).
- `make lint` (GOTOOLCHAIN=go1.25.5): 0 issues.
- `go vet -tags simdjson .` clean; `go test -tags simdjson -run IsNullUniformRootPresenceSIMD` PASS (ran, not skipped).
- `git diff df42273 -- testdata/ builder.go serialize.go` empty; `Version = 11`.

## Deviations from Plan

None in code or scope. Environment note: with the default local Go 1.27.1, `make lint` fails with a typecheck error in untouched `logging/attrs.go` (golangci-lint export-data version mismatch). Running with `GOTOOLCHAIN=go1.25.5` (the version in go.mod) gives 0 issues.

## Known Stubs

None.
