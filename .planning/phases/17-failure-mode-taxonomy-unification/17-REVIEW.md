---
phase: 17-failure-mode-taxonomy-unification
reviewed: 2026-04-23T16:59:23Z
depth: standard
files_reviewed: 12
files_reviewed_list:
  - CHANGELOG.md
  - builder.go
  - examples/failure-modes/main.go
  - examples/failure-modes/main_test.go
  - failure_modes_test.go
  - gin.go
  - parser_sink.go
  - phase09_review_test.go
  - serialize.go
  - serialize_security_test.go
  - transformer_registry.go
  - transformers_test.go
findings:
  critical: 0
  warning: 3
  info: 0
  total: 3
status: issues_found
---

# Phase 17: Code Review Report

**Reviewed:** 2026-04-23T16:59:23Z
**Depth:** standard
**Files Reviewed:** 12
**Status:** issues_found

## Summary

Reviewed the failure-mode API rename, builder soft-skip routing, v9 transformer metadata compatibility, serialization tests, changelog, and failure-mode example. The intended hard/soft behavior is well covered and `go test ./...` passes. Three uncovered edge cases remain: two regex transformer reconstruction issues and one decode error-classification inconsistency.

## Warnings

### WR-01: Negative Regex Capture Groups Can Panic During Ingest

**File:** `transformer_registry.go:126-129`
**Issue:** `ReconstructTransformer` accepts a negative `RegexParams.Group`. For both regex transformer variants, `len(matches) <= p.Group` is false when `p.Group` is negative, so `matches[p.Group]` panics during `AddDocument` for configs created with `WithRegexExtractTransformer(..., -1)` or decoded from serialized metadata. That bypasses the hard/soft failure-mode contract and can crash callers instead of returning a config error or transformer failure.
**Fix:**
```go
if p.Group < 0 {
	return nil, errors.New("regex group must be non-negative")
}

// Keep the runtime guard defensive too.
if p.Group < 0 || len(matches) <= p.Group {
	return nil, false
}
```
Apply the same validation in the `TransformerRegexExtract` and `TransformerRegexExtractInt` reconstruction branches.

### WR-02: RegexExtractInt Treats Empty Captures As Numeric Zero

**File:** `transformer_registry.go:192-224`
**Issue:** `parseFloatSimple("")`, `parseFloatSimple("-")`, and `parseFloatSimple(".")` return `0, nil` because the parser never verifies that at least one digit was consumed. A regex with an optional or empty capture can therefore materialize a derived numeric value `0` instead of failing the transformer, which changes query results and soft-failure routing.
**Fix:**
```go
digits := 0
for i := 0; i < len(s); i++ {
	c := s[i]
	// existing dot handling...
	if c < '0' || c > '9' {
		return 0, errors.New("invalid number")
	}
	digits++
	// existing accumulation...
}
if digits == 0 {
	return 0, errors.New("invalid number")
}
```
Add tests for empty, sign-only, and dot-only captures in `TestRegexExtractInt`.

### WR-03: Oversized Config Decode Errors Lose ErrInvalidFormat

**File:** `serialize.go:1665-1666`
**Issue:** Most decode bounds checks wrap `ErrInvalidFormat`, but an oversized serialized config returns a plain formatted error. `Decode` callers and telemetry classification that branch on `errors.Is(err, ErrInvalidFormat)` will miss this corrupt-input case even though it is a format violation.
**Fix:**
```go
if configLen > maxConfigSize {
	return nil, errors.Wrapf(ErrInvalidFormat, "config size %d exceeds max %d", configLen, maxConfigSize)
}
```
Add a regression test that calls `readConfig` or `Decode` with `configLen = maxConfigSize + 1` and asserts `errors.Is(err, ErrInvalidFormat)`.

---

_Reviewed: 2026-04-23T16:59:23Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
