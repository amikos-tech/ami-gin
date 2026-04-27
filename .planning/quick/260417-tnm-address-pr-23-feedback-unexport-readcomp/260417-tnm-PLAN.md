---
phase: quick-260417-tnm
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - prefix.go
  - serialize.go
  - gin.go
  - gin_test.go
  - serialize_security_test.go
autonomous: true
requirements:
  - PR-23-FIX-1
  - PR-23-FIX-2
  - PR-23-FIX-3
  - PR-23-FIX-4
  - PR-23-FIX-5

must_haves:
  truths:
    - "ReadCompressedTerms is unexported (readCompressedTerms); public API surface of prefix.go no longer exposes the reader helper."
    - "readFrontCodedOrderedStrings no longer constructs a zero-value PrefixCompressor; it calls the package-level decompress function (or equivalent non-receiver path)."
    - "The redundant post-decompress count check (serialize.go:603-605) is removed because the pre-check at line 598 already guarantees count equality."
    - "writeOrderedStrings short-circuits when len(values) <= 1, writing the raw payload directly without invoking the front-coded encoder."
    - "WithPrefixBlockSize ConfigOption exists on GINConfig, rejects negative values and values > math.MaxUint16, and applies the value to c.PrefixBlockSize."
    - "TestDecodeRejectsCompactPathSectionCorruption has a one-line byte-layout comment above the corruption bytes."
    - "All existing tests continue to pass after every fix."
  artifacts:
    - path: prefix.go
      provides: "lowercase readCompressedTerms; decompressBlocks package-level helper (if Decompress is lifted)"
      contains: "func readCompressedTerms"
    - path: serialize.go
      provides: "updated readFrontCodedOrderedStrings (no zero-value PrefixCompressor, no redundant count check); writeOrderedStrings short-circuit for <=1 value"
      contains: "len(values) <= 1"
    - path: gin.go
      provides: "WithPrefixBlockSize ConfigOption"
      contains: "func WithPrefixBlockSize"
    - path: gin_test.go
      provides: "unit tests for WithPrefixBlockSize (valid, negative, overflow)"
      contains: "TestWithPrefixBlockSize"
    - path: serialize_security_test.go
      provides: "byte-layout comment documenting the corruption offset"
      contains: "Layout: mode(1) | blockCount(4) | firstLen(2)"
  key_links:
    - from: serialize.go (readFrontCodedOrderedStrings)
      to: prefix.go (decompress helper)
      via: "package-level function call (no receiver)"
      pattern: "decompressBlocks\\(|decompressCompressedTerms\\("
    - from: gin.go (WithPrefixBlockSize)
      to: GINConfig.PrefixBlockSize
      via: "ConfigOption closure writing c.PrefixBlockSize"
      pattern: "c\\.PrefixBlockSize = "
---

<objective>
Address five concrete, already-researched fixes from the @claude review on PR #23.
Each fix is landed as its own atomic commit so the series can be bisected.

Purpose: Tighten the public surface and remove fragile construction patterns introduced
in Phase 10's serialization compaction work before the PR merges.
Output:
  - Unexported reader helper in prefix.go
  - Cleaner readFrontCodedOrderedStrings (no zero-value receiver, no redundant check)
  - writeOrderedStrings fast-path for trivial inputs
  - Public WithPrefixBlockSize option with validation
  - Clarifying byte-layout comment in a corruption test
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@CLAUDE.md
@prefix.go
@serialize.go
@gin.go
@gin_test.go
@serialize_security_test.go
@prefix_test.go

<interfaces>
<!-- Key contracts the executor needs. Already extracted from the codebase. -->

From prefix.go:
```go
// Current (to be renamed):
func ReadCompressedTerms(r io.Reader) ([]CompressedTermBlock, error) // prefix.go:179

// Receiver-free method (candidate to lift to package-level func):
func (pc *PrefixCompressor) Decompress(blocks []CompressedTermBlock) []string // prefix.go:110
// Body does NOT reference pc — lifting to a package-level function is safe.

type PrefixCompressor struct { /* ... */ }
type CompressedTermBlock struct {
    FirstTerm string
    Entries   []PrefixEntry
}
type PrefixEntry struct {
    PrefixLen uint16
    Suffix    string
}
```

From serialize.go (call sites to update):
```go
// serialize.go:602 (inside readFrontCodedOrderedStrings):
values := (&PrefixCompressor{}).Decompress(blocks)
if uint32(len(values)) != expectedCount {
    return nil, errors.Wrapf(ErrInvalidFormat, "ordered string count mismatch: got %d want %d", len(values), expectedCount)
}
return values, nil

// serialize.go:440-464 (writeOrderedStrings): needs early return for len(values) <= 1
func writeOrderedStrings(w io.Writer, values []string, blockSize int) error { /* ... */ }
```

From gin.go (template to follow — WithAdaptiveMinRGCoverage at :563):
```go
func WithAdaptiveMinRGCoverage(minCoverage int) ConfigOption {
    return func(c *GINConfig) error {
        if minCoverage < 0 {
            return errors.New("adaptive min RG coverage must be non-negative")
        }
        c.AdaptiveMinRGCoverage = minCoverage
        return nil
    }
}

// GINConfig.PrefixBlockSize already exists at gin.go:357 and is validated in validate()
// at gin.go:691-696 (rejects <0 and >math.MaxUint16).
// Imports include pkg/errors and "math" already.
```

From serialize_security_test.go (test to annotate):
```go
// Line 642 context:
func TestDecodeRejectsCompactPathSectionCorruption(t *testing.T) {
    idx := buildCompactPathFixture(t)
    data := mustEncodeUncompressed(t, idx)

    body, offset := locatePathOrderedStringsOffset(t, data)
    if body[offset] != compactStringModeFrontCoded { /* ... */ }

    // ← ADD COMMENT HERE explaining byte layout
    body[offset+1+4] = 0xff       // line 651
    body[offset+1+4+1] = 0x7f     // line 652
```

Existing callers of Decompress (stay working after the rename/lift):
  - benchmark_test.go:1872  (`pc.Decompress(blocks)`)
  - property_test.go:244    (`pc.Decompress(compressed)`)
  - serialize_security_test.go:1091 (`pc.Decompress(...)`)
  - gin_test.go:1865        (`pc.Decompress(blocks)`)

Decision for Task 2: Keep `(*PrefixCompressor).Decompress` as a thin wrapper around a new
package-level helper so existing call sites continue to compile unchanged, AND
serialize.go can use the package-level helper without a zero-value receiver.
Preferred helper name: `decompressCompressedTerms(blocks []CompressedTermBlock) []string`
(matches the lowercase style of the newly-unexported `readCompressedTerms`).
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Unexport ReadCompressedTerms</name>
  <files>prefix.go</files>
  <action>
Rename `ReadCompressedTerms` to `readCompressedTerms` at prefix.go:179. No behavior
change. This function has zero callers (verified via grep: the only reference is the
definition itself). Do NOT update any other files.

After the edit, commit as:
  `fix(quick-260417-tnm): unexport readCompressedTerms`
Rationale in commit body (one line): the function has no external callers and its
error-wrapping contract diverged from readFrontCodedOrderedStrings, so removing it
from the public surface eliminates a misleading option.
  </action>
  <verify>
    <automated>go build ./... &amp;&amp; go test -run 'TestNewPrefixCompressor' -count=1 ./...</automated>
  </verify>
  <done>
- prefix.go:179 declares `func readCompressedTerms`.
- `go build ./...` succeeds.
- `grep -n ReadCompressedTerms` returns no matches.
- Commit created with conventional message above.
  </done>
</task>

<task type="auto">
  <name>Task 2: Replace zero-value PrefixCompressor + drop redundant count check</name>
  <files>prefix.go, serialize.go</files>
  <action>
Two changes in one commit (they are tightly coupled — both live inside the
"readFrontCodedOrderedStrings" fix).

Step A (prefix.go):
1. Introduce a package-level helper next to the existing Decompress method:
   ```go
   func decompressCompressedTerms(blocks []CompressedTermBlock) []string {
       var result []string
       for _, block := range blocks {
           result = append(result, block.FirstTerm)
           prev := block.FirstTerm
           for _, entry := range block.Entries {
               term := prev[:entry.PrefixLen] + entry.Suffix
               result = append(result, term)
               prev = term
           }
       }
       return result
   }
   ```
2. Reduce `(*PrefixCompressor).Decompress` to a one-line wrapper that delegates:
   ```go
   func (pc *PrefixCompressor) Decompress(blocks []CompressedTermBlock) []string {
       return decompressCompressedTerms(blocks)
   }
   ```
   Keeping the wrapper preserves the four existing call sites
   (benchmark_test.go, property_test.go, serialize_security_test.go, gin_test.go)
   without touching them.

Step B (serialize.go, inside readFrontCodedOrderedStrings ~line 598-606):
Replace:
```go
values := (&PrefixCompressor{}).Decompress(blocks)
if uint32(len(values)) != expectedCount {
    return nil, errors.Wrapf(ErrInvalidFormat, "ordered string count mismatch: got %d want %d", len(values), expectedCount)
}
return values, nil
```
With:
```go
values := decompressCompressedTerms(blocks)
return values, nil
```
The removed check is unreachable: the preceding `if decodedCount != expectedCount`
at line 598 already rejects any mismatch before we reach decompress, and
decompressCompressedTerms emits exactly one output string per block entry plus one
per FirstTerm — i.e. exactly `decodedCount` entries.

Commit as:
  `refactor(quick-260417-tnm): drop zero-value PrefixCompressor in ordered-string decode`
Body should note that the second count check is unreachable post-refactor.
  </action>
  <verify>
    <automated>go build ./... &amp;&amp; go test -run 'TestDecode|TestEncode|TestPrefixCompressor|TestOrderedStrings|TestFrontCoded|TestCompactPath' -count=1 ./...</automated>
  </verify>
  <done>
- prefix.go exports `decompressCompressedTerms` (package-private) and `Decompress` delegates to it.
- serialize.go no longer contains `(&amp;PrefixCompressor{}).Decompress`.
- The redundant post-decompress length check is removed.
- All targeted tests pass.
- Existing call sites of `pc.Decompress` (4 files) remain unchanged.
  </done>
</task>

<task type="auto">
  <name>Task 3: Short-circuit writeOrderedStrings for len(values) &lt;= 1</name>
  <files>serialize.go</files>
  <action>
In `writeOrderedStrings` (serialize.go:440), add an early-return path that skips the
front-coded encoder entirely when there are zero or one values — front-coding a single
value cannot beat raw and allocating the second encoder is pure waste.

After the `if blockSize &lt; 1 { blockSize = defaultPrefixBlockSize }` guard, insert:
```go
if len(values) &lt;= 1 {
    rawPayload, err := encodeRawOrderedStrings(values)
    if err != nil {
        return err
    }
    _, err = w.Write(rawPayload.Bytes())
    return err
}
```
Leave the existing "run both encoders, prefer raw on tie" logic intact for the
&gt;=2 case. Do NOT add the "first two values share no prefix" heuristic — later
blocks can still compress and the review explicitly flagged that heuristic as unsound.

Add a narrow regression test in `serialize_security_test.go` (or a new
`serialize_ordered_strings_test.go`) covering:
  1. round-trip of empty slice through writeOrderedStrings + readOrderedStrings
  2. round-trip of single-element slice ["solo"]
Both cases should decode back to the input exactly. Use the existing
`writeOrderedStrings(..., defaultPrefixBlockSize)` pattern seen at
serialize_security_test.go:805 as the template.

Commit as:
  `perf(quick-260417-tnm): short-circuit writeOrderedStrings for trivial inputs`
  </action>
  <verify>
    <automated>go build ./... &amp;&amp; go test -run 'TestWriteOrderedStrings|TestReadOrderedStrings|TestEncode|TestDecode|TestOrderedStringsShortCircuit' -count=1 ./...</automated>
  </verify>
  <done>
- writeOrderedStrings returns after writing the raw payload when len(values) &lt;= 1.
- New regression test(s) pass and explicitly exercise len==0 and len==1.
- All pre-existing serialize tests still pass.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 4: Add WithPrefixBlockSize ConfigOption</name>
  <files>gin.go, gin_test.go</files>
  <behavior>
- `WithPrefixBlockSize(0)` applied to GINConfig leaves the zero sentinel in place (PrefixBlockSize == 0) and returns nil.
- `WithPrefixBlockSize(16)` sets PrefixBlockSize to 16 and returns nil.
- `WithPrefixBlockSize(-1)` returns an error mentioning "non-negative".
- `WithPrefixBlockSize(math.MaxUint16 + 1)` returns an error mentioning `<= 65535` (MaxUint16).
- `WithPrefixBlockSize(math.MaxUint16)` is accepted.
  </behavior>
  <action>
In gin.go (next to WithAdaptiveMinRGCoverage at :563), add:
```go
// WithPrefixBlockSize configures the block size for front-coded prefix
// compression used in ordered string sections. Zero uses the library default
// (defaultPrefixBlockSize). Values above math.MaxUint16 are rejected because
// the on-wire entry count is encoded as uint16.
func WithPrefixBlockSize(blockSize int) ConfigOption {
    return func(c *GINConfig) error {
        if blockSize &lt; 0 {
            return errors.New("prefix block size must be non-negative")
        }
        if blockSize &gt; math.MaxUint16 {
            return errors.Errorf("prefix block size must be &lt;= %d", math.MaxUint16)
        }
        c.PrefixBlockSize = blockSize
        return nil
    }
}
```
Verify that `math` and `github.com/pkg/errors` are already imported in gin.go
(they are — used by existing validation). No new imports required.

In gin_test.go add `TestWithPrefixBlockSize` covering the behavior block above.
Use table-driven subtests. Apply the option to an empty `&amp;GINConfig{}` via
`WithPrefixBlockSize(tc.input)(cfg)` and assert on the returned error plus the
resulting `cfg.PrefixBlockSize`.

Follow the TDD cycle:
  1. RED: write the test; run; expect compile failure (WithPrefixBlockSize undefined).
  2. GREEN: add WithPrefixBlockSize to gin.go; run; all subtests pass.
  3. REFACTOR: only if the test reveals awkward shape — otherwise skip.

Commit as:
  `feat(quick-260417-tnm): add WithPrefixBlockSize ConfigOption`
  </action>
  <verify>
    <automated>go build ./... &amp;&amp; go test -run 'TestWithPrefixBlockSize|TestGINConfigValidateRejectsPrefixBlockSizeOverflow' -count=1 ./...</automated>
  </verify>
  <done>
- gin.go exports `WithPrefixBlockSize` with the validation rules above.
- gin_test.go has TestWithPrefixBlockSize with subtests covering: valid small, valid MaxUint16, zero sentinel, negative rejection, overflow rejection.
- All tests pass.
  </done>
</task>

<task type="auto">
  <name>Task 5: Document byte layout in TestDecodeRejectsCompactPathSectionCorruption</name>
  <files>serialize_security_test.go</files>
  <action>
In `TestDecodeRejectsCompactPathSectionCorruption` (serialize_security_test.go:642),
add a single-line comment immediately above the two corruption assignments at
lines 651-652. The target layout:

```go
if body[offset] != compactStringModeFrontCoded {
    t.Fatalf(...)
}

// Layout: mode(1) | blockCount(4) | firstLen(2) — corrupt the firstLen uint16 bytes.
body[offset+1+4] = 0xff
body[offset+1+4+1] = 0x7f
```

Pure documentation change. Do NOT edit any other tests or touch the sibling
`TestDecodeRejectsCompactTermSectionCorruption` (scope discipline per PR feedback).

Commit as:
  `docs(quick-260417-tnm): document compact-path corruption byte layout`
  </action>
  <verify>
    <automated>go build ./... &amp;&amp; go test -run 'TestDecodeRejectsCompactPathSectionCorruption' -count=1 ./...</automated>
  </verify>
  <done>
- The comment `// Layout: mode(1) | blockCount(4) | firstLen(2) ...` appears directly above the `body[offset+1+4] = 0xff` line.
- Test still passes unchanged.
- No other files modified.
  </done>
</task>

</tasks>

<verification>
After all 5 tasks land, run the full local quality gate:
- `go build ./...`
- `go test -count=1 ./...`
- `make lint` (if golangci-lint is available)

Inspect the git log to confirm 5 conventional commits in order:
  fix  — Task 1 (unexport)
  refactor — Task 2 (drop zero-value PrefixCompressor)
  perf — Task 3 (short-circuit)
  feat — Task 4 (WithPrefixBlockSize)
  docs — Task 5 (byte-layout comment)
</verification>

<success_criteria>
- Five atomic commits, each compiling and passing tests in isolation.
- `grep -n ReadCompressedTerms` returns no results anywhere in the repo.
- `grep -n 'PrefixCompressor{}' serialize.go` returns no results.
- `writeOrderedStrings` contains a `len(values) <= 1` guard before the dual-encoder logic.
- `go doc github.com/amikos-tech/ami-gin WithPrefixBlockSize` renders the new option.
- TestDecodeRejectsCompactPathSectionCorruption contains the byte-layout comment.
- `go test -count=1 ./...` is green end-to-end.
</success_criteria>

<output>
After completion, create `.planning/quick/260417-tnm-address-pr-23-feedback-unexport-readcomp/260417-tnm-SUMMARY.md`
summarizing: commits produced, files touched, any deviations, and follow-up items (expected: none).
</output>
