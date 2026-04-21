---
phase: 13-parser-seam-extraction
plan: 01
subsystem: api
tags: [parser, builder, json, refactor, testing]
requires: []
provides:
  - additive parser seam surface in the root package
  - package-private sink adapters on GINBuilder
  - stdlib parser wrapper that preserves the existing JSON walk behavior
affects: [builder, parser, telemetry, phase-14]
tech-stack:
  added: []
  patterns: [exported parser entry point with private sink, parser code-move into dedicated file, package-local seam tests]
key-files:
  created:
    - parser.go
    - parser_sink.go
    - parser_stdlib.go
    - parser_test.go
    - .planning/phases/13-parser-seam-extraction/13-parser-seam-extraction-01-SUMMARY.md
  modified:
    - builder.go
key-decisions:
  - "Kept Parser and WithParser exported while leaving parserSink and stdlibParser package-private per D-02."
  - "Preserved container presence marking inside stdlibParser via documentBuildState until a future sink method such as MarkPresent is justified."
  - "Added the parser seam fields to GINBuilder without wiring AddDocument yet so wave 1 stays behavior-neutral."
patterns-established:
  - "Parser seam pattern: Parser owns traversal, parserSink owns staging, builder retains numeric classification and merge semantics."
  - "Compatibility pattern: move existing walker logic into a dedicated parser file before switching the hot path."
requirements-completed: [PARSER-01]
duration: 5 min
completed: 2026-04-21
---

# Phase 13 Plan 01: Parser Seam Surface Summary

**Additive parser seam types, sink adapters, and a stdlib parser wrapper landed without changing the active AddDocument path**

## Performance

- **Duration:** 5 min
- **Started:** 2026-04-21T10:44:31Z
- **Completed:** 2026-04-21T10:49:11Z
- **Tasks:** 5
- **Files modified:** 5

## Accomplishments

- Added `Parser` and `WithParser` as the public seam entry point while keeping `parserSink` and `stdlibParser` package-private.
- Moved the existing streaming JSON walker into `parser_stdlib.go` so the hot-path switch in plan 13-02 can be a narrow wiring change instead of a mixed refactor.
- Added package-local seam tests that pin the nil-parser error, default parser name, sink buffering signal, and `BeginDocument` state handoff.

## Task Commits

Plan 01 landed as one atomic implementation commit because `parser.go`, `parser_sink.go`, `parser_stdlib.go`, the new `GINBuilder` fields, and the package-local tests form one compile unit once the seam files exist.

1. **Tasks 1-5: Parser seam surface, sink adapters, stdlib parser extraction, builder fields, and seam tests** - `3f4d844` (refactor/test)

## Files Created/Modified

- `parser.go` - Defines the exported `Parser` interface and `WithParser` builder option with the exact nil-guard contract.
- `parser_sink.go` - Declares the six-method private sink contract and forwards it to existing builder staging methods.
- `parser_stdlib.go` - Hosts the extracted `encoding/json` walker behind `stdlibParser`.
- `parser_test.go` - Covers the seam contracts that can be asserted before the hot-path wiring lands.
- `builder.go` - Adds the private parser seam fields needed by later plans.

## Decisions Made

- Exported only `Parser` and `WithParser` now, leaving `parserSink` package-private so the seam can widen later without breaking callers.
- Kept the numeric classifier in `builder.go`; `parser_stdlib.go` only forwards tokens and materialized values.
- Documented container presence marking as a future sink-extension point rather than widening the sink prematurely in wave 1.

## Deviations from Plan

### Auto-fixed Issues

**1. [Buildability] Grouped the wave-1 tasks into one code commit**
- **Found during:** Plan 01 execution
- **Issue:** The new parser files reference the new `GINBuilder` seam fields, so splitting Tasks 1-4 into separate commits would leave intermediate commits uncompilable.
- **Fix:** Landed the additive seam files, builder fields, and package-local tests together in one green commit.
- **Files modified:** `builder.go`, `parser.go`, `parser_sink.go`, `parser_stdlib.go`, `parser_test.go`
- **Verification:** `go build ./...`, `go test ./... -count=1`, `make lint`
- **Committed in:** `3f4d844`

---

**Total deviations:** 1 auto-fixed
**Impact on plan:** No scope change. The deviation only changed commit granularity so the tree stayed buildable and green.

## Issues Encountered

- A stale `.git/index.lock` was left behind by an accidental parallel commit attempt during execution. The lock was removed before any further git operations, and no repo content was lost.

## User Setup Required

None - plan 01 is repository-local refactor and test work only.

## Next Phase Readiness

- `13-02-PLAN.md` can now wire `AddDocument` through `b.parser.Parse` without introducing new seam types in the same diff.
- Phase 14 can already rely on `Parser.Name()` existing as the telemetry hook once `NewBuilder` starts caching it in plan 13-02.
- The parity harness in `13-03-PLAN.md` can focus on behavioral proof rather than defining seam primitives.

## Self-Check: PASSED

- `go test -run "TestWithParserRejectsNil|TestStdlibParserName|TestBuilderHasParserFields|TestShouldBufferForTransformSignalWhenRegistered|TestBeginDocumentStashesState" -count=1 -v .`
- `go test ./... -count=1`
- `go build ./...`
- `make lint`

---
*Phase: 13-parser-seam-extraction*
*Completed: 2026-04-21*
