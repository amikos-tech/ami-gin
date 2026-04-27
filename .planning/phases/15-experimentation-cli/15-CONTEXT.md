# Phase 15: Experimentation CLI - Context

**Gathered:** 2026-04-22
**Status:** Ready for planning

<domain>
## Phase Boundary

Add a new `gin-index experiment` subcommand that accepts JSONL from a file path or `-`, builds an index in bounded memory, emits a human-readable or JSON-readable per-path summary, optionally writes a `.gin` sidecar, and can run one inline predicate test.

This phase stays within the existing pruning-first library contract. It does not add a REPL, TUI, color output, a new query DSL, or broader product-positioning changes about how the library should be described outside the command itself.

**Carrying forward from earlier phases:**
- Phase 13 established the parser seam, but Phase 15 should not expose a user-facing parser-choice surface until there is a meaningful non-stdlib option to select.
- Phase 14 established the logging/telemetry seam and silent-by-default behavior, so Phase 15 may expose a CLI-facing `--log-level` flag without leaking backend types.
- The CLI remains stdlib-first and dependency-light: keep using `flag`, current output conventions, and existing helper paths rather than introducing a framework or interactive terminal surface.

</domain>

<decisions>
## Implementation Decisions

### Row-Group Modeling
- **D-01:** `experiment` keeps row groups as an internal modeling concept rather than removing them from the command entirely.
- **D-02:** Default `--rg-size` to `1`, so the out-of-the-box experience behaves like row-level pruning and does not force callers to think in Parquet-sized chunks before they can learn from the command.
- **D-03:** Keep `--rg-size N` as an advanced override for users who want grouped-pruning realism on larger datasets.
- **D-04:** Predicate-test output still reports row-group counts and ratios because the library contract and the locked Phase 15 success criteria are still expressed in RG terms, even when the default is effectively row-level pruning.

### Failure Handling
- **D-05:** Omitted `--on-error` means `abort`.
- **D-06:** `--on-error continue` prints per-line diagnostics to `stderr` and keeps ingesting subsequent lines.
- **D-07:** The final run summary includes processed, skipped, and error totals when error-tolerant mode is used.

### Report Shape
- **D-08:** Default human-readable output is: run summary first, per-path table second, and an optional predicate-result block last when `--test` is set.
- **D-09:** The per-path table should reuse the existing `info`-style rendering logic instead of inventing a parallel reporting surface.
- **D-10:** `--json` stays additive and machine-oriented; it does not replace the default text-mode experience.

### Predicate Tester
- **D-11:** Text-mode `--test` output stays count-focused: `matched`, `pruned`, and `pruning_ratio`.
- **D-12:** Phase 15 does not print matching or pruned RG ID lists by default; that would turn `experiment` into a heavier diagnostic/debugging surface than this phase intends to ship.

### CLI Surface From Prior Seams
- **D-13:** Expose `--log-level off|info|debug` in Phase 15 because the observability seam is already real and useful in current builds.
- **D-14:** Do not expose a user-facing `--parser` flag in Phase 15. The only meaningful parser path today is the stdlib default, and exposing a partly empty parser-choice surface now would get ahead of Phase 16.

### the agent's Discretion
- Exact text-mode field order and headings inside the run summary block.
- Exact JSON schema layout, as long as it is stable, testable, and includes the locked summary and predicate-result data.
- Exact logger wiring behind `--log-level`, as long as the default remains silent and no backend-specific types leak into the public CLI contract.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase Scope And Constraints
- `.planning/ROADMAP.md` §Phase 15 — Goal, success criteria, and the dependency on the Phase 13 parser seam and Phase 14 observability seam.
- `.planning/REQUIREMENTS.md` §Experimentation CLI — `CLI-01` through `CLI-08`.
- `.planning/PROJECT.md` §Current Milestone / Constraints — Pruning-first scope, additive-change bias, and no-gratuitous-dependency churn.
- `.planning/STATE.md` §Current Focus / Accumulated Context — Confirms Phase 15 is next and carries forward the “JSONL-in, summary-out” charter.

### Prior Phase Constraints
- `.planning/phases/13-parser-seam-extraction/13-CONTEXT.md` — Parser seam exists, exported parser surface stays intentionally narrow, and parser choice should not be broadened casually.
- `.planning/phases/14-observability-seams/14-CONTEXT.md` — Silent-by-default logging/telemetry seam and CLI-owned observability bootstrap/log-level direction.

### Research Guidance
- `.planning/research/SUMMARY.md` §Theme 3 / §Phase D — Recommended `experiment` shape, stdlib `flag` usage, and no REPL/TUI/color scope.
- `.planning/research/FEATURES.md` §Theme 3 — Synthetic RG modeling, per-path summary, sample mode, JSON mode, and inline predicate tester options.
- `.planning/research/PITFALLS.md` §Pitfall 8 / §Pitfall 9 / §Pitfall 10 — CLI charter, large-line streaming guidance, and no ANSI/color leakage.
- `.planning/research/ARCHITECTURE.md` §Pattern 3 / §Phase D — Concrete integration points for a new `experiment` subcommand consuming existing seams.

### Current Code Anchors
- `cmd/gin-index/main.go` — Existing command dispatcher, `flag.NewFlagSet` conventions, stdout/stderr split, `writeIndexInfo`, `formatPathInfo`, and `parsePredicate`.
- `cmd/gin-index/main_test.go` — Current CLI testing patterns for `runX(...)`, parse-predicate coverage, and output assertions.
- `gin.go` — `WithLogger`, `WithSignals`, and silent default config behavior that Phase 15's `--log-level` surface can build on.
- `logging/slogadapter/slog.go` and `logging/stdadapter/std.go` — Existing adapter surfaces available for CLI-owned logging integration.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`writeIndexInfo` / `formatPathInfo`** in `cmd/gin-index/main.go`: already render the path summary shape Phase 15 wants; planner should extend or wrap them instead of cloning a second formatter.
- **`parsePredicate`** in `cmd/gin-index/main.go`: already supports the operator family the experiment predicate tester needs.
- **`runBuild` / `runQuery` / `runInfo` / `runExtract`** patterns: existing subcommands already separate `runX(args, stdout, stderr)` from `cmdX(...)`, which keeps CLI behavior easy to test.
- **Local artifact helpers** such as `writeLocalIndexFile`: sidecar writing should reuse the existing file-output conventions and permission handling.

### Established Patterns
- **Subcommand structure:** each CLI command uses `flag.NewFlagSet`, validates positional args explicitly, and reports usage/parse failures in a consistent style.
- **Output discipline:** user-facing results go to `stdout`; diagnostics and errors go to `stderr`.
- **Dependency discipline:** the CLI stays on stdlib patterns instead of moving to `cobra`/`urfave/cli` or adding UI-oriented packages.
- **Test style:** `cmd/gin-index/main_test.go` favors direct `runX(...)` coverage and focused helper subprocesses for parse-failure exits.

### Integration Points
- **Dispatcher and usage text:** register `experiment` in `cmd/gin-index/main.go` alongside `build`, `query`, `info`, and `extract`.
- **New command implementation:** a dedicated `cmd/gin-index/experiment.go` file is the expected integration shape from the research docs.
- **Index construction path:** JSONL ingest should build through `gin.NewBuilder`, `AddDocument`, and `Finalize`, with RG assignment derived from the chosen `--rg-size`.
- **Predicate test path:** `--test` should reuse `parsePredicate` and `idx.Evaluate(...)`/`EvaluateContext(...)` rather than introducing another parser or evaluation branch.
- **Observability path:** `--log-level` should wire the existing logging seam while preserving the default silent behavior from Phase 14.

</code_context>

<specifics>
## Specific Ideas

- The default experience should feel like one command for “show me what the index would do with this dataset,” not like a Parquet-oriented tuning surface.
- `rg-size=1` is the preferred default because it makes the first-run mental model effectively row-level while still staying within the library’s existing RG-based contract.
- The user wants the broader product messaging captured for future work: GIN Index should be explainable as useful for both grouped pruning and row-level pruning via `rg=1`, not only as a Parquet row-group tool.
- `--on-error` should have a conservative default with explicit override, rather than silently tolerating malformed input unless the caller opts in.

</specifics>

<deferred>
## Deferred Ideas

- **Phase 999.6: Row-Level Pruning Messaging And Positioning** — Clarify in README/docs/CLI guidance that the library supports both grouped pruning and row-level pruning when callers choose `rg=1`.
- **Scrapping row groups from the experiment command entirely** — Deferred because it would conflict with the current library contract and the locked Phase 15 success criteria, which are still framed in RG counts and pruning ratios.
- **Expose `--parser` before SIMD is a real choice** — Deferred until Phase 16 or later, when parser selection becomes more than a stdlib-only surface.

</deferred>

---

*Phase: 15-experimentation-cli*
*Context gathered: 2026-04-22*
