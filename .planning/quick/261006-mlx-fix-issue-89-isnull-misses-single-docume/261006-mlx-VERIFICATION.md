---
quick: 261006-mlx
verified: 2026-10-06
status: passed
score: 18/19 expectations PASS (E1-E16, E18, E19); E17 PASS on the rule, FAIL on wording accuracy (docs only, no code gap)
branch: fix/issue-89-isnull-absent (base df42273, commits a5c1534..b43bcf8)
gaps:
  - truth: "E17/E18 docs state a correct rule"
    status: partial
    reason: "godoc and CHANGELOG say v10 indexes get the new answer / are fixed on read. Decode rejects any version other than 11 with ErrVersionMismatch, so a v10 file cannot be read at all."
    artifacts:
      - path: "query.go"
        issue: "IsNull godoc (line ~933): 'Indexes written by v10 and v11 builders give the same answer' is ambiguous (same as what?) and wrong for v10 (not decodable)."
      - path: "CHANGELOG.md"
        issue: "Unreleased/Fixed: 'existing v10 and v11 indexes are fixed on read with no rebuild' is wrong for v10."
    missing:
      - "Reword godoc: 'An index decoded from a v11 file gives the same answer as a freshly built one, because one-document row groups are derived at query time from root presence.'"
      - "Reword CHANGELOG Fixed: 'existing v11 indexes are fixed on read with no rebuild' (v10 files are already rejected by Decode and must be rebuilt, as before)."
---

# Quick 261006-mlx Verification

**Goal:** Fix #89. `IsNull` selects a row group whose only document lacks the path, via a uniform rule derived at query time from root `$` presence, no wire format change.

**Result:** The code goal is achieved. Every behavioral expectation passes in my own runs. One documentation accuracy defect (v10 wording) remains. It is a two-line fix.

Method: no executor tests were trusted. I wrote (1) a throwaway in-package test `zz_verify_tmp_test.go` (deleted), (2) a throwaway external module in the scratchpad (`diffprog`, public API only) that built 400 seeded random indexes and ran 11 operators over 8 paths, against both the new tree and a `git worktree` of df42273 (removed). Repo status after my work: clean (`git status --short` empty, `git worktree prune` done).

## Expectations E1-E19

| # | Result | Evidence |
|---|--------|----------|
| E1 | PASS | Own test, repro index: `IsNull($.env) => [2 3]`, `NE($.env,prod) => []`, `NE ∪ IsNull => [2 3]` |
| E2 | PASS | Own test, 1:1 `{"env":"prod"}`, `{"env":null}`, `{"app":"x"}`: `IsNull($.env) => [1 2]` |
| E3 | PASS | Own test, RG0 two docs without env, RG1 one doc without env, RG2 with env: `=> [0 1]` |
| E4 | PASS | Own test, RG0 `{"env"}`+`{"app"}`, RG1 env twice: `=> [0]` (mixed RG selected, all-present RG not) |
| E5 | PASS | Repro: RG0 and RG1 absent from `[2 3]`. E4: all-present RG1 not selected |
| E6 | PASS | numRGs=6, docs only in RGs 0..3, one-doc RG lacking env: `=> [1 2]`, no 4 or 5 |
| E7 | PASS | numRGs=8, DocID5 x2 with env, DocID2 x1 without: `=> [1]`, which is the position of DocID 2 (first-seen order: DocID5=pos0, DocID2=pos1); exactly one position |
| E8 | PASS | `{"a":null}`, `{"tags":[]}`, `{"a":{"b":1},"tags":["x"]}`: `IsNull($.a.b) => [0 1]`, `IsNull($.tags[*]) => [0 1]` (RG2 not selected) |
| E9 | PASS | Differential vs df42273 on 400 random indexes x 8 paths: 813 IsNull answers differ, all 813 are strict supersets of the old answer, 0 violations (`python3 -I cmp.py`). Independent oracle (doc-level: absent or null) on 1485 (index,path) cases, non-codec indexes: 0 mismatches, 0 under-selects (unknown path fail-open to all numRGs taken into account). Executor's `PropertyNegation|Negation` tests also pass, with and without `-tags simdjson` |
| E10 | PASS | Same differential: EQ, EQnum, NE, NIN, IN, IsNotNull, GT, LT, Contains, Regex over all paths incl. `$`, `$.nope`, `$.tags[*]`: `diff by op {'IsNull': 813}`, no other operator differs across 35200 results (includes RowGroupCodec and sparse-DocID indexes) |
| E11 | PASS | Own test: `IsNull($.nope)` on repro `=> [0 1 2 3]` |
| E12 | PASS | Own test: `delete(NullIndexes, pathLookup["$"])` => `[0 1 2 3]`; also `delete(pathLookup,"$")` => `[0 1 2 3]`; also nil root `PresentRGBitmap` => `[0 1 2 3]` (no panic) |
| E13 | PASS | Own test, docs `null`,`5`,`[]`,`"s"`,`{}`,`{"env":"p"}`: root `PresentRGBitmap=[0 1 2 3 4 5]` and `IsNull($.env)=[0 1 2 3 4]` for stdlibParser and materializingParser. SIMD: `GOTOOLCHAIN=go1.25.5 go test -tags simdjson -count=1 -v -run IsNullUniform .` => `--- PASS: TestIsNullUniformRootPresenceSIMD` (ran, not skipped). I did not write an independent SIMD test; relied on the executor's helper plus parity matrix run with the tag (PASS) |
| E14 | PASS | `git diff --exit-code df42273 -- testdata/ builder.go serialize.go` => exit 0. `gin.go:36: Version = 11` |
| E15 | PASS | Own test decodes committed `nulls-and-missing.bin`: `Header.Version=11`, `IsNull($.b) => [2 3]`, `IsNull($.a) => [0 1 2]`. All 14 goldens decode; for each queryable path IsNull equals `Null ∪ (root present − path present) ∪ AbsentRGs`. Only `__derived:*` paths differ, because those are not queryable by raw name and hit the unknown-path fail-open (not a defect). Parity case `IsNull("$.status") => [2 3]` is in the passing parity matrix |
| E16 | PASS | Differential program Encode/Decode round trip on every (index, path, operator): 35200 comparisons, no `ROUNDTRIP MISMATCH`, exit 0 |
| E17 | PASS on the rule, FAIL on one claim | README line 344, IsNull godoc, `gin.go` AbsentRGs and AggregateIndex comments state the same uniform rule. `grep -rniE "historical"` finds no "keeps the historical reading" in README, gin.go, query.go, tests (it is quoted once in the CHANGELOG Changed entry as the replaced rule, which is fine). **Defect:** see gap below |
| E18 | PASS with the same defect | CHANGELOG has `## Unreleased` with `### Changed` and `### Fixed` (#89). No version heading cut. The Fixed text claims v10 indexes are fixed on read, which is wrong |
| E19 | PASS | `GOTOOLCHAIN=go1.25.5 make lint` => `0 issues.` `GOTOOLCHAIN=go1.25.5 make test` => `DONE 1259 tests, 1 skipped` (skip is the pre-existing missing `testdata/test.parquet`). `go test -tags simdjson -count=1 -run IsNullUniform .` => ok. Note: the local default Go 1.27.1 breaks golangci-lint export data on untouched files, so I used `GOTOOLCHAIN=go1.25.5` (go.mod version) and did not run lint on 1.27.1 |

## Gap (documentation, not behavior)

`Decode` (`serialize.go:751`) returns `ErrVersionMismatch` for any header version other than `Version` (11). The `Version` comment in `gin.go` says the only migration path is a rebuild. So a v10 file is never read.

- `query.go` IsNull godoc: "Indexes written by v10 and v11 builders give the same answer, because one-document row groups are derived at query time from root presence." Ambiguous (same as what?) and misleading for v10.
- `CHANGELOG.md` Unreleased/Fixed: "existing v10 and v11 indexes are fixed on read with no rebuild." Wrong for v10. This repeats the premise in CONTEXT D2 ("Old v10/v11 indexes give correct answers when read"), which does not hold for v10.

Suggested wording:
- godoc: "An index decoded from a v11 file gives the same answer as a freshly built one, because one-document row groups are derived at query time from root presence."
- CHANGELOG Fixed: "...the wire format stays v11, and existing v11 indexes are fixed on read with no rebuild."

## Other observations (info, no action required)

- `evaluateIsNull` fails open to `AllRGs` when the root path, root NullIndex or a PresentRGBitmap/NullRGBitmap is missing. A path with no NullIndex still returns `NoRGs` (unchanged branch, unreachable from the builder).
- `IsNull($)` on the root returns only explicit-null documents (root present minus root present is empty). Not specified by E1-E19; consistent with the rule.
- Fail-open for unknown paths returns all `numRGs` including RGs that received no document (pre-existing E11 behavior; E6 only applies to known paths).
- builder.go comment at line 1435 is still accurate. No stale text found in README line 1252.
- `git diff df42273 --stat`: CHANGELOG.md, README.md, aggregate_negation_test.go, gin.go, isnull_uniform_simd_test.go, isnull_uniform_test.go, parser_parity_test.go, query.go. builder.go, serialize.go, testdata/ untouched.

## Gap closure

E17/E18 wording gap closed by the orchestrator in `ee61686`: the IsNull godoc and the CHANGELOG Fixed entry now claim the read-time fix for v11 indexes only (Decode rejects other versions). Focused tests re-run: ok. Status changed gaps_found -> passed.
