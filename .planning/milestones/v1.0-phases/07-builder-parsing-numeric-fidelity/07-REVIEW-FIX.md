---
phase: 07-builder-parsing-numeric-fidelity
fixed_at: 2026-04-15T12:10:19Z
review_path: /Users/tazarov/experiments/amikos/custom-gin/.planning/phases/07-builder-parsing-numeric-fidelity/07-REVIEW.md
iteration: 1
findings_in_scope: 2
fixed: 2
skipped: 0
status: all_fixed
---

# Phase 07: Code Review Fix Report

**Fixed at:** 2026-04-15T12:10:19Z
**Source review:** /Users/tazarov/experiments/amikos/custom-gin/.planning/phases/07-builder-parsing-numeric-fidelity/07-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 2
- Fixed: 2
- Skipped: 0

## Fixed Issues

### WR-01: Int-Only Range Queries Drop Fractional Predicate Pruning

**Status:** fixed: requires human verification
**Files modified:** `query.go`, `gin_test.go`
**Commit:** `37456b2`
**Applied fix:** Rounded fractional bounds to the correct integer boundary for int-only `GT`/`GTE`/`LT`/`LTE` pruning and added regression coverage for fractional predicates on int-only indexes.

### WR-02: Transformers Under Materialized Subtrees Still See Raw `json.Number`

**Status:** fixed
**Files modified:** `builder.go`, `transformers_test.go`
**Commit:** `3f0daac`
**Applied fix:** Normalized materialized values with `prepareTransformerValue()` before wildcard/subtree transformer execution and added a regression test for a transformer attached to `$.items[*].metrics`.

---

_Fixed: 2026-04-15T12:10:19Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
