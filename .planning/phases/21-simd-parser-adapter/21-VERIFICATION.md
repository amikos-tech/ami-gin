---
phase: 21-simd-parser-adapter
verified: 2026-07-23T08:26:25Z
status: passed
score: "16/16 must-haves verified"
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 15/16
  gaps_closed:
    - "Every failed native document close is now fatal on every walk outcome, including a concurrent walk panic"
  gaps_remaining: []
  regressions: []
deferred:
  - truth: "Real native SIMD execution and encoded/query parity across authored and realistic fixtures"
    addressed_in: "Phase 22"
    evidence: "Phase 22 success criterion 1 explicitly requires SIMD/stdlib encoded-index and query-result parity."
  - truth: "Required tagged CI, platform behavior, and native-runtime availability gates"
    addressed_in: "Phase 22"
    evidence: "Phase 22 success criterion 3 explicitly requires default and -tags simdjson CI with platform/shared-library behavior."
---

# Phase 21: SIMD Parser Adapter Verification Report

**Phase Goal:** Land an opt-in same-package SIMD parser implementation behind the existing parser seam without changing default stdlib behavior.
**Verified:** 2026-07-23T08:26:25Z
**Status:** passed
**Re-verification:** Yes — after Plan 21-05 gap closure

## Goal Achievement

Plan 21-05 closes the remaining lifecycle blocker. `finishSIMDDocument` now recovers a walk panic before cleanup. A successful close resumes the identical panic value; a failed close instead returns a multi-cause `parserLifecycleError`, preserving the recovered panic while allowing `AddDocument` to store the terminal builder error before any soft-parser routing. The committed native-free regression performs a second `AddDocument` and proves it is rejected before parser redispatch.

The roadmap's four success criteria and every truth from all five PLAN frontmatters are merged below. Repeated plan truths map to one observable contract rather than inflating the score.

### Observable Truths

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | A tagged caller can construct `NewSIMDParser() (Parser, error)`, observe `Name()=="pure-simdjson"`, and select it through `WithParser`, with no CLI activation path. | ✓ VERIFIED | Constructor/name are substantive at `parser_simd.go:17-35`; the tagged export archive contains `NewSIMDParser` and `(*simdParser).Name`; `WithParser` installs any `Parser` at `parser.go:85-101`; CLI policy tests forbid `--parser`. |
| 2 | Default builds remain stdlib-only, and every product importer of pure-simdjson has the exact first-line build tag. | ✓ VERIFIED | `NewBuilder` defaults to `stdlibParser` at `builder.go:207-242`; default `go list` places `parser_simd.go` in `IgnoredGoFiles`; only `parser_simd.go` imports pure-simdjson and line 1 is exact; `make simd-isolation-check` passed. |
| 3 | Exact int64/uint64 routing preserves `9007199254740993` and rejects values above `math.MaxInt64` through existing hard/soft numeric policy without float coercion or partial commit. | ✓ VERIFIED | Type-directed integer accessors at `parser_simd.go:105-116`; typed sink routing at `parser_sink.go:66-72`; policy at `builder.go:732-768`; focused exact-int and overflow tests passed. |
| 4 | The parser sink exposes string, bool, int64, uint64, and float64 fast paths, and non-null SIMD scalar leaves use them instead of `any`. | ✓ VERIFIED | Five typed methods exist at `parser_sink.go:28-40,58-90`; direct SIMD dispatch uses the matching method at `parser_simd.go:95-128`. |
| 5 | `StageFloat64` preserves float classification, and hard staged numeric errors retain their provenance even when cleanup also fails. | ✓ VERIFIED | `StageFloat64` bypasses the native whole-float fold at `parser_sink.go:74-90`; multi-cause lifecycle unwrapping is at `parser.go:7-42`; combined hard/soft lifecycle tests passed. |
| 6 | Typed-sink comparisons commit through `AddDocument` before `Finalize` and cannot pass by comparing empty indexes. | ✓ VERIFIED | `parser_simd_test.go:31-68` requires successful `AddDocument`, committed counters/mapping, and root path state before encoding. |
| 7 | The numeric parity fixture keeps incompatible numeric classes on separate paths, and its isolated non-empty golden does not alter earlier goldens. | ✓ VERIFIED | Fixture fields are separate at `parser_parity_fixtures_test.go:23-31`; golden is 604 bytes with SHA-256 `696b8a619eea2f5629bcf8fa6d285473a12c09a0e580f312fcbf2bcf2aa5ef8f`; the focused golden test and excluded-golden diff passed. |
| 8 | The root NOTICE traces pure-simdjson v0.1.4 MIT licensing and bundled simdjson Apache-2.0/MIT attribution to the pinned files. | ✓ VERIFIED | `NOTICE.md:3-24` agrees with the cached v0.1.4 `LICENSE` and `NOTICE`. |
| 9 | Documentation separates tagged build, fallible construction, and explicit builder selection with a valid two-step example. | ✓ VERIFIED | `docs/simd-deployment.md:9-61` and `README.md:80-112`; no invalid `WithParser(NewSIMDParser())` use exists. |
| 10 | Air-gapped/mirror guidance names all four bootstrap variables and separates wrapper, downloaded-native, and explicit-path integrity guarantees. | ✓ VERIFIED | `docs/simd-deployment.md:63-164` matches the pinned v0.1.4 bootstrap guide. |
| 11 | The accepted greater-than-uint64 BIGINT divergence is documented by layer, path, governing mode, and atomic document disposition. | ✓ VERIFIED | `docs/simd-deployment.md:166-183` and `CHANGELOG.md:5`. |
| 12 | README and CHANGELOG preserve stdlib defaults and avoid claiming Phase 22 parity, performance, or platform evidence. | ✓ VERIFIED | `README.md:80-112`, `CHANGELOG.md:3-5`, and the explicit validation boundary at `docs/simd-deployment.md:185-190`. |
| 13 | Every successfully parsed native document is closed exactly once. Successful cleanup preserves an identical walk panic; any close failure, including during a panic, becomes fatal, poisons the builder, and blocks every retry under parser soft mode. | ✓ VERIFIED | Panic-aware combiner at `parser_simd.go:51-77`; fatal route precedes soft handling at `builder.go:389-437`; identity, error/non-error panic, terminal state, and actual retry coverage at `parser_simd_lifecycle_test.go:51-220`; focused tagged tests passed. |
| 14 | Transform-buffered subtrees preserve stdlib `json.Number` and float-lexeme classification semantics. | ✓ VERIFIED | Transform guard precedes type dispatch at `parser_simd.go:86-93`; materialization returns `json.Number` for every numeric type and preserves whole-float class at `parser_simd.go:184-220`. |
| 15 | SIMD objects are last-key-wins with stdlib path construction; containers and both array path variants re-check transforms. | ✓ VERIFIED | `parser_simd.go:129-176,227-265` collapses duplicate keys, sorts direct-walk keys, mirrors stdlib raw paths, emits indexed/wildcard paths, and recurses through the transform guard. |
| 16 | Focused lifecycle tests are deterministic and native-free, covering close-only, returned stage-plus-close, successful-close panic, panic-plus-close, caller recovery, and retry refusal without constructing `NewSIMDParser`. | ✓ VERIFIED | `parser_simd_lifecycle_test.go:14-398` uses injected callbacks/test parsers, contains no native constructor call, and all `TestSIMDDocumentLifecycle*` tests passed. |

**Score:** 16/16 truths verified

### PLAN Frontmatter Coverage

| Plan | PLAN truth | Merged truth | Status |
| --- | --- | --- | --- |
| 21-01 | Five typed scalar boundary methods; no `any` for non-null SIMD leaves | #4 | ✓ VERIFIED |
| 21-01 | Float classification and hard non-finite stage-error provenance | #5 | ✓ VERIFIED |
| 21-01 | Exact int64 and oversized uint64 hard/soft atomic policy | #3 | ✓ VERIFIED |
| 21-01 | Typed comparisons commit through `AddDocument` | #6 | ✓ VERIFIED |
| 21-01 | Separate-path numeric fixture and isolated golden | #7 | ✓ VERIFIED |
| 21-02 | Pinned license/NOTICE attribution chain | #8 | ✓ VERIFIED |
| 21-02 | Three activation gates and two-step example | #9 | ✓ VERIFIED |
| 21-02 | Four bootstrap variables and route-specific integrity | #10 | ✓ VERIFIED |
| 21-02 | Accurate BIGINT divergence | #11 | ✓ VERIFIED |
| 21-02 | README/CHANGELOG preserve defaults and evidence boundaries | #12 | ✓ VERIFIED |
| 21-03 | Constructor/name/WithParser selection and no CLI path | #1 | ✓ VERIFIED |
| 21-03 | Native document lifecycle remains reusable across all walk outcomes | #13 | ✓ VERIFIED |
| 21-03 | Typed scalar routing strictly from element kind | #4 | ✓ VERIFIED |
| 21-03 | Transform-buffered numeric semantics | #14 | ✓ VERIFIED |
| 21-03 | Duplicate-key, path, container, and transform behavior | #15 | ✓ VERIFIED |
| 21-03 | Exact source tag and default dependency isolation | #2 | ✓ VERIFIED |
| 21-04 | Failed close is terminal and blocks every later parser invocation | #13 | ✓ VERIFIED |
| 21-04 | Returned walk/stage plus close errors retain both cause chains | #5 | ✓ VERIFIED |
| 21-04 | Native-free injected lifecycle tests cover close-only and returned stage-plus-close outcomes | #16 | ✓ VERIFIED |
| 21-05 | Successful-close panic cleanup runs once and resumes the identical value | #13 | ✓ VERIFIED |
| 21-05 | Panic plus failed close returns a diagnosable fatal lifecycle error under soft mode | #13 | ✓ VERIFIED |
| 21-05 | Caller recovery is followed by an actual retry that fails before parser redispatch | #13 | ✓ VERIFIED |
| 21-05 | Panic-plus-close regression is deterministic and native-free | #16 | ✓ VERIFIED |

### Re-verification Result

| Previous gap | Current result | Evidence |
| --- | --- | --- |
| A walk panic plus failed close discarded the cleanup failure and left the builder reusable | CLOSED | `finishSIMDDocument` now recovers before close and returns `newParserLifecycleError` on close failure (`parser_simd.go:51-77`). `TestSIMDDocumentLifecyclePanicCloseFailurePoisonsBuilder` performs two `AddDocument` calls and proves parser/walk/close counts remain `(1,1,1)`. |

### Deferred Items

Items below are explicitly owned by a later milestone phase and are not Phase 21 gaps.

| # | Item | Addressed In | Evidence |
| --- | --- | --- | --- |
| 1 | Real native adapter execution and encoded/query parity | Phase 22 | Phase 22 success criterion 1 requires SIMD/stdlib parity across authored and realistic fixtures. |
| 2 | Enforced tagged CI/platform/native-runtime gates | Phase 22 | Phase 22 success criterion 3 requires default and tagged CI with explicit platform/shared-library behavior. |

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `parser_sink.go` | Five typed scalar methods | ✓ VERIFIED | Substantive implementations; used by the adapter and committed-state tests. |
| `parser.go` | Parser contract and fatal lifecycle marker | ✓ VERIFIED | Typed routes plus multi-cause private lifecycle error. |
| `parser_test.go` | Expanded recording sink | ✓ VERIFIED | Implements all five typed methods. |
| `parser_simd_test.go` | Committed typed numeric tests | ✓ VERIFIED | Substantive, passing, and checks durable/non-durable builder state. |
| `parser_parity_fixtures_test.go` | SIMD numeric fixture | ✓ VERIFIED | Registered and consumed by parity/golden tests. |
| `testdata/parity-golden/simd-numeric-parity.bin` | Non-empty stdlib reference | ✓ VERIFIED | 604 bytes; focused parity test passed. |
| `NOTICE.md` | Pinned attribution | ✓ VERIFIED | Matches pinned upstream files. |
| `docs/simd-deployment.md` | Activation/operations guidance | ✓ VERIFIED | Complete and linked from README/CHANGELOG. |
| `README.md` | Optional SIMD installation | ✓ VERIFIED | Correct two-step example and unchanged-default statement. |
| `CHANGELOG.md` | Unreleased adapter entry | ✓ VERIFIED | Accurate limitation/default wording. |
| `parser_simd.go` | Tagged lifecycle-safe adapter | ✓ VERIFIED | Substantive, compiled under the tag, wired through the parser seam, and panic-aware. |
| `go.mod` | Exact v0.1.4 dependency pin | ✓ VERIFIED | Direct pin at line 12. |
| `go.sum` | Pinned checksums | ✓ VERIFIED | v0.1.4 module and go.mod entries present; module verification passed. |
| `Makefile` | Default dependency-isolation guard | ✓ VERIFIED | Standalone target passed. |
| `builder.go` | Fatal lifecycle routing | ✓ VERIFIED | Marker detection precedes skip, stage, and parser-soft branches. |
| `parser_lifecycle_test.go` | Generic marker/terminal-builder tests | ✓ VERIFIED | Default focused suite passed. |
| `parser_simd_lifecycle_test.go` | Injected SIMD lifecycle tests | ✓ VERIFIED | Close-only, combined returned errors, and both panic cleanup outcomes are covered. |

All PLAN-declared artifacts pass existence and substantive checks. The automated key-link query could not parse symbolic `from` labels such as `parser_simd.go finishSIMDDocument`; every such link was therefore checked manually below.

### Key Link Verification

| From | To | Via | Status | Details |
| --- | --- | --- | --- | --- |
| `parser_simd_test.go` | `AddDocument` / `mergeDocumentState` | Parser → AddDocument → commit → Finalize/Encode | ✓ WIRED | Durable counters, mapping, and root path are asserted before encoding. |
| `StageFloat64` | `stageNumericObservation` | Direct non-folding staged value with `tagStageError` | ✓ WIRED | `parser_sink.go:74-90`. |
| `StageInt64` / `StageUint64` | `stageNativeNumeric` | Typed wrapper reuse | ✓ WIRED | `parser_sink.go:66-72`. |
| README SIMD section | Deployment guide | Relative Markdown link | ✓ WIRED | `README.md:110`. |
| Constructor error | Deployment recovery | `PURE_SIMDJSON_LIB_PATH` hint | ✓ WIRED | `parser_simd.go:28-32`; guide lines 58-61. |
| Root NOTICE | Pinned dependency NOTICE | Versioned attribution | ✓ WIRED | Local claims match cached v0.1.4 files. |
| `walkElement` | Typed sink methods | Element-kind dispatch | ✓ WIRED | `parser_simd.go:95-128`. |
| `materializeElement` | `StageMaterialized` | `json.Number` subtree values | ✓ WIRED | `parser_simd.go:86-93,184-220`. |
| `Parse` | `Doc.Close` | `finishSIMDDocument` callback | ✓ WIRED | `parser_simd.go:37-48`; close is owned by the helper on every walk outcome. |
| `simd-isolation-check` | Default dependency graph | Exact source and graph assertions | ✓ WIRED | Target passed; default graph contains no pure-simdjson package. |
| `finishSIMDDocument` | `parserLifecycleError` | Failed-close combiner, including recovered panic | ✓ WIRED | `parser_simd.go:51-77`. |
| `AddDocument` | `GINBuilder.tragicErr` | Lifecycle detection before recoverable branches | ✓ WIRED | `builder.go:389-437`. |
| Lifecycle tests | Cleanup helper and `AddDocument` | Injected callbacks, caller recovery, retry, and counters | ✓ WIRED | `parser_simd_lifecycle_test.go:76-220,223-374`. |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Produces Real Data | Status |
| --- | --- | --- | --- | --- |
| Tagged parser selection | Parser implementation | `NewSIMDParser` → `WithParser` → `NewBuilder` parser field | Yes by tagged compile/export trace | ✓ FLOWING |
| Direct SIMD scalar path | Native typed leaves | `Parse` → `Element.Type`/typed accessor → typed sink → staged state → merge | Yes by static trace; runtime parity belongs to Phase 22 | ✓ FLOWING |
| Transform path | Materialized subtree | Native element → `json.Number`/Go values → `StageMaterialized` | Yes by static trace; runtime parity belongs to Phase 22 | ✓ FLOWING |
| Returned close-failure path | Cleanup and concurrent returned error | `finishSIMDDocument` → `parserLifecycleError` → `AddDocument` → `tragicErr` | Yes | ✓ FLOWING |
| Panic-plus-close path | Cleanup failure during walk panic | recover → close → lifecycle marker → terminal builder → blocked retry | Yes; committed regression executes the retry | ✓ FLOWING |
| Default build selection | Parser and dependency graph | No tag → adapter ignored → `stdlibParser` default | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Full default build | `make build` | Orchestrator-established post-Plan-05 pass | ✓ PASS |
| Full default suite | `make test` | Orchestrator ran twice after Plan 21-05: 1,042 passed, one fixture-dependent skip | ✓ PASS |
| Typed scalar and numeric contracts | `go test -count=1 -run '^TestTypedSink' .` | Exit 0; exact int, float class, hard/soft overflow, and non-finite cases passed | ✓ PASS |
| Generic fatal lifecycle contract | `go test -count=1 -run '^TestParserLifecycle' .` | Exit 0 | ✓ PASS |
| Default parser selection | `go test -count=1 -run '^TestNewBuilderDefaultsToStdlibParser$' .` | Exit 0 | ✓ PASS |
| Panic-plus-close blocker regression | `go test -tags simdjson -count=1 -run '^TestSIMDDocumentLifecyclePanicCloseFailurePoisonsBuilder$' .` | Exit 0; terminal state and blocked retry passed | ✓ PASS |
| Full native-free lifecycle set | `go test -tags simdjson -count=1 -run '^TestSIMDDocumentLifecycle' .` | Exit 0 | ✓ PASS |
| Tagged package compile | `go test -tags simdjson -count=1 -run '^$' ./...` | All packages compiled; no tests or native constructor executed | ✓ PASS |
| Numeric fixture golden | `go test -count=1 -run 'TestParserParity_AuthoredFixtures/simd-numeric-parity' .` | Focused subtest passed | ✓ PASS |
| Default dependency isolation | `make simd-isolation-check` | Exit 0; build and vet passed | ✓ PASS |
| Module integrity/tidiness | `go mod verify && go mod tidy -diff` | `all modules verified`; no diff | ✓ PASS |

### Probe Execution

Step 7c was skipped: no `probe-*.sh` files or probe declarations exist for this phase.

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| SIMD-04 | 21-02, 21-03, 21-04, 21-05 | Explicit same-package SIMD parser selection through the existing seam | ✓ SATISFIED | Tagged constructor/name compile and export; `WithParser` accepts the returned `Parser`; lifecycle failures are terminal; no default or CLI activation. |
| SIMD-05 | 21-02, 21-03 | Default stdlib-only build/runtime | ✓ SATISFIED | Default builder is stdlib; default source/package/test graph excludes pure-simdjson; isolation target passed. |
| SIMD-06 | 21-01, 21-02, 21-03, 21-04 | Exact-int semantics and no silent float64 coercion | ✓ SATISFIED | Type-directed integer accessors, non-folding float staging, `json.Number` materialization, and focused committed-state tests. |
| SIMD-07 | 21-01, 21-03, 21-04 | Typed scalar fast paths avoid `any` | ✓ SATISFIED | Five typed methods exist and direct adapter dispatch is wired for every non-null scalar kind. |

All four Phase 21 requirement IDs appear in PLAN frontmatter and `.planning/REQUIREMENTS.md`; none is orphaned.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| --- | --- | --- | --- | --- |
| Phase-owned files | — | Unreferenced `TBD`, `FIXME`, or `XXX` | None | No debt-marker blocker found. |
| `parser_simd.go` | 95, 186 | `Element.Type()` collapses native type-read causes to `TypeInvalid` | ℹ INFO | A generic invalid-element error is returned instead of the native cause. The plan explicitly locked single `Element.Type()` dispatch, so this does not fail a Phase 21 must-have. |
| `parser_simd.go` | 19-35 | Returned `Parser` interface has no deterministic parser-handle close method | ℹ INFO | Upstream finalization owns eventual parser release after it becomes unreachable. The public constructor/interface shape was locked before this phase. |
| Native execution / tagged automation | — | Runtime adapter parity and platform coverage are absent | ⚠ DEFERRED | Phase 22 success criteria 1 and 3 explicitly own these checks. |

Broad empty-value matches were normal initial state, test assertions, or valid error slices, not stubs. No placeholder implementation, console-only handler, or hardcoded empty data path was found.

### Disconfirmation Pass

- **Weakest evidence edge:** SIMD-04's tagged constructor and seam are compiled and present in the export archive, but Phase 21's deterministic tests intentionally do not construct the native parser. Phase 22 owns actual runtime/platform execution.
- **Potentially misleading green test:** lifecycle tests prove the adapter's cleanup/routing helper with injected callbacks, not the upstream FFI's real `Doc.Close`. This is sufficient for the Phase 21 implementation contract but not parity or operational certification.
- **Uncovered diagnostic path:** `Element.Type()` converts native type-read errors to `TypeInvalid`, so the adapter cannot retain the underlying native cause on that branch. This affects diagnostic detail, not type routing or index correctness under the phase contract.

### Human Verification Required

None within the Phase 21 boundary. All Phase 21 truths are programmatically verifiable. Real native execution and platform behavior are explicit Phase 22 deliverables rather than deferred human checks for this phase.

### Gaps Summary

No Phase 21 gaps remain. Plan 21-05 closes the panic-plus-close lifecycle failure, and regression checks found no breakage in the previously passing typed-sink, numeric, documentation, attribution, default-isolation, transform, duplicate-key, or returned-error contracts.

---

_Verified: 2026-07-23T08:26:25Z_
_Verifier: the agent (gsd-verifier)_
