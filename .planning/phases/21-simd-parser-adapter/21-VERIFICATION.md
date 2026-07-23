---
phase: 21-simd-parser-adapter
verified: 2026-07-23T07:04:54Z
status: gaps_found
score: "15/16 must-haves verified"
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 13/15
  gaps_closed:
    - "Every hard staged numeric error retains its stage provenance even when document cleanup also fails"
  gaps_remaining:
    - "Every failed native document close is fatal on every walk outcome, including a concurrent walk panic"
  regressions: []
gaps:
  - truth: "Every failed native document close is a fatal parser-lifecycle failure, including when document walking panics"
    status: failed
    reason: "finishSIMDDocument attempts cleanup during a walk panic but does not recover the panic. If Close also fails, the defer assigns parserLifecycleError to a named return that is never returned because the original panic resumes. AddDocument cannot set tragicErr; after caller recovery the builder can dispatch the parser again, and ParserFailureMode soft can suppress the upstream ErrParserBusy retry."
    artifacts:
      - path: "parser_simd.go"
        issue: "Lines 51-63 do not recover a walk panic before combining it with a close failure, so the cleanup failure is discarded."
      - path: "parser_simd_lifecycle_test.go"
        issue: "Lines 51-73 cover panic plus successful close, but no test combines a walk panic with a failed close and verifies terminal builder state."
      - path: "builder.go"
        issue: "Lines 412-437 correctly poison the builder only when Parse returns parserLifecycleError; a propagated parser panic bypasses that branch."
    missing:
      - "Lifecycle handling that preserves the original panic when close succeeds but makes a concurrent close failure terminal and observable to AddDocument."
      - "A native-free regression combining walk panic, close failure, ParserFailureMode soft, caller recovery, and a second AddDocument attempt."
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
**Verified:** 2026-07-23T07:04:54Z
**Status:** gaps_found
**Re-verification:** Yes — after Plan 21-04 gap closure

## Goal Achievement

Plan 21-04 closes the earlier returned-error defects: an ordinary close failure is now terminal, and simultaneous returned stage/soft-skip plus close errors retain both cause chains. One lifecycle branch remains unsafe. When walking panics and closing also fails, the close failure is discarded and the builder is not poisoned. This independently reproduces the advisory review's critical claim and fails an absolute Plan 21-04 must-have.

### Observable Truths

Roadmap success criteria and all PLAN frontmatter truths are merged below. Repeated PLAN truths map to the same observable contract; the native-free lifecycle-test truth added by Plan 21-04 is listed separately.

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | A tagged caller can construct `NewSIMDParser() (Parser, error)`, observe `Name()=="pure-simdjson"`, and select it through `WithParser`, with no CLI activation path. | ✓ VERIFIED | `parser_simd.go:17-35`; `parser.go:85-101`; tagged build/test passed; product CLI policy tests reject `--parser`. |
| 2 | Default builds remain stdlib-only and every product importer of pure-simdjson has the exact first-line build tag. | ✓ VERIFIED | `builder.go:207-242`; only `parser_simd.go` imports the module and line 1 is exact; default dependency graph excludes it; `make simd-isolation-check` passed. |
| 3 | Exact int64/uint64 routing preserves `9007199254740993` and rejects values above `math.MaxInt64` through existing hard/soft numeric policy without float coercion or partial commit. | ✓ VERIFIED | `parser_simd.go:91-108`; `parser_sink.go:66-90`; `builder.go:697-768`; focused typed-sink tests passed. |
| 4 | The parser sink exposes string, bool, int64, uint64, and float64 fast paths, and non-null SIMD scalar leaves use them instead of `any`. | ✓ VERIFIED | Interface/implementations at `parser_sink.go:28-40,58-102`; direct SIMD dispatch at `parser_simd.go:81-114`. |
| 5 | `StageFloat64` preserves float classification and hard staged errors retain their stage provenance when cleanup also fails. | ✓ VERIFIED | `parser_sink.go:74-90`; multi-cause marker at `parser.go:7-42`; combined hard/soft lifecycle tests at `parser_simd_lifecycle_test.go:162-250` passed. |
| 6 | Typed-sink comparisons commit through `AddDocument` before `Finalize` and cannot pass by comparing empty indexes. | ✓ VERIFIED | `parser_simd_test.go:31-68` asserts counters, doc mapping, and root path before encoding; focused tests passed. |
| 7 | The numeric parity fixture keeps incompatible numeric classes on separate paths and its isolated non-empty golden does not alter earlier goldens. | ✓ VERIFIED | `parser_parity_fixtures_test.go:23-31`; golden is 604 bytes, SHA-256 `696b8a619eea2f5629bcf8fa6d285473a12c09a0e580f312fcbf2bcf2aa5ef8f`; focused parity and excluded-golden diff passed. |
| 8 | The root NOTICE traces pure-simdjson v0.1.4 MIT licensing and bundled simdjson Apache-2.0/MIT attribution to the pinned files. | ✓ VERIFIED | `NOTICE.md:3-24` agrees with the cached v0.1.4 `LICENSE` and `NOTICE`. |
| 9 | Documentation separates tagged build, fallible construction, and explicit builder selection with a valid two-step example. | ✓ VERIFIED | `docs/simd-deployment.md:9-61`; `README.md:80-112`; no invalid direct constructor use. |
| 10 | Air-gapped/mirror guidance names all four bootstrap variables and separates wrapper, downloaded-native, and explicit-path integrity guarantees. | ✓ VERIFIED | `docs/simd-deployment.md:63-164` matches the pinned v0.1.4 bootstrap guide. |
| 11 | The accepted greater-than-uint64 BIGINT divergence is documented by layer, path, governing mode, and atomic document disposition. | ✓ VERIFIED | `docs/simd-deployment.md:166-183`; `CHANGELOG.md:5`. |
| 12 | README and CHANGELOG preserve stdlib defaults and avoid claiming Phase 22 parity, performance, or platform evidence. | ✓ VERIFIED | `README.md:80-112`; `CHANGELOG.md:3-5`; validation boundary at `docs/simd-deployment.md:185-190`. |
| 13 | Every successfully parsed native document is closed exactly once, and every close failure makes the builder terminal before any parser retry, on every walk outcome. | ✗ FAILED | Returned success/error paths are handled, but `parser_simd.go:51-63` discards close failure during a walk panic. An ephemeral overlay test reproduced `builder.Err()==nil`, a second parser dispatch, and one soft-skipped simulated busy error after application recovery. |
| 14 | Transform-buffered subtrees preserve stdlib `json.Number` and float-lexeme classification semantics. | ✓ VERIFIED | Transform guard precedes type dispatch (`parser_simd.go:72-79`); materializer returns `json.Number` for all numeric kinds and appends `.0` to whole floats (`parser_simd.go:170-206`). |
| 15 | SIMD objects are last-key-wins with stdlib path construction; containers and both array path variants re-check transforms. | ✓ VERIFIED | `parser_simd.go:115-162,213-251` collapses duplicates, sorts direct-walk keys, mirrors raw paths, emits indexed/wildcard paths, and recurses through the transform guard. |
| 16 | Focused native-free tests inject lifecycle outcomes and deterministically cover close-only and returned stage-plus-close failures without constructing `NewSIMDParser`. | ✓ VERIFIED | `parser_simd_lifecycle_test.go:14-275` is tagged, uses callbacks/test parsers, and contains no native constructor call; focused tagged tests passed. Coverage omits panic-plus-close failure, which is captured under truth 13. |

**Score:** 15/16 truths verified

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
| 21-03 | Native document lifecycle remains reusable across all walk outcomes | #13 | ✗ FAILED |
| 21-03 | Typed scalar routing strictly from element kind | #4 | ✓ VERIFIED |
| 21-03 | Transform-buffered numeric semantics | #14 | ✓ VERIFIED |
| 21-03 | Duplicate-key, path, container, and transform behavior | #15 | ✓ VERIFIED |
| 21-03 | Exact source tag and default dependency isolation | #2 | ✓ VERIFIED |
| 21-04 | Failed close is terminal and blocks every later parser invocation | #13 | ✗ FAILED |
| 21-04 | Returned walk/stage plus close errors retain both cause chains | #5 | ✓ VERIFIED |
| 21-04 | Native-free injected lifecycle tests cover close-only and returned stage-plus-close outcomes | #16 | ✓ VERIFIED |

### Re-verification Result

| Previous gap | Current result | Evidence |
| --- | --- | --- |
| Close failure could be soft-skipped and leave the parser busy | PARTIAL — ordinary returned paths fixed; panic-plus-close remains | `parser.go:7-42`, `builder.go:412-416`, and close-only tests pass; overlay reproduction proves the panic branch still bypasses poisoning. |
| Combined stage and close failure lost stage provenance | CLOSED | Multi-cause `Unwrap() []error` plus tagged hard/soft tests preserve stage, soft-skip, `IngestError`, and sentinel causes. |

### Deferred Items

| # | Item | Addressed In | Evidence |
| --- | --- | --- | --- |
| 1 | Real native adapter execution and encoded/query parity | Phase 22 | Phase 22 success criterion 1. |
| 2 | Enforced tagged CI/platform/native-runtime gates | Phase 22 | Phase 22 success criterion 3. |

The lifecycle blocker is not deferred: no later milestone phase specifically owns failed-close behavior during a walk panic.

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `parser_sink.go` | Five typed scalar methods | ✓ VERIFIED | Substantive implementations, used by adapter/tests. |
| `parser.go` | Parser contract and fatal lifecycle marker | ✓ VERIFIED | Typed routes plus multi-cause unexported marker. |
| `parser_test.go` | Expanded recording sink | ✓ VERIFIED | All five typed methods implemented. |
| `parser_simd_test.go` | Committed typed numeric tests | ✓ VERIFIED | Substantive, passing, and asserts durable commit state. |
| `parser_parity_fixtures_test.go` | SIMD numeric fixture | ✓ VERIFIED | Registered and consumed by parity/golden tests. |
| `testdata/parity-golden/simd-numeric-parity.bin` | Non-empty stdlib reference | ✓ VERIFIED | 604 bytes; focused golden test passed. |
| `NOTICE.md` | Pinned attribution | ✓ VERIFIED | Matches pinned upstream files. |
| `docs/simd-deployment.md` | Activation/operations guidance | ✓ VERIFIED | Complete and linked. |
| `README.md` | Optional SIMD installation | ✓ VERIFIED | Correct two-step example and default statement. |
| `CHANGELOG.md` | Unreleased adapter entry | ✓ VERIFIED | Accurate limitation/default wording. |
| `parser_simd.go` | Tagged lifecycle-safe adapter | ✗ PARTIAL | Substantive and wired, but panic-plus-close loses cleanup failure. |
| `go.mod` | Exact v0.1.4 dependency pin | ✓ VERIFIED | Direct pin at line 12. |
| `go.sum` | Pinned checksums | ✓ VERIFIED | v0.1.4 entries present; module verification/tidy diff passed. |
| `Makefile` | Default dependency isolation guard | ✓ VERIFIED | Standalone target passed. |
| `builder.go` | Fatal lifecycle routing | ✓ VERIFIED | Correct for returned marker errors; cannot route a propagated panic. |
| `parser_lifecycle_test.go` | Generic marker/terminal-builder tests | ✓ VERIFIED | Default focused suite passed. |
| `parser_simd_lifecycle_test.go` | Injected SIMD lifecycle tests | ⚠ PARTIAL | Close-only and returned combined errors covered; panic-plus-close is missing. |

### Key Link Verification

| From | To | Via | Status | Details |
| --- | --- | --- | --- | --- |
| `parser_simd_test.go` | `AddDocument` / `mergeDocumentState` | Parser → AddDocument → commit → Finalize/Encode | ✓ WIRED | Durable counters and path state asserted. |
| `StageFloat64` | `stageNumericObservation` | Direct non-folding staged value with `tagStageError` | ✓ WIRED | `parser_sink.go:74-90`. |
| `StageInt64` / `StageUint64` | `stageNativeNumeric` | Typed wrapper reuse | ✓ WIRED | `parser_sink.go:66-72`. |
| README SIMD section | Deployment guide | Relative link | ✓ WIRED | `README.md:110`. |
| Constructor error | Deployment recovery | `PURE_SIMDJSON_LIB_PATH` hint | ✓ WIRED | `parser_simd.go:28`; guide lines 58-61. |
| Root NOTICE | Pinned dependency NOTICE | Versioned attribution | ✓ VERIFIED | Local claims match pinned files. |
| `walkElement` | Typed sink methods | Element-kind dispatch | ✓ WIRED | `parser_simd.go:81-114`. |
| `materializeElement` | `StageMaterialized` | `json.Number` subtree values | ✓ WIRED | `parser_simd.go:73-79,170-206`. |
| `Parse` | `Doc.Close` | `finishSIMDDocument` callback | ⚠ PARTIAL | Close runs once, including panic, but a close error during panic is not returned. |
| `simd-isolation-check` | Default dependency graph | Exact source/graph assertions | ✓ WIRED | Target passed. |
| `finishSIMDDocument` | `parserLifecycleError` | Failed-close combiner | ⚠ PARTIAL | Works for returned walk errors, not panics. |
| `AddDocument` | `GINBuilder.tragicErr` | Lifecycle detection before recoverable branches | ✓ WIRED | `builder.go:412-416`. |
| Lifecycle tests | Cleanup helper | Injected callbacks and counters | ⚠ PARTIAL | Required returned-error cases pass; panic-plus-close missing. |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Produces Real Data | Status |
| --- | --- | --- | --- | --- |
| Direct SIMD scalar path | Native typed leaves | `Parser.Parse` → element kind/accessor → typed sink → staged state → merge | Yes by static trace; runtime parity deferred | ✓ FLOWING |
| Transform path | Materialized subtree | Native element → `json.Number`/Go values → `StageMaterialized` | Yes by static trace; runtime parity deferred | ✓ FLOWING |
| Returned close-failure path | Cleanup and concurrent returned error | `finishSIMDDocument` → `parserLifecycleError` → `AddDocument` → `tragicErr` | Yes | ✓ FLOWING |
| Panic-plus-close path | Cleanup failure during walk panic | Deferred close assigns named return, then panic resumes | No lifecycle marker reaches builder | ✗ DISCONNECTED |
| Default build selection | Parser implementation/dependency graph | No tag → adapter excluded → `stdlibParser` default | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Full default suite | `go test -count=1 ./...` | All packages passed; root package 25.542s | ✓ PASS |
| Full tagged suite | `go test -tags simdjson -count=1 ./...` | All packages passed; root package 27.162s | ✓ PASS |
| Default lifecycle/typed sink | `go test -run '^Test(ParserLifecycle|TypedSink)' -count=1 .` | `ok`, package test 1.055s | ✓ PASS |
| Tagged lifecycle/typed sink | `go test -tags simdjson -run '^Test(SIMDDocumentLifecycle|TypedSink)' -count=1 .` | `ok`, package test 0.461s | ✓ PASS |
| Numeric fixture golden | `go test -run '^TestParserParity_AuthoredFixtures/simd-numeric-parity$' -count=1 .` | `ok`, package test 0.765s | ✓ PASS |
| Default build/vet | `go build ./... && go vet ./...` | Exit 0 | ✓ PASS |
| Tagged build/vet | `go build -tags simdjson ./... && go vet -tags simdjson ./...` | Exit 0 | ✓ PASS |
| Default dependency isolation | `make simd-isolation-check` | Exit 0 | ✓ PASS |
| Module integrity/tidiness | `go mod verify && go mod tidy -diff` | `all modules verified`, exit 0 | ✓ PASS |
| Walk panic plus close failure | `go test -tags simdjson -overlay <ephemeral verifier test> -run '^TestPhase21VerifierReproducesPanicCloseFailure$' -count=1 -v .` | Reproduced: close error discarded, `builder.Err()` nil, second parser call soft-skipped | ✗ FAIL |

### Probe Execution

Step 7c was skipped: no `probe-*.sh` files or probe declarations exist for this phase.

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| SIMD-04 | 21-02, 21-03, 21-04 | Explicit same-package SIMD parser selection through the existing seam | ✓ SATISFIED | Tagged constructor/name compile and `WithParser` accepts the returned `Parser`; no default or CLI activation. |
| SIMD-05 | 21-02, 21-03 | Default stdlib-only build/runtime | ✓ SATISFIED | Default builder is stdlib; default package/test graph excludes pure-simdjson; isolation target passed. |
| SIMD-06 | 21-01, 21-02, 21-03, 21-04 | Exact-int semantics and no silent float64 coercion | ✓ SATISFIED | Type-directed accessors, non-folding float stage, `json.Number` materialization, and focused tests. |
| SIMD-07 | 21-01, 21-03, 21-04 | Typed scalar fast paths avoid `any` | ✓ SATISFIED | Five typed methods and direct adapter dispatch exist and are wired. |

All four Phase 21 requirement IDs appear in PLAN frontmatter and `.planning/REQUIREMENTS.md`; none is orphaned.

### Anti-Patterns and Advisory Review Triage

| File | Line | Pattern | Severity | Impact |
| --- | --- | --- | --- | --- |
| `parser_simd.go` | 51-63 | Deferred cleanup assigns an error during panic without recovery | 🛑 BLOCKER | Close failure is lost; builder remains open and a later busy error can be soft-skipped. Independently reproduced. |
| `parser_simd.go` | 19-35 | Native parser handle has no deterministic close path through the returned interface | ⚠ WARNING | Resource lifetime relies on upstream finalization; constructor shape was explicitly locked, so changing ownership needs a developer decision. |
| `parser_simd.go` | 81, 172 | `Element.Type()` collapses native causes to `TypeInvalid` | ⚠ WARNING | Diagnostics lose underlying native error causes; strict type-directed routing still holds. |
| `parser_simd_lifecycle_test.go` | 51-73 | Panic test covers only successful close | ⚠ WARNING | The blocking combined branch is absent from committed tests. |
| Tagged tests / normal automation | — | No real native adapter execution and no enforced tagged CI | ⚠ DEFERRED | Phase 22 success criteria 1 and 3 explicitly own these checks. |
| `builder.go` | 642-654 | Representation-skip metric is published before document commit | ℹ OUT OF PHASE | Git blame shows this behavior predates Phase 21; it does not change the Phase 21 verdict. |

No unreferenced `TBD`, `FIXME`, or `XXX` marker was found in phase-owned files. Broad empty-value matches were ordinary test assertions, initialized state, or valid slice/error returns, not stubs.

### Disconfirmation Pass

- **Partial requirement detail:** returned close failures are terminal, but the same close failure during a walk panic is not.
- **Misleading green test:** `TestSIMDDocumentLifecycleCleanupRunsWhenWalkPanics` proves cleanup is called only when cleanup succeeds; it cannot detect loss of a simultaneous close error.
- **Uncovered error path:** no committed test combines walk panic, close failure, parser soft mode, caller recovery, and retry.

### Human Verification Required

None within the Phase 21 boundary. The blocker is programmatically reproducible. Real native runtime parity and platform behavior are explicitly deferred to Phase 22.

### Gaps Summary

The lifecycle design is correct for ordinary returned errors but incomplete for Go panic unwinding. A close failure must still reach the fatal builder route when walking panics; otherwise the upstream parser can remain busy while the builder remains reusable. The closure plan needs one combined panic/close regression and lifecycle handling that preserves the intended panic semantics without losing the fatal cleanup failure.

---

_Verified: 2026-07-23T07:04:54Z_
_Verifier: the agent (gsd-verifier)_
