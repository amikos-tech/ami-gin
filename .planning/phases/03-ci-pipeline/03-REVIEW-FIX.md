---
phase: 03-ci-pipeline
fixed_at: 2026-04-11T16:59:23Z
review_path: .planning/phases/03-ci-pipeline/03-REVIEW.md
iteration: 1
findings_in_scope: 2
fixed: 2
skipped: 0
status: all_fixed
---

# Phase 03: Code Review Fix Report

**Fixed at:** 2026-04-11T16:59:23Z
**Source review:** `.planning/phases/03-ci-pipeline/03-REVIEW.md`
**Iteration:** 1

**Summary:**
- Findings in scope: 2
- Fixed: 2
- Skipped: 0

## Fixed Issues

### WR-01: Documented CLI `REGEX` Queries Still Fail to Parse

**Files modified:** `cmd/gin-index/main.go`, `cmd/gin-index/main_test.go`
**Commit:** `7b4d0f6`
**Applied fix:** Added the missing `REGEX` operator mapping in `parsePredicate()` and added CLI regression coverage for `REGEX`, `CONTAINS`, `IN`, and null predicate parsing.

### WR-02: Local Write Hardening Now Changes Artifact Permissions

**Files modified:** `cmd/gin-index/main.go`, `cmd/gin-index/main_test.go`, `parquet.go`, `parquet_test.go`
**Commit:** `72e5485`
**Applied fix:** Preserved source Parquet permissions for new local sidecars and extracted index files, and reused the original Parquet mode when rewriting embedded indexes. Added regression tests for CLI sidecar/extract flows plus sidecar and embed library paths.

---

_Fixed: 2026-04-11T16:59:23Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
