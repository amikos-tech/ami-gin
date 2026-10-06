# Quick 261006-mlx: Issue #89 — IsNull misses a single-document row group lacking the path — Research

**Researched:** 2026-10-06
**Domain:** query semantics of `IsNull` under DocID aggregation; wire format v11
**Confidence:** HIGH (all facts below verified by code reading, git history, or a run against HEAD `df42273`)

## Summary

The bug reproduces exactly as reported on HEAD (v1.4.0). It is not an oversight in code: the
"one-document row group keeps the historical reading — an absent key is not null" rule was a
deliberate choice in #61 (v1.1.0, format v10) and it is documented in README, CHANGELOG and pinned
by tests. The issue is that this rule is decided per row group by document count, so an aggregated
index (TCLR, `RowGroupCodec`) gets two different meanings of `IsNull` in one index, and the short
last row group falls into the "absent is not null" side.

The finalized index already holds the information to fix it without a format change: the root path
`$` is staged for every committed document, so `NullIndexes[$].PresentRGBitmap` is exactly "positions
that received at least one document". `rootPresent ∖ present(P)` is "a document lacks P" for every
one-document row group, and old v10/v11 indexes would be fixed retroactively. The open decision is
semantic: whether one-document row groups in a pure 1:1 index also change (absent → selected by
`IsNull`). That change only over-selects relative to the old reading, never under-selects.

## Facts

### Reproduction (scratch module with `replace` to the repo; repo untouched)

**F1. Issue repro on HEAD, verbatim output:**
```
== issue repro (numRGs=4, NumDocs=6)
  paths: 0:$ 1:$.app 2:$.env
  null[0] null=[] present=[0 1 2 3]          <- root "$"
  null[1] null=[] present=[2 3]              <- $.app
  null[2] null=[] present=[0 1]              <- $.env
  agg[1] multi=[2] absent=[0] counts=[2]
  agg[2] multi=[] absent=[2] counts=[]
  IsNull($.env)              -> [2]          (want [2 3])
  IsNotNull($.env)           -> [0 1]
  NE($.env,"prod")           -> []
  NIN($.env,["prod"])        -> []
  EQ($.env,"prod")           -> [0 1]
  IsNull($.app)              -> [0]          (RG1 has one doc without app: also missed)
  IsNull($.nope)             -> [0 1 2 3]    (unknown path fails open)
  decoded IsNull($.env)      -> [2]          (same after Encode/Decode)
```
**F2. (a) Every RG has one document** (`{env:prod}`,`{env:dev}`,`{app:x}`,`{env:null}`): no
`AggregateIndexes` at all; `IsNull($.env) -> [3]` (explicit null only; RG2 absent not selected),
`NE($.env,"prod") -> [1 3]`, `IsNull($.app) -> []`. Variant with env absent in RG1, RG2:
`IsNull($.env) -> []`.
**F3. (b) Multi-doc RG where the path is absent from every document** (RG1 = two docs without env):
`IsNull($.env) -> [1]` — correct, because `presentDocs[rg]=0 < docCounts[rg]=2` sets AbsentRGs.
**F4. (c) Empty RGs** (numRGs=6, docs only in 0..3): `IsNull($.env) -> [2]`; RGs 4,5 never selected
by IsNull; root present = `[0 1 2 3]`, so empty positions are distinguishable. Unknown path returns
`[0..5]`.
**F5. (d) Sparse DocIDs** (DocID 5 ×2, DocID 2 ×1, numRGs=8): positions are assigned in insertion order
(`DocIDMapping=[5 2]`); root present `[0 1]`; `IsNull($.env) -> []` although DocID 2 lacks env.
Bitmaps are in position space, not DocID space (`query.go:965` `MatchingDocIDs`).
**F6. Root presence for non-object docs:** docs `null`,`5`,`[]`,`"s"`,`{}` → `$ null [0] present [0 1 2 3 4]`.
`$` is marked present for every committed document, including a JSON `null` root.

### Root cause

**F7.** `IsNull` = `NullRGBitmap ∪ aggregateAbsentRGs` (`query.go:704-710`). Path with no NullIndex → `NoRGs`.
**F8.** Builder counts docs per position in `docCounts` (`builder.go:103-105`, incremented
`builder.go:945` in `mergeDocumentState`) and docs carrying each path in `pathBuildData.presentDocs`
(`builder.go:145-146`, incremented `builder.go:1095-1096`).
**F9.** `aggregatedRowGroups()` keeps only positions with `docCounts >= 2` (`builder.go:1436-1445`);
`buildAggregateIndex` returns nil when that list is empty and sets `absent` only for those positions
when `presentDocs[rg] < docCounts[rg]` (`builder.go:1447-1471`). So a one-document position is never
in AbsentRGs — this is the exact gate.
**F10. Deliberate, not an oversight.** Introduced in `954d689` (PR #61, closes #60, 2026-09-14, format
9→10). PR body: "Both bitmaps are only ever set on row groups holding more than one document, so an
index with one document per DocID behaves exactly as before and carries no AggregateIndex at all."
Documented in `gin.go:235-249` (AggregateIndex/AbsentRGs comments), `README.md:344` ("A row group
holding a single document keeps the historical reading: an absent key is not null."),
`CHANGELOG.md:74-80` (v1.1.0, "One-document row groups keep their previous answers."). Issue #60
itself only asked about multi-doc RGs; the mixed case (aggregated index with a one-doc RG) was not
considered. No `.planning/phases` or `.planning/quick` dir exists for #60/#61.
**F11.** The issue text's claim that the AbsentRGs comment and IsNull godoc "describe the same rule"
as the expected behaviour is not accurate: `gin.go:246-249` says "at least two documents", and
`query.go:913` `IsNull` has no godoc. README:344 explicitly states the current behaviour.

### Information available

**F12.** Finalized index already carries "position received ≥1 document": `NullIndexes[pathLookup["$"]].PresentRGBitmap`
(F1, F4, F6). For a one-document position, `rootPresent ∖ present(P)` is exactly "the document lacks P".
For multi-doc positions it is a subset of AbsentRGs (all docs lack P ⇒ `presentDocs=0 < docCounts`), so
`null ∪ AbsentRGs ∪ (rootPresent ∖ present(P))` is correct for every position. Per-position doc counts are
NOT persisted (`Header.NumDocs` is a total, `gin.go:104-112`).
**F13.** Root staging is guaranteed for built-in parsers: `walkJSON` stages from `"$"` (`builder.go:503-512`),
SIMD parser walks `doc.Root()` at `"$"` (`parser_simd.go:69`). `Parser.Parse` takes the unexported
`parserSink` (`parser.go:73-92`), so third-party parsers cannot exist. Soft-skipped documents are
counted neither in `docCounts` nor root presence (`builder.go:925-946`).
**F14.** "Is the index aggregated" is derivable (`NumDocs > rootPresent.Count()`), but an aggregated
index where every RG got one doc is indistinguishable from a 1:1 index. A per-index switch therefore
cannot fully fix TCLR (e.g. a file with one one-line row group).

### Query semantics today

**F15.** `NE` = `present ∩ (¬eq ∪ MultiValueRGs)` (`query.go:336-364`); `NIN` via `negateTerms` (`query.go:676-702`).
Both require presence, in all RG shapes — absent docs never satisfy NE/NIN (oracle `docMatches`,
`aggregate_negation_test.go:55-60`). They are consistent; only `IsNull` changes meaning by doc count.
`IsNotNull` = `PresentRGBitmap` (`query.go:712-718`), correct in all shapes. `EQ` unaffected.
**F16.** Unknown path / invalid path → `AllRGs` for every operator (`query.go:77-81`). Evaluate godoc:
"returns the set of row groups that may contain matching documents" (`query.go:23-29`). The repo's
correctness bar since #60 is "no operator under-selects" (oracle and property tests assert `superset`).

### Wire format

**F17.** `Version = 11` (`gin.go:14-36`). Decode rejects any other version with `ErrVersionMismatch`
(`serialize.go:748-753`); "the only migration path is to rebuild" (`gin.go:16-18`). No upgrade path.
**F18.** Aggregate section: `uint32 count`, then per path `uint16 pathID`, `RGSet MultiValueRGs`, `RGSet AbsentRGs`,
`uint32 len + []uint32 DistinctCounts` (`serialize.go:1594-1670`); after null indexes (`serialize.go:508`, `658`).
**F19.** No capability/completeness flags: `Header.Flags` has only `FlagHasDocIDMap` (`gin.go:39-41`).
`FlagTrigramIndex` is a per-path flag. A reader can detect an old index only via `Version`, and Decode
already refuses it.
**F20.** Prior bumps: v10 (#61) and v11 (#63) each bumped `Version`, added a CHANGELOG line, and
regenerated `testdata/parity-golden/*.bin` (README there: "Regenerate only when the serialization
format bumps ... or an approved behavior change requires it, and record the review in the audit trail").

### Tests

**F21.** Tests that pin the CURRENT one-doc behaviour and would change under a uniform fix:
- `aggregate_negation_test.go:486-503` `TestNegationSingleDocumentSemanticsUnchanged` — `IsNull($.tags[*])` want `[]` for 1:1, and `len(AggregateIndexes)==0`.
- `parser_parity_test.go:375` `{"IsNull-match", IsNull("$.status"), []int{2}}` — doc 3 lacks `status`; uniform fix gives `[2 3]`.
- `gin_test.go:1301-1319` `TestQueryNull` only asserts RG0 is set — unaffected.
**F22. Why the property test missed it.** `TestPropertyNegationUnderAggregation` (`aggregate_negation_test.go:673-725`)
generates 18 docs over 6 RGs (`gen.SliceOfN(numRGs*3, …)`, `:653`), so one-doc RGs do occur, but the oracle encodes the same
rule: `OpIsNull: (present && value == nil) || (!present && aggregated)` where `aggregated = perRG >= 2`
(`aggregate_negation_test.go:61-62`, `:163-175`). Exactness is also only checked on `multiDoc` positions (`:712-715`).
The oracle mirrors the implementation, so it cannot find this.
**F23.** `integration_property_test.go:536,585` only call IsNull without asserting results.

## Candidate approaches (no final design)

| # | Approach | Format change | Size cost | Old v10/v11 indexes | 1:1 index behaviour |
|---|----------|---------------|-----------|---------------------|---------------------|
| A | **Query-side, uniform:** `IsNull(P) = null ∪ AbsentRGs ∪ (rootPresent ∖ present(P))` | none | none | fixed on read (same code path) | changes: absent counts as null everywhere (over-select vs. old reading, never under-select). README:344, CHANGELOG, F21 tests change. |
| B | **Query-side, gated on "index is aggregated"** (`NumDocs > rootPresent.Count()`) | none | none | fixed on read | unchanged | 
| C | **Build-side, widen AbsentRGs to all positions with ≥1 doc** | layout same; semantics change → should bump to v12 so old files are rejected (F17, F19) | one extra bitmap per path that is absent in any one-doc RG; `AggregateIndex` now exists for 1:1 indexes | rejected by Decode after bump (rebuild) | same change as A |
| D | **Build-side + persisted semantics flag** (header flag or config field "absent is null"), builder option chosen by caller | yes, v12 | 2 bytes / tiny | rejected after bump | opt-in; default unchanged |
| E | **New operator** (e.g. `IsNullOrMissing`/`IsMissing`) derived as in A; `IsNull` keeps explicit-null meaning | none | none | works on old indexes | unchanged; API addition, and existing aggregated-RG `IsNull` semantics stay mixed unless also changed |

Trade-off notes:
- A/B/E rely on the invariant "root `$` is present for every committed document" (F6, F13). It holds for both built-in parsers today; a plan should pin it with a test.
- B does not fully fix the issue (F14): an aggregated index whose row groups each got one document reads as 1:1.
- A without a version bump silently changes answers for existing 1:1 indexes, but only toward more row groups (safe for pruning). The issue's acceptance "old indexes must not be reported as complete if they can still under-select" is met by A/B/E because old indexes stop under-selecting at query time; C needs a bump to meet it.
- C/D cost a v12 bump, golden regeneration (F20), and force all users to rebuild.
- Fallback when `$` has no NullIndex (should not happen; e.g. a hand-built or empty index): returning `AllRGs` for IsNull keeps the no-under-select bar.

## Validation Architecture

| Property | Value |
|----------|-------|
| Framework | Go `testing` + `gopter` v0.2.11 |
| Quick run | `go test -run 'Negation|IsNull|QueryNull|Parity' -count=1 .` |
| Full suite | `make test` (or `go test ./...`) |

Tests the fix needs (per issue acceptance): issue repro as a table test (`[2 3]`); one-doc RG lacking path; RG where path is
absent from every doc (multi-doc and one-doc); empty RG never selected; sparse DocIDs (F5); Encode/Decode round trip; root
`$` presence invariant for scalar/null roots. The property-test oracle at `aggregate_negation_test.go:61-62` must drop the
`aggregated` gate (or match the chosen semantics) and exactness should be checked on all non-empty positions, not only `multiDoc`.

## Security Domain

No new input surface. If a format bump is chosen, the new/changed section must keep the existing bounds checks and duplicate-path
rejection (`serialize.go:1622-1670`, `serialize_security_test.go`).

## Sources

- Code at HEAD `df42273`: `query.go`, `builder.go`, `gin.go`, `serialize.go`, `parser.go`, `aggregate_negation_test.go`, `parser_parity_test.go`, `gin_test.go`
- `git show 954d689` (PR #61), `gh issue view 60`, `gh pr view 61`
- `README.md:340-344`, `CHANGELOG.md:72-80`, `testdata/parity-golden/README.md`
- Scratch repro: `$SCRATCHPAD/repro/main.go`, `$SCRATCHPAD/root/main.go` (outside repo)
