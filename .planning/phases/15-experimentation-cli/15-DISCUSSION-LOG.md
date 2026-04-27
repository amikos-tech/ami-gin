# Phase 15: Experimentation CLI - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in `15-CONTEXT.md` — this log preserves the alternatives considered.

**Date:** 2026-04-22
**Phase:** 15-experimentation-cli
**Areas discussed:** Row-group modeling, Failure handling, Experiment report, Predicate tester depth, Phase-13/14 flag surface

---

## Row-Group Modeling

| Option | Description | Selected |
|--------|-------------|----------|
| RGs stay internal, default to 1 | Frictionless default; effectively row-level pruning on first use while preserving the existing RG-based contract. | ✓ |
| RGs stay internal, default to a larger fixed size | Better grouped-pruning realism, but imposes a more opinionated default. | |
| RGs stay internal, no default, caller must choose | Most explicit, but adds startup friction to the one-command experiment flow. | |

**User's choice:** Keep row groups internal and default `rg-size` to `1`, with an override for realism.
**Notes:** The user questioned whether row groups should matter at all for the command, pushed toward row-level behavior as the default, and asked to capture the broader positioning change as a separate future item rather than force it into Phase 15.

---

## Failure Handling

| Option | Description | Selected |
|--------|-------------|----------|
| Per-line stderr + final summary | Print `line N` diagnostics to `stderr` and still summarize processed/skipped/error totals at the end. | ✓ |
| Summary-first, minimal stderr | Keep continue mode quieter and rely more heavily on the final summary. | |
| Structured JSON errors in `--json` mode | Include machine-readable error details in JSON output as a broader output surface. | |

**User's choice:** Per-line diagnostics in continue mode, plus final summary counts.
**Notes:** The user also asked for a sensible default with explicit override. The phase keeps `abort` as the default when `--on-error` is omitted.

---

## Experiment Report

| Option | Description | Selected |
|--------|-------------|----------|
| Run summary + path table + optional predicate block | Experiment mode shows run-level context first, then the path summary, then predicate results when present. | ✓ |
| Path table only | Closest to current `info`, but loses experiment-specific run context. | |
| Summary only unless `--json` | Keeps text output lean, but makes the human mode much less useful. | |

**User's choice:** Run summary first, path table second, optional predicate block last.
**Notes:** This keeps the command aligned with the “one command, useful answer” goal instead of feeling like a lightly renamed `info`.

---

## Predicate Tester Depth

| Option | Description | Selected |
|--------|-------------|----------|
| Counts only | Show `matched`, `pruned`, and `pruning_ratio` without ID lists. | ✓ |
| Counts + matching RG IDs | Add kept RG IDs for small datasets. | |
| Counts + matching/pruned RG IDs | Turn the output into a broader diagnostic/debug surface. | |

**User's choice:** Counts only.
**Notes:** This keeps `experiment` compact, especially because the chosen default `rg-size=1` would make ID-list output noisy very quickly.

---

## Phase-13/14 Flag Surface

| Option | Description | Selected |
|--------|-------------|----------|
| Expose `--log-level` now, defer `--parser` | Use the live observability seam now, but wait on parser-choice UX until Phase 16 gives users a real non-stdlib option. | ✓ |
| Expose both `--log-level` and `--parser` now | Future-facing, but exposes a partly empty parser-choice surface today. | |
| Expose neither in Phase 15 | Smallest surface, but leaves current seam value mostly unused. | |

**User's choice:** Expose `--log-level` now and defer `--parser`.
**Notes:** The user agreed that log level is a real current capability, while parser choice is not yet meaningful enough to deserve a public flag.

---

## the agent's Discretion

- Exact run-summary headings and field order.
- Exact JSON schema layout, as long as it remains stable and covers the locked summary/predicate data.
- Exact backend wiring behind `--log-level`, while preserving the default silent behavior.

## Deferred Ideas

- Phase `999.6`: Row-level pruning messaging and positioning, so users are told clearly that the library is not only for Parquet-style row-group pruning and can also be used in `rg=1` row-level mode.
- Removing row groups from the experiment command entirely, which is outside Phase 15 because the command and library semantics are still expressed in row-group terms.
- Exposing `--parser` before Phase 16 makes that a real user choice.
