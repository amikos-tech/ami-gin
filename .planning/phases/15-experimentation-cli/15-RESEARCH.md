---
phase: 15
phase_name: "Experimentation CLI"
project: "GIN Index"
generated: "2026-04-22"
---

# Phase 15: Experimentation CLI - Research

**Researched:** 2026-04-22  
**Domain:** JSONL-driven experimentation command for the existing `gin-index` CLI  
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** `experiment` keeps row groups as an internal modeling concept rather than removing them from the command.
- **D-02:** Default `--rg-size` to `1` so first-run behavior is effectively row-level pruning.
- **D-03:** Keep `--rg-size N` as an advanced override for grouped-pruning realism.
- **D-05:** Default `--on-error` to `abort`.
- **D-06:** `--on-error continue` prints per-line diagnostics to `stderr` and keeps ingesting later lines.
- **D-07:** Continue mode must report processed, skipped, and error totals in the final summary.
- **D-08:** Text output order is run summary first, per-path table second, optional predicate block last.
- **D-09:** Reuse the existing `info` rendering logic instead of inventing a second unrelated formatter.
- **D-10:** `--json` is additive and machine-oriented; it does not replace the default text mode.
- **D-11:** `--test` output is count-focused: `matched`, `pruned`, `pruning_ratio`.
- **D-12:** Do not print matching/pruned RG ID lists by default.
- **D-13:** Expose `--log-level off|info|debug` in Phase 15.
- **D-14:** Do not expose a user-facing `--parser` flag in Phase 15.

### Explicit Non-Goals

- No REPL, TUI, auto-colour output, or terminal detection.
- No new query DSL; reuse `parsePredicate(...)`.
- No broader product-positioning rewrite about row-level pruning in this phase.
- No new non-stdlib CLI framework.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CLI-01 | New `experiment` subcommand in `cmd/gin-index/main.go` accepts JSONL from a file path or `-` (stdin). | Keep the established dispatcher pattern from `cmd/gin-index/main.go`, but use `runExperiment(args, stdin, stdout, stderr)` because this subcommand is the first one that must read `stdin` directly. |
| CLI-02 | Emit a per-path summary table reusing `writeIndexInfo` / `formatPathInfo`. | Reuse the existing text formatter by collecting experiment metadata separately, then calling the same path-summary helpers. Extend the helper path with bloom-occupancy reporting derived from available index metadata instead of building a second path renderer. |
| CLI-03 | Streaming JSONL ingest with bounded memory; no 64 KiB truncation. | Use `bufio.Reader.ReadBytes('\n')`, trim trailing `\n` / `\r\n`, and process one line at a time. Avoid `bufio.Scanner` default limits and avoid reading the whole file into memory. |
| CLI-04 | Optional `-o out.gin` writes the built index sidecar. | Reuse `gin.Encode(...)` plus `writeLocalIndexFile(...)`; preserve source-file permissions for local file inputs and fall back to `defaultLocalArtifactMode` for stdin. |
| CLI-05 | `--json` emits a stable schema for CI / `jq`. | Build one explicit `experimentReport` struct and marshal it with `encoding/json`; lock the schema with golden tests rather than relying on ad hoc map serialization. |
| CLI-06 | `--test '<predicate>'` shows matched/pruned row-group counts and pruning ratio. | Reuse `parsePredicate(...)` plus `idx.EvaluateContext(...)`; compute `matched=len(result.ToSlice())`, `pruned=total-matched`, `pruning_ratio=float64(pruned)/float64(total)`. |
| CLI-07 | `--on-error continue|abort` is configurable; default `abort`. | Continue mode should stream diagnostics to `stderr` immediately and store only counters, not full error payloads. Abort mode returns non-zero on the first invalid line. |
| CLI-08 | `--sample N` limits ingested documents for quick inspection. | Apply the limit to successfully ingested JSONL documents, not raw lines, so malformed skipped lines in continue mode do not prematurely consume the sample budget. |
</phase_requirements>

## Summary

The current CLI already provides the right structural template for Phase 15: command dispatch stays in `cmd/gin-index/main.go`, each subcommand owns a `flag.FlagSet`, and the testable surface is `runX(args, stdout, stderr)` plus lower-level helpers. The only structural addition Phase 15 needs is a stdin-aware runner signature: `runExperiment(args []string, stdin io.Reader, stdout, stderr io.Writer) int`, with `cmdExperiment(args)` delegating `os.Stdin` into it. That is the cleanest way to support both `gin-index experiment file.jsonl` and `cat file.jsonl | gin-index experiment -` without smuggling global `os.Stdin` reads into helpers or making tests shell out.

The output path should be report-driven rather than print-driven. Build one in-memory `experimentReport` with three layers: run summary, path summaries, and optional predicate result. Text mode prints the run summary first and then reuses the existing path rendering logic; JSON mode marshals the same report into a stable schema. This avoids the most likely Phase 15 failure mode: text and JSON diverging because they were built by separate code paths. The per-path rows should continue to surface `types`, `cardinality`, `mode`, and adaptive promoted-term metadata from `formatPathInfo(...)`. For bloom occupancy, the current library exposes one global bloom filter rather than per-path filter slices, so the practical phase-safe answer is an **estimate** derived from per-path cardinality and the global bloom parameters: `1 - exp(-k*n/m)` where `n` is `PathEntry.Cardinality`, `k` is `GlobalBloom.NumHashes()`, and `m` is `GlobalBloom.NumBits()`. That keeps the field useful for experimentation without requiring a new serialized index section or a builder-internal leak.

The ingest loop should stay deliberately boring, but the builder constraint matters: `GINBuilder` requires `numRGs` up front. That means the experiment command needs a **two-pass** strategy, not a single-pass parser loop. For local files, the first pass counts lines with bounded memory and the second pass performs the real build. For stdin, the first pass must spool the raw bytes to a temporary file while counting lines, then reopen that temp file for the real build pass. This still satisfies the roadmap's bounded-memory requirement because memory stays independent of total input size; disk is the tradeoff. In the second pass, read one line at a time with `ReadBytes('\n')`, accept a final line without a newline terminator, and trim only line endings, not other whitespace. Empty lines should be treated as invalid JSON input rather than silently skipped unless the line is the final trailing newline-only case that `ReadBytes` returns at EOF. Synthetic row-group assignment is then straightforward: keep a count of successfully ingested documents and derive `rgID = ingestedDocs / rgSize`. Because `GINBuilder.AddDocument(...)` maps repeated `DocID`s onto the same internal row-group position, using `DocID(rgID)` reproduces the existing grouped-ingest pattern already used by `BuildFromParquet(...)`.

Phase 14's observability seam is usable as-is. The CLI should not bootstrap telemetry providers in this phase, but `--log-level` can still route library log events into `stderr` by building a repo-owned logger from the existing adapters. The cleanest implementation is to use `logging/slogadapter` backed by a `slog.TextHandler` with an explicit level and no colour or TTY logic. That preserves real `debug` filtering semantics for future log sites, keeps diagnostics on `stderr`, and avoids the limitation of `logging/stdadapter`, which intentionally drops debug events. `off` should leave `gin.DefaultConfig()` untouched so the command remains silent by default.

The largest execution risk is not the command wiring; it is end-to-end correctness around malformed and oversized lines. The phase therefore needs dedicated tests for three cases that the existing CLI test suite does not cover today: stdin-driven execution, a line larger than 64 KiB, and mixed good/bad JSONL with `--on-error continue`. Those tests should be first-class merge gates because they are the easiest place for a future refactor to regress the phase goal while still keeping `go test ./...` green.

## Recommended Report Shape

### Text Mode

1. `Experiment Summary:` block
2. Existing `GIN Index Info:` / per-path section via `writeIndexInfo(...)`
3. Optional `Predicate Test:` block when `--test` is set

### JSON Mode

```json
{
  "source": {
    "input": "docs.jsonl",
    "stdin": false
  },
  "summary": {
    "documents": 1000,
    "row_groups": 1000,
    "paths": 12,
    "rg_size": 1,
    "sample_limit": 0,
    "processed_lines": 1000,
    "skipped_lines": 0,
    "error_count": 0,
    "sidecar_path": ""
  },
  "paths": [
    {
      "path": "$.status",
      "path_id": 0,
      "types": ["string"],
      "cardinality_estimate": 3,
      "mode": "exact",
      "bloom_occupancy_estimate": 0.02,
      "promoted_hot_terms": 0,
      "bucket_count": 0,
      "representations": []
    }
  ],
  "predicate_test": {
    "predicate": "$.status = \"error\"",
    "matched": 10,
    "pruned": 990,
    "pruning_ratio": 0.99
  }
}
```

Rules:

- `predicate_test` is omitted when `--test` is not set.
- `sidecar_path` is empty when `-o` is not set.
- `sample_limit` is `0` when sampling is disabled.
- `representations` is always an array, not `null`.

## Architecture Patterns

### Pattern 1: stdin-aware `runX` wrapper

Use:

```go
func cmdExperiment(args []string) {
	if code := runExperiment(args, os.Stdin, os.Stdout, os.Stderr); code != 0 {
		os.Exit(code)
	}
}
```

Why: this preserves the CLI's current "wrapper + testable runner" pattern while avoiding hidden reads from the process-global stdin stream.

### Pattern 2: report-first rendering

Build a single report struct:

- ingest populates counters and source metadata
- finalize populates row-group/path totals
- path-summary collection populates path rows
- predicate evaluation populates optional predicate block

Then:

- text mode prints from the report plus `writeIndexInfo(...)`
- JSON mode marshals the report directly

Why: one source of truth keeps text mode, JSON mode, and tests aligned.

### Pattern 3: explicit continue/abort line handling

For each line:

1. increment `processed_lines`
2. attempt `builder.AddDocument(...)`
3. on success: increment `documents`
4. on failure:
   - `abort`: print `line N: ...` to `stderr`, return non-zero
   - `continue`: print `line N: ...` to `stderr`, increment `skipped_lines` and `error_count`, continue

Why: the summary and diagnostics stay derivable from counters; there is no need to retain full error arrays in memory.

### Pattern 4: two-pass RG sizing

Pass 1:

- local file: count raw lines
- stdin: copy to a temp file while counting raw lines

Pass 2:

- create `GINBuilder` with `max(1, ceil(rawLineCount/rgSize))`
- perform real ingest and stop early when `--sample` has reached `N` successful documents

Why: `GINBuilder` needs row-group capacity before ingest begins, and temp-file spooling is the only bounded-memory way to satisfy that requirement for stdin.

## Validation Architecture

| Property | Value |
|----------|-------|
| **Framework** | `go test` |
| **Config file** | none |
| **Quick run command** | `go test ./cmd/gin-index -run 'Test(RunExperiment|Experiment)' -count=1` |
| **Full suite command** | `go test ./...` |
| **Estimated runtime** | quick: ~5s, full: ~45s |

Validation gates the plan should enforce:

- large-line ingest test (`>64 KiB`) proves no default scanner truncation
- stdin test proves `-` support without shelling out
- JSON golden test locks schema and field names
- continue/abort tests lock stderr line diagnostics and summary counters
- sidecar roundtrip test proves output is readable
- policy test proves no forbidden CLI deps or TUI/colour imports appear

## Recommended Plan Split

### Plan 01 — Core command, streaming ingest, text summary

Deliver the new dispatcher entry, stdin-aware runner, line-by-line ingest, `--rg-size` defaulting, and the base text report.

### Plan 02 — JSON output, predicate test, sidecar write, log-level wiring

Deliver one stable report schema, `--test`, `-o`, and the Phase 14 logger integration that writes only to `stderr`.

### Plan 03 — Sample / error-tolerance modes and policy guards

Deliver `--sample`, `--on-error continue|abort`, the final summary counters, and the charter guard that prevents dependency/UI drift.

## Open Edge Cases To Lock During Execution

- input path exists but is a directory: fail with a direct error, do not glob
- `--rg-size <= 0`: reject before any ingest begins
- `--sample < 0`: reject before any ingest begins
- `--on-error` outside `continue|abort`: reject with flag validation error
- stdin requires temp-file spooling for the counting pass because the builder needs `numRGs` before the real ingest pass begins
- empty input after all filtering: still print a coherent zero-document report rather than panic in `Finalize()`
- local file input should preserve source permissions for `-o`; stdin should fall back to `0o600`
- `--log-level debug` may currently emit the same events as `info`, because the library presently logs only info/warn; the command should still preserve the distinct flag value and adapter level
