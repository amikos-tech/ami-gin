# Phase 18: Structured IngestError + CLI integration - Patterns

**Generated:** 2026-04-24
**Status:** Complete

## Files To Create Or Modify

| File | Role | Closest analog | Pattern to preserve |
|------|------|----------------|---------------------|
| `ingest_error.go` | New public structured ingest error API | `builder.go` local `softSkipDocumentError` and `stageCallbackError` | Error structs implement `Error()`, `Unwrap()`, and `Cause()` for Go wrapping plus `github.com/pkg/errors.Cause`. |
| `builder.go` | Wrap hard document failures with `*IngestError` | Existing Phase 17 failure-mode branches | Preserve soft skip behavior and keep parser contract/tragic errors non-`IngestError`. |
| `parser_stdlib.go` / `parser_sink.go` | Parser and sink error propagation | Parser docs in `parser.go`; `tagStageError` provenance | Parser errors remain parser-owned; sink callback errors propagate back to builder without becoming parser failures. |
| `failure_modes_test.go` | Per-layer hard/soft behavior matrix | Existing parser/transformer/numeric failure-mode tests | Table-driven tests assert exact layer/path/value/cause semantics and compatibility with `errors.As`. |
| `atomicity_test.go` | Public failure usability/atomicity fixtures | Existing unsupported parser fixtures | Reuse `unsupportedTokenAtomicityParser`, numeric parser fixtures, and non-tragic assertions. |
| `cmd/gin-index/experiment.go` | Collect structured ingest failures during JSONL ingest | Existing `buildExperimentIndex` counters | Keep accepted documents densely packed via `result.ingestedDocs/rgSize`; store line/input sample metadata separately. |
| `cmd/gin-index/experiment_output.go` | Emit text and JSON failure summaries | Existing `experimentReport`, `experimentSummary`, `writeExperimentText`, `writeExperimentJSON` | Preserve single-object JSON shape and additive summary fields. |
| `cmd/gin-index/experiment_test.go` | CLI golden behavior | Existing `TestRunExperimentOnErrorContinue` text/JSON tests | Add tests for failure groups without breaking existing skipped/error counts. |
| `Makefile` | Scoped static guard | Existing `check-validator-markers` | Add narrow guard target and include it in `lint` if low-noise. |
| `CHANGELOG.md` | Public API note | Existing Unreleased notes | Mention exported `IngestError`, layers, and verbatim non-redacted `Value`. |

## Code Excerpts And Reuse Points

### Public Error Type Shape

Current local pattern in `builder.go`:

```go
type softSkipDocumentError struct {
	kind softSkipKind
	err  error
}

func (e *softSkipDocumentError) Error() string { return e.err.Error() }
func (e *softSkipDocumentError) Unwrap() error { return e.err }
func (e *softSkipDocumentError) Cause() error  { return e.err }
```

`IngestError` should use the same method trio, but with exported fields:

```go
type IngestError struct {
	Path  string
	Layer IngestLayer
	Value string
	Err   error
}
```

### Parser Error Boundary

`AddDocument` currently distinguishes callback failures before parser soft handling:

```go
if err := b.parser.Parse(jsonDoc, pos, b); err != nil {
	if isSkipDocument(err) {
		kind := softSkipDocumentKind(err)
		if kind == softSkipKindOther {
			kind = softSkipKindParser
		}
		b.recordSoftDocumentSkip(kind)
		return nil
	}
	if isStageCallbackError(err) {
		return unwrapStageCallbackError(err)
	}
	if b.config.ParserFailureMode == IngestFailureSoft {
		b.recordSoftDocumentSkip(softSkipKindParser)
		return nil
	}
	return err
}
```

Phase 18 should keep the first two branches unchanged and replace the final `return err` with parser-layer `IngestError` construction.

### Transformer Failure Site

Current hard transformer branch:

```go
transformed, ok := registration.FieldTransformer(prepared)
if !ok {
	if normalizeTransformerFailureMode(registration.Transformer.FailureMode) == IngestFailureSoft {
		b.numSoftRepresentationSkips++
		logging.Info(...)
		continue
	}
	return errors.Errorf("companion transformer %q on %s failed to produce a value", registration.Alias, canonicalPath)
}
```

Keep the soft branch exactly. The hard branch should return a transformer-layer `IngestError` with `Path: canonicalPath` and `Value` derived from the source value, not `registration.TargetPath`.

### Numeric Failure Sites

Current hard numeric returns:

```go
return errors.Wrapf(err, "parse numeric at %s", path)
return errors.Errorf("unsupported mixed numeric promotion at %s", path)
```

Soft numeric mode already has a typed skip:

```go
return newSoftSkipNumericDocumentError(path)
```

Phase 18 should leave soft mode unchanged and wrap only hard numeric returns.

### CLI Result And Output Shape

Current result/report pipeline:

```go
type experimentBuildResult struct {
	idx            *gin.GINIndex
	processedLines int
	ingestedDocs   int
	rowGroups      int
	skippedLines   int
	errorCount     int
	sampleCapped   bool
}
```

```go
type experimentSummary struct {
	Documents      int    `json:"documents"`
	RowGroups      int    `json:"row_groups"`
	RGSize         int    `json:"rg_size"`
	SampleLimit    int    `json:"sample_limit"`
	ProcessedLines int    `json:"processed_lines"`
	SkippedLines   int    `json:"skipped_lines"`
	ErrorCount     int    `json:"error_count"`
	Status         string `json:"status"`
	SidecarPath    string `json:"sidecar_path"`
}
```

Add failure groups as an additive summary field:

```go
Failures []experimentFailureGroup `json:"failures,omitempty"`
```

Keep accepted-document packing:

```go
lineErr := validateExperimentRecord(builder, record, result.ingestedDocs/rgSize)
```

The failure sample should use `lineNumber` and `lineNumber - 1`, not accepted document position.

## Test Fixture Patterns

Use existing fixtures instead of inventing new low-level hooks:

- Parser hard failure: `builder.AddDocument(DocID(0), []byte("not-json"))`
- Transformer hard failure: config with `WithEmailDomainTransformer("$.email", "domain")`, then `{"email":42}`
- Numeric hard failure: seed large int then float on same path, or custom parser fixtures from `atomicity_test.go`
- Schema hard failure: `unsupportedTokenAtomicityParser` or a strict transformer returning an unsupported value type
- Parser contract non-`IngestError`: `skipBeginDocumentParser`, `doubleBeginDocumentParser`, `wrongRGIDParser`

## Data Flow

```text
AddDocument(jsonDoc)
  -> parser.Parse(...)
     -> parser errors: builder wraps as IngestLayerParser unless soft
     -> sink callback errors: parser returns stageCallbackError
        -> builder unwraps and returns existing layer-specific IngestError
  -> validateStagedPaths(...)
     -> numeric promotion failures: IngestLayerNumeric unless soft
  -> mergeDocumentState(...)
     -> tragic/internal errors remain non-IngestError and close builder

gin-index experiment
  -> validateExperimentRecord(...)
     -> AddDocument returns *gin.IngestError
  -> buildExperimentIndex records skipped/error counters
  -> aggregate failures by IngestError.Layer
  -> report.Summary.Failures in text and JSON
```

## PATTERN MAPPING COMPLETE
