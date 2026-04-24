# Phase 1: Foundation - Context

**Gathered:** 2026-03-24
**Status:** Ready for planning

<domain>
## Phase Boundary

Establish legal identity (MIT LICENSE), fix module path from `gin-index` to `ami-gin`, and remove internal artifacts so the repo is ready for public visibility. After this phase, `go get github.com/amikos-tech/ami-gin` works and no internal references leak.

</domain>

<decisions>
## Implementation Decisions

### .planning/ Directory
- **D-01:** Add `.planning/` to `.gitignore` and remove from git tracking before going public. GSD workflow continues locally but process artifacts are not exposed to external users.
- **D-02:** No scrubbing of `.planning/` contents needed since the directory won't be tracked. Sensitive content (Kiba Team refs, private repo URL, security vulnerability catalog in CONCERNS.md) stays local-only.

### Scrub Thoroughness
- **D-03:** Minimal scrub — the module path rename (FOUND-02) and PRD deletion (FOUND-03) handle all material internal references. No additional grep-and-clean pass needed beyond what those two changes cover.

### Commit Strategy
- **D-04:** One commit per requirement — 4 separate commits in this order:
  1. Add MIT LICENSE file (FOUND-01)
  2. Module path rename across all 26 files (FOUND-02) — single isolated commit per STATE.md decision
  3. Delete `gin-index-prd.md` (FOUND-03)
  4. Add `.planning/` to `.gitignore` and untrack (FOUND-04)

### Claude's Discretion
- MIT LICENSE file content (standard template, year and copyright holder)
- Exact order of file edits within the module rename commit
- Whether to run `go mod tidy` as part of the rename or separately

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Module Identity
- `go.mod` — Current module path (`github.com/amikos-tech/gin-index`) that must change to `github.com/amikos-tech/ami-gin`
- `.golangci.yml` — Contains module path in `gci` import prefix config

### Internal Artifacts
- `gin-index-prd.md` — Internal PRD to be deleted (contains "Kiba Team" references)

### Project Constraints
- `.planning/PROJECT.md` §Constraints — License must be MIT, module path must be `github.com/amikos-tech/ami-gin`
- `.planning/REQUIREMENTS.md` §Foundation — FOUND-01 through FOUND-04 acceptance criteria

</canonical_refs>

<code_context>
## Existing Code Insights

### Files Affected by Module Rename
- 26 files reference `github.com/amikos-tech/gin-index`: go.mod, all example mains (11 files), CLI entry point, .golangci.yml, README.md, CLAUDE.md, and 6 .planning/ files (which will be gitignored anyway)

### Integration Points
- `go.sum` will need regeneration after module path change (`go mod tidy`)
- `.golangci.yml` has `gci` formatter prefix set to old module path
- `README.md` has `go get` installation instructions with old path
- `CLAUDE.md` references old module path in project overview

### No Breaking Changes
- This is a Go library with no known downstream consumers yet (private repo) — module rename is safe

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

*Phase: 01-foundation*
*Context gathered: 2026-03-24*
