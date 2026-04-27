# Phase 2: Security Hardening - Research

**Researched:** 2026-03-26
**Domain:** Binary deserialization safety, bounds checking, denial-of-service prevention
**Confidence:** HIGH

## Summary

Phase 2 hardens the `Decode()` deserialization path in `serialize.go` against crafted or corrupted `.gin` files. The scope is narrow and well-defined: 5 unbounded allocation sites need bounds checks, the header needs version validation, and the legacy fallback decompression branch should be removed.

The codebase already has a precedent for bounds checking (`maxConfigSize = 1 << 20` in `readConfig`) and uses `github.com/pkg/errors` throughout. The work is purely additive guards in existing functions, plus two new sentinel errors. No new dependencies, no API changes, no new files beyond test additions.

**Primary recommendation:** Follow the existing `maxConfigSize` pattern. Add constants at the top of `serialize.go`, add guard checks at each allocation site, validate version in `readHeader`, and remove the legacy decompression branch. Two commits as decided in CONTEXT.md.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Hybrid approach -- header-derived bounds where structural anchors exist, absolute constants where not
- **D-02:** `readDocIDMapping` bounded by `NumRowGroups` (exact anchor: DocIDs map to row-group positions in `[0, NumRowGroups)`)
- **D-03:** `readRGSet` bounded by `NumRowGroups` (roaring bitmap size derivable from row group count) or a generous absolute constant (e.g., 16MB) to avoid parameter threading
- **D-04:** `readPathDirectory` bounded by `NumPaths <= 65535` (PathID is uint16 -- structurally impossible to exceed)
- **D-05:** `readStringIndexes` bounded by absolute constant `maxTermsPerPath` (e.g., 1,000,000 -- generous given default CardinalityThreshold is 10,000)
- **D-06:** `readTrigramIndexes` bounded by absolute constant `maxTrigramsPerPath` (e.g., 10,000,000 -- covers extreme unicode FTS; ASCII ceiling is ~857K)
- **D-07:** Follow `maxConfigSize` precedent for constant naming and documentation
- **D-08:** Exact match -- reject if `Header.Version != Version`. No installed base exists.
- **D-09:** Remove the legacy fallback branch in `Decode()` (lines ~181-193)
- **D-10:** When a real format split happens (Version bumps to 4), revisit range-based acceptance
- **D-11:** Add two sentinel errors: `ErrVersionMismatch` and `ErrInvalidFormat`
- **D-12:** Declare as `var ErrVersionMismatch = errors.New("version mismatch")` and `var ErrInvalidFormat = errors.New("invalid format")` using `github.com/pkg/errors`
- **D-13:** Wrap at detection point: `errors.Wrap(ErrVersionMismatch, "read header")`
- **D-14:** Bounds violations use `errors.Errorf` with actual and max values, wrapped with `ErrInvalidFormat`
- **D-15:** 2 commits: (1) version validation + legacy removal (SEC-02), (2) bounds checks (SEC-01 + SEC-03)
- **D-16:** SEC-01 and SEC-03 grouped because they address the same concern

### Claude's Discretion
- Exact constant values for `maxTermsPerPath`, `maxTrigramsPerPath`, `maxRGSetSize` -- domain math should guide, documented with comments
- Whether to thread `numRGs` as a parameter into `readRGSet` or use an absolute constant -- either approach is acceptable
- Test structure -- how to organize crafted-payload tests (table-driven, per-site, fuzz)
- Whether `ErrInvalidFormat` wraps further sub-categories or stays flat

### Deferred Ideas (OUT OF SCOPE)
None -- discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SEC-01 | Deserialization bounds checks added for all 5 unbounded allocation sites | Vulnerability catalog in CONCERNS.md identifies all 5 sites. Bounds math below provides constant values. Existing `maxConfigSize` pattern provides template. |
| SEC-02 | Binary format version validation added to `Decode()` -- reject unknown versions | `readHeader` (serialize.go:275) reads version but never validates. `Version` constant is `uint16(3)` in gin.go:7. Exact match per D-08. |
| SEC-03 | Maximum size limits enforced during deserialization to prevent memory exhaustion | Same 5 sites as SEC-01. The bounds checks ARE the size limits. Grouped per D-16. |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **Error handling:** Use `github.com/pkg/errors` -- `errors.New()`, `errors.Wrap()`, `errors.Errorf()`
- **No breaking API changes:** Existing `Decode()` signature must stay. Sentinel errors are additive.
- **Conventional commits:** `fix(serialize):` prefix per CLAUDE.md
- **Minimal comments:** Constants should have brief domain-math comments, not verbose explanations
- **Linting:** `golangci-lint run` must pass. Linters: `dupword`, `gocritic`, `mirror`, `staticcheck`
- **Import order:** standard, third-party, project prefix (enforced by `gci`)
- **Test command:** `go test -v` or `gotestsum` via Makefile

## Standard Stack

No new dependencies required. All work uses existing project infrastructure:

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/pkg/errors` | v0.9.1 | Error creation and wrapping | Already used throughout; project mandates it |
| `testing` (stdlib) | Go 1.26.1 | Test framework | Standard library, already used |
| `encoding/binary` (stdlib) | Go 1.26.1 | Binary read/write | Already used in serialize.go |

### Supporting
No additional libraries needed. No installation step required.

## Architecture Patterns

### Recommended Constants Structure

Place all bounds constants together at the top of `serialize.go`, following the existing `maxConfigSize` precedent:

```
serialize.go (top of file, after imports):
├── maxConfigSize    = 1 << 20     # existing (1MB)
├── maxRGSetSize     = ...         # new
├── maxTermsPerPath  = ...         # new
├── maxTrigramsPerPath = ...       # new
├── maxDocIDMappingLen = ...       # new (or header-derived)
├── maxNumPaths      = 65535       # new (uint16 ceiling, structural)
```

### Pattern 1: Header-Derived Bounds Check
**What:** Use values from the already-parsed header to compute bounds for downstream allocations
**When to use:** When a structural relationship exists between the header field and the allocation size
**Example:**
```go
// readDocIDMapping -- numDocs derived from Header.NumDocs (already read)
func readDocIDMapping(r io.Reader, maxDocs uint64) ([]DocID, error) {
    var numDocs uint64
    if err := binary.Read(r, binary.LittleEndian, &numDocs); err != nil {
        return nil, err
    }
    if numDocs > maxDocs {
        return nil, errors.Wrapf(ErrInvalidFormat, "docid mapping count %d exceeds max %d", numDocs, maxDocs)
    }
    // ... rest unchanged
}
```

### Pattern 2: Absolute Constant Bounds Check
**What:** Use a generous fixed constant when no structural anchor exists
**When to use:** When the allocation size has no relationship to header values, or when threading parameters would complicate the code
**Example:**
```go
// readStringIndexes -- numTerms per path bounded by absolute constant
var numTerms uint32
if err := binary.Read(r, binary.LittleEndian, &numTerms); err != nil {
    return err
}
if numTerms > maxTermsPerPath {
    return errors.Wrapf(ErrInvalidFormat, "terms count %d for path %d exceeds max %d", numTerms, pathID, maxTermsPerPath)
}
```

### Pattern 3: Sentinel Error Declaration
**What:** Package-level error variables for programmatic error checking
**When to use:** When callers need `errors.Is()` matching
**Example:**
```go
var (
    ErrVersionMismatch = errors.New("version mismatch")
    ErrInvalidFormat   = errors.New("invalid format")
)
```

### Pattern 4: Version Validation in readHeader
**What:** Strict version check after reading the version field
**When to use:** Immediately after parsing the version field in `readHeader`
**Example:**
```go
if idx.Header.Version != Version {
    return errors.Wrapf(ErrVersionMismatch, "got version %d, expected %d", idx.Header.Version, Version)
}
```

### Anti-Patterns to Avoid
- **Wrapping sentinel with `errors.Errorf`:** Use `errors.Wrapf(ErrInvalidFormat, ...)` not `errors.Errorf(...)` for bounds violations. The latter loses the sentinel for `errors.Is()` matching.
- **Changing function signatures unnecessarily:** If an absolute constant suffices (e.g., `maxRGSetSize`), do not thread header values as parameters.
- **Over-tight bounds:** Constants must be generous. The goal is DoS prevention (reject 4GB allocations), not input validation (reject valid-but-large indexes).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Error wrapping with sentinel | Custom error types with `Is()` method | `errors.Wrapf(ErrInvalidFormat, ...)` | `pkg/errors` wrapping works with `errors.Is()` out of the box |
| Fuzzing framework | Custom mutation-based fuzzer | `testing.F` (Go native fuzz) | stdlib fuzzer finds edge cases the planner cannot predict |

## Common Pitfalls

### Pitfall 1: errors.Wrap vs errors.Wrapf with Sentinel Errors
**What goes wrong:** Using `errors.Wrap(ErrInvalidFormat, fmt.Sprintf(...))` instead of `errors.Wrapf(ErrInvalidFormat, ...)`. The `fmt.Sprintf` import is unnecessary and adds a dependency.
**Why it happens:** Habit from other codebases.
**How to avoid:** Use `errors.Wrapf(ErrInvalidFormat, "format %d", val)` consistently.
**Warning signs:** `fmt` imported in serialize.go when it was not imported before.

### Pitfall 2: errors.Is() Chain With pkg/errors
**What goes wrong:** `errors.Is(err, ErrInvalidFormat)` may not work if the sentinel is wrapped with `errors.Wrap` from `pkg/errors` and the caller uses `stdlib errors.Is()`.
**Why it happens:** `pkg/errors` and `stdlib errors` have different wrapping semantics. `pkg/errors.Wrap` creates a `withMessage` type; `stdlib errors.Is` uses `Unwrap()`.
**How to avoid:** `pkg/errors` v0.9.1 implements `Unwrap()` on its wrapper types, so `stdlib errors.Is()` works correctly with `pkg/errors.Wrap(sentinel, msg)`. This is safe. Verify in tests.
**Warning signs:** Tests that check `errors.Is()` failing unexpectedly.

### Pitfall 3: Forgetting a Bounds Check Site
**What goes wrong:** One of the 5 allocation sites is missed, leaving a DoS vector.
**Why it happens:** The sites are spread across different read functions.
**How to avoid:** Checklist of all 5 sites from CONCERNS.md:
1. `readRGSet` (line 69): `make([]byte, dataLen)`
2. `readPathDirectory` (line 335): `make([]byte, pathLen)` -- already bounded by uint16 max (65535), but the loop count `NumPaths` needs guarding
3. `readStringIndexes` (lines 437-438): `make([]string, numTerms)` + `make([]*RGSet, numTerms)`
4. `readTrigramIndexes` (line 757): loop `numTrigrams` times creating trigram entries
5. `readDocIDMapping` (line 838): `make([]DocID, numDocs)`
**Warning signs:** `grep -n 'make(' serialize.go` should show a bounds check before every allocation controlled by a value from the binary stream.

### Pitfall 4: Legacy Branch Removal Breaking Existing Tests
**What goes wrong:** Removing the legacy fallback branch (lines 181-193) could break tests that rely on deserializing data without magic bytes.
**Why it happens:** If any test creates data in the old format.
**How to avoid:** Search for tests calling `Decode` with raw data (no magic prefix). Currently, all tests use `Encode()` which always produces magic-prefixed data. No existing tests exercise the legacy branch.
**Warning signs:** Tests failing after the legacy branch is removed.

### Pitfall 5: readRGSet Called from Multiple Places
**What goes wrong:** `readRGSet` is called from `readNullIndexes` (2 calls per path), `readStringIndexes` (1 per term), and `readTrigramIndexes` (1 per trigram). The bounds check in `readRGSet` must work for all callers.
**Why it happens:** `readRGSet` is a shared helper.
**How to avoid:** If using an absolute constant for `maxRGSetSize`, it applies uniformly. If threading `numRGs` from the header, it must be passed through all callers.
**Warning signs:** Signature change in `readRGSet` requires changes in 3 other functions.

### Pitfall 6: NumPaths Exceeding uint16 Range
**What goes wrong:** `Header.NumPaths` is `uint32` but `PathID` is `uint16`. If a crafted header claims `NumPaths = 4_000_000_000`, the loop in `readPathDirectory` will iterate billions of times, each attempting to read fixed-size structs from the reader.
**Why it happens:** The uint32/uint16 mismatch between header and PathID types.
**How to avoid:** Guard `NumPaths <= 65535` (uint16 max) immediately in `readPathDirectory` or in `readHeader` itself. This is D-04.
**Warning signs:** Extremely long decode times without OOM (the loop body allocates small amounts per iteration).

## Code Examples

### Sentinel Error Declaration
```go
// Source: serialize.go (new, following pkg/errors convention used throughout)
var (
    ErrVersionMismatch = errors.New("version mismatch")
    ErrInvalidFormat   = errors.New("invalid format")
)
```

### Version Validation in readHeader
```go
// Source: serialize.go readHeader() -- add after reading version
if idx.Header.Version != Version {
    return errors.Wrapf(ErrVersionMismatch, "got version %d, expected %d", idx.Header.Version, Version)
}
```

### Bounds Check Pattern (absolute constant)
```go
// Source: serialize.go readStringIndexes() -- add after reading numTerms
if numTerms > maxTermsPerPath {
    return errors.Wrapf(ErrInvalidFormat, "terms count %d for path %d exceeds max %d", numTerms, pathID, maxTermsPerPath)
}
```

### Bounds Check Pattern (header-derived)
```go
// Source: serialize.go readDocIDMapping() -- add after reading numDocs
if numDocs > maxDocs {
    return errors.Wrapf(ErrInvalidFormat, "docid mapping count %d exceeds max %d", numDocs, maxDocs)
}
```

### Legacy Branch Removal
```go
// BEFORE: Decode() default case (lines 181-193)
default:
    // Legacy format: try zstd decompression without magic (backward compatibility)
    decoder, err := zstd.NewReader(nil)
    ...

// AFTER: Decode() default case
default:
    return nil, errors.Wrapf(ErrInvalidFormat, "unrecognized magic bytes: %q", magic)
```

## Allocation Site Bounds Analysis

Detailed domain math for each of the 5 sites, to guide constant selection:

### Site 1: readRGSet (serialize.go:69)
**Allocation:** `make([]byte, dataLen)` where `dataLen` is `uint32` from stream
**Domain math:** A roaring bitmap for N row groups has worst-case serialized size of about `8KB + 2*N` bytes (8KB per full bitmap container, plus overhead). For 1M row groups, this is under 2MB. For 100M row groups (extreme), under 200MB.
**Recommended constant:** `maxRGSetSize = 16 << 20` (16MB). This covers any realistic row-group count while preventing 4GB allocations from `uint32` max.
**Alternative:** Thread `numRGs` from header and compute `max(numRGs * 2 + 8192, 16384)`. This is tighter but requires changing `readRGSet` signature and all 3+ callers.

### Site 2: readPathDirectory (serialize.go:326)
**Allocation:** Loop bounded by `idx.Header.NumPaths` (uint32), each iteration does `make([]byte, pathLen)` where `pathLen` is uint16.
**Domain math:** `PathID` is uint16, so there can be at most 65535 paths. The per-path `pathLen` is already uint16, capping individual allocations at 64KB. The real risk is the loop count.
**Recommended constant:** `maxNumPaths = 65535` (uint16 max). Check `idx.Header.NumPaths <= maxNumPaths` before the loop.

### Site 3: readStringIndexes (serialize.go:437-438)
**Allocation:** `make([]string, numTerms)` + `make([]*RGSet, numTerms)` where `numTerms` is uint32.
**Domain math:** Default `CardinalityThreshold` is 10,000. Paths exceeding this threshold get bloom-only treatment (no string index). So realistic term counts are under 10,000. However, custom configs can set higher thresholds. 1,000,000 is extremely generous.
**Recommended constant:** `maxTermsPerPath = 1_000_000` (per D-05).

### Site 4: readTrigramIndexes (serialize.go:748-757)
**Allocation:** Loop `numTrigrams` times, each creating a trigram key + `readRGSet`.
**Domain math:** For ASCII text with N=3, the ceiling is `128^3 = 2,097,152` possible trigrams (but practically far fewer). For Unicode with N=3, the theoretical space is enormous, but actual trigrams observed in real text are bounded. 10,000,000 is generous for Unicode FTS.
**Recommended constant:** `maxTrigramsPerPath = 10_000_000` (per D-06).

### Site 5: readDocIDMapping (serialize.go:838)
**Allocation:** `make([]DocID, numDocs)` where `numDocs` is uint64.
**Domain math:** Each DocID maps to a row-group position. The mapping length should equal the number of documents. `Header.NumDocs` is uint64. The mapping should not exceed `NumDocs` (or `NumDocs + some tolerance`).
**Recommended approach:** Pass `Header.NumDocs` as the bound (per D-02). If threading is too complex, use a generous absolute constant like `maxDocIDMappingLen = 100_000_000` (100M docs; each DocID is 8 bytes = 800MB which is still bounded).

### Additional Sites to Guard

Beyond the 5 primary sites, these allocations also read sizes from the stream:

- **readBloomFilter (line 386):** `make([]uint64, numWords)` where `numWords` is uint32. A bloom filter with 65536 bits needs 1024 words (8KB). Add a guard: `maxBloomWords = 1 << 20` (~8MB of bloom data).
- **readHyperLogLogs (line 812):** `make([]uint8, numRegisters)` where `numRegisters` is uint32. HLL with precision 16 needs `2^16 = 65536` registers. Add a guard: `maxHLLRegisters = 1 << 16` (64KB).
- **readNumericIndexes (line 604-605):** `make([]RGNumericStat, numRGs)` where `numRGs` is uint32. Should be bounded by `Header.NumRowGroups` or an absolute constant.
- **readStringLengthIndexes (line 519):** `make([]RGStringLengthStat, numRGs)` same as above.

These are lower priority since the primary 5 are the ones called out in CONCERNS.md, but a thorough implementation should guard all stream-controlled allocations. The planner should decide whether to include these as part of SEC-01 scope or note them as incremental hardening.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `gotestsum` v1.13.0 |
| Config file | `Makefile` (test target) |
| Quick run command | `go test -v -run TestDecode -count=1` |
| Full suite command | `go test -v -count=1 ./...` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SEC-01 | Bounds check rejects oversized allocation in readRGSet | unit | `go test -v -run TestDecodeBoundsRGSet -count=1` | Wave 0 |
| SEC-01 | Bounds check rejects oversized allocation in readStringIndexes | unit | `go test -v -run TestDecodeBoundsStringIndexes -count=1` | Wave 0 |
| SEC-01 | Bounds check rejects oversized allocation in readTrigramIndexes | unit | `go test -v -run TestDecodeBoundsTrigramIndexes -count=1` | Wave 0 |
| SEC-01 | Bounds check rejects oversized allocation in readDocIDMapping | unit | `go test -v -run TestDecodeBoundsDocIDMapping -count=1` | Wave 0 |
| SEC-01 | Bounds check rejects oversized allocation in readPathDirectory | unit | `go test -v -run TestDecodeBoundsPathDirectory -count=1` | Wave 0 |
| SEC-02 | Decode rejects unknown version number | unit | `go test -v -run TestDecodeVersionMismatch -count=1` | Wave 0 |
| SEC-02 | Decode rejects data without magic bytes (legacy branch removed) | unit | `go test -v -run TestDecodeLegacyRejected -count=1` | Wave 0 |
| SEC-03 | Crafted payload exceeding limits returns error, not OOM | unit | `go test -v -run TestDecodeCraftedPayload -count=1` | Wave 0 |
| SEC-03 | ErrVersionMismatch is detectable via errors.Is | unit | `go test -v -run TestSentinelErrors -count=1` | Wave 0 |
| SEC-03 | ErrInvalidFormat is detectable via errors.Is | unit | `go test -v -run TestSentinelErrors -count=1` | Wave 0 |
| ALL | Existing serialization round-trip tests still pass | regression | `go test -v -run TestSerializeRoundTrip -count=1` | Exists |
| ALL | All compression level tests still pass | regression | `go test -v -run TestCompression -count=1` | Exists |

### Sampling Rate
- **Per task commit:** `go test -v -count=1`
- **Per wave merge:** `go test -v -count=1 ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] Tests for bounds check rejection at each allocation site (crafted payloads)
- [ ] Tests for version mismatch rejection
- [ ] Tests for legacy format rejection
- [ ] Tests for `errors.Is()` on sentinel errors
- [ ] No framework install needed (Go stdlib `testing`)
- [ ] No config files needed (tests go in `gin_test.go` or a new `serialize_test.go`)

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| No bounds checking | `maxConfigSize` guard in `readConfig` | Already in codebase | Template for new guards |
| Legacy format fallback | Magic bytes required | This phase (D-09) | Removes second attack surface |
| String-based errors only | Sentinel errors + wrapping | This phase (D-11) | Enables programmatic error handling |

## Open Questions

1. **How many additional allocation sites beyond the 5 primaries?**
   - What we know: `readBloomFilter`, `readHyperLogLogs`, `readNumericIndexes`, `readStringLengthIndexes` also have stream-controlled allocations.
   - What's unclear: Whether the phase scope includes these or only the 5 from CONCERNS.md.
   - Recommendation: Include all stream-controlled allocations. The marginal cost is a few lines per site, and leaving gaps undermines the security story. The commit message already says "all allocation sites."

2. **readRGSet: absolute constant vs parameter threading?**
   - What we know: Both approaches are acceptable per CONTEXT.md discretion.
   - What's unclear: Which produces cleaner code.
   - Recommendation: Use an absolute constant (`maxRGSetSize = 16 << 20`). Avoids changing `readRGSet` signature and cascading to 3+ callers. The constant is generous enough that no valid index hits it.

3. **Test approach: crafted binary payloads or builder-based?**
   - What we know: Bounds tests need payloads that trigger the guards. Building a valid index via `Builder` + `Encode` then mutating specific bytes is more maintainable than hand-crafting binary.
   - What's unclear: Whether fuzz tests should be part of this phase.
   - Recommendation: Table-driven tests with `Encode()` output + byte mutation. Consider a single `FuzzDecode` test as a bonus but not required for SEC-01/02/03.

## Sources

### Primary (HIGH confidence)
- `serialize.go` -- direct code inspection of all deserialization functions
- `gin.go` -- `Version` constant, `Header` struct, type definitions
- `.planning/codebase/CONCERNS.md` -- vulnerability catalog identifying all 5 sites
- `gin_test.go` -- existing serialization test patterns

### Secondary (MEDIUM confidence)
- [RoaringBitmap/RoaringFormatSpec](https://github.com/RoaringBitmap/RoaringFormatSpec) -- roaring bitmap serialized size characteristics
- [pkg/errors](https://github.com/pkg/errors) -- Unwrap() compatibility with stdlib `errors.Is()`

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- no new deps, all existing tooling
- Architecture: HIGH -- patterns are direct extension of existing `maxConfigSize` guard
- Pitfalls: HIGH -- code is fully inspected, all allocation sites enumerated
- Bounds math: MEDIUM -- trigram/unicode ceiling estimates are approximations, but constants are intentionally generous

**Research date:** 2026-03-26
**Valid until:** 2026-04-26 (stable domain, no moving parts)
