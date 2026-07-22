---
phase: quick-260722-eje
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - benchmark_test.go
  - README.md
  - testdata/phase20/README.md
  - .planning/phases/20-realistic-benchmark-dataset-foundation/20-SECURITY.md
  - testdata/phase20/generate.go
  - testdata/phase20/generate_test.go
  - Makefile
  - .planning/STATE.md
autonomous: true
requirements:
  - P20-REVIEW-EXTERNAL-CORPUS
  - P20-REVIEW-GENERATOR-DISCOVERY
  - P20-REVIEW-PLANNING-STATE
tags: [benchmarks, fixtures, tests, planning-state]

must_haves:
  truths:
    - "The optional Phase 20 corpus gate is disabled only when unset or empty, enables only for the exact value 1, and rejects every other nonempty value before accessing the directory setting."
    - "External input remains limited to 64 regular top-level files, 8 MiB total, 2 MiB per document, and JSON depth 64; symlinks are not followed and a current-size citm_catalog.json-shaped document fits under the per-document cap."
    - "Every external JSON document is fully traversed for depth during loading, before Build, Encode, or Query construction; an early shallow scalar cannot hide a later branch deeper than 64."
    - "Every Phase 20 Query benchmark resolves its predicate to a real PathDirectory entry before timing; external derivation treats user-name and a.b as unresolved negative candidates, skips them for a later compatible scalar, and never guesses an unsupported raw builder path."
    - "JSONL tests prove final unterminated-record handling, LF/CRLF byte accounting, scanner token overflow, and aggregate-budget decrement across multiple files."
    - "The fixture generator has stable ordered output, reports ordinary failures without panic, follows pkg/errors conventions, and byte-for-byte drift against all four committed fixtures is tested by the standard make test target."
    - "The root README, fixture README, and Phase 20 security register agree on the exact gate and resource limits, while STATE.md points to planned Phase 21 with exactly 3 completed v1.3 plans and leaves Phase 999.7 in the ROADMAP backlog."
    - "Focused Phase 20 tests, the generator package, full make test, deterministic generator drift, and the offline smoke benchmark all pass."
  artifacts:
    - path: "benchmark_test.go"
      provides: "Exact external gate, bounded multi-file loading, full-document depth validation, resolvable predicate derivation/preflight, and regression tests"
      contains: "phase20ValidateExternalDocumentDepth"
    - path: "testdata/phase20/generate.go"
      provides: "Ordered deterministic fixture model and non-panicking command boundary"
      contains: "generatedFixtures"
    - path: "testdata/phase20/generate_test.go"
      provides: "Symlink protection and byte-for-byte committed-fixture drift coverage"
      contains: "TestGeneratedFixturesMatchCommitted"
    - path: "Makefile"
      provides: "Normal test-target discovery for the otherwise excluded testdata package"
      contains: "--packages=\"./... ./testdata/phase20\""
    - path: "README.md"
      provides: "Contributor-facing exact-gate, regular-file, and resource-limit contract"
      contains: "GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL"
    - path: "testdata/phase20/README.md"
      provides: "Fixture-local present-tense workflow and executable-contract metadata"
      contains: "2 MiB"
    - path: ".planning/phases/20-realistic-benchmark-dataset-foundation/20-SECURITY.md"
      provides: "Updated T20-D01 per-document mitigation value"
      contains: "2 MiB per document"
    - path: ".planning/STATE.md"
      provides: "Phase 21 current position and corrected v1.3 plan count"
      contains: "completed_plans: 3"
  key_links:
    - from: "benchmark_test.go:phase20BenchmarkQuery"
      to: "GINIndex.resolvePredicatePath"
      via: "mandatory preflight before warm-up and b.ResetTimer"
      pattern: "resolvePredicatePath"
    - from: "benchmark_test.go:phase20DeriveScalarPathPredicate"
      to: "GINIndex.findPath / PathEntry.PathName"
      via: "candidate canonicalization followed by actual index resolution"
      pattern: "findPath"
    - from: "benchmark_test.go:phase20LoadExternalDocuments"
      to: "benchmark_test.go:phase20ValidateExternalDocumentDepth"
      via: "full-tree validation of every accepted JSON or JSONL document before it enters any benchmark fixture"
      pattern: "phase20ValidateExternalDocumentDepth"
    - from: "benchmark_test.go:phase20DiscoverExternalDocuments"
      to: "phase20LoadExternalDocuments"
      via: "shared injected-budget loop that decrements bytes across sorted files"
      pattern: "remainingBytes"
    - from: "Makefile:test"
      to: "testdata/phase20/generate_test.go"
      via: "explicit gotestsum package list because ./... excludes testdata"
      pattern: "testdata/phase20"
    - from: "testdata/phase20/generate_test.go"
      to: "testdata/phase20/*.jsonl"
      via: "generatedFixtures output rendered with the final newline and byte-compared"
      pattern: "bytes.Equal"
    - from: ".planning/STATE.md"
      to: ".planning/ROADMAP.md"
      via: "Phase 21 current position while Phase 999.7 remains backlog-only"
      pattern: "Phase 21"
---

<objective>
Close every Phase 20 benchmark-review finding with bounded external-corpus handling, honest query timing, self-verifying fixtures, normal test discovery, and consistent documentation/planning state.

Purpose: Keep the realistic corpus useful for Phase 21/22 measurements without widening the trust boundary or redesigning public JSON-path/index behavior.
Output: Three atomic commits covering benchmark hardening, generator/test integration, and planning-state repair.
</objective>

<execution_context>
@$HOME/.codex/get-shit-done/workflows/execute-plan.md
@$HOME/.codex/get-shit-done/templates/summary.md
</execution_context>

<context>
@AGENTS.md
@.planning/STATE.md
@.planning/ROADMAP.md
@.planning/quick/260722-eje-address-phase-20-benchmark-review-findin/260722-eje-RESEARCH.md
@benchmark_test.go
@builder.go
@query.go
@jsonpath.go
@Makefile
@testdata/phase20/generate.go
@testdata/phase20/generate_test.go
@testdata/phase20/README.md
@README.md
@.planning/phases/20-realistic-benchmark-dataset-foundation/20-SECURITY.md

<interfaces>
Existing contracts to reuse without public redesign:

- `func (idx *GINIndex) resolvePredicatePath(path string, value any) (int, *PathEntry, any)` returns `-1, nil, value` for unsupported or unresolved paths; `Evaluate` otherwise fails open to all row groups.
- `func (idx *GINIndex) findPath(path string) (int, *PathEntry)` canonicalizes supported syntax and returns the concrete `PathEntry`; its `PathName` is the indexed spelling a derived predicate must retain.
- `func canonicalizeSupportedPath(path string) (string, error)` validates before normalizing query syntax.
- `stageMaterializedValue` currently descends objects with `path+"."+key`; this quick task must not change that builder behavior, serialized path names, or public JSONPath rules.
- Under that unchanged model, query candidates produced by `phase20JSONPathChild` for both `user-name` and `a.b` remain unresolved by `idx.findPath`; derivation must skip them and must not fall back to guessed raw dot spellings.
- `phase20BenchmarkFixture` already carries raw `docs` and a `Predicate`; an empty predicate can identify the external Query case while Build and Encode remain schema-independent.
</interfaces>
</context>

<commit_policy>
Create one commit after each task passes its verification, using these messages in order:

1. `fix(phase20): harden external benchmark corpus`
2. `test(phase20): verify generated fixture drift`
3. `docs(planning): sync Phase 21 project position`

Stage only each task's explicit `<files>` paths. Never use broad staging, never stage or commit the user's existing `.planning/config.json` modification, and verify its cached diff remains empty before every commit. Commit and PR artifacts must follow AGENTS.md restrictions. This plan does not merge anything; any later merge must be a squash merge.
</commit_policy>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Bound external loading and require a real indexed Query path</name>
  <files>benchmark_test.go, README.md, testdata/phase20/README.md, .planning/phases/20-realistic-benchmark-dataset-foundation/20-SECURITY.md</files>
  <behavior>
    - Gate table: unset/empty returns disabled without reading the directory; exactly `1` proceeds; `0`, `true`, `yes`, and whitespace-padded ` 1 ` return configuration errors before directory access.
    - Discovery accepts only sorted regular top-level `.ndjson`, `.jsonl`, and `.json` files. A symlink-only directory reports that no supported regular files were found and that symlinks are not followed.
    - A valid single-document JSON file of 1,727,204 bytes loads successfully under the new 2 MiB cap; a JSONL token over 2 MiB reaches a contextual scanner-too-long error. The 64-file, 8 MiB aggregate, and depth-64 limits do not change.
    - A valid final JSONL record without a newline is returned and charged by its exact byte length; existing LF/CRLF accounting stays covered; two or more files exercise decrement of an injected small aggregate budget.
    - Every accepted external document is decoded and fully depth-validated during loading. A document whose first sorted branch is a shallow scalar but whose later branch exceeds depth 64 is rejected by direct loading and enabled discovery before any Build, Encode, or Query benchmark constructs an index.
    - A real built index from `{"a.b":"first","user-name":"second","z_safe":true}` proves `idx.findPath(phase20JSONPathChild("$", "a.b"))` and the corresponding `user-name` lookup both return invalid ID/nil entry; derivation skips both and selects the compatible `$.z_safe` scalar. Null-only/no-scalar, invalid-syntax, and unresolved-only inputs return distinguishable contextual errors.
    - Both smoke and external Query paths fail preflight when `resolvePredicatePath` returns an invalid ID or nil entry. The positive built-index test must assert a valid resolved ID/entry before calling `Evaluate`; a valid all-row-groups `IsNotNull` result remains allowed, while a zero warm-up result still fails.
  </behavior>
  <action>
    In `benchmark_test.go`, remove the duplicated SCREAMING_SNAKE environment-name constants and retain only `phase20ExternalEnableEnvVar` and `phase20ExternalDirectoryEnvVar` with the existing string values. Parse the enable value before reading the directory variable: empty means disabled, exact `1` means enabled, and every other nonempty byte sequence is an actionable configuration error. Do not trim the enable value. Keep directory trimming after successful enablement. Change the empty-discovery error to state that supported regular top-level files were not found and symlinks are not followed.

    Raise only `phase20MaxExternalDocumentBytes` to `2 * 1024 * 1024`; retain 64 files, 8 MiB aggregate, and depth 64. Extract the existing sorted-path loading loop into a small helper that accepts an explicit byte budget, decrements it by each file's returned `bytesRead`, and is used by production discovery and multi-file tests. Add the why-comment above `phase20ScanLineWithDelimiter`: retaining LF/CRLF delimiters makes aggregate accounting include newline bytes. Keep the final unterminated token behavior.

    Add private `phase20ValidateExternalDocumentDepth` and recursive full-tree validation helpers. After `json.Valid` succeeds, decode with `UseNumber` and traverse every map value and array element (sorted keys and array order for deterministic context), returning a corpus-validation error as soon as any branch exceeds `phase20MaxExternalJSONDepth`. Call this validator inside `phase20LoadExternalDocuments` for a single `.json` document and for every nonblank `.jsonl`/`.ndjson` record before appending it to `docs`; wrap failures with file and line context. This loading boundary is shared by external Build, Encode, and Query, so all three actions reject over-depth corpora before benchmark construction. Depth overflow is not a scalar-candidate rejection and must not be handled or skipped by predicate derivation.

    Let `phase20ExternalBenchmarkFixture` return documents with an empty predicate. In `phase20BenchmarkQuery`, build the index first, derive the external predicate only when the fixture predicate is empty, then run a shared testable preflight helper. Change `phase20DeriveScalarPathPredicate` and its recursive helper to accept the finished index. Use a `(path, ok, firstRejection error)`-equivalent result so traversal remains deterministic but can continue after null, invalid-syntax, or unresolved scalar candidates; full-document depth enforcement belongs exclusively to loading. For each scalar candidate, canonicalize supported syntax, call `idx.findPath`, and return `IsNotNull(entry.PathName)` only for a valid path ID/entry; retain the first rejection for a final contextual error. Treat the generated bracket paths for both `user-name` and `a.b` as unresolved when the built index says so, continue to the later `z_safe` positive control, and never retry raw `$.user-name` or guessed `$.a.b` spellings. Before warm-up and `b.ResetTimer`, preflight every fixed or derived predicate with `idx.resolvePredicatePath` and reject invalid IDs/nil entries. Do not require the warm-up count to be smaller than all row groups, and do not change builder traversal, `NormalizePath`, `findPath`, `resolvePredicatePath`, or public APIs.

    Extend the existing Phase 20 tests with all cases in `<behavior>`. Build a real index for the special-key regression, assert negative `findPath` ID/entry pairs for the generated `user-name` and `a.b` candidates, assert derivation selects `$.z_safe`, then assert `resolvePredicatePath` returns a valid ID/entry before `Evaluate` produces a nonzero result. Add a depth regression file with an early shallow scalar and a later branch deeper than 64; exercise both direct `phase20LoadExternalDocuments` and enabled `phase20DiscoverExternalDocuments` and require contextual corpus-validation failures. Assert stable contextual wrapping, not the standard library scanner's exact text. Skip the symlink-only case only when the platform cannot create the symlink. Exercise the missing-predicate failure through the extracted preflight helper rather than trying to construct a `testing.B`. The large accepted test must synthesize bytes locally and must not copy or download upstream data.

    Update the root Phase 20 benchmark section and `testdata/phase20/README.md` in the same atomic change: say the enable flag must be exactly `1`; describe regular top-level files, no symlink following, and the 64-file/8-MiB-total/2-MiB-document/depth-64 limits. Use present tense for the optional local run. Remove the fixture README's `Current bytes` column while retaining document counts and byte caps. In `20-SECURITY.md`, change only T20-D01's per-document value from 1 MiB to 2 MiB and keep the other limits, status, counts, and threat dispositions unchanged.
  </action>
  <verify>
    <automated>go test ./... -run '^TestPhase20' -count=1 &amp;&amp; env -u GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL -u GIN_PHASE20_SIMDJSON_DIR go test ./... -run '^$' -bench '^BenchmarkPhase20RealisticJSON/tier=smoke' -benchtime=1x -count=1 -benchmem &amp;&amp; git diff --check &amp;&amp; git diff --quiet --cached -- .planning/config.json</automated>
  </verify>
  <done>Every external-loading, full-tree depth, accounting, gate, and path-resolution branch above has focused coverage; user-name and a.b are proven unresolved and skipped for a compatible scalar whose predicate is resolved before Evaluate; Build, Encode, and Query all reject over-depth external documents; the smoke benchmark times only resolved paths; all three Phase 20 documentation/security locations state the same bounded contract; no builder/query public behavior changed; Task 1 is committed alone with the prescribed message while `.planning/config.json` remains unstaged.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Make fixture generation deterministic, drift-tested, and normally discovered</name>
  <files>testdata/phase20/generate.go, testdata/phase20/generate_test.go, Makefile</files>
  <behavior>
    - One ordered `generatedFixtures()` result supplies `nested_high_cardinality.jsonl`, `mixed_type_arrays.jsonl`, `number_heavy.jsonl`, then `combined.jsonl` to both the command and tests.
    - Rendering uses the same record order and exactly one final newline; each rendered byte slice equals its committed JSONL file, with failure guidance naming the fixture and documented generator command.
    - The existing symlink-destination test remains green; ordinary command failures produce one contextual stderr error and a nonzero exit instead of a panic.
    - `make test` explicitly runs both `./...` and `./testdata/phase20`, so Go's intentional exclusion of `testdata` directories cannot hide the drift or symlink tests.
  </behavior>
  <action>
    Refactor `generate.go` only enough to introduce an ordered fixture descriptor and `generatedFixtures()` shared by `run() error` and tests. Replace the nondeterministic map iteration with the fixed order in `<behavior>`, and centralize rendering so the byte comparison and writer both use `strings.Join(records, "\n") + "\n"`. Keep the four generated fixture contents unchanged.

    Add `github.com/pkg/errors`. Retain `fmt` for record formatting and stderr output, but replace every new formatted `fmt.Errorf` with `errors.Errorf` and every OS-error propagation with `errors.Wrapf` as appropriate. Add a small `run() error` boundary; `main` prints one contextual error to stderr and calls `os.Exit(1)` instead of panicking. Preserve temporary-file creation in the target directory, permission setting, symlink/non-regular rejection, atomic rename, and cleanup.

    In `generate_test.go`, preserve `TestWriteFixtureToDirRejectsSymlink` and add `TestGeneratedFixturesMatchCommitted`. Iterate the ordered descriptors, render through the shared helper, read the corresponding committed file from the package test working directory, compare exact bytes, and on mismatch name the fixture plus `go run ./testdata/phase20/generate.go`. Do not spawn subprocesses or maintain hashes/current-byte prose.

    Change only the gotestsum package selection in `Makefile:test` to the verified space-separated `--packages="./... ./testdata/phase20"` form. Do not add a second test target or alter the required Makefile targets.
  </action>
  <verify>
    <automated>go test ./testdata/phase20 -count=1 &amp;&amp; make -n test | rg --fixed-strings -- '--packages="./... ./testdata/phase20"' &amp;&amp; go run ./testdata/phase20/generate.go &amp;&amp; git diff --exit-code -- testdata/phase20/*.jsonl &amp;&amp; ! rg -n 'fmt\.Errorf|panic\(' testdata/phase20/generate.go &amp;&amp; git diff --check &amp;&amp; git diff --quiet --cached -- .planning/config.json</automated>
  </verify>
  <done>The generator package independently passes, exact generated bytes match all committed fixtures, the standard target advertises explicit package discovery, deprecated/wrapping and panic findings are gone, and Task 2 is committed alone with the prescribed message while `.planning/config.json` remains unstaged.</done>
</task>

<task type="auto" tdd="false">
  <name>Task 3: Reconcile project position and run the complete quick-task gate</name>
  <files>.planning/STATE.md</files>
  <action>
    Update only the four inconsistent current-position values in `.planning/STATE.md`: set `stopped_at` to `Phase 20 complete (2/2) — ready to plan Phase 21`, set `**Current focus:**` to `Phase 21 — SIMD Parser Adapter`, set the Current Position `Phase:` value to `21`, and set `progress.completed_plans` to `3` (Phase 19 has one plan and Phase 20 has two). Keep status `ready_to_plan`, completed-phase counts, progress percentage, last activity, pending Phase 21 todo, Session Continuity, all quick-task history, and other state text unchanged. Do not edit `.planning/ROADMAP.md`; Phase 999.7 must remain its unpromoted backlog entry.

    Run the complete verification block below after Tasks 1 and 2 are committed. Confirm `gsd-sdk` can parse STATE.md, the Makefile's normal target includes the generator package, generated fixtures have no drift, focused Phase 20 tests and the explicit generator package pass, full `make test` passes, and the documented offline smoke benchmark completes with both external variables unset. Before committing STATE.md, stage only that file and prove the user's `.planning/config.json` change is still present only in the unstaged worktree.
  </action>
  <verify>
    <automated>gsd-sdk query state.load &gt;/dev/null &amp;&amp; rg -q '^stopped_at: Phase 20 complete \(2/2\) — ready to plan Phase 21$' .planning/STATE.md &amp;&amp; rg -q '^\*\*Current focus:\*\* Phase 21 — SIMD Parser Adapter$' .planning/STATE.md &amp;&amp; rg -q '^Phase: 21$' .planning/STATE.md &amp;&amp; rg -q '^  completed_plans: 3$' .planning/STATE.md &amp;&amp; rg -q '^### Phase 999\.7: SIMD Parser Runtime Fallback DX \(BACKLOG\)$' .planning/ROADMAP.md &amp;&amp; git diff --exit-code -- .planning/ROADMAP.md &amp;&amp; make -n test | rg --fixed-strings -- '--packages="./... ./testdata/phase20"' &amp;&amp; go test ./... -run '^TestPhase20' -count=1 &amp;&amp; go test ./testdata/phase20 -count=1 &amp;&amp; make test &amp;&amp; go run ./testdata/phase20/generate.go &amp;&amp; git diff --exit-code -- testdata/phase20/*.jsonl &amp;&amp; env -u GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL -u GIN_PHASE20_SIMDJSON_DIR go test ./... -run '^$' -bench '^BenchmarkPhase20RealisticJSON/tier=smoke' -benchtime=1x -count=1 -benchmem &amp;&amp; git diff --check &amp;&amp; git diff --quiet --cached -- .planning/config.json &amp;&amp; test "$(git status --short -- .planning/config.json)" = " M .planning/config.json"</automated>
  </verify>
  <done>STATE.md and ROADMAP.md agree that Phase 21 is next and Phase 999.7 remains backlog-only; all required focused, discovery, drift, full-suite, and smoke-benchmark gates pass; Task 3 is committed alone with the prescribed message; the pre-existing `.planning/config.json` change remains intact and uncommitted.</done>
</task>

</tasks>

<source_audit>

| Research finding | Covered by | Status |
| --- | --- | --- |
| 1 MiB rejects the current 1,727,204-byte example | Task 1 cap, synthetic acceptance case, README/security sync | COVERED |
| Unresolved Query paths can time the `AllRGs` fallback | Task 1 mandatory `resolvePredicatePath` preflight | COVERED |
| Builder/query path construction differs for special keys | Task 1 real built-index tests prove generated user-name and a.b candidates are unresolved, skip both for `$.z_safe`, and prohibit raw-path guessing; public model stays unchanged | COVERED |
| Scalar traversal collapses null/syntax/unresolved causes | Task 1 first-rejection propagation and branch-specific tests | COVERED |
| Query-time scalar selection cannot protect Build/Encode from depth overflow and can short-circuit before a later deep branch | Task 1 validates every branch of every document during loading and rejects an early-scalar/later-over-depth regression before all benchmark actions | COVERED |
| Non-`1` gate values silently disable and env constants are duplicated | Task 1 exact three-state gate and camelCase-only constants | COVERED |
| Unterminated tokens, delimiters, scanner overflow, and cross-file budgets lack direct coverage | Task 1 loader comment, injected-budget helper, and focused tests | COVERED |
| Generator tests are excluded from `./...`, drift is unchecked, and command errors use deprecated wrapping/panic | Task 2 ordered generator, exact-byte test, pkg/errors/run boundary, and Makefile package list | COVERED |
| Contributor/security documentation omits the hardened exact limits | Task 1 atomic documentation and T20-D01 update | COVERED |
| STATE.md points at backlog Phase 999.7 and reports 34 instead of 3 completed v1.3 plans | Task 3 four-field repair with ROADMAP consistency guard | COVERED |

No research item is intentionally excluded.
</source_audit>

<threat_model>
## Trust Boundaries

| Boundary | Mitigation retained or strengthened |
| --- | --- |
| Explicit environment opt-in to local external files | T20-S01 exact-`1` gate rejects ambiguous nonempty values before directory access. |
| Untrusted local JSON/JSONL entering benchmark setup | T20-D01 keeps 64 files, 8 MiB total, full-tree depth 64, regular top-level files only, and raises only the per-document cap to a documented 2 MiB. |
| Fixture refresh writing tracked corpus files | T20-T02 retains symlink/non-regular rejection and atomic rename; exact-byte drift tests make generator changes visible in `make test`. |

No new accepted risk is introduced. The unchanged 8 MiB aggregate cap bounds the extra per-document headroom.
</threat_model>

<verification>
Run in this order after implementation:

1. `make -n test | rg --fixed-strings -- '--packages="./... ./testdata/phase20"'` — standard target discovers the generator package.
2. `go test ./... -run '^TestPhase20' -count=1` — focused root Phase 20 coverage.
3. `go test ./testdata/phase20 -count=1` — generator drift and symlink coverage.
4. `make test` — full repository test target.
5. `go run ./testdata/phase20/generate.go &amp;&amp; git diff --exit-code -- testdata/phase20/*.jsonl` — deterministic committed output.
6. `env -u GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL -u GIN_PHASE20_SIMDJSON_DIR go test ./... -run '^$' -bench '^BenchmarkPhase20RealisticJSON/tier=smoke' -benchtime=1x -count=1 -benchmem` — offline smoke benchmark.
7. `gsd-sdk query state.load` plus the exact STATE/ROADMAP assertions from Task 3 — planning-state consistency.
8. `git diff --check`, `git diff --quiet --cached -- .planning/config.json`, and the expected unstaged config status — clean patch and preservation of the user's change.
</verification>

<success_criteria>
- All findings in `260722-eje-RESEARCH.md` are covered; no public builder/path-model redesign, downloader, vendored upstream row, recursive discovery, or widened aggregate limit is introduced.
- An unresolved predicate can no longer be benchmarked through the `AllRGs` fallback, and derivation reports why no safe indexed scalar exists.
- Real built-index coverage proves `user-name` and `a.b` remain unresolved negative candidates under the unchanged path model, skips them for a compatible scalar, and resolves that predicate before evaluation.
- Full-document loading traverses every branch and rejects depth overflow before external Build, Encode, or Query setup, even when a shallow scalar appears first.
- External input accepts the current documented simdjson example size while remaining explicitly enabled, regular-file-only, deterministic, and bounded.
- Generator drift and symlink safety run under `make test`, and all generator errors follow repository conventions.
- Documentation, security evidence, and planning state match executable constants and ROADMAP position.
- Three scoped commits exist with the prescribed safe messages; `.planning/config.json` remains an unstaged user change; any later integration uses squash merge.
</success_criteria>

<output>
After completion, create `.planning/quick/260722-eje-address-phase-20-benchmark-review-findin/260722-eje-SUMMARY.md` with the three commit hashes and verification results. Do not include `.planning/config.json` in any commit.
</output>
