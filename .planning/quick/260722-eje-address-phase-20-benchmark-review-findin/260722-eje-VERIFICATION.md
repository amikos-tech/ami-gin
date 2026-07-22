---
phase: quick-260722-eje
plan: 01
verified: 2026-07-22T11:36:20+03:00
status: passed
score: 8/8 must-haves verified
---

# Quick Task 260722-eje Verification

**Goal:** Address Phase 20 benchmark review findings and harden external corpus coverage.
**Status:** passed

## Truths

| # | Observable truth | Status | Evidence |
|---|---|---|---|
| 1 | The external tier is disabled only for an unset/empty gate, enabled only by exact `1`, and rejects all other nonempty values before directory access; input remains bounded to 64 regular top-level files, 8 MiB total, 2 MiB per document, and depth 64 without following symlinks. | VERIFIED | `phase20ExternalBenchmarkPaths` checks the enable value before reading the directory and filters sorted regular top-level JSON-family files. Constants retain 64/8 MiB/64 and set only the document cap to 2 MiB. `TestPhase20ExternalTier` and `TestPhase20ExternalResourceLimitsAndAccounting` cover the gate table, symlink-only discovery, file count, a 1,727,204-byte accepted document, and document/scanner limits. |
| 2 | Every accepted external document is fully depth-validated before benchmark index construction, including a shallow-first/later-over-depth document. | VERIFIED | Both single-document JSON and every nonblank JSONL/NDJSON record call `phase20ValidateExternalDocumentDepth` inside `phase20LoadExternalDocuments`. The recursive walk visits sorted object keys and all array elements. `TestPhase20ExternalDepthValidation` rejects the early-`a_shallow`/later-`z_deep` document through direct JSON, direct JSONL, and enabled discovery. |
| 3 | External predicate derivation skips null, invalid, and unresolved candidates; generated paths for `a.b` and `user-name` are proven unresolved, while a later safe scalar resolves through the built index. | VERIFIED | `phase20FirstScalarPath` continues after candidate rejection and accepts only a successful `idx.findPath`, returning `entry.PathName`. `TestPhase20DerivedQueryUsesResolvedPath` uses a real index to assert `(-1, nil)` for both special-key candidates, selects `$.z_safe`, verifies `resolvePredicatePath`, covers null-first continuation, and checks null-only/no-scalar/unresolved/invalid-syntax diagnostics. |
| 4 | Query timing cannot accept the unresolved-path `AllRGs` fallback, while a valid all-row-groups `IsNotNull` result remains legal. | VERIFIED | `phase20BenchmarkQuery` builds first, derives when needed, and calls `phase20PreflightQuery` before `b.ResetTimer`. Preflight requires a valid path ID and nonnil `PathEntry`, then rejects only a zero warm-up result. `TestPhase20QueryPreflight` rejects an empty predicate, accepts all-RG `IsNotNull("$.safe")`, and rejects a resolved zero-result predicate. |
| 5 | JSONL loading covers exact LF/CRLF bytes, final unterminated records, scanner overflow, and aggregate-budget decrement across files. | VERIFIED | The delimiter-preserving split function includes newline bytes; focused tests assert exact byte counts for LF, CRLF, and an unterminated final record, contextual overflow, and success/failure at exact vs one-byte-short multi-file budgets. |
| 6 | Fixture generation is ordered, deterministic, exact-byte tested by the normal test target, uses `pkg/errors`, and returns expected I/O errors without panic. | VERIFIED | `generatedFixtures` has the required four-file order; `renderFixture` supplies the shared single-final-newline bytes; `TestGeneratedFixturesMatchCommitted` uses `bytes.Equal`; and Makefile `test` selects `./... ./testdata/phase20`. `generate.go` uses `errors.Errorf`/`errors.Wrapf`, has a `run() error` boundary, prints one error, and exits nonzero. The symlink-destination failure test passes and no `fmt.Errorf` or `panic(` remains. |
| 7 | Documentation/security limits are consistent and present tense; planning state points to Phase 21 with three completed plans while Phase 999.7 remains backlog-only. | VERIFIED | Root and fixture READMEs state exact `1`, regular top-level files, no symlinks, and 64 files/8 MiB total/2 MiB per document/depth 64. T20-D01 records the same resource bounds. Exact assertions passed for Phase 21 and `completed_plans: 3`; ROADMAP remains unchanged and retains `Phase 999.7 ... (BACKLOG)`. |
| 8 | The focused tests, generator package, deterministic generation, standard-target discovery, and offline smoke benchmark pass without disturbing the user's config edit. | VERIFIED | Fresh commands below passed. The executor's recorded `make test` result was also inspected: 1,015 tests passed with one pre-existing optional-Parquet-data skip. The config diff hash stayed `e79b4b5e...f77e81`, its cached diff is empty, and its status remains exactly unstaged. |

## Artifacts

| Artifact | Status | Evidence |
|---|---|---|
| `benchmark_test.go` | VERIFIED | Contains exact gating, bounded path loading, full-document depth validation, real-index derivation/preflight, and focused regressions. |
| `testdata/phase20/generate.go` | VERIFIED | Contains ordered `generatedFixtures`, shared rendering, `pkg/errors`, atomic writes, and a non-panicking command boundary. |
| `testdata/phase20/generate_test.go` | VERIFIED | Preserves symlink rejection and exact committed-byte drift coverage for all four fixtures. |
| `Makefile` | VERIFIED | `test` contains `--packages="./... ./testdata/phase20"`. |
| `README.md`, `testdata/phase20/README.md`, `20-SECURITY.md` | VERIFIED | Exact gate and resource contracts agree; optional local-run wording is present tense. |
| `.planning/STATE.md`, `.planning/ROADMAP.md` | VERIFIED | Phase 21/current-plan count assertions pass; ROADMAP is unchanged and Phase 999.7 is backlog-only. |

## Key Links

| From | To | Status | Evidence |
|---|---|---|---|
| `phase20BenchmarkQuery` | `GINIndex.resolvePredicatePath` | WIRED | Query calls the shared preflight after index build/derivation and before reset/timing. |
| `phase20DeriveScalarPathPredicate` | `GINIndex.findPath` / `PathEntry.PathName` | WIRED | Only a concrete built-index entry can produce the derived predicate. |
| `phase20LoadExternalDocuments` | `phase20ValidateExternalDocumentDepth` | WIRED | JSON and each JSONL/NDJSON document validate before being appended. |
| `phase20DiscoverExternalDocuments` | `phase20LoadExternalDocumentPaths` | WIRED | One remaining-byte budget is decremented across sorted files. |
| Makefile `test` | generator package tests | WIRED | Dry-run output contains the explicit `./testdata/phase20` package selection. |
| `TestGeneratedFixturesMatchCommitted` | committed JSONL fixtures | WIRED | Shared rendered bytes are compared with `bytes.Equal`. |
| `.planning/STATE.md` | `.planning/ROADMAP.md` | CONSISTENT | Phase 21 is current; Phase 999.7 remains backlog-only. |

## Verification Evidence

| Check | Result |
|---|---|
| `go test ./... -run '^TestPhase20' -count=1` with external variables unset | PASS (`ami-gin` package `ok`; focused resource, depth, derivation, and preflight regressions executed). |
| `go test ./testdata/phase20 -count=1` | PASS (`ok .../testdata/phase20`). |
| `make -n test` package-selection assertion | PASS; emitted `--packages="./... ./testdata/phase20"`. |
| `go run ./testdata/phase20/generate.go` then exact four-file `git diff --exit-code` | PASS; no committed fixture drift. |
| Offline smoke benchmark, `-benchtime=1x -count=1 -benchmem` | PASS; all 12 Build/Encode/Query leaves completed. |
| STATE/ROADMAP/documentation assertions | PASS. No state loader was run against the main worktree. |
| `git diff --check` and config preservation checks | PASS; config remains the same unstaged user edit with no cached diff. |
| Executor-recorded full `make test` | PASS; 1,015 tests, one existing optional fixture skip. Repetition was redundant after the fresh focused/package/smoke checks. |

## Residual Manual Item

No external simdjson directory was supplied, so the optional real-corpus benchmark was not run. This is not an acceptance gap: focused tests exercise enabled discovery and loading, including the documented 1,727,204-byte shape, all resource bounds, full-tree depth rejection, and resolved-query behavior.

## Gaps Summary

No gaps found. The implementation satisfies the plan truths, artifacts, and key links without changing the public builder/query path model. `.planning/STATE.md` remains ready for the orchestrator's scoped planning-artifact commit; the unrelated `.planning/config.json` edit remains untouched and unstaged.

---

_Verifier: Codex (independent quick-full verification)_
