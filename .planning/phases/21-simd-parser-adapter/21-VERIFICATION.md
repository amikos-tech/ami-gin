---
phase: 21-simd-parser-adapter
verified: 2026-07-22T19:06:38Z
status: gaps_found
score: "13/15 must-haves verified"
overrides_applied: 0
gaps:
  - truth: "Every successful upstream Parse closes its Doc safely so a reusable parser cannot become permanently busy after a cleanup fault"
    status: failed
    reason: "A Doc.Close failure is returned as an ordinary parser error. Parser soft mode suppresses it, while upstream leaves Parser.liveDoc set; every later AddDocument then receives ErrParserBusy and can also be silently soft-skipped."
    artifacts:
      - path: "parser_simd.go"
        issue: "Lines 42-51 propagate Doc.Close failure without marking the parser or builder fatally unusable."
      - path: "builder.go"
        issue: "Lines 412-433 apply ParserFailureMode to the cleanup error and never set tragicErr."
    missing:
      - "A fatal parser-lifecycle error path that bypasses ParserFailureMode, closes the builder, and rejects subsequent AddDocument calls hard."
      - "An injectable lifecycle test covering Doc.Close failure followed by another parse."
  - truth: "Every hard staged numeric error retains its stage provenance even when document cleanup also fails"
    status: failed
    reason: "When walking and Doc.Close both fail, parser_simd.go wraps closeErr and interpolates the walk error with %v. The stageCallbackError is removed from the unwrap chain, so AddDocument reclassifies the failure as parser-layer or soft-skips it under ParserFailureMode."
    artifacts:
      - path: "parser_simd.go"
        issue: "Line 48 stringifies the prior walk error instead of preserving it as the wrapped cause."
      - path: "builder.go"
        issue: "isStageCallbackError at lines 426-428 can no longer detect the staged error."
    missing:
      - "Combined-error handling that preserves stageCallbackError/softSkipDocumentError in the unwrap chain while also reporting the close failure."
      - "A deterministic test for simultaneous walk/stage and Doc.Close failures."
deferred:
  - truth: "Real native SIMD execution and byte/query parity across authored and realistic fixtures"
    addressed_in: "Phase 22"
    evidence: "Phase 22 success criterion 1 requires SIMD/stdlib encoded-index and query-result parity; Phase 21 context explicitly defers real simdParser parity tests."
  - truth: "Required tagged CI, platform behavior, and native-runtime availability gates"
    addressed_in: "Phase 22"
    evidence: "Phase 22 success criterion 3 requires default and -tags simdjson CI with explicit shared-library behavior."
---

# Phase 21: SIMD Parser Adapter Verification Report

**Phase Goal:** Land an opt-in same-package SIMD parser implementation behind the existing parser seam without changing default stdlib behavior.
**Verified:** 2026-07-22T19:06:38Z
**Status:** gaps_found
**Re-verification:** No — initial verification

## Goal Achievement

The normal path is implemented and all requested default, tagged, focused, and isolation checks pass. The phase nevertheless misses two absolute PLAN must-haves on the same cleanup-failure path: a failed native document close can poison the reusable parser while soft mode hides every subsequent failure, and a simultaneous close plus staged error loses the staged error's layer.

### Observable Truths

Roadmap success criteria and PLAN frontmatter truths were merged and deduplicated below. Plan-specific details remain separate where they impose behavior beyond the roadmap wording.

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | A tagged caller can construct the same-package `NewSIMDParser() (Parser, error)`, observe `Name()=="pure-simdjson"`, and select it with `WithParser`, with no CLI path. | ✓ VERIFIED | `parser_simd.go:1,17-35`; `parser.go:54-61`; tagged build/test compile; no product CLI SIMD selector found. |
| 2 | Default builds remain stdlib-only and every product importer of pure-simdjson has the exact first-line tag. | ✓ VERIFIED | `builder.go:234-236` selects `stdlibParser`; default dependency graph has no pure-simdjson line; tagged graph does; only `parser_simd.go` imports it and line 1 is exact; `make simd-isolation-check` passed. |
| 3 | Exact int64/uint64 routing preserves `9007199254740993` and rejects values above `math.MaxInt64` through the existing hard/soft numeric policy without float coercion or partial commits. | ✓ VERIFIED | `parser_simd.go:83-100`; `parser_sink.go:66-89`; `builder.go:728-760`; focused typed-sink tests passed. |
| 4 | The parser sink exposes string, bool, int64, uint64, and float64 fast paths, and non-null SIMD scalar leaves use them rather than `any`. | ✓ VERIFIED | Interface and GINBuilder implementations at `parser_sink.go:28-40,58-90`; strict SIMD dispatch at `parser_simd.go:73-106`. |
| 5 | `StageFloat64` preserves float classification and every hard staged numeric error retains numeric-stage provenance. | ✗ FAILED | Normal `1.0`, `1e18`, and non-finite tests pass, but `parser_simd.go:47-49` removes a staged error from the unwrap chain when `Doc.Close` also fails; `builder.go:426-433` then reclassifies it as parser-layer. |
| 6 | Typed-sink comparisons commit through `AddDocument` before `Finalize` and cannot pass by comparing empty indexes. | ✓ VERIFIED | `parser_simd_test.go:31-68` asserts `numDocs`, `nextPos`, doc mapping, and root path after `AddDocument`; all `TestTypedSink*` tests passed. |
| 7 | The numeric parity fixture keeps numeric classes on separate paths and its isolated non-empty golden does not alter prior goldens. | ✓ VERIFIED | `parser_parity_fixtures_test.go:23-31`; 604-byte golden SHA-256 `696b8a619eea2f5629bcf8fa6d285473a12c09a0e580f312fcbf2bcf2aa5ef8f`; focused authored-fixture test and excluded-golden diff passed. |
| 8 | The root NOTICE traces pure-simdjson v0.1.4 MIT licensing and bundled simdjson Apache-2.0/MIT attribution to pinned upstream files. | ✓ VERIFIED | `NOTICE.md:3-24` matches the cached v0.1.4 `LICENSE` and `NOTICE`. |
| 9 | Documentation separates tagged build, fallible construction, and explicit builder selection with a valid two-step example. | ✓ VERIFIED | `docs/simd-deployment.md:17-61`; README example at `README.md:80-112`; no invalid `WithParser(NewSIMDParser())` use. |
| 10 | Air-gapped/mirror guidance names all four bootstrap variables and accurately separates integrity guarantees. | ✓ VERIFIED | `docs/simd-deployment.md:63-164` names `PURE_SIMDJSON_LIB_PATH`, `PURE_SIMDJSON_BINARY_MIRROR`, `PURE_SIMDJSON_DISABLE_GH_FALLBACK`, and `PURE_SIMDJSON_CACHE_DIR`, matching pinned upstream bootstrap docs. |
| 11 | The accepted greater-than-uint64 BIGINT divergence is documented by layer, path, governing mode, and atomic document disposition. | ✓ VERIFIED | `docs/simd-deployment.md:166-183`; `CHANGELOG.md:5`. |
| 12 | README and CHANGELOG preserve stdlib defaults and avoid claiming Phase 22 parity, performance, or platform evidence. | ✓ VERIFIED | `README.md:80-112`; `CHANGELOG.md:3-5`; deployment validation scope at `docs/simd-deployment.md:185-190`. |
| 13 | Every successful upstream parse closes its document safely on success and walker-error paths so parser reuse cannot become `ErrParserBusy`. | ✗ FAILED | A close is attempted once (`parser_simd.go:42-52`), but upstream `Doc.Close` leaves `liveDoc` set on failure (`pure-simdjson@v0.1.4/doc.go:39-56`); `Parser.Parse` then returns `ErrParserBusy` (`parser.go:64-76`). The builder soft-skips both errors. |
| 14 | Transform-buffered subtrees preserve stdlib `json.Number` semantics and float lexeme classification. | ✓ VERIFIED | Transform guard precedes type inspection (`parser_simd.go:64-71`); materializer returns `json.Number` for every numeric tag and appends `.0` to whole floats (`parser_simd.go:162-198`); `StageMaterialized` routes it through raw-number classification. |
| 15 | SIMD objects are last-key-wins with stdlib path construction; containers and both array paths re-check transforms. | ✓ VERIFIED | `parser_simd.go:107-154,224-243` collapses duplicates into maps, sorts direct-walk keys, uses `rawPath+"."+key`, emits both `[i]` and `[*]`, and recurses through the transform guard. |

**Score:** 13/15 truths verified

### PLAN Frontmatter Coverage

| Plan | PLAN truth | Merged truth | Status |
| --- | --- | --- | --- |
| 21-01 | Five typed scalar boundary methods; no `any` for non-null SIMD leaves | #4 | ✓ VERIFIED |
| 21-01 | Float classification and hard non-finite stage-error provenance | #5 | ✗ FAILED |
| 21-01 | Exact int64 and oversized uint64 hard/soft atomic policy | #3 | ✓ VERIFIED |
| 21-01 | Typed comparisons commit through `AddDocument` | #6 | ✓ VERIFIED |
| 21-01 | Separate-path numeric fixture and isolated golden | #7 | ✓ VERIFIED |
| 21-02 | Pinned license/NOTICE attribution chain | #8 | ✓ VERIFIED |
| 21-02 | Three activation gates and two-step example | #9 | ✓ VERIFIED |
| 21-02 | Four bootstrap variables and route-specific integrity | #10 | ✓ VERIFIED |
| 21-02 | Accurate BIGINT divergence | #11 | ✓ VERIFIED |
| 21-02 | README/CHANGELOG preserve defaults and evidence boundaries | #12 | ✓ VERIFIED |
| 21-03 | Constructor/name/WithParser selection and no CLI path | #1 | ✓ VERIFIED |
| 21-03 | Doc lifecycle remains reusable across all walk outcomes | #13 | ✗ FAILED |
| 21-03 | Typed scalar routing strictly from element kind | #4 | ✓ VERIFIED |
| 21-03 | Transform-buffered numeric semantics | #14 | ✓ VERIFIED |
| 21-03 | Duplicate-key, path, container, and transform behavior | #15 | ✓ VERIFIED |
| 21-03 | Exact source tag and default dependency isolation | #2 | ✓ VERIFIED |

### Deferred Items

| # | Item | Addressed In | Evidence |
| --- | --- | --- | --- |
| 1 | Real native adapter execution and encoded/query parity | Phase 22 | Phase 22 SC1 and the Phase 21 context explicitly assign tagged parity to Phase 22. |
| 2 | Enforced tagged CI/platform/native-runtime gates | Phase 22 | Phase 22 SC3 explicitly owns default and tagged CI behavior. |

The lifecycle failures are **not deferred**: no later roadmap goal or success criterion specifically owns fatal `Doc.Close` handling or staged-error provenance.

## Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `parser_sink.go` | Five typed sink methods | ✓ VERIFIED | 108 lines; substantive implementations; used by SIMD walker and tests. |
| `parser.go` | Parser contract and `WithParser` seam | ✓ VERIFIED | Contract lists exact numeric routes and stage family; builder consumes the option. |
| `parser_test.go` | Expanded recording sink | ✓ VERIFIED | All five typed methods implemented at lines 199-221. |
| `parser_simd_test.go` | Committed typed numeric tests | ✓ VERIFIED | 303 lines; substantive and passing. It intentionally tests the sink wrapper, not the native adapter. |
| `parser_parity_fixtures_test.go` | SIMD numeric fixture | ✓ VERIFIED | Fixture is registered and consumed by authored parity and regenerator tests. |
| `testdata/parity-golden/simd-numeric-parity.bin` | Non-empty stdlib reference | ✓ VERIFIED | 604 bytes; consumed through `loadGolden`. |
| `NOTICE.md` | Pinned attribution | ✓ VERIFIED | Content agrees with cached upstream v0.1.4 files. |
| `docs/simd-deployment.md` | Activation/operations guidance | ✓ VERIFIED | 190 lines; complete and linked. |
| `README.md` | Optional SIMD installation | ✓ VERIFIED | One section between Installation and Quick Start, linked to guide. |
| `CHANGELOG.md` | Unreleased adapter entry | ✓ VERIFIED | Accurate defaults and BIGINT limitation; no premature validation claim. |
| `parser_simd.go` | Tagged lifecycle-safe adapter | ✗ PARTIAL | 255 substantive, wired lines; normal path is complete, but cleanup failure is soft-skippable and combined errors lose stage provenance. |
| `go.mod` | Exact v0.1.4 dependency pin | ✓ VERIFIED | Direct pin at line 12. |
| `go.sum` | Pinned wrapper checksums | ✓ VERIFIED | v0.1.4 entries at lines 50-51; `go mod verify` and `go mod tidy -diff` passed. |
| `Makefile` | Default dependency isolation guard | ✓ VERIFIED | Target at lines 7-31; source and graph assertions execute successfully. |

## Key Link Verification

| From | To | Via | Status | Details |
| --- | --- | --- | --- | --- |
| `parser_simd_test.go` | `AddDocument` / `mergeDocumentState` | Test Parser → AddDocument → commit → Finalize/Encode | ✓ WIRED | Commit counters and path data asserted before byte comparison. |
| `StageFloat64` | `stageNumericObservation` | Direct `stagedNumericValue{isInt:false}` with `tagStageError` | ✓ WIRED | `parser_sink.go:74-89`; no `stageNativeNumeric` call. |
| `StageInt64` / `StageUint64` | `stageNativeNumeric` | Typed wrapper reuse | ✓ WIRED | `parser_sink.go:66-72`. |
| README SIMD section | Deployment guide | Relative Markdown link | ✓ WIRED | `README.md:110`. |
| Constructor error | Deployment recovery | `PURE_SIMDJSON_LIB_PATH` hint | ✓ WIRED | `parser_simd.go:28`; guide lines 58-61. |
| Root NOTICE | Pinned upstream NOTICE | Versioned attribution | ✓ VERIFIED | Local claims match cached upstream NOTICE/LICENSE. |
| `walkElement` | Typed sink methods | Element-kind dispatch | ✓ WIRED | `parser_simd.go:73-106`. |
| `materializeElement` | Builder materialized staging | `json.Number` → `StageMaterialized` | ✓ WIRED | `parser_simd.go:65-70,174-198`; `parser_sink.go:100-102`. |
| `Parse` | `Doc.Close` / AddDocument error routing | Deferred close | ✗ PARTIAL | Close is called, but failure does not invalidate the builder and combined failures break stage-error unwrapping. |
| `simd-isolation-check` | Default dependency graph | `go list -deps -test ./...` exact absence | ✓ WIRED | Target passed; default graph absent, tagged graph present. |

## Data-Flow Trace (Level 4)

| Artifact | Data | Source | Produces real data | Status |
| --- | --- | --- | --- | --- |
| `parser_simd.go` direct scalar path | Typed JSON leaves | `purejson.Parser.Parse` → `Element.Type`/typed accessor → typed sink → builder staged state → merge | Yes on normal path | ✓ FLOWING |
| `parser_simd.go` transform path | Materialized subtree | Native element → `json.Number`/Go values → `StageMaterialized` → transformer/builder | Yes on normal path | ✓ FLOWING |
| `parser_simd.go` cleanup failure path | Parse/walk error provenance and parser reuse | `Doc.Close` → `Parse` return → `AddDocument` failure policy | No safe terminal state | ✗ BROKEN |
| Default build selection | Parser implementation/dependency graph | No build tag → `parser_simd.go` excluded → `stdlibParser` default | Yes | ✓ FLOWING |

## Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Typed sink exactness/classification/atomic policy | `go test -run '^TestTypedSink' -count=1 .` | `ok`, 0.884s package time | ✓ PASS |
| Numeric fixture matches committed golden | `go test -run '^TestParserParity_AuthoredFixtures/simd-numeric-parity$' -count=1 .` | `ok`, 0.581s package time | ✓ PASS |
| Default suite | `go test ./...` | All packages passed, 26.421s root package | ✓ PASS |
| Tagged suite | `go test -tags simdjson ./...` | All packages passed, 26.087s root package | ✓ PASS, compile/test coverage only; no test constructs `NewSIMDParser` |
| Default build and vet | `go build ./... && go vet ./...` | Exit 0 | ✓ PASS |
| Tagged build and vet | `go build -tags simdjson ./... && go vet -tags simdjson ./...` | Exit 0 | ✓ PASS |
| Source/default-graph isolation | `make simd-isolation-check` | Exit 0 | ✓ PASS |
| Module integrity/tidiness | `go mod verify && go mod tidy -diff` | `all modules verified`, exit 0 | ✓ PASS |
| Dependency split | default/tagged `go list -deps -test ./...` | Default: absent; tagged: exact package present | ✓ PASS |
| Existing golden isolation | excluded-golden diff command | Exit 0 | ✓ PASS |

## Probe Execution

Step 7c was skipped: no `probe-*.sh` files or probe declarations were found for this phase.

## Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| SIMD-04 | 21-02, 21-03 | Explicit same-package SIMD parser selection through existing seam | ✓ SATISFIED | Tagged constructor/name exist and compile; `WithParser` accepts the returned `Parser`; no default/CLI activation. |
| SIMD-05 | 21-02, 21-03 | Default stdlib-only build/runtime | ✓ SATISFIED | Default builder is stdlib; default package/test graph excludes pure-simdjson; isolation target passes. |
| SIMD-06 | 21-01, 21-02, 21-03 | Exact-int semantics and no silent float64 coercion | ✓ SATISFIED | Type-driven exact accessors, non-folding `StageFloat64`, `json.Number` transform materialization, and focused tests. Cleanup-error provenance remains a separate failed PLAN must-have. |
| SIMD-07 | 21-01, 21-03 | Typed scalar fast paths avoid `any` | ✓ SATISFIED | Five typed sink methods and direct adapter dispatch are present and wired. |

No Phase 21 requirement is orphaned: all four roadmap IDs appear in PLAN frontmatter and are mapped above.

## Anti-Patterns and Adversarial Findings

| File | Line | Pattern | Severity | Impact |
| --- | --- | --- | --- | --- |
| `parser_simd.go` | 42-51 | Cleanup failure treated as ordinary parser failure | 🛑 BLOCKER | Parser soft mode can silently drop the failed and all subsequent documents after `liveDoc` remains set. |
| `parser_simd.go` | 47-49 | Prior staged error interpolated with `%v` | 🛑 BLOCKER | Numeric/transformer/schema provenance is lost on a simultaneous close failure. |
| `parser_simd.go` | 19-35 | Public constructor returns non-closeable base `Parser` | ⚠ WARNING | Callers cannot deterministically release the native parser handle; the API shape was explicitly locked before Phase 21, so changing it needs a developer decision. |
| `parser_simd.go` | 73, 164 | `Element.Type()` collapses native inspection errors | ⚠ WARNING | Native causes such as closed/invalid handle become generic `TypeInvalid`; this does not violate the type-driven routing requirement but weakens diagnostics. |
| `parser_simd_test.go` | 1-303 | SIMD-named tests use only `typedSinkTestParser` | ⚠ DEFERRED | Tagged tests prove compilation, not real native execution; Phase 22 explicitly owns parity/runtime validation. |
| `Makefile` | 7-31 | SIMD gates are standalone | ⚠ DEFERRED | Normal targets do not enforce tagged checks; Phase 22 explicitly owns CI coverage. |

No unreferenced `TBD`, `FIXME`, or `XXX` debt marker was found in phase source files. Empty returns found by broad scanning are ordinary test doubles or valid branches, not stubs.

### Disconfirmation Pass

- **Partial must-have:** cleanup is attempted exactly once, but a failed close does not produce a safe terminal state.
- **Misleading green test:** `go test -tags simdjson ./...` passes without constructing or executing `NewSIMDParser`.
- **Uncovered error path:** no seam or test injects `Doc.Close`, `TypeErr`, accessor, or iterator failures; the close-plus-stage path is observably incorrect by static trace.

## Human Verification Required

None within the Phase 21 boundary. The blocking findings are code-verifiable. Real native runtime parity and platform behavior are explicitly deferred to Phase 22 rather than converted into Phase 21 human checks.

## Gaps Summary

Both blocking truths have one root cause: native document cleanup errors are not modeled as fatal lifecycle failures. The adapter must prevent parser soft mode from hiding a failed close, poison the builder so later documents cannot be silently discarded, and preserve an earlier staged error in the unwrap chain when walking and cleanup both fail. The required fix should include an injectable failure seam because the current tests cannot deterministically exercise either branch.

---

_Verified: 2026-07-22T19:06:38Z_
_Verifier: the agent (gsd-verifier)_
