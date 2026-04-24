---
phase: quick-260417-pvi
plan: 01
subsystem: serialization
tags: [hardening, decode-path, review-follow-up, phase-10]
requires: []
provides:
  - "errors.Is(err, ErrInvalidFormat) discriminates truncation in all wire-format readers"
  - "GINConfig rejects PrefixBlockSize > math.MaxUint16"
  - "NewPrefixCompressor rejects blockSize > math.MaxUint16"
  - "Table-driven regression suite for readPathDirectory field truncation"
  - "Second-entry regression test for front-coded prefix-length guard"
affects:
  - serialize.go
  - serialize_security_test.go
  - prefix.go
  - prefix_test.go
  - gin.go
  - gin_test.go
tech-stack:
  added: []
  patterns:
    - "errors.Wrapf(ErrInvalidFormat, \"read <field>: %v\", err) on every binary.Read / io.ReadFull truncation in wire-format readers"
    - "Use-default sentinel (0) + explicit upper-bound guard for fields encoded as fixed-width ints on the wire"
key-files:
  created:
    - prefix_test.go
  modified:
    - serialize.go
    - serialize_security_test.go
    - prefix.go
    - gin.go
    - gin_test.go
decisions:
  - "PrefixBlockSize=0 stays accepted as the use-default sentinel (orderedStringBlockSize falls back to defaultPrefixBlockSize). Only negative and > MaxUint16 are rejected."
  - "Delegating helpers (readRGSet, readOrderedStrings, rejectDuplicateSectionPath, NewTrigramIndex) keep bare `return err` because they already own ErrInvalidFormat wrapping."
  - "Did not add a WithPrefixBlockSize option; reviewer asked only for struct-field validation, not a new functional option."
metrics:
  duration: "~15 min"
  completed: "2026-04-17"
commits:
  - c6600cf  # Task 1: extend serialize security coverage
  - 0c83c6c  # Task 2: wrap bare io.EOF leaks in serialize readers
  - 8eb78f5  # Task 3: reject PrefixBlockSize > MaxUint16
---

# Quick Task 260417-pvi Plan 01: Phase 10 Review Follow-ups Summary

**One-liner:** Harden the GIN serialize decode path so truncated wire data surfaces as `ErrInvalidFormat` instead of bare `io.EOF`, extend front-coded and path-directory regression coverage, and reject `PrefixBlockSize > math.MaxUint16` at config/constructor time.

## Objective

Close four Phase-10 reviewer follow-ups on the serialize decode path:

1. Front-coded regression with `numEntries=2` that triggers the `PrefixLen` guard on the second entry (after `prev` has evolved).
2. Table-driven truncation test for `readPathDirectory` covering all five wrapped `PathEntry` fields.
3. Wrap every bare `io.EOF` / `io.ErrUnexpectedEOF` leak in the listed `serialize.go` readers with `errors.Wrapf(ErrInvalidFormat, "read <field>: %v", err)` plus smoke tests.
4. Reject `GINConfig.PrefixBlockSize > math.MaxUint16` in both `NewPrefixCompressor` and `GINConfig.validate()` — `WriteCompressedTerms` casts block entry counts to `uint16` and silently overflows otherwise.

## What Shipped

### Task 1: Regression coverage (`c6600cf`)

- `TestReadOrderedStringsRejectsFrontCodedOversizedPrefixLengthSecondEntry` — constructs a front-coded block with `FirstTerm="ab"`, `numEntries=2`, entry 0 `{PrefixLen:1, Suffix:"c"}` (prev evolves to `"ac"`, len 2), entry 1 `{PrefixLen:5, Suffix:""}` (exceeds evolved prev). Asserts `errors.Is(err, ErrInvalidFormat)`, `"prefix length"` context, and `"entry 1"` to confirm the guard fired on the non-first-entry iteration.
- `TestReadPathDirectoryWrapsFieldReadErrors` — table-driven, one `t.Run` per wrapped field (`path id`, `observed types`, `cardinality`, `mode`, `flags`). Each sub-test pads the `pathNames` stream with the byte count for preceding fields and asserts `errors.Is(err, ErrInvalidFormat)` + substring match for the field-specific context.
- Folded the old single-field `TestReadPathDirectoryWrapsPathIDReadErrors` into the table as the `path id` case; removed the original test to keep the suite DRY.

### Task 2: Wrap bare `io.EOF` leaks (`0c83c6c`)

- In `serialize.go`, every `return err` / `return nil, err` immediately after a `binary.Read` / `io.ReadFull` on wire-format bytes inside `readHeader`, `readStringIndexes`, `readAdaptiveStringIndexes`, `readStringLengthIndexes`, `readNumericIndexes`, `readNullIndexes`, `readTrigramIndexes`, and `readHyperLogLogs` now returns `errors.Wrapf(ErrInvalidFormat, "read <field>: %v", err)`.
- Delegating helpers (`readRGSet`, `readOrderedStrings`, `rejectDuplicateSectionPath`, `NewTrigramIndex`) were left untouched — they already own `ErrInvalidFormat` wrapping.
- Existing domain-level `ErrInvalidFormat` checks (e.g. `maxNumPaths` / `maxBloomWords` guards) preserved.
- `TestSerializeReadersWrapTruncationAsInvalidFormat` — table-driven smoke test, one truncation per reader. Each case asserts `errors.Is(err, ErrInvalidFormat)` AND the error does NOT satisfy `errors.Is(err, io.EOF)` / `io.ErrUnexpectedEOF` (because `%v` formatting drops the sentinel from the chain, which is the intended behavior).

### Task 3: `PrefixBlockSize` overflow guard (`8eb78f5`)

- `prefix.go::NewPrefixCompressor`: adds `if blockSize > math.MaxUint16 { return ..., errors.Errorf("blockSize %d exceeds max %d", blockSize, math.MaxUint16) }` after the existing `< 1` guard. Added `"math"` to the import block.
- `gin.go::(GINConfig).validate`: adds both `< 0` and `> math.MaxUint16` guards for `PrefixBlockSize`. `0` is deliberately accepted as the use-default sentinel (see `orderedStringBlockSize` at `serialize.go:~423`, which falls back to `defaultPrefixBlockSize` when `PrefixBlockSize <= 0`). Added `"math"` to the import block.
- Did NOT add `WithPrefixBlockSize` option — none existed and the reviewer only requested validation.
- `prefix_test.go` (new file): `TestNewPrefixCompressorRejectsOverflowBlockSize` (asserts `blockSize` and `65535` in the error), `TestNewPrefixCompressorAcceptsMaxUint16` (boundary).
- `gin_test.go`: `TestGINConfigValidateRejectsPrefixBlockSizeOverflow` with sub-tests `default_accepted`, `zero_accepted`, `negative_rejected`, `overflow_rejected`, `maxuint16_accepted`. Added `"math"` to the import block.

## Verification

| Check | Result |
|-------|--------|
| `go test -v ./...` after Task 1 | green |
| `go test -v ./...` after Task 2 | green |
| `go test -v ./...` after Task 3 | green |
| `golangci-lint run ./...` after Task 2 | 0 issues |
| `golangci-lint run ./...` after Task 3 | 0 issues |
| Existing `TestNewBuilderAllowsLegacyConfigLiteralWhenAdaptiveDisabled` (uses `PrefixBlockSize: 16`) | still green |
| `DefaultConfig().validate()` | still returns nil |

## Deviations from Plan

### 1. Path-directory truncation byte counts (plan hint vs. actual struct layout)

- **Found during:** Task 1 sizing of the table cases.
- **Issue:** The plan's hinted padding sizes (`2 bytes for observed types, 2+4 bytes for cardinality, 2+4+4 bytes for mode, 2+4+4+1 bytes for flags`) assumed `ObservedTypes` was a 4-byte field. The actual `PathEntry` wire layout is `PathID uint16 (2) + ObservedTypes uint8 (1) + Cardinality uint32 (4) + Mode uint8 (1) + Flags uint8 (1)`.
- **Fix:** Used the actual struct widths: `{0, 2, 3, 7, 8}` bytes of preceding padding for the five sub-tests. Comments in the test document the layout with a reference back to `gin.go:102-109`.
- **Files modified:** `serialize_security_test.go`.
- **Commit:** `c6600cf`.

### 2. Task 1 TDD gate — tests pass on add

- **Found during:** Task 1 planning.
- **Issue:** Task 1 is marked `tdd="true"`, but both new tests exercise guards that are already implemented (`serialize.go:580` for the front-coded prefix-length check and `serialize.go:672-687` for the path-directory wrapping). The plan body explicitly frames Task 1 as "extend regression coverage" and "table-drive" an existing test — not a new feature.
- **Resolution:** Added the tests and confirmed they pass on the first run without code changes. This is a coverage extension, not a RED/GREEN cycle. No new implementation was required.
- **Files modified:** `serialize_security_test.go`.
- **Commit:** `c6600cf`.

### 3. `GINConfig.validate()` accepts `PrefixBlockSize=0`

- **Found during:** Task 3 action planning.
- **Issue:** The plan explicitly spells out that 0 must stay accepted as the use-default sentinel. I verified by re-reading `orderedStringBlockSize` in `serialize.go` — it falls back to `defaultPrefixBlockSize` when `PrefixBlockSize <= 0`. `DefaultConfig()` still uses the real default of 16, so this change doesn't silently move anything.
- **Not a deviation**, just noting that `zero_accepted` is an intentional part of the new validator and is covered by the test.

## Auth Gates

None.

## Known Stubs

None. Every code change is wired end-to-end and covered by tests.

## Commits

| Task | Type | Hash | Subject |
|------|------|------|---------|
| 1 | test | `c6600cf` | extend serialize security coverage |
| 2 | fix  | `0c83c6c` | wrap bare io.EOF leaks in serialize readers |
| 3 | fix  | `8eb78f5` | reject PrefixBlockSize > MaxUint16 |

## Self-Check

- `serialize.go` — exists, 8 readers hardened.
- `serialize_security_test.go` — exists, new tests present.
- `prefix.go` — exists, `math.MaxUint16` guard present in `NewPrefixCompressor`.
- `prefix_test.go` — exists (new file).
- `gin.go` — exists, `PrefixBlockSize` guard present in `validate`.
- `gin_test.go` — exists, new overflow test present.
- Commit `c6600cf` — present in git log.
- Commit `0c83c6c` — present in git log.
- Commit `8eb78f5` — present in git log.

## Self-Check: PASSED
