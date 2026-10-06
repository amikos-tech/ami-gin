---
phase: quick-261006-mlx
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - query.go
  - isnull_uniform_test.go
  - isnull_uniform_simd_test.go
  - aggregate_negation_test.go
  - parser_parity_test.go
  - README.md
  - gin.go
  - CHANGELOG.md
autonomous: true
requirements: [ISSUE-89]

must_haves:
  truths:
    - "E1: On the repro index (RG0 env x2, RG1 env, RG2 app x2, RG3 app), IsNull($.env) is [2 3]; NE($.env,prod) is []; NE union IsNull is [2 3]"
    - "E2: With one document per RG (env prod, env null, app only), IsNull($.env) is [1 2]"
    - "E3: RGs where every document lacks the path (multi-doc and one-doc) are all selected by IsNull"
    - "E4/E5: A mixed RG (one doc with env, one without) is still selected; an RG where every document has a non-null env is never selected"
    - "E6/E7: IsNull never selects an RG that received no document; with sparse DocIDs the lone document's position is selected"
    - "E8: Nested ($.a.b) and array ($.tags[*]) paths follow the same rule for one-document RGs"
    - "E9: New IsNull is a superset of the v1.4.0 rule (NullRGBitmap union AbsentRGs) on every generated index, and the oracle uses the uniform rule with exactness on every RG that received documents"
    - "E10: EQ, NE, NIN, IN, IsNotNull answers on the repro index are unchanged; only evaluateIsNull changes"
    - "E11/E12: IsNull on an unknown path returns all RGs; an index whose root $ has no NullIndex returns all RGs for IsNull"
    - "E13: Documents null, 5, [], \"s\", {} each mark $ present, under stdlib, materializing and (with -tags simdjson) the SIMD parser"
    - "E14/E15/E16: Version stays 11, parity-golden bytes are untouched, a committed v11 golden decodes to the uniform IsNull answer, Encode then Decode keeps IsNull answers"
    - "E17/E18: README, IsNull godoc and the AbsentRGs comment state one rule; CHANGELOG Unreleased has Changed and Fixed (#89) entries with no version cut"
    - "E19: make test and make lint pass"
  artifacts:
    - path: "query.go"
      provides: "evaluateIsNull = NullRGBitmap union AbsentRGs union (root present minus path present); IsNull godoc"
      contains: "evaluateIsNull"
    - path: "isnull_uniform_test.go"
      provides: "Tests for E1-E8, E11-E13, E14-E16"
      contains: "TestIsNullUniform"
    - path: "isnull_uniform_simd_test.go"
      provides: "SIMD-parser root presence test (build tag simdjson)"
      contains: "//go:build simdjson"
    - path: "CHANGELOG.md"
      provides: "Unreleased Changed and Fixed entries"
      contains: "## Unreleased"
  key_links:
    - from: "query.go evaluateIsNull"
      to: "idx.NullIndexes[idx.pathLookup[\"$\"]].PresentRGBitmap"
      via: "root presence minus path presence"
      pattern: "pathLookup\\[\"\\$\"\\]"
    - from: "aggregate_negation_test.go docMatches"
      to: "uniform rule"
      via: "OpIsNull returns !present || value == nil"
      pattern: "!present \\|\\| value == nil"
---

<objective>
Fix issue #89: `IsNull(P)` must select every row group (RG) holding a document with explicit `null` for P or without P, whatever the RG's document count (D1). Derive the one-document case at query time from root `$` presence (D2). No builder, serialize.go, or `Version` change.

Purpose: `NE ∪ IsNull` stops under-selecting on aggregated indexes with a one-document RG (for example a short last row group).
Output: query.go change, new tests, updated oracle and pinned tests, docs and CHANGELOG Unreleased entries.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@./CLAUDE.md
@.planning/quick/261006-mlx-fix-issue-89-isnull-misses-single-docume/261006-mlx-CONTEXT.md
@.planning/quick/261006-mlx-fix-issue-89-isnull-misses-single-docume/261006-mlx-RESEARCH.md
@query.go
@aggregate_negation_test.go
@parser_parity_test.go

<interfaces>
<!-- Extracted from the codebase. Use directly; no exploration needed. -->

query.go (current, to be changed):
  func (idx *GINIndex) evaluateIsNull(pathID int) *RGSet   // today: NullIndexes[id].NullRGBitmap.Union(idx.aggregateAbsentRGs(pathID)); no NullIndex -> NoRGs
  func (idx *GINIndex) aggregateAbsentRGs(pathID int) *RGSet  // returns AbsentRGs or NoRGs
  func IsNull(path string) Predicate   // no godoc today
  Unknown/invalid path is handled earlier in evaluatePredicate -> AllRGs (do not touch).

gin.go: type NullIndex struct { NullRGBitmap, PresentRGBitmap *RGSet }; idx.NullIndexes map[uint16]*NullIndex; idx.pathLookup map[string]uint16 (canonical path -> id; root is "$"); const Version = 11.

bitmap.go (all return new sets, none mutate the receiver): Intersect(other), Union(other), Invert() (complement within rs.NumRGs), Clone(), ToSlice() []int, IsSet(int), AllRGs(n), NoRGs(n).

Test helpers already in package gin:
  buildAggregated(t, numRGs int, docs []aggregatedDoc) *GINIndex   // aggregatedDoc{rg int, data map[string]any}; DocID == rg
  indexDocIDs(idx, p) docIDSet ; docIDSet.sorted() []int / superset / equals / intersect
  mustNewBuilder(t, cfg, numRGs) ; NewBuilder(cfg, numRGs, WithParser(p)) ; stdlibParser{} ; materializingParser{} (parser_parity_test.go)
  loadGolden(t, name) []byte ; Decode(data) (*GINIndex, error) ; Encode(idx) ([]byte, error)
  newTestSIMDParser(tb) CloseableParser (simdjson tag only) ; CloseableParser has Close()
RG positions follow first-seen DocID order. For indexes where DocID order equals insertion order (DocIDs 0..n-1 in order) positions equal DocIDs, so ToSlice() can be asserted directly. For anything else compare DocIDs with indexDocIDs.
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Uniform IsNull in query.go, with tests for E1-E8, E11-E16 and the updated oracle</name>
  <files>query.go, isnull_uniform_test.go, aggregate_negation_test.go, parser_parity_test.go</files>
  <behavior>
    Create isnull_uniform_test.go first (package gin) and run it RED, then implement. Use github.com/pkg/errors only if an error is created; tests use t.Fatalf. Group as subtests of TestIsNullUniform* functions:
    - E1 TestIsNullUniformIssue89Repro: buildAggregated(t, 4, ...) with RG0 {"env":"prod"} x2, RG1 {"env":"prod"}, RG2 {"app":"x"} and {"app":"y"}, RG3 {"app":"z"}. Expect IsNull($.env)=[2 3], NE($.env,"prod")=[], union of NE and IsNull results=[2 3], IsNull($.app)=[0 1]. E10 spot check on the same index: EQ($.env,"prod")=[0 1], IsNotNull($.env)=[0 1], NIN($.env,"prod")=[].
    - E2: one document per RG: {"env":"prod"}, {"env":null}, {"app":"x"} over 3 RGs -> IsNull($.env)=[1 2].
    - E3: RG0 {"x":1} twice, RG1 {"x":1}, RG2 {"env":"p"} -> IsNull($.env)=[0 1].
    - E4: RG with {"env":"prod"} and {"app":"x"} -> selected (use 2 RGs; second RG {"env":"p"} twice is not selected).
    - E5: covered by E1 (RG0, RG1 absent from result); also assert in E4 the all-present RG is not selected.
    - E6: numRGs=6, documents only in RGs 0..3 (include one one-document RG lacking env); IsNull($.env) must not contain 4 or 5.
    - E7: buildAggregated(t, 8, ...) with DocID 5 twice with env, DocID 2 once without env (path env must exist via DocID 5): indexDocIDs(IsNull($.env)).sorted()==[2].
    - E8: RG0 {"a":null}, RG1 {"tags":[]}, RG2 {"a":{"b":1},"tags":["x"]}. Expect IsNull($.a.b) contains 0 and 1 and not 2; IsNull($.tags[*]) contains 0 and 1 and not 2. If an assertion fails because of staging details, inspect idx.PathDirectory and adjust the corpus, keeping the intent (one-document RGs that are null or empty are selected, the RG with real values is not).
    - E11: IsNull($.nope) on the repro index = [0 1 2 3].
    - E12: build the repro index, then delete(idx.NullIndexes, idx.pathLookup["$"]); IsNull($.env)=[0 1 2 3].
    - E13: helper assertRootPresenceAllDocShapes(t, parser Parser): NewBuilder(DefaultConfig(), 6, WithParser(parser)); AddDocument for RG0 `null`, RG1 `5`, RG2 `[]`, RG3 `"s"`, RG4 `{}`, RG5 `{"env":"p"}`. Assert root NullIndexes[pathLookup["$"]].PresentRGBitmap.ToSlice()==[0 1 2 3 4 5] and IsNull($.env)=[0 1 2 3 4]. Call it for stdlibParser{} and materializingParser{} as subtests.
    - E14: assert Version == 11 (the constant) in a test named TestIsNullUniformFormatUnchanged; the byte-identity of goldens is checked by the existing TestStdlibParserGolden_AuthoredFixtures plus the git diff gate in verify.
    - E15 TestIsNullUniformOldGoldenFixedOnRead: Decode(loadGolden(t, "nulls-and-missing")); assert Header.Version == 11; IsNull($.b)=[2 3] (doc 2 lacks b, doc 3 has null; v1.4.0 gave [3]); IsNull($.a)=[0 1 2] (v1.4.0 gave [0 2]); IsNotNull($.a)=[0 2 3] unchanged.
    - E16: Encode then Decode the repro index; IsNull results for $.env, $.app and $.nope equal the in-memory index.
    Also edit existing tests so the suite stays green after the implementation:
    - aggregate_negation_test.go: docMatches drops its `aggregated bool` parameter; the OpIsNull case becomes `!present || value == nil`; rewrite the doc comment to state the uniform rule (absent counts as null); oracleDocIDs drops the perRG counting and the aggregated argument. TestNegationUnderAggregationOracle already checks IsNull exactness on all RGs; keep it.
    - aggregate_negation_test.go TestNegationSingleDocumentSemanticsUnchanged: rename to TestNegationSingleDocumentSemantics, update the doc comment (a document without the key IS null for IsNull), and change the IsNull($.tags[*]) expectation to [2] (the third RG holds {"other":"x"}). Keep the `len(idx.AggregateIndexes) != 0` assertion (the builder is unchanged) and the NE assertion.
    - parser_parity_test.go line 375: `{"IsNull-match", IsNull("$.status"), []int{2, 3}}` (doc 3 lacks status; the SIMD matrix test shares these cases).
  </behavior>
  <action>
    Per D1 and D2. Step 1: write isnull_uniform_test.go and the three edits listed in behavior; run the new tests and confirm the IsNull cases fail (RED) with the old rule. Step 2: change evaluateIsNull in query.go (and nothing else in evaluation code):
    - Keep the early return NoRGs(numRGs) when the path has no NullIndex.
    - Look up the root: `rootID, ok := idx.pathLookup["$"]`, then `idx.NullIndexes[rootID]`. If the root path or its NullIndex is missing, or the root's or the path's PresentRGBitmap is nil, return AllRGs(numRGs) (fail open, never under-select; per "Settled without a question").
    - Compute lacking := root.PresentRGBitmap.Intersect(ni.PresentRGBitmap.Invert()) (RGs that received a document where this path is not present).
    - Return ni.NullRGBitmap.Union(idx.aggregateAbsentRGs(pathID)).Union(lacking). AbsentRGs stays necessary for mixed RGs (an RG with {"env":"prod"} and {"app":"x"} is present for env, so only the builder's per-document count sees the absent document).
    - Do not modify builder.go, serialize.go or the Version constant (D2). Do not touch any operator other than IsNull (E10).
    Step 3: add a godoc to IsNull in query.go stating the rule once: "IsNull selects every row group holding at least one document whose value for path is an explicit JSON null, or that does not carry the path. It does so whatever the row group's document count. Combine it with NE or NIN to keep a row group whose document lacks the path." Mention that old v10/v11 indexes get the same answer because one-document row groups are derived at query time from root presence. Add a short comment above evaluateIsNull explaining the three-term union and the root-presence invariant (every committed document marks "$" present, F6/F13) and the fail-open fallback.
    Use gci import order (standard, third-party, then github.com/amikos-tech/ami-gin). Errors, if any, via github.com/pkg/errors. Do not edit CLAUDE.md.
  </action>
  <verify>
    <automated>cd /Users/tazarov/experiments/amikos/ami-gin && go build ./... && go test -count=1 -run 'IsNullUniform|Negation|QueryNull|ParserParity_EvaluateMatrix|StdlibParserGolden' .</automated>
  </verify>
  <done>New tests exist, failed before the query.go change, and pass after it. The oracle uses the uniform rule. The two pinned tests expect the uniform answer. Repro index gives IsNull($.env)=[2 3]. builder.go, serialize.go and Version are unchanged (`git diff --stat -- builder.go serialize.go` is empty).</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Property test against the v1.4.0 rule (E9) and SIMD root-presence test</name>
  <files>aggregate_negation_test.go, isnull_uniform_simd_test.go</files>
  <behavior>
    In TestPropertyNegationUnderAggregation (aggregate_negation_test.go):
    - For each of "$.env", "$.n", "$.id": if id, ok := idx.pathLookup[path] exists, compute legacy := idx.NullIndexes[id].NullRGBitmap.Union(idx.aggregateAbsentRGs(int(id))) (the v1.4.0 rule) and fail with the docs in the message when any set position of legacy is not set in idx.Evaluate([]Predicate{IsNull(path)}) (E9 superset).
    - Exactness: for OpIsNull compare got and want on ALL documents' DocIDs (no `.intersect(multiDoc)`): every RG that received documents must be exact. NE and NIN keep the multiDoc-restricted exactness unchanged.
    - Update the test's doc comment: IsNull is exact on every row group that received documents; NE and NIN are exact on row groups holding at least two documents.
    isnull_uniform_simd_test.go (first line exactly `//go:build simdjson`, package gin): TestIsNullUniformRootPresenceSIMD gets parser := newTestSIMDParser(t) (it skips on unsupported platforms; do NOT close it, the helper registers tb.Cleanup(parser.Close) — match parser_parity_simd_test.go), and calls assertRootPresenceAllDocShapes(t, parser) from isnull_uniform_test.go (E13 with the SIMD parser). Do not import pure-simdjson in this file.
  </behavior>
  <action>
    Per E9 and E13. The legacy rule is reconstructed from fields that still exist (NullRGBitmap and AbsentRGs), so the superset check pins "never fewer RGs than v1.4.0" without keeping old code. Keep the gopter budgets and generators as they are. Do NOT regenerate or edit anything under testdata/parity-golden (E14). Compile-check the tagged file with `go vet -tags simdjson .` AND run it with `go test -tags simdjson -count=1 -run IsNullUniformRootPresenceSIMD .`; if the simdjson library is unavailable, report explicitly that E13-SIMD was not run, do not drop the file.
  </action>
  <verify>
    <automated>cd /Users/tazarov/experiments/amikos/ami-gin && go test -count=1 -run 'PropertyNegation|IsNullUniform|NegationUnderAggregationOracle' . && go vet -tags simdjson . && go test -tags simdjson -count=1 -run IsNullUniformRootPresenceSIMD . && git diff --exit-code df42273 -- testdata/ builder.go serialize.go</automated>
  </verify>
  <done>The property test checks superset against the v1.4.0 rule and exactness on every RG that received documents, and passes. The tagged SIMD test compiles. testdata/, builder.go and serialize.go show no diff.</done>
</task>

<task type="auto">
  <name>Task 3: Docs and CHANGELOG, then full gates</name>
  <files>README.md, gin.go, CHANGELOG.md</files>
  <action>
    Per D3. State one rule everywhere (E17): IsNull selects every row group holding a document with explicit null or without the path, at any document count.
    - README.md around line 344: replace the paragraph starting "`IsNull` selects row groups holding an explicit JSON `null`..." with the uniform rule. Say that an absent key counts as null, in a one-document row group and in a 1:1 index too, and that `NE` plus `IsNull` therefore covers a document that lacks the path. Delete the sentence "A row group holding a single document keeps the historical reading: an absent key is not null."
    - gin.go: update the AggregateIndex type comment (lines ~235-241, which says a path without an entry keeps the one-document semantics) so it stays true for IsNull, and update the AbsentRGs field comment (lines ~246-249): AbsentRGs marks row groups holding at least two documents where at least one lacks the path; one-document row groups are not recorded here, IsNull derives them at query time from root presence (the root path's PresentRGBitmap minus this path's PresentRGBitmap), so no extra data is stored. Leave the historical `v10:` version-history comment and the Version constant alone.
    - CHANGELOG.md: insert above "## v1.4.0 (2026-10-06)" a "## Unreleased" section with "### Changed" and "### Fixed" subsections. Changed: IsNull now selects a row group whose single document lacks the path, in aggregated and plain 1:1 indexes; this replaces the README rule that a one-document row group keeps the historical reading; callers who read IsNull as "explicit null only" now get more row groups (safe for pruning, over-selection only). Fixed: IsNull no longer under-selects a one-document row group without the path, so `NE` union `IsNull` is complete (#89); the answer is derived from root presence at query time, the wire format stays v11, and existing v10/v11 indexes are fixed on read with no rebuild. Do not cut a version (release is a separate chore(release) commit; deferred). Use the repo's plain ASCII hyphen style, no emojis.
    Then run the gates. Do not edit CLAUDE.md.
  </action>
  <verify>
    <automated>cd /Users/tazarov/experiments/amikos/ami-gin && ! grep -n "historical reading" README.md gin.go query.go aggregate_negation_test.go && grep -v '^#' CHANGELOG.md | head -40 | grep -c "#89" && grep -n "^## Unreleased" CHANGELOG.md && make lint && make test</automated>
  </verify>
  <done>README, IsNull godoc and AbsentRGs comment state the same rule and no text keeps the "historical reading" wording. CHANGELOG has Unreleased with Changed and Fixed (#89) entries and no new version heading. make lint and make test pass. git status shows only the files in files_modified (plus the plan directory).</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| decoded index bytes to evaluateIsNull | A crafted or hand-built index may lack the root NullIndex or hold nil bitmaps |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-261006-01 | Denial of Service | evaluateIsNull | mitigate | Nil-check root path, root NullIndex and both PresentRGBitmaps; return AllRGs instead of dereferencing nil (tested by E12) |
| T-261006-02 | Tampering | IsNull result correctness | mitigate | Fail open: any missing evidence yields AllRGs, so a malformed index can only over-select, never skip a matching RG |
| T-261006-03 | Information Disclosure | none | accept | No new input surface, no format change, no new serialized field |
</threat_model>

<verification>
- `go test -count=1 -run 'IsNullUniform|Negation|QueryNull|ParserParity|StdlibParserGolden|PropertyNegation' .`
- `git diff --exit-code df42273 -- testdata/ builder.go serialize.go` (E14; df42273 = origin/main base, so committed changes are caught too)
- `grep -n "Version *= *11" gin.go` still matches
- `make lint && make test` (E19)
</verification>

<success_criteria>
All E1-E19 hold. Repro index returns IsNull($.env)=[2 3]. Wire format untouched (Version 11, goldens byte-identical). A v11 golden decoded by the new code returns the uniform IsNull answer. Property test shows the new rule is a superset of v1.4.0 and exact on every RG that received documents. Docs and CHANGELOG state one rule.
</success_criteria>

<output>
Create `.planning/quick/261006-mlx-fix-issue-89-isnull-misses-single-docume/261006-mlx-SUMMARY.md` when done
</output>
