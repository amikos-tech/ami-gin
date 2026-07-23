---
phase: 21-simd-parser-adapter
reviewed: 2026-07-23T06:53:53Z
depth: standard
files_reviewed: 17
files_reviewed_list:
  - CHANGELOG.md
  - Makefile
  - NOTICE.md
  - README.md
  - builder.go
  - docs/simd-deployment.md
  - go.mod
  - go.sum
  - parser.go
  - parser_lifecycle_test.go
  - parser_parity_fixtures_test.go
  - parser_simd.go
  - parser_simd_lifecycle_test.go
  - parser_simd_test.go
  - parser_sink.go
  - parser_test.go
  - testdata/parity-golden/simd-numeric-parity.bin
findings:
  critical: 1
  warning: 5
  info: 0
  total: 6
status: issues_found
---

# Phase 21: Code Review Report

**Reviewed:** 2026-07-23T06:53:53Z
**Depth:** standard
**Files Reviewed:** 17
**Status:** issues_found

## Summary

The current lifecycle marker correctly makes an ordinarily returned document-close error terminal and keeps returned stage/soft-skip causes discoverable. It still misses the panic path: when walking panics and document cleanup also fails, the cleanup failure is discarded, the builder remains open, and the native parser remains busy. A caller that recovers the panic can then silently lose subsequent documents under parser soft mode.

Five additional robustness and quality defects remain: callers cannot deterministically close the native parser, element type inspection erases native causes, representation-skip observability is published before document commit, no test executes the real adapter, and normal automation does not enforce the SIMD-tagged gates.

Verification completed successfully for default and tagged `go test ./...`, default and tagged `go vet ./...`, `make simd-isolation-check`, `go mod verify`, `go mod tidy -diff`, and tagged `govulncheck` (no reachable vulnerabilities). Those passing gates do not exercise `NewSIMDParser` or the real native parse path. The binary golden was reviewed for provenance and usage: it is deterministically generated through the stdlib golden generator, consumed by the authored-fixture parity test, and currently has SHA-256 `696b8a619eea2f5629bcf8fa6d285473a12c09a0e580f312fcbf2bcf2aa5ef8f`.

## Narrative Findings (AI reviewer)

### Critical Issues

#### CR-01: A walk panic can hide a failed close and leave soft-mode ingestion silently dropping documents

**Classification:** BLOCKER

**File:** `parser_simd.go:51-63`, `parser_simd_lifecycle_test.go:51-73`

**Issue:** `finishSIMDDocument` defers cleanup, but it does not recover a panic from `walk`. If `walk` panics and `closeDocument` also returns an error, the defer assigns a `parserLifecycleError` to the named return value and then the original panic resumes; the assigned error is never returned. `GINBuilder.AddDocument` therefore never sees the lifecycle marker and never sets `tragicErr`.

This is reachable through arbitrary field transformers, which execute inside the SIMD walk and may panic. The pinned native parser leaves its live-document handle set when `Doc.Close` fails, so it remains busy. If an application-level recovery boundary catches the transformer panic and reuses the builder, later `ErrParserBusy` failures are ordinary parser errors; `ParserFailureMode=IngestFailureSoft` converts them to `nil` and silently drops valid documents. The panic test covers only a successful close, so this combined failure is untested.

**Fix:** Capture the panic in the cleanup defer. If close succeeds, re-panic unchanged. If close fails, convert the recovered value to an error, return a `parserLifecycleError`, and let `AddDocument` poison the builder. Add a regression covering panic plus failed close under parser soft mode.

```go
func finishSIMDDocument(walk func() error, closeDocument func() error) (err error) {
	defer func() {
		recovered := recover()
		closeErr := closeDocument()
		if closeErr != nil {
			concurrentErr := err
			if recovered != nil {
				if panicErr, ok := recovered.(error); ok {
					concurrentErr = panicErr
				} else {
					concurrentErr = errors.Errorf("panic while walking SIMD document: %v", recovered)
				}
			}
			err = newParserLifecycleError(
				errors.Wrap(closeErr, "close pure-simdjson document"),
				concurrentErr,
			)
			return
		}
		if recovered != nil {
			panic(recovered)
		}
	}()
	return walk()
}
```

### Warnings

#### WR-01: The adapter exposes no deterministic native-parser cleanup path

**Classification:** WARNING

**File:** `parser_simd.go:19-35`, `README.md:96-107`, `docs/simd-deployment.md:43-55`

**Issue:** The wrapped native parser owns a handle and its upstream API expects callers to invoke `Parser.Close`. `NewSIMDParser` returns the base `Parser` interface, the concrete wrapper is private, and the wrapper has no `Close` method. The builder also does not own or release it. Callers can therefore only wait for a GC finalizer, and the documented helpers discard the parser handle after passing it into the builder. Repeated construction in a long-lived process can retain native resources for an unbounded time.

**Fix:** Preserve the constructor shape if required, but make `simdParser` implement `io.Closer` and document that the returned value must be retained and closed after all builders using it are finished. An exported interface embedding `Parser` and `io.Closer` would make the ownership contract compile-time visible.

```go
func (s *simdParser) Close() error {
	return s.parser.Close()
}

var _ io.Closer = (*simdParser)(nil)
```

#### WR-02: Element type inspection erases native failure causes

**Classification:** WARNING

**File:** `parser_simd.go:81,172`

**Issue:** Both traversal paths use `element.Type()`. In the pinned dependency, `Type()` intentionally collapses closed handles, invalid handles, precision-loss failures, and native panic failures to `TypeInvalid`. The adapter then replaces the cause with a generic “invalid element” error. Callers cannot use `errors.Is`/`errors.As` to diagnose the native failure, and parser policy receives less accurate provenance.

**Fix:** Call `TypeErr()` in both `walkElement` and `materializeElement`, wrapping the returned cause with path context. Reserve the generic invalid-kind error for a successful `TypeErr()` call that actually reports `TypeInvalid`.

```go
elementType, err := element.TypeErr()
if err != nil {
	return errors.Wrapf(err, "read pure-simdjson element type at %s", canonicalPath)
}
```

#### WR-03: Representation-skip metrics and logs are emitted for documents that never commit

**Classification:** WARNING

**File:** `builder.go:642-664`, `builder.go:980-984`

**Issue:** A soft transformer failure increments `numSoftRepresentationSkips` and emits its log immediately during staging. A later numeric failure, parser failure after staging (such as trailing JSON), hard failure from another companion, or merge failure can reject the entire document. The public counter says it represents cases where the source document was kept, but it still counts these discarded documents. Logs and counts can also depend on field traversal order.

**Fix:** Accumulate representation-skip events in `documentBuildState` and publish the counter/logs only after `commitStagedPaths` succeeds. Add tests where a soft companion failure precedes a later hard and soft document failure, including a trailing-content parser error.

#### WR-04: The test suite never constructs or executes the real SIMD adapter

**Classification:** WARNING

**File:** `parser_simd_test.go:11-29`, `parser_simd_lifecycle_test.go:75-98`, `parser_parity_fixtures_test.go:23-31`

**Issue:** `parser_simd_test.go` exercises a synthetic typed-sink parser, the lifecycle tests exercise callback doubles, and the SIMD-named golden is generated and asserted through the stdlib parser. No test calls `NewSIMDParser`, `simdParser.Parse`, `walkElement`, or `materializeElement` against a native document. Consequently, the tagged suite proves compilation and helper behavior but cannot catch adapter wiring, traversal, native loading, or numeric-parity defects.

**Fix:** Add a tagged smoke/parity suite using a controlled native artifact. It should construct and close the adapter, parse representative scalar/container/transformer fixtures, and compare encoded output and query results with stdlib. In the configured tagged CI job, absence of the required artifact should fail rather than skip.

#### WR-05: SIMD isolation and tagged checks are outside normal automation

**Classification:** WARNING

**File:** `Makefile:7-31,37-90`

**Issue:** `simd-isolation-check` is standalone; `build`, `test`, `integration-test`, `lint`, and `security-scan` do not depend on it. The standard targets also never run tests, vet, or vulnerability analysis with `-tags simdjson`, and the current CI invokes only the standard targets. An accidental default import or tagged-only regression can therefore pass every required gate.

**Fix:** Add a required SIMD verification target that depends on the isolation check and runs tagged tests, vet, and vulnerability scanning, then invoke it from CI:

```make
.PHONY: verify-simd
verify-simd: simd-isolation-check
	go test -tags simdjson ./...
	go vet -tags simdjson ./...
	govulncheck -tags simdjson ./...
```

---

_Reviewed: 2026-07-23T06:53:53Z_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: standard_
