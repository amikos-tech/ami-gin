---
phase: 03-ci-pipeline
reviewed: 2026-04-11T16:40:31Z
depth: standard
files_reviewed: 8
files_reviewed_list:
  - .github/workflows/ci.yml
  - .github/workflows/security.yml
  - .golangci.yml
  - Makefile
  - README.md
  - cmd/gin-index/main.go
  - parquet.go
  - serialize.go
findings:
  critical: 0
  warning: 2
  info: 1
  total: 3
critical: 0
warning: 2
info: 1
total: 3
status: issues_found
---

# Phase 03: Code Review Report

**Reviewed:** 2026-04-11T16:40:31Z
**Depth:** standard
**Files Reviewed:** 8
**Status:** issues_found

## Summary

Reviewed the new CI/security workflows, linter config, Makefile targets, README updates, and the local Parquet/CLI I/O changes in `cmd/gin-index`, `parquet.go`, and `serialize.go`.

`go test ./...` passes, but the review found two real regressions in the changed CLI/local-file paths and one test-coverage gap that explains why they were not caught.

## Warnings

### WR-01: Documented CLI `REGEX` Queries Still Fail to Parse

**File:** `cmd/gin-index/main.go:618-646`
**Issue:** `parsePredicate()` maps `EQ`, `NE`, range operators, `CONTAINS`, `IN`, and `NOT IN`, but never maps `REGEX` to `gin.OpRegex`. That leaves a documented CLI feature unusable: `README.md:519-525` advertises `$.field REGEX "pattern"`, and the library itself supports `OpRegex` in `query.go`, but `go run ./cmd/gin-index query missing.gin '$.brand REGEX "Toyota|Tesla"'` fails immediately with `Failed to parse predicate`.
**Fix:**

```go
patterns := []struct {
	regex *regexp.Regexp
	op    gin.Operator
}{
	{regexp.MustCompile(`^(.+?)\s*!=\s*(.+)$`), gin.OpNE},
	{regexp.MustCompile(`^(.+?)\s*>=\s*(.+)$`), gin.OpGTE},
	{regexp.MustCompile(`^(.+?)\s*<=\s*(.+)$`), gin.OpLTE},
	{regexp.MustCompile(`^(.+?)\s*>\s*(.+)$`), gin.OpGT},
	{regexp.MustCompile(`^(.+?)\s*<\s*(.+)$`), gin.OpLT},
	{regexp.MustCompile(`^(.+?)\s*=\s*(.+)$`), gin.OpEQ},
	{regexp.MustCompile(`(?i)^(.+?)\s+CONTAINS\s+(.+)$`), gin.OpContains},
	{regexp.MustCompile(`(?i)^(.+?)\s+REGEX\s+(.+)$`), gin.OpRegex},
	{regexp.MustCompile(`(?i)^(.+?)\s+IN\s+\((.+)\)$`), gin.OpIN},
	{regexp.MustCompile(`(?i)^(.+?)\s+NOT\s+IN\s+\((.+)\)$`), gin.OpNIN},
}
```

Also add table-driven CLI parser tests for `REGEX`, `CONTAINS`, `IN`, and null predicates.

### WR-02: Local Write Hardening Now Changes Artifact Permissions

**File:** `parquet.go:31-37`, `parquet.go:279-306`, `cmd/gin-index/main.go:498-500`
**Issue:** the new local write helpers force `0600` on every generated sidecar and rewritten Parquet file. For sidecars, that is a compatibility regression for shared environments where multiple users/processes can read the `.parquet` but need to read the adjacent `.gin`. For embedded indexes it is more severe: `RebuildWithIndex()` now rewrites the destination through a `0600` temp file, so an existing `0644` Parquet becomes `0600` after `gin-index build -embed` or direct library use. I reproduced the changed modes locally: `before=0644`, `sidecar=0600`, `after=0600`.
**Fix:**

```go
st, err := os.Stat(parquetFile)
if err != nil {
	return errors.Wrap(err, "stat parquet file")
}
mode := st.Mode().Perm()

f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
if err != nil {
	return errors.Wrap(err, "create temp file")
}
```

For sidecars and generic CLI outputs, either preserve the existing destination mode when overwriting, derive the default from the source Parquet file, or make the stricter mode opt-in/configurable instead of unconditional.

## Info

### IN-01: The Changed CLI Surface Still Has No Automated Coverage

**File:** `cmd/gin-index/main.go:488-687`
**Issue:** `cmd/gin-index` still has no test files. `go test ./...` reports `github.com/amikos-tech/ami-gin/cmd/gin-index [no test files]`, even though this phase changed local file I/O, permissions, and predicate parsing in that package. That gap is why the broken `REGEX` path and the file-mode regression shipped together.
**Fix:** Add `cmd/gin-index/main_test.go` with table-driven tests for `parsePredicate()` / `parseValueList()` plus temp-dir tests covering `writeLocalIndexFile()`, overwrite behavior, and expected file permissions for sidecar and `-embed` flows.

---

_Reviewed: 2026-04-11T16:40:31Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
