---
phase: quick-260417-pvi
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - serialize.go
  - serialize_security_test.go
  - prefix.go
  - gin.go
  - gin_test.go
autonomous: true
requirements:
  - "review-follow-up-T1-C1-extend"
  - "review-follow-up-T2-path-directory"
  - "review-follow-up-wrap-bare-EOF"
  - "review-follow-up-prefix-block-overflow"

must_haves:
  truths:
    - "Front-coded regression test with numEntries=2 rejects oversized PrefixLen on the second entry with ErrInvalidFormat and 'prefix length' context."
    - "Path-directory truncation is table-driven and covers all 5 wrapped fields (PathID, ObservedTypes, Cardinality, Mode, Flags)."
    - "Every binary.Read / io.ReadFull truncation on wire-format data in the listed readers returns an error satisfying errors.Is(err, ErrInvalidFormat) rather than bare io.EOF / io.ErrUnexpectedEOF."
    - "GINConfig.PrefixBlockSize > math.MaxUint16 is rejected at config validation time; NewPrefixCompressor rejects blockSize > math.MaxUint16."
    - "go test -v ./... passes after every task."
  artifacts:
    - path: "serialize_security_test.go"
      provides: "TestReadOrderedStringsRejectsFrontCodedOversizedPrefixLengthSecondEntry, TestReadPathDirectoryWrapsFieldReadErrors (table-driven), smoke tests for wrapped readers, PrefixBlockSize overflow test"
    - path: "serialize.go"
      provides: "errors.Wrapf(ErrInvalidFormat, ...) wrapping on binary.Read / io.ReadFull in readHeader, readStringIndexes, readAdaptiveStringIndexes, readStringLengthIndexes, readNumericIndexes, readNullIndexes, readTrigramIndexes, readHyperLogLogs"
    - path: "prefix.go"
      provides: "NewPrefixCompressor validates blockSize <= math.MaxUint16"
    - path: "gin.go"
      provides: "GINConfig.validate() rejects PrefixBlockSize > math.MaxUint16 and < 1"
  key_links:
    - from: "serialize_security_test.go"
      to: "serialize.go"
      via: "exercises wrapped readers and path-directory truncation"
      pattern: "errors\\.Is\\(err, ErrInvalidFormat\\)"
    - from: "gin.go::validate"
      to: "prefix.go::NewPrefixCompressor"
      via: "both reject PrefixBlockSize > math.MaxUint16"
      pattern: "math.MaxUint16"
---

<objective>
Four Phase-10 review follow-ups on the serialize.go decode path:

1. Extend the existing C1 front-coded regression test so the second entry (numEntries=2) triggers the oversized PrefixLen check after prev has evolved past the FirstTerm.
2. Convert the path-directory truncation test into a table-driven test that covers all 5 wrapped fields in `readPathDirectory`.
3. Wrap remaining bare `io.EOF` / `io.ErrUnexpectedEOF` leaks in listed serialize.go readers with `errors.Wrapf(ErrInvalidFormat, "read <field>: %v", err)` so callers can discriminate truncation from real I/O errors via `errors.Is(err, ErrInvalidFormat)`. Add smoke regression tests.
4. Tighten `GINConfig.PrefixBlockSize` validation to reject `> math.MaxUint16` (and verify `NewPrefixCompressor` does the same), since `prefix.go:152` silently `uint16`-casts block entry counts.

Purpose: Phase-10 reviewers flagged these as decode-path hardening gaps. Closing them prevents silent truncation leaks and overflow bugs ahead of the v1.0 milestone.

Output: Hardened decoders + regression tests for all four items.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@./CLAUDE.md
@.planning/STATE.md
@serialize.go
@serialize_security_test.go
@prefix.go
@gin.go

<interfaces>
<!-- Key patterns and contracts extracted from the codebase so executor works directly from the plan. -->

From serialize.go (error-wrap pattern already established in readPathDirectory, lines 669-691):
```go
if err := binary.Read(r, binary.LittleEndian, &entry.PathID); err != nil {
    return errors.Wrapf(ErrInvalidFormat, "read path id for entry %d: %v", i, err)
}
```

From serialize.go (sentinel errors, lines 73-81):
```go
ErrVersionMismatch = errors.New("version mismatch")
ErrInvalidFormat   = errors.New("invalid format")
```

From serialize.go (C1 front-coded prefix-length check, lines 579-589):
```go
prefixLen := int(blocks[i].Entries[j].PrefixLen)
if prefixLen > len(prev) {
    return nil, errors.Wrapf(
        ErrInvalidFormat,
        "front-coded block %d entry %d prefix length %d exceeds previous term length %d",
        i, j, blocks[i].Entries[j].PrefixLen, len(prev),
    )
}
prev = prev[:prefixLen] + blocks[i].Entries[j].Suffix
```

From prefix.go:20-31 (NewPrefixCompressor — currently only enforces blockSize >= 1):
```go
func NewPrefixCompressor(blockSize int, opts ...PrefixCompressorOption) (*PrefixCompressor, error) {
    if blockSize < 1 {
        return nil, errors.New("blockSize must be at least 1")
    }
    // ... no upper bound
}
```

From prefix.go:152 (the silent-overflow site in WriteCompressedTerms):
```go
if err := binary.Write(w, binary.LittleEndian, uint16(len(block.Entries))); err != nil {
```

From gin.go (GINConfig.validate, lines 670-727 — currently does NOT validate PrefixBlockSize).
From gin.go:629-643 (DefaultConfig) — PrefixBlockSize defaults to 16.

From serialize_security_test.go imports (use these — no new imports needed):
```go
import (
    "bytes"
    "encoding/binary"
    stderrors "errors"
    "io"
    "strings"
    "testing"
    "github.com/pkg/errors"
)
```

From serialize_security_test.go existing helpers (reuse):
- mustWriteOrderedStrings(t, w, values)     // line 168
- Existing pattern: assert errors.Is(err, ErrInvalidFormat) + strings.Contains(err.Error(), "<context>")

Readers in scope for task 2 (exact function boundaries confirmed via Read):
- readHeader: lines 376-411
- readStringIndexes: lines 763-804
- readAdaptiveStringIndexes: lines 870-958
- readStringLengthIndexes: lines 997-1044
- readNumericIndexes: lines 1098-1166
- readNullIndexes: lines 1187-1217
- readTrigramIndexes: lines 1268-1333
- readHyperLogLogs: lines 1358-1392

Wrapping rule: every `return err` (or `return nil, err`) immediately following `binary.Read(...)` or `io.ReadFull(...)` over wire-format bytes becomes:
```go
return errors.Wrapf(ErrInvalidFormat, "read <field>: %v", err)
```
Preserve existing ErrInvalidFormat wraps. Do NOT wrap errors from downstream helpers that already wrap (e.g. `readRGSet`, `readOrderedStrings`, `rejectDuplicateSectionPath`, `NewTrigramIndex`, `NewAdaptiveStringIndex`).
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Extend C1 front-coded regression test + table-drive path-directory truncation test</name>
  <files>serialize_security_test.go</files>
  <behavior>
    - `TestReadOrderedStringsRejectsFrontCodedOversizedPrefixLengthSecondEntry`:
      * Build a front-coded block with `numBlocks=1`, `firstLen=len("ab")`, `FirstTerm="ab"`, `numEntries=2`.
      * Entry 0: `PrefixLen=1`, `Suffix="c"` (valid — prev evolves to "ac", length 2).
      * Entry 1: `PrefixLen=5`, `Suffix=""` (invalid — exceeds evolved prev length 2).
      * `readOrderedStrings(r, 3)` returns an error where `errors.Is(err, ErrInvalidFormat)` is true AND `err.Error()` contains `"prefix length"`.
      * Confirms the guard fires on a non-first-entry iteration where `prev` has evolved.
    - `TestReadPathDirectoryWrapsFieldReadErrors`:
      * Table-driven via `t.Run`. One sub-test per wrapped field: `path id`, `observed types`, `cardinality`, `mode`, `flags`.
      * Each sub-test builds a valid `pathNames` stream via `mustWriteOrderedStrings`, then appends a truncation-aligned number of bytes for the preceding fields (0 bytes for `path id`, 2 bytes for `observed types`, 2+4 bytes for `cardinality`, 2+4+4 bytes for `mode`, 2+4+4+1 bytes for `flags`) and invokes `readPathDirectory`.
      * Each case asserts `errors.Is(err, ErrInvalidFormat)` and `strings.Contains(err.Error(), <wantContext>)` where wantContext matches the `errors.Wrapf(...)` strings in serialize.go:672-687 (e.g. "path id", "observed types", "cardinality", "mode", "flags").
      * Keep the existing `TestReadPathDirectoryWrapsPathIDReadErrors` or fold it into the table cases — prefer folding to keep the suite DRY (delete the old test if replaced).
    - Reference PathEntry field widths from gin.go to pick truncation byte counts. If unsure, use `Read` on gin.go to confirm before sizing the truncation payloads.
  </behavior>
  <action>
    Per project conventions (table-driven tests with `t.Run`, `stderrors.Is`, `strings.Contains`), add both tests to serialize_security_test.go. Reuse existing helpers (`mustWriteOrderedStrings`). Fold the existing `TestReadPathDirectoryWrapsPathIDReadErrors` into the new table-driven test as the `path id` case, and remove the old single-field test to keep the file DRY. No new imports required.
  </action>
  <verify>
    <automated>cd /Users/tazarov/experiments/amikos/custom-gin && go test -v -run 'TestReadOrderedStringsRejectsFrontCodedOversizedPrefixLengthSecondEntry|TestReadPathDirectoryWrapsFieldReadErrors' ./...</automated>
  </verify>
  <done>
    Both new tests pass. Old `TestReadPathDirectoryWrapsPathIDReadErrors` removed (folded into table). Full `go test -v ./...` green.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Wrap bare io.EOF leaks in remaining serialize.go readers + smoke tests</name>
  <files>serialize.go, serialize_security_test.go</files>
  <behavior>
    - After the change, every `return err` (or `return nil, err`) immediately following a `binary.Read` / `io.ReadFull` on wire-format bytes inside the listed readers returns an error satisfying `errors.Is(err, ErrInvalidFormat)`.
    - `TestSerializeReadersWrapTruncationAsInvalidFormat` (smoke): table-driven — for each reader in {readHeader, readStringIndexes, readAdaptiveStringIndexes, readStringLengthIndexes, readNumericIndexes, readNullIndexes, readTrigramIndexes, readHyperLogLogs}, construct a minimally-valid preamble that truncates partway through the first wire-format read and assert `errors.Is(err, ErrInvalidFormat)` (NOT bare `io.EOF`).
    - Exhaustive per-field coverage is NOT required — one truncation point per reader is sufficient to prove wrapping is in place.
  </behavior>
  <action>
    In serialize.go, for each reader listed in Task 2 scope (readHeader, readStringIndexes, readAdaptiveStringIndexes, readStringLengthIndexes, readNumericIndexes, readNullIndexes, readTrigramIndexes, readHyperLogLogs):
    1. Replace every `return err` / `return nil, err` directly after `binary.Read(...)` or `io.ReadFull(...)` on wire-format bytes with `return errors.Wrapf(ErrInvalidFormat, "read <field>: %v", err)` using a stable, human-readable field name (e.g. "header version", "header num row groups", "string index path count", "string index path id", "numeric index path id", "hll precision", etc.).
    2. Do NOT double-wrap: if the line already uses `errors.Wrapf(ErrInvalidFormat, ...)` leave it alone.
    3. Do NOT wrap errors returned by delegating helpers that already own their wrapping (`readRGSet`, `readOrderedStrings`, `rejectDuplicateSectionPath`, `NewTrigramIndex`, `NewAdaptiveStringIndex`). These can keep `return err` / `return nil, err`.
    4. Preserve all existing domain-level `errors.Wrapf(ErrInvalidFormat, ...)` checks (e.g. `maxNumPaths` / `maxBloomWords` guards) and all struct-value-validation errors.

    In serialize_security_test.go, add `TestSerializeReadersWrapTruncationAsInvalidFormat` as a table-driven test. Each case supplies:
    - a name (matching the reader),
    - a `setup func(t *testing.T) ([]byte, *GINIndex)` that builds the minimum prefix to reach the target reader and truncates mid-field,
    - an invocation closure that calls the reader and returns its error.

    Keep setups minimal: readHeader can be tested with just the outer magic + truncated version bytes; the remaining readers can be tested by handing them a buffer that is too short to satisfy the leading uint32 path count. Use a fresh `NewGINIndex()` per case so `rejectDuplicateSectionPath` doesn't interfere. Do not assert message substrings — `errors.Is(err, ErrInvalidFormat)` is enough for this smoke test (avoid coupling to field-name strings that may evolve).
  </action>
  <verify>
    <automated>cd /Users/tazarov/experiments/amikos/custom-gin && go test -v ./... && golangci-lint run ./...</automated>
  </verify>
  <done>
    All existing tests + new smoke test pass. `golangci-lint run` clean. Manual spot-check: `grep -n 'return err$' serialize.go` shows zero occurrences inside the listed readers that directly follow `binary.Read` / `io.ReadFull` on wire-format data (allowed exceptions: delegations to already-wrapping helpers).
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: PrefixBlockSize overflow guard in NewPrefixCompressor and GINConfig.validate</name>
  <files>prefix.go, gin.go, gin_test.go, serialize_security_test.go</files>
  <behavior>
    - `NewPrefixCompressor(math.MaxUint16 + 1)` returns an error whose message mentions `blockSize` and references the max (e.g. "blockSize must be <= 65535").
    - `GINConfig{PrefixBlockSize: math.MaxUint16 + 1, ...}.validate()` returns a non-nil error whose message mentions `PrefixBlockSize` and the max.
    - `GINConfig{PrefixBlockSize: 0, ...}.validate()` is ACCEPTED (0 is the "use default" sentinel — see `orderedStringBlockSize` at serialize.go:420-425 which falls back to `defaultPrefixBlockSize` when `PrefixBlockSize <= 0`). Negative values are rejected.
    - `NewConfig(WithPrefixBlockSize(math.MaxUint16 + 1))` returns an error if we expose such an option; if no `WithPrefixBlockSize` option exists today, DO NOT add one — only validate the struct field in `validate()`.
    - `DefaultConfig()` still passes `validate()` (PrefixBlockSize=16).
  </behavior>
  <action>
    1. In `prefix.go::NewPrefixCompressor`, add an upper-bound check after the `blockSize < 1` guard:
       ```go
       if blockSize > math.MaxUint16 {
           return nil, errors.Errorf("blockSize %d exceeds max %d", blockSize, math.MaxUint16)
       }
       ```
       Add `"math"` to prefix.go's import block.

    2. In `gin.go::(GINConfig).validate`, insert a PrefixBlockSize check. Accept 0 (use-default sentinel) and positive values up to `math.MaxUint16`; reject negative and > MaxUint16:
       ```go
       if c.PrefixBlockSize < 0 {
           return errors.New("prefix block size must be non-negative")
       }
       if c.PrefixBlockSize > math.MaxUint16 {
           return errors.Errorf("prefix block size must be <= %d", math.MaxUint16)
       }
       ```
       Add `"math"` to gin.go's import block if not already present.

    3. Grep for existing `WithPrefixBlockSize` option — if present, propagate the same bounds. If not, do NOT add one (per scope-fidelity rule; reviewer did not request an option, only validation).

    4. Add a regression test `TestNewPrefixCompressorRejectsOverflowBlockSize` in a NEW file `prefix_test.go` (create only if no existing prefix_test.go — otherwise append). Construct with `math.MaxUint16 + 1` and assert error mentions "blockSize".

    5. Add `TestGINConfigValidateRejectsPrefixBlockSizeOverflow` to `gin_test.go` (which already exists per the grep). Two sub-tests:
       * `overflow`: `PrefixBlockSize: math.MaxUint16 + 1` → validate() returns error mentioning "prefix block size".
       * `default_accepted`: `DefaultConfig().validate()` returns nil (sanity).
       * `zero_accepted`: `GINConfig{PrefixBlockSize: 0, BloomFilterSize: 1, BloomFilterHashes: 1, HLLPrecision: 12}.validate()` returns nil.
       * `negative_rejected`: `PrefixBlockSize: -1` → returns error.
       Use existing test structure in gin_test.go as reference for minimal valid config literal.
  </action>
  <verify>
    <automated>cd /Users/tazarov/experiments/amikos/custom-gin && go test -v -run 'TestNewPrefixCompressorRejectsOverflowBlockSize|TestGINConfigValidateRejectsPrefixBlockSizeOverflow' ./... && go test -v ./...</automated>
  </verify>
  <done>
    Both new tests pass. `DefaultConfig().validate()` still returns nil. `gin_test.go:269` existing literal with `PrefixBlockSize: 16` still passes. Full `go test -v ./...` green. `golangci-lint run ./...` clean.
  </done>
</task>

</tasks>

<verification>
- `cd /Users/tazarov/experiments/amikos/custom-gin && go test -v ./...` green after each task.
- `cd /Users/tazarov/experiments/amikos/custom-gin && golangci-lint run ./...` clean after Task 2 and Task 3.
- No regressions in existing serialize tests or property tests.
- No new bare `return err` after `binary.Read` / `io.ReadFull` in the 8 listed readers (delegating helpers excepted).
</verification>

<success_criteria>
- Task 1 tests added and passing; old single-field path-directory truncation test folded into the new table-driven test.
- Task 2: every listed reader wraps its `binary.Read` / `io.ReadFull` failures with `errors.Wrapf(ErrInvalidFormat, ...)`; smoke test asserts `errors.Is(err, ErrInvalidFormat)` for each.
- Task 3: `PrefixBlockSize > math.MaxUint16` rejected in both `NewPrefixCompressor` and `GINConfig.validate`; zero remains accepted as use-default sentinel; DefaultConfig still valid.
- All tests pass; linter clean; zero changes to unrelated code.
</success_criteria>

<output>
After completion, create `.planning/quick/260417-pvi-phase-10-review-follow-ups-t1-subsequent/260417-pvi-SUMMARY.md` per `$HOME/.claude/get-shit-done/templates/summary.md`.
</output>
