# Phase 2: Security Hardening - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-26
**Phase:** 02-security-hardening
**Areas discussed:** Size limit strategy, Version rejection policy, Error behavior, Commit granularity

---

## Size Limit Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Hard-coded constants only | Simple absolute caps everywhere. No header threading. | |
| Header-derived only | Tightest bounds where relationships exist. Insufficient alone. | |
| Configurable via DecodeOptions | Functional options for caller-tunable limits. More API surface. | |
| Header cross-validation only | All checks in readHeader. Leaves worst sites uncapped. | |
| Hybrid: header-derived + absolute fallback | Exact bounds where anchors exist, absolute constants where not. | ✓ |

**User's choice:** Hybrid — header-derived bounds where structural anchors exist, absolute constants where not
**Notes:** Research identified per-site risk profiles: readDocIDMapping (uint64, critical), readRGSet (uint32, high), readStringIndexes (uint32, high), readTrigramIndexes (uint32, medium), readPathDirectory (uint16, low). Hybrid approach addresses each proportionally.

---

## Version Rejection Policy

| Option | Description | Selected |
|--------|-------------|----------|
| Exact match (version != Version) | Reject anything other than current version. Simplest, safest. | ✓ |
| Accept past, reject future (v <= Version) | Accept lower versions, reject higher. | |
| Range-based (minVersion..Version) | Accept defined range. Premature for pre-release. | |
| Accept any, warn | Non-breaking but defeats security purpose. | |

**User's choice:** Exact match
**Notes:** Also decided to remove the legacy fallback branch in Decode() (pre-magic-byte format handler) — no such format exists in the wild, and it's a second attack surface.

---

## Error Behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Plain strings only | Keep current pattern. No programmatic distinction. | |
| 2-3 sentinel errors + inline bounds checks | ErrVersionMismatch + ErrInvalidFormat. errors.Is() support. | ✓ |
| Typed error struct (*DecodeError) | Maximum introspection with Kind/Got/Max fields. | |
| Panic on invariant violations | Mirrors Must* pattern. Wrong for untrusted input. | |

**User's choice:** Sentinel errors — ErrVersionMismatch and ErrInvalidFormat
**Notes:** Declared as package-level vars using pkg/errors. Wrapped at detection point for human-readable chain + programmatic handling.

---

## Commit Granularity

| Option | Description | Selected |
|--------|-------------|----------|
| 1 per requirement (3 commits) | Mirrors Phase 1 pattern. 1:1 traceability. | |
| 2 commits (SEC-02 alone; SEC-01+SEC-03 together) | Groups coupled allocation concerns. Natural cohesion. | ✓ |
| 1 commit (all together) | Single atomic commit. Less traceable. | |
| 4+ commits (per allocation site) | Maximum granularity. Excessive for repetitive changes. | |

**User's choice:** 2 commits — version validation separate, bounds+limits together
**Notes:** Phase 1's 4-commit pattern made sense for 4 distinct files/concerns. Here SEC-01 and SEC-03 are the same concern in the same file.

---

## Claude's Discretion

- Exact constant values for maxTermsPerPath, maxTrigramsPerPath, maxRGSetSize
- Whether to thread numRGs as parameter or use absolute constant for readRGSet
- Test structure for crafted-payload tests
- Whether ErrInvalidFormat wraps sub-categories or stays flat

## Deferred Ideas

None — discussion stayed within phase scope.
