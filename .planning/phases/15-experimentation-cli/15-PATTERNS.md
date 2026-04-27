# Phase 15: Experimentation CLI - Pattern Map

**Mapped:** 2026-04-22
**Files analyzed:** 6 planned touchpoints
**Analogs found:** 6 / 6

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `cmd/gin-index/main.go` (MODIFIED) | dispatcher + usage text | request-response | `cmd/gin-index/main.go` self-modify | exact |
| `cmd/gin-index/experiment.go` (NEW) | command entry, flag parsing, ingest orchestration | streaming | `runBuild` / `runInfo` in `cmd/gin-index/main.go` | exact shape, new stdin arg |
| `cmd/gin-index/experiment_output.go` (NEW) | report structs, text/JSON rendering, path-row collection | batch | `writeIndexInfo` / `formatPathInfo` in `cmd/gin-index/main.go` | exact with richer report model |
| `cmd/gin-index/experiment_test.go` (NEW) | command E2E tests | request-response + file I/O | `cmd/gin-index/main_test.go` | exact |
| `cmd/gin-index/experiment_policy_test.go` (NEW) | charter/dependency guard tests | static inspection | `observability_policy_test.go` | exact role |
| `cmd/gin-index/main_test.go` (MODIFIED) | parse-error helper expansion | subprocess / CLI | `TestRunParseErrorHelper` in `cmd/gin-index/main_test.go` | exact |

## Pattern Assignments

### `cmd/gin-index/main.go`

**Role:** register the new `experiment` subcommand and keep usage text coherent.

**Primary analog:** `cmd/gin-index/main.go` itself

Preserve:

- top-level `switch cmd`
- one wrapper per subcommand (`cmdBuild`, `cmdQuery`, `cmdInfo`, `cmdExtract`)
- usage text grouped by command examples

Required adaptation:

- add `cmdExperiment(args)` wrapper
- mention JSONL file and stdin examples
- keep diagnostics on `stderr`, user-facing results on `stdout`

### `cmd/gin-index/experiment.go`

**Role:** parse flags, open the input source, stream JSONL, build the index, and hand a fully populated report to renderers.

**Primary analogs:** `runBuild(...)` and `runInfo(...)` in `cmd/gin-index/main.go`

Carry over:

- `flag.NewFlagSet(..., flag.ContinueOnError)`
- explicit positional-arg validation
- helpers that return exit codes instead of calling `os.Exit`
- reuse of `writeLocalIndexFile(...)` and `localOutputMode(...)` for output writes

Required adaptation:

- accept an explicit `stdin io.Reader` parameter
- use a two-pass strategy because `GINBuilder` needs `numRGs` up front:
  - local files: count lines, then build
  - stdin: spool to temp file while counting, then build from the temp file
- use `bufio.Reader.ReadBytes('\n')` instead of path glob / parquet iteration
- derive synthetic `rgID` from ingested document count and `--rg-size`

### `cmd/gin-index/experiment_output.go`

**Role:** convert a built index plus run metadata into text and JSON output.

**Primary analog:** `writeIndexInfo(...)` / `formatPathInfo(...)`

Preserve:

- path rows come from existing path metadata, not custom ad hoc maps
- adaptive-hybrid reporting stays consistent with `formatPathInfo(...)`
- internal representation paths stay hidden

Required adaptation:

- introduce a first-class report struct so text mode and JSON mode share one data model
- compute an estimated bloom occupancy field for each path from `PathEntry.Cardinality` and the global bloom parameters
- add an optional predicate-result block after the path table, never before it

### `cmd/gin-index/experiment_test.go`

**Role:** cover end-to-end command behavior without shelling out for normal cases.

**Primary analog:** `cmd/gin-index/main_test.go`

Preserve:

- direct `runX(...)` testing with `bytes.Buffer`
- temp files from `t.TempDir()`
- table-driven command-failure tests

Required additions:

- helper to write JSONL fixtures directly
- stdin-backed tests by passing a `strings.Reader` / `bytes.Reader`
- a >64 KiB line fixture built in-memory rather than committed to git

### `cmd/gin-index/experiment_policy_test.go`

**Role:** keep the phase charter executable.

**Primary analog:** `observability_policy_test.go`

Preserve:

- codebase inspection via file reads / grep-like checks in Go tests
- no reliance on documentation alone

Recommended guards:

- `go.mod` must not gain CLI frameworks or colour/TUI deps
- `cmd/gin-index/*.go` must not import forbidden packages such as `cobra`, `urfave/cli`, `fatih/color`, `bubbletea`, `lipgloss`, `readline`
- `cmd/gin-index/*.go` must not reference TTY detection helpers

### `cmd/gin-index/main_test.go`

**Role:** extend the existing subprocess parse-failure helper to include `experiment`.

**Primary analog:** `TestRunParseErrorHelper`

Required adaptation:

- add a new `experiment` branch in the helper switch
- pass a harmless stdin reader when the helper invokes `runExperiment(...)`
- extend `TestRunCommandsReturnParseFailureCode` to cover the new subcommand

## Implementation Notes That Should Shape Planning

- `runExperiment(...)` should be the only command in the CLI that accepts an explicit stdin reader; no other command currently needs this.
- `gin.BuildFromParquet(...)` is not a usable analog for JSONL ingest beyond the "one builder, many documents" pattern. The experiment command should not round-trip through Parquet helpers.
- `gin.ReadSidecar(...)` derives `parquetFile + ".gin"`, so arbitrary `-o path.gin` roundtrip tests should either decode the output bytes directly or copy the produced file to a temporary sidecar-compatible basename before calling `gin.ReadSidecar(...)`.
- `logging/slogadapter` is the best fit for `--log-level off|info|debug` because it preserves level filtering; `logging/stdadapter` intentionally drops debug.
- The phase should prefer new focused test files over continuing to grow `cmd/gin-index/main_test.go`, except for the shared parse-error helper that already lives there.
