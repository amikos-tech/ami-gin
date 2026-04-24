# Phase 2: Security Hardening - Context

**Gathered:** 2026-03-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Make the deserialization path safe to expose to untrusted inputs. Add bounds checks on all unbounded allocation sites in `Decode()`, validate the binary format version, and enforce maximum size limits to prevent memory exhaustion. After this phase, a crafted or corrupted `.gin` file produces a clean error instead of OOM or silent corruption.

</domain>

<decisions>
## Implementation Decisions

### Size Limit Strategy
- **D-01:** Hybrid approach — header-derived bounds where structural anchors exist, absolute constants where not
- **D-02:** `readDocIDMapping` bounded by `NumRowGroups` (exact anchor: DocIDs map to row-group positions in `[0, NumRowGroups)`)
- **D-03:** `readRGSet` bounded by `NumRowGroups` (roaring bitmap size derivable from row group count) or a generous absolute constant (e.g., 16MB) to avoid parameter threading
- **D-04:** `readPathDirectory` bounded by `NumPaths <= 65535` (PathID is uint16 — structurally impossible to exceed)
- **D-05:** `readStringIndexes` bounded by absolute constant `maxTermsPerPath` (e.g., 1,000,000 — generous given default CardinalityThreshold is 10,000)
- **D-06:** `readTrigramIndexes` bounded by absolute constant `maxTrigramsPerPath` (e.g., 10,000,000 — covers extreme unicode FTS; ASCII ceiling is ~857K)
- **D-07:** Follow `maxConfigSize` precedent for constant naming and documentation

### Version Rejection Policy
- **D-08:** Exact match — reject if `Header.Version != Version`. No installed base exists (private repo, pre-release). Safe to be strict.
- **D-09:** Remove the legacy fallback branch in `Decode()` (lines ~181-193) that re-attempts decompression without magic bytes. No pre-magic format exists in the wild. This branch is a second attack surface for malformed data.
- **D-10:** When a real format split happens (Version bumps to 4), revisit range-based acceptance at that point with a concrete migration story.

### Error Behavior
- **D-11:** Add two sentinel errors as package-level vars: `ErrVersionMismatch` and `ErrInvalidFormat`
- **D-12:** Declare as `var ErrVersionMismatch = errors.New("version mismatch")` and `var ErrInvalidFormat = errors.New("invalid format")` using `github.com/pkg/errors`
- **D-13:** Wrap at detection point: `errors.Wrap(ErrVersionMismatch, "read header")` — preserves human-readable chain while enabling `errors.Is()` for programmatic handling
- **D-14:** Bounds violations use `errors.Errorf` with actual and max values in message (matching existing `configLen` pattern), wrapped with `ErrInvalidFormat`

### Commit Strategy
- **D-15:** 2 commits on the feature branch:
  1. `fix(serialize): reject unknown index versions in Decode` — version validation + legacy fallback removal (SEC-02)
  2. `fix(serialize): add bounds checks and size limits to all allocation sites` — all allocation guards (SEC-01 + SEC-03)
- **D-16:** SEC-01 and SEC-03 are grouped because they are the same concern (allocation safety) in the same file. Phase 1's 4-commit pattern made sense for 4 distinct files/concerns; here grouping is more coherent.

### Claude's Discretion
- Exact constant values for `maxTermsPerPath`, `maxTrigramsPerPath`, `maxRGSetSize` — domain math should guide, documented with comments
- Whether to thread `numRGs` as a parameter into `readRGSet` or use an absolute constant — either approach is acceptable
- Test structure — how to organize crafted-payload tests (table-driven, per-site, fuzz)
- Whether `ErrInvalidFormat` wraps further sub-categories or stays flat

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Security Requirements
- `.planning/REQUIREMENTS.md` §Security Hardening — SEC-01, SEC-02, SEC-03 acceptance criteria

### Vulnerability Catalog
- `.planning/codebase/CONCERNS.md` §Critical Issues — Detailed analysis of all 5 unbounded allocation sites and version validation gap
- `.planning/codebase/CONCERNS.md` §Security Considerations — Regex compilation limits, integrity checks, InSubnet panics

### Serialization Code
- `serialize.go` — All deserialization functions (`readRGSet`, `readHeader`, `readPathDirectory`, `readStringIndexes`, `readTrigramIndexes`, `readDocIDMapping`, `readConfig`)
- `gin.go:7` — `Version` constant (currently 3)
- `serialize.go:15` — `maxConfigSize` precedent for bounds constants

### Project Constraints
- `.planning/PROJECT.md` §Constraints — No breaking API changes

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `maxConfigSize = 1 << 20` in `serialize.go:15` — established pattern for bounds constants
- `readConfig` (serialize.go:~893) — existing guard pattern: `if configLen > maxConfigSize { return error }`
- `errors.New`, `errors.Wrap`, `errors.Errorf` from `github.com/pkg/errors` — used throughout

### Established Patterns
- Sequential decode: header is read first, then sections in fixed order — header values are available for downstream bounds checks
- All read functions take `io.Reader` — adding bounds parameters requires signature changes only for header-derived limits
- `GINIndex.Header` struct holds `NumRowGroups`, `NumDocs`, `NumPaths`, `CardinalityThresh` — all available after `readHeader`

### Integration Points
- `Decode()` / `DecodeReader()` in `serialize.go` — main entry points that call all read functions
- `readHeader()` — where version validation must go
- CLI `cmd/gin-index/main.go` — consumer that loads user-supplied files; will benefit from sentinel errors for better messages

</code_context>

<specifics>
## Specific Ideas

No specific requirements — standard approaches for all items.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 02-security-hardening*
*Context gathered: 2026-03-26*
