# Quick Task 261006-mlx: Fix issue #89, IsNull misses a row group whose only document lacks the path - Context

**Gathered:** 2026-10-06
**Status:** Grilling in progress
**Ledger:** `261006-mlx-GRILLING.jsonl` (every question, options, recommendation, answer)
**Research:** `261006-mlx-RESEARCH.md` (facts F1-F23)

<domain>
## Task Boundary

Validate and address GitHub issue #89 (`issue-89.md`). `IsNull(path)` does not select a row group (RG)
whose only document lacks the path, so `NE ∪ IsNull` under-selects.

</domain>

## Terms

- **RG (row group):** one position in the index bitmaps. Several documents can share one RG (DocID aggregation).
- **present / null / absent:** for a path P, a document has a value for P, has an explicit JSON `null`, or does not carry the key.
- **under-select:** the index tells the caller to skip an RG that holds a matching document. Wrong result. Forbidden.
- **over-select:** the index keeps an RG that holds no match. Slower scan, still correct.
- **1:1 index:** every RG received exactly one document.
- **root presence:** `NullIndexes[$].PresentRGBitmap`, the RGs that received at least one document.

## Problem statement (P0)

`IsNull(P) = NullRGBitmap ∪ AggregateIndex.AbsentRGs` (`query.go:704-710`). The builder sets `AbsentRGs`
only for RGs with 2+ documents (`builder.go:1437-1462`). A one-document RG without P is in neither set.

This was a deliberate choice in PR #61 (v1.1.0) and is documented at `README.md:344`. The issue's claim
that docs describe the expected rule is not accurate. The real defect: one index has two meanings for
`IsNull`, chosen per RG by document count. `NE`, `NIN`, `IsNotNull` are consistent across RG shapes.

The finalized index can already answer "RG has a document without P": `present($) ∖ present(P)`
(research F12, F13). A query-side fix needs no wire format change and repairs old v11 indexes on read (Decode rejects every other version, research F17).

Status: confirmed by user (P0).

<decisions>
## Locked Decisions

### D1 (Q1): One meaning for IsNull, absent counts as null everywhere
- `IsNull(P)` selects every RG holding at least one document with explicit `null` for P, or without P.
  This holds whatever the RG's document count, including plain 1:1 indexes.
- Why: prior art (`261006-mlx-RESEARCH-NULL-SEMANTICS.md`). Most systems collapse absent into the default
  null check (MongoDB `{f:null}`, ClickHouse JSON, Iceberg/Delta, SQL `->>`/`JSON_VALUE`). Three-state systems
  (Couchbase, Snowflake) use separate operators; none changes one operator's meaning by data shape.
  TCLR's `NE ∪ IsNull` is the standard SQL `x <> v OR x IS NULL` pattern and needs this meaning.
- Cost: 1:1 callers who read IsNull as "explicit null only" get more RGs (over-select, safe for pruning).
- Not done now: a strict explicit-null-only operator. It stays possible later as a separate addition.

### D2 (Q2): Fix on the query side, no wire format change
- `IsNull(P) = NullRGBitmap ∪ AbsentRGs ∪ (present($) ∖ present(P))`.
- Format `Version` stays 11. Builder and the `AbsentRGs` layout do not change.
- Old v11 indexes give correct answers when read, so no readable old index keeps under-selecting. (Correction after verification: Decode rejects v10 and older, so those were already forced to rebuild.)
  This is the issue's required wire format statement.
- The rule "every committed document marks `$` present" gets a pinning test (research F6, F13).

### D3 (Q3): Minor release signal
- CHANGELOG Unreleased: a "Changed" entry (IsNull in one-document RGs and 1:1 indexes now selects RGs
  whose document lacks the path; replaces the README rule) and a "Fixed" entry (#89).
- `README.md:344` rewritten to the uniform rule. `IsNull` gets a godoc. The `AbsentRGs` field comment
  (`gin.go:235-249`) states that one-document RGs are derived at query time from root presence.
- Release cut (v1.5.0) is a separate `chore(release)` commit, not part of this task.

### Settled without a question (facts, not choices)
- `IsNull` takes no value, so `As()` cannot route it to a derived companion. Companions are out of scope.
- The builder makes a NullIndex for every path (`builder.go:1409`).
- If the root `$` has no NullIndex (not produced by the builder; hand-built or crafted index), IsNull
  returns `AllRGs`. This keeps the no-under-select bar.
- `AbsentRGs` stays necessary: in an RG with `{"env":"prod"}` and `{"app":"x"}`, `present($.env)` holds
  the RG, so only the builder's per-document count sees the absent document.
- The property-test oracle (`aggregate_negation_test.go:61-62`) drops the `aggregated` gate and checks
  exactness on every RG that received documents, not only multi-document RGs.
- Two tests that pin the old one-document rule change: `TestNegationSingleDocumentSemanticsUnchanged`
  (`aggregate_negation_test.go:486-503`) and the parity case `IsNull("$.status")` (`parser_parity_test.go:375`).

### Deferred
See `261006-mlx-DEFERRED.md`: NE matching absent, strict explicit-null operator, release cut.

</decisions>

## Expectations (hold-out set, BDD)

Hold-out set for post-implementation validation. The verifier checks each one with a run, not by reading
the plan. Terms as defined above. "Repro index" = the 4-RG index from issue #89:
RG0 `{"env":"prod"}`×2, RG1 `{"env":"prod"}`, RG2 `{"app":"x"}`,`{"app":"y"}`, RG3 `{"app":"z"}`.

**Core rule**
- **E1 (issue repro).** Given the repro index, when I evaluate `IsNull($.env)`, then I get `[2 3]`.
  And `NE($.env,"prod")` gives `[]`, and `NE ∪ IsNull` gives `[2 3]`.
- **E2 (1:1 index, one document lacks the path).** Given RG0 `{"env":"prod"}`, RG1 `{"env":null}`,
  RG2 `{"app":"x"}`, one document each, when I evaluate `IsNull($.env)`, then I get `[1 2]`.
- **E3 (path absent from every document).** Given an RG with two documents that both lack P, and another RG
  with one document that lacks P, then `IsNull(P)` selects both RGs.
- **E4 (mixed RG, unchanged).** Given an RG with `{"env":"prod"}` and `{"app":"x"}`, then `IsNull($.env)`
  selects it (as before the fix).
- **E5 (no false additions).** Given an RG where every document carries P with a non-null value, then
  `IsNull(P)` does not select it. In the repro index RG0 and RG1 are not selected.
- **E6 (empty RG).** Given `numRGs` larger than the RGs that received documents, then `IsNull(P)` never
  selects an RG that received no document.
- **E7 (sparse DocIDs).** Given DocID 5 with two documents with env and DocID 2 with one document without
  env, then `IsNull($.env)` selects the position of DocID 2.
- **E8 (nested and array paths).** Given one-document RGs `{"a":null}` and `{"tags":[]}`, then
  `IsNull($.a.b)` selects the first and `IsNull($.tags[*])` selects the second.

**Safety and other operators**
- **E9 (never fewer).** For every generated index and path, the new `IsNull(P)` is a superset of the
  v1.4.0 `IsNull(P)`. The property test oracle uses the uniform rule and checks exactness on every RG
  that received documents.
- **E10 (other operators unchanged).** `EQ`, `NE`, `NIN`, `IN`, `IsNotNull`, range operators, `Contains`
  and `Regex` give the same results as v1.4.0 for the same index.
- **E11 (unknown path).** `IsNull` on a path the index never saw still returns all RGs.
- **E12 (missing root NullIndex).** Given an index whose `$` path has no NullIndex, then `IsNull(P)`
  returns all RGs (fail open, no under-select).
- **E13 (root presence rule).** Given documents `null`, `5`, `[]`, `"s"`, `{}`, then each one marks `$`
  present for its RG, with both built-in parsers.

**Wire format and old indexes**
- **E14 (format unchanged).** `Version` stays 11. The committed files in `testdata/parity-golden/` are
  not regenerated, and their bytes do not change.
- **E15 (old index fixed on read).** Given a v11 index file written before the fix (committed golden),
  when the new code decodes it and evaluates `IsNull`, then it returns the uniform answer
  (for example the parity case `IsNull("$.status")` gives `[2 3]`).
- **E16 (round trip).** Encode then Decode gives the same `IsNull` results as the in-memory index.

**Documentation**
- **E17 (one rule in all docs).** `README.md` (old line 344), the new `IsNull` godoc and the `AbsentRGs`
  comment state the same uniform rule. No text says "a one-document row group keeps the historical
  reading".
- **E18 (CHANGELOG).** Unreleased has a "Changed" entry for the 1:1 behavior and a "Fixed" entry for #89.
  No version number is cut.
- **E19 (gates).** `make test` and `make lint` pass.

<canonical_refs>
## Canonical References

- GitHub issue #89 (`issue-89.md`)
- PR #61 / commit `954d689` (introduced AbsentRGs, format v10)
- `README.md:344`, `CHANGELOG.md:74-80`, `gin.go:235-249`
- No `.planning/ONTOLOGY.md` exists; CLAUDE.md Architecture section is the domain reference.

</canonical_refs>

## Post-review changes (after /simplify and /pr-review-toolkit:review-pr)

- `evaluateIsNull` computes the complement against `Header.NumRowGroups`, not the path bitmap's own
  `NumRGs`. A decoded bitmap may be shorter (`readRGSet` only checks `<=`), which would have dropped high
  row groups (under-select). Test: short path bitmap gives `[2 3]` (failed before with `[2]`).
- A path with no NullIndex now fails open (`AllRGs`, was `NoRGs`). Built indexes never reach it; a crafted
  index could. Same no-under-select bar as the root branch (E12).
- The E9 legacy oracle is derived from the documents, not from live index internals.
- CHANGELOG: after shipping PR #90 the user chose flat bullets (repo style) over `### Changed` /
  `### Fixed` subsections. Both entries (behavior change, #89 fix) stay as two bullets under Unreleased.
- Skipped: Warn logging on the fail-open branches (low severity, new behavior).
