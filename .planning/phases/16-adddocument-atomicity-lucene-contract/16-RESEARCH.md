# Phase 16: AddDocument Atomicity (Lucene contract) - Research

**Researched:** 2026-04-23 [VERIFIED: `date +%F`]
**Domain:** Go builder ingest atomicity, per-document failure isolation, merge-layer invariant enforcement [VERIFIED: `.planning/REQUIREMENTS.md`; VERIFIED: `builder.go` audit]
**Confidence:** HIGH for current code/failure-site audit; MEDIUM for recommended plan sizing because it depends on implementation diff size [VERIFIED: code audit; ASSUMED]

<user_constraints>
## User Constraints (from CONTEXT.md)

**Source:** Copied from `.planning/phases/16-adddocument-atomicity-lucene-contract/16-CONTEXT.md`; statements in this section inherit that provenance. [VERIFIED: `16-CONTEXT.md`]

### Locked Decisions

#### Validator Simulation Strategy
- **D-01:** Extend the existing **shadow-fields pattern on `stagedPathData`** (today's `numericSim*` fields seeded by `seedNumericSimulation` at `builder.go:671`). As the merge-hoisting catalog grows, add new `*Sim*` fields onto `stagedPathData`; the staging functions transparently re-seed from `b.pathData` on first touch. No typed `pathPreview` clone, no `commit-bool` shared helper. Keep the mutation site obvious.

#### Failure-Mode Catalogs (two catalogs, deliberately separate)

Following Lucene's `IndexWriter` structure (per-document exceptions vs `tragicEvent` — different routing, different assurance bars).

- **D-02 — Merge-hoisting catalog (narrow):** exactly ONE failure mode reaches merge today and therefore needs hoisting into `validateStagedPaths`:
  - `"unsupported mixed numeric promotion at %s"` — fires at `mergeNumericObservation:839` and `promoteNumericPathToFloat:860,867`.
  - All other ingest errors (parser, stage, companion-transformer, JSON literal parse, unsupported type) already fire **before** merge against throwaway state (`currentDocState` reset via `defer`), so they are atomic by construction and do NOT need validator hoisting.
  - The `// MUST_BE_CHECKED_BY_VALIDATOR` marker is placed on `mergeStagedPaths`, `mergeNumericObservation`, and `promoteNumericPathToFloat` only.

- **D-03 — `tragicErr`-stays-nil catalog (broad):** the ATOMIC-03 unit test iterates the **full public failure-mode catalog** and asserts `tragicErr` stays nil throughout:
  1. Parser errors (malformed JSON, non-UTF8, trailing bytes)
  2. Unsupported JSON token type (`stageScalarToken:436`, `stageMaterializedValue:514`)
  3. Hard-mode companion transformer rejection (`stageCompanionRepresentations:531`)
  4. Malformed numeric literal (`stageJSONNumberLiteral:544`)
  5. Non-finite numeric (`parseJSONNumberLiteral:560`, `stagedNumericFromValue:586`)
  6. Validator-rejected numeric promotion (post-hoist)
  7. Pre-parser gate errors: `position >= numRGs` (`AddDocument:312`), `BeginDocument` not called / called >1x (`AddDocument:331,333`), `rgID` mismatch (`AddDocument:341`)
  8. Unsupported `uint`/`uint64` > `MaxInt64` (`stageMaterializedValue:482,493`)

#### Contract Enforcement (composed)
- **D-04 — Mechanism composes four defenses:**
  1. **Compile-time signature change** — `mergeStagedPaths`, `mergeNumericObservation`, `promoteNumericPathToFloat` drop their `error` return. Go compiler rejects any future reintroduction.
  2. **`// MUST_BE_CHECKED_BY_VALIDATOR` comment marker** — placed on each of the three merge-layer functions above.
  3. **CI grep** — `make lint` (or equivalent) asserts: any function whose signature is preceded by `// MUST_BE_CHECKED_BY_VALIDATOR` MUST NOT contain `error` in its return types. Implemented as a shell/grep check in the Makefile lint target, not a golangci-lint custom rule.
  4. **`recover()` belt-and-suspenders** — wraps **`mergeStagedPaths` only** (narrowest scope that still catches all merge-reachable panics). Not `mergeDocumentState`, not `AddDocument`. Factored into a testable helper `runMergeWithRecover(func())` so the recovery path has a direct unit test (see D-06).

#### Tragic Observability (Phase 14 seam integration)
- **D-07 — Logger emission at the recover site:** when `recover()` converts a panic to `tragicErr`, emit exactly ONE log event at Error level through the existing `gin.WithLogger` seam: `Logger.Error("builder tragic: recovered panic in merge", attrs={...})`. Silent for default users (Phase 14's silent-default); visible to callers who wired slog/stdlib. No Telemetry counter, no Signals emission — Logger only. Phase 18 can extend.

#### Error-Message Wording
- **D-09 — Mirror today's exact wording.** The hoisted numeric-promotion failure returns the same string the merge layer produces today: `"unsupported mixed numeric promotion at %s"` (and the variant from `promoteNumericPathToFloat`). No restructuring, no layer-prefix. Phase 18 owns the structured migration to `IngestError` with a `Layer=numeric` field; Phase 16 keeps wording stable to minimize that diff and avoid test-assertion churn.

#### Test Infrastructure
- **D-05 — Atomicity property test shape:**
  - File: `atomicity_test.go` (new, at repo root).
  - **Encode-determinism sanity test (prerequisite):** build the same clean corpus twice, `Encode` both, assert `bytes.Equal`. This fires FIRST so any encode non-determinism surfaces as a test failure of its own, not as a confusing atomicity-violation failure.
  - **Typed failure-intent gopter generators** (new generator family alongside existing `property_test.go` ones):
    - `genParserMalformedDoc` — produces raw bytes guaranteed to fail `parser.Parse` (non-UTF8, truncated JSON, trailing-content varieties).
    - `genHardTransformerRejectingDoc` — pairs with a registered hard-mode transformer and a value the transformer will reject.
    - `genNumericPromotionFailingDoc` — produces a doc whose numeric observation will cause `!canRepresentIntAsExactFloat` at merge/validator time.
  - **Corpus:** gopter `gen.SliceOfN(≥1000, …)` interleaving clean docs and the three failure-intent generators; verify ≥10% failing docs per generated corpus.
  - **Assertion:** `Encode(builderWithFullCorpus)` `bytes.Equal` `Encode(builderWithCleanSubsetOnly)`.

- **D-06 — Existing-test migration:**
  - **Gate test:** `gin_test.go:435` — rename field access `builder.poisonErr` → `builder.tragicErr`, update the asserted substring `"builder poisoned"` → new tragic wording (e.g., `"builder closed by prior tragic failure"`); keep the test's field-level unit-test shape.
  - **Factor `runMergeWithRecover(fn func()) error`** as a package-level helper that runs `fn` under `defer recover()` and returns a `tragicErr`-shaped error if `fn` panics.
  - **Trigger test (new):** directly invoke `runMergeWithRecover` with a closure that `panic("simulated")`; assert the returned error has tragic wording and that a subsequent `AddDocument` on the same builder is refused by the gate.
  - **End-to-end refusal test (new):** go through a real merge path that panics (e.g., using the helper); assert subsequent `AddDocument` refused.

#### Pre-existing Exceptions (documented, not decisions)
- **`walkJSON` at `builder.go:358`** is test-only internal (called only from `gin_test.go:2366` and `phase07_review_test.go:16`). It bypasses `validateStagedPaths` by design. Never reachable from user input. **No validator gate needed**; Phase 16 does not modify it. Document its role so reviewers don't flag it as a missed site.

### Claude's Discretion
- Exact wording of the new `tragicErr` wrap message in `AddDocument:305` (replacing today's `"builder poisoned by prior merge failure; discard and rebuild"`). Must remain Go-idiomatic, informative, and stable for this phase's tests.
- Exact structure of the log attributes on the `Logger.Error` emission (which fields beyond `panic_value` to include).
- Naming of the `runMergeWithRecover` helper (equivalent names acceptable).
- Exact CI grep invocation (awk/grep/rg all acceptable), as long as it fails `make lint` on reintroduction of `error` returns after the marker.

### Deferred Ideas (OUT OF SCOPE)
- **Structured `IngestError` type with `Path`/`Layer`/`Cause`/`Value` fields** — Phase 18 (IERR-01..03).
- **Unified `IngestFailureMode` (`Hard`/`Soft`) across parser / transformer / numeric layers** — Phase 17 (FAIL-01..02).
- **Telemetry counter for tragic events** (`gin_builder_tragic_total` or similar) — considered and deferred. Phase 17/18 can revisit with a structured signal.
- **Signals seam emission on tragic** — same rationale; deferred with Telemetry.
- **Layer-prefix on error wording (`"numeric: …"`)** — considered as a Phase 18 migration smoother; rejected in favor of Phase 18 designing the structured shape deliberately.
- **File-level "no error returns" convention with merge functions moved to `builder_merge.go`** — considered; rejected in favor of function-level marker to minimize diff in this phase.
- **`recover()` wrapping `mergeDocumentState` (broader net)** — considered; rejected in favor of the narrow `mergeStagedPaths`-only scope specified by ATOMIC-03.
- **`ValidateDocument` dry-run public API** — already deferred at the milestone level (PROJECT.md §Out of Scope).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| ATOMIC-01 | `AddDocument` non-tragic errors leave the builder indistinguishable from skipping the failed call, verified by byte-identical `Encode` against a clean subset. [VERIFIED: `.planning/REQUIREMENTS.md`] | Use `atomicity_test.go` with gopter `SliceOfN(1000, ...)`, deterministic clean-corpus sanity check, and full-vs-clean `Encode` byte comparison. [VERIFIED: `go doc github.com/leanovate/gopter/gen.SliceOfN`; VERIFIED: `serialize.go:211-218`; VERIFIED: `16-CONTEXT.md`] |
| ATOMIC-02 | `mergeStagedPaths` and `mergeNumericObservation` become infallible, with `validateStagedPaths` simulating merge failures against real `pathData`. [VERIFIED: `.planning/REQUIREMENTS.md`] | Current simulator path is `validateStagedPaths` -> `stageNumericObservation` -> `seedNumericSimulation`, and `seedNumericSimulation` reads `b.pathData` for existing numeric state. [VERIFIED: `builder.go:724-740`; VERIFIED: `builder.go:597-691`] |
| ATOMIC-03 | `tragicErr` replaces `poisonErr`, is not set by user-input failures, and `recover()` in merge converts reachable panic to tragic failure. [VERIFIED: `.planning/REQUIREMENTS.md`] | Current tragic predecessor is `poisonErr` at `builder.go:34`, current gate wraps it at `builder.go:305-307`, and current merge failure assignment is `builder.go:701-708`. [VERIFIED: `builder.go:30-34`; VERIFIED: `builder.go:304-308`; VERIFIED: `builder.go:697-709`] |
</phase_requirements>

## Summary

Phase 16 should be planned as a contained internal refactor of `builder.go`, plus tests and CI enforcement, not as a new transaction layer. [VERIFIED: `16-CONTEXT.md`; VERIFIED: `builder.go` audit] The current ingest path already stages parser and transformer work in `documentBuildState`, resets parser handoff state with a `defer`, validates staged numeric observations, and only mutates shared index state in `mergeStagedPaths`. [VERIFIED: `builder.go:316-348`; VERIFIED: `builder.go:697-779`]

The one user-reachable merge failure category still in scope is unsupported mixed numeric promotion, and the validator already uses the right shadow-field mechanism but needs to become the complete gate for the merge signatures. [VERIFIED: `builder.go:597-691`; VERIFIED: `builder.go:724-740`; VERIFIED: `builder.go:799-880`; VERIFIED: `16-CONTEXT.md`] Lucene is the reference because its `IndexWriter.addDocument` documents a consistent-index-after-exception contract, while separate tragic-exception APIs report unrecoverable writer failures. [CITED: https://lucene.apache.org/core/10_1_0/core/org/apache/lucene/index/IndexWriter.html]

**Primary recommendation:** Implement in four dependent slices: validator/merge signature refactor, `poisonErr`→`tragicErr` plus recovery/logging, atomicity/failure-catalog property tests, then Makefile and GitHub workflow enforcement. [VERIFIED: code audit; VERIFIED: `.github/workflows/ci.yml:56-72`; VERIFIED: `16-CONTEXT.md`]

## Project Constraints (from CLAUDE.md and AGENTS.md)

- Preserve the public `AddDocument` and `Finalize` API surface for this phase. [VERIFIED: `16-CONTEXT.md`; VERIFIED: `.planning/PROJECT.md`]
- Use `go build ./...`, `go test -v` or focused `go test -v -run ...`, and existing Makefile targets for verification. [VERIFIED: `CLAUDE.md`; VERIFIED: `Makefile:3-28`]
- Use `github.com/pkg/errors` for new error creation/wrapping instead of `fmt.Errorf` wrapping. [VERIFIED: `CLAUDE.md`; VERIFIED: `builder.go` existing imports]
- Keep observability silent by default and route any tragic recovery log through the existing `Logger` seam. [VERIFIED: `16-CONTEXT.md`; VERIFIED: `gin.go:655-683`; VERIFIED: `logging/logger.go:42-45`]
- Do not introduce new public error types or failure-mode taxonomy in Phase 16; those are deferred to Phases 17 and 18. [VERIFIED: `.planning/REQUIREMENTS.md`; VERIFIED: `16-CONTEXT.md`]
- Do not include private repository host or company-identifying repository details in commits, PR text, or generated artifacts. [VERIFIED: `AGENTS.md`]
- Required Makefile targets currently exist, but `lint` only runs `golangci-lint run` today. [VERIFIED: `Makefile:25-28`]

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|--------------|----------------|-----------|
| Per-document ingest atomicity | API / Backend library | Database / Storage state | `AddDocument` is the public library entry point, while the atomicity invariant is about shared builder maps, bitmaps, bloom filter, and numeric stats. [VERIFIED: `builder.go:304-348`; VERIFIED: `builder.go:743-880`] |
| Numeric promotion validation | API / Backend library | Database / Storage state | `validateStagedPaths` must simulate against existing `pathData` numeric state before merge mutates storage structures. [VERIFIED: `builder.go:724-740`; VERIFIED: `builder.go:671-691`] |
| Tragic failure handling | API / Backend library | Observability seam | The builder gate owns refusal after tragedy, and the logger seam owns opt-in visibility for recovered panics. [VERIFIED: `builder.go:305-307`; VERIFIED: `gin.go:370`; VERIFIED: `logging/logger.go:42-45`] |
| CI invariant enforcement | CI / Static tooling | Makefile | Local `make lint` can host the grep, but GitHub CI currently calls the golangci action directly, so CI must also run the grep or call `make lint`. [VERIFIED: `Makefile:25-28`; VERIFIED: `.github/workflows/ci.yml:56-72`] |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go toolchain | Module declares `go 1.25.5`; local tool is `go1.26.2`; CI matrix is Go `1.25` and `1.26`. [VERIFIED: `go.mod`; VERIFIED: `go version`; VERIFIED: `.github/workflows/ci.yml:20-36`] | Build, test, and compiler-enforce merge signature changes. [VERIFIED: `Makefile:3-20`] | The project is a standalone Go module and CI already validates the supported toolchain range. [VERIFIED: `go.mod`; VERIFIED: `.github/workflows/ci.yml`] |
| `github.com/leanovate/gopter` | `v0.2.11`, published `2024-04-03T11:45:41Z`. [VERIFIED: `go list -m -json github.com/leanovate/gopter`] | Property-based test corpus for atomicity. [VERIFIED: `property_test.go:14-40`; VERIFIED: `go doc github.com/leanovate/gopter/prop.ForAll`] | Existing property helpers already standardize 1000 successful tests in normal mode and 100 in short mode. [VERIFIED: `property_test.go:14-40`] |
| `github.com/pkg/errors` | `v0.9.1`, published `2020-01-14T19:47:44Z`. [VERIFIED: `go list -m -json github.com/pkg/errors`] | Error creation and wrapping for validator and tragic gate messages. [VERIFIED: `builder.go:12-14`; VERIFIED: `CLAUDE.md`] | Repository convention requires it for stack-capturing errors and forbids migrating this phase to `fmt.Errorf` wrapping. [VERIFIED: `CLAUDE.md`] |
| Internal `logging` package | In-repo package, no external version. [VERIFIED: `logging/logger.go`] | Error-level tragic recovery log. [VERIFIED: `16-CONTEXT.md`; VERIFIED: `logging/logger.go:42-45`] | Phase 14 already installed silent defaults and backend-neutral adapters. [VERIFIED: `gin.go:655-683`; VERIFIED: `.planning/phases/14-observability-seams/14-CONTEXT.md`] |

### Supporting

| Tool | Version | Purpose | When to Use |
|------|---------|---------|-------------|
| `gotestsum` | Local `v1.13.0`; Makefile also installs `v1.13.0`. [VERIFIED: `gotestsum --version`; VERIFIED: `Makefile:1-20`] | Full suite and CI-like local runs. [VERIFIED: `Makefile:11-20`; VERIFIED: `.github/workflows/ci.yml:32-36`] | Use for phase gate/full suite; focused `go test` is acceptable during implementation. [VERIFIED: `Makefile`; VERIFIED: `ROADMAP.md`] |
| `golangci-lint` | Local `2.11.4`; CI action pins `v2.11.4`. [VERIFIED: `golangci-lint --version`; VERIFIED: `.github/workflows/ci.yml:68-72`] | Existing lint gate. [VERIFIED: `.golangci.yml`] | Keep for normal linting; add a separate grep because this is a signature policy, not an existing linter rule. [VERIFIED: `16-CONTEXT.md`; VERIFIED: `.golangci.yml`] |
| `ripgrep` (`rg`) | Local `15.1.0`. [VERIFIED: `rg --version`] | Marker/signature grep and artifact audits. [VERIFIED: environment audit] | Use if available; POSIX `grep`/`awk` fallback is acceptable for CI portability. [ASSUMED] |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Validate-before-mutate | Snapshot/restore around `AddDocument` | Snapshot/restore is explicitly deferred and would duplicate state-copy complexity across maps, bitmaps, bloom filter, HLL, trigrams, and docID bookkeeping. [VERIFIED: `.planning/REQUIREMENTS.md`; VERIFIED: `16-CONTEXT.md`; VERIFIED: `builder.go` audit] |
| Function marker + grep | Custom `go/analysis` linter | Context locks a shell/grep check, and a custom analyzer would add scope for little gain in this phase. [VERIFIED: `16-CONTEXT.md`] |
| New structured `IngestError` | Exported error type now | Structured ingest errors are Phase 18 scope, so introducing them now would violate the phase boundary. [VERIFIED: `.planning/REQUIREMENTS.md`; VERIFIED: `16-CONTEXT.md`] |

**Installation:** No new dependency install is required for implementation; all required Go modules are already in `go.mod`. [VERIFIED: `go.mod`; VERIFIED: `16-CONTEXT.md`]

## Current Code Audit

### AddDocument Flow

| Step | Function / Lines | Shared State Mutated? | Failure Behavior |
|------|------------------|-----------------------|------------------|
| Tragic gate | `AddDocument` checks `b.poisonErr` at `builder.go:305-307`. [VERIFIED: `builder.go`] | No new document mutation before the gate. [VERIFIED: `builder.go:304-314`] | Currently returns wrapped `"builder poisoned..."`; rename to `tragicErr` and update wording. [VERIFIED: `builder.go:305-307`; VERIFIED: `16-CONTEXT.md`] |
| Doc position allocation | `AddDocument` computes `pos` and rejects `pos >= numRGs` before parser dispatch. [VERIFIED: `builder.go:308-313`] | No `docIDToPos`, `posToDocID`, `nextPos`, or `numDocs` mutation yet. [VERIFIED: `builder.go:308-348`; VERIFIED: `builder.go:711-720`] | This must stay non-tragic and included in the public catalog test. [VERIFIED: `16-CONTEXT.md`] |
| Parser staging | `b.parser.Parse(jsonDoc, pos, b)` populates `currentDocState` through `BeginDocument`. [VERIFIED: `builder.go:316-348`; VERIFIED: `parser_sink.go:31-36`] | Staged values live in `documentBuildState`, not `b.pathData`. [VERIFIED: `builder.go:89-109`; VERIFIED: `parser_sink.go`] | Parser errors are returned verbatim. [VERIFIED: `builder.go:326-327`; VERIFIED: `parser_test.go:282-295`] |
| Parser contract checks | `AddDocument` rejects missing, duplicate, or mismatched `BeginDocument`. [VERIFIED: `builder.go:330-347`] | No merge mutation yet. [VERIFIED: `builder.go:330-348`] | Existing tests cover skip/double/mismatch and should be included in tragic-nil catalog. [VERIFIED: `parser_test.go:337-439`; VERIFIED: `16-CONTEXT.md`] |
| Validate then merge | `mergeDocumentState` calls `validateStagedPaths` before `mergeStagedPaths`. [VERIFIED: `builder.go:697-701`] | `validateStagedPaths` uses a preview state; `mergeStagedPaths` mutates shared indexes. [VERIFIED: `builder.go:724-779`] | Plan must remove merge error returns and make recovered panics the only merge-origin tragic path. [VERIFIED: `16-CONTEXT.md`] |
| Doc bookkeeping | `docIDToPos`, `posToDocID`, `nextPos`, `maxRGID`, and `numDocs` update after merge returns. [VERIFIED: `builder.go:711-720`] | Yes, but only after merge success today. [VERIFIED: `builder.go:711-720`] | If merge recovery returns tragic error, bookkeeping must remain skipped. [VERIFIED: `builder.go:701-720`; VERIFIED: `16-CONTEXT.md`] |

### Exact Failure Sites

| Site | Current Error | Pre-Merge or Merge | Planning Action |
|------|---------------|--------------------|-----------------|
| `AddDocument:312` | `position %d exceeds numRGs %d`. [VERIFIED: `builder.go:311-313`] | Pre-parser. [VERIFIED: `builder.go:308-326`] | Include in `tragicErr`-nil public failure catalog. [VERIFIED: `16-CONTEXT.md`] |
| `AddDocument:331,333,341` | Parser `BeginDocument` contract errors. [VERIFIED: `builder.go:330-347`] | Pre-merge. [VERIFIED: `builder.go:330-348`] | Include in `tragicErr`-nil public failure catalog; existing parser tests are anchors. [VERIFIED: `parser_test.go:337-439`] |
| `stdlibParser.Parse` / `streamValue` | Malformed JSON, trailing JSON, malformed object/array errors. [VERIFIED: `parser_stdlib.go:21-32`; VERIFIED: `parser_stdlib.go:45-119`] | Pre-merge. [VERIFIED: `builder.go:326-348`] | Include malformed/trailing/non-UTF8 parser cases in catalog and property generators. [VERIFIED: `parser_test.go:297-327`; VERIFIED: `16-CONTEXT.md`] |
| `stageScalarToken:436` | Unsupported token type. [VERIFIED: `builder.go:416-437`] | Staging. [VERIFIED: `builder.go:416-437`] | User-reachable only through custom parser misuse; still in broad catalog if using test parser. [VERIFIED: `parser_sink.go`; VERIFIED: `16-CONTEXT.md`] |
| `stageMaterializedValue:482,493,514` | Unsupported integer overflow or transformed value type. [VERIFIED: `builder.go:480-514`] | Staging. [VERIFIED: `builder.go:440-515`] | Include `uint`/`uint64` direct internal tests or custom parser/materialized tests for catalog coverage. [VERIFIED: `16-CONTEXT.md`; VERIFIED: `phase07_review_test.go:16-21`] |
| `stageCompanionRepresentations:531` | Strict companion transformer rejection. [VERIFIED: `builder.go:518-533`] | Staging. [VERIFIED: `builder.go:518-539`] | Existing strict transformer test is reusable and should add `tragicErr` nil assertion. [VERIFIED: `transformers_test.go:481-535`] |
| `stageJSONNumberLiteral:544` | Numeric literal parse error wraps path. [VERIFIED: `builder.go:541-545`] | Staging. [VERIFIED: `builder.go:541-550`] | Existing unsupported-number test proves no partial mutation; add `tragicErr` nil assertion or catalog table. [VERIFIED: `gin_test.go:2904-2957`] |
| `parseJSONNumberLiteral:560,567` | Non-finite or unsupported integer literal. [VERIFIED: `builder.go:553-569`] | Staging. [VERIFIED: `builder.go:541-550`] | Include non-finite and overflow cases in public failure catalog. [VERIFIED: `16-CONTEXT.md`] |
| `stagedNumericFromValue:586,593` | Non-finite or unsupported native numeric. [VERIFIED: `builder.go:580-594`] | Staging. [VERIFIED: `builder.go:572-577`] | Include direct materialized/custom-parser cases if reachable through public parser seam. [VERIFIED: `parser_sink.go`; VERIFIED: `16-CONTEXT.md`] |
| `stageNumericObservation:632-647` | Unsupported mixed numeric promotion during simulation. [VERIFIED: `builder.go:597-668`] | Validator/staging. [VERIFIED: `builder.go:724-740`] | This is the validator-side gate that should fully cover merge numeric promotion before merge becomes infallible. [VERIFIED: `builder.go:724-740`; VERIFIED: `16-CONTEXT.md`] |
| `mergeNumericObservation:818-840` | Promotion failure via `promoteNumericPathToFloat` or non-exact int into float-mixed path. [VERIFIED: `builder.go:799-840`] | Merge. [VERIFIED: `builder.go:743-779`] | Remove `error` return after validator proves these conditions; preserve behavior in validator tests. [VERIFIED: `16-CONTEXT.md`] |
| `promoteNumericPathToFloat:859-868` | Promotion rejects non-exact existing int bounds. [VERIFIED: `builder.go:855-868`] | Merge. [VERIFIED: `builder.go:818-821`] | Remove `error` return and leave it as invariant-only mutation; validator must catch before this path. [VERIFIED: `16-CONTEXT.md`] |

### Infallibility Audit Inside Merge

- `mergeStagedPaths` currently mutates `pathData`, `presentRGs`, `nullRGs`, string indexes, bloom filter, string-length stats, and numeric stats in sorted path order. [VERIFIED: `builder.go:743-779`; VERIFIED: `builder.go:781-940`]
- `getOrCreatePath` uses `MustNewRGSet`, `MustNewHyperLogLog`, and `MustNewTrigramIndex`, so constructor panics inside merge are internal-invariant failures rather than user-input validation errors. [VERIFIED: `builder.go:283-301`]
- `addStringTerm` uses `MustNewRGSet`, HLL, bloom, string-length stats, and trigram addition without returning errors. [VERIFIED: `builder.go:781-797`]
- `addIntNumericValue`, `addFloatNumericValue`, and `addStringLengthStat` allocate/update per-RG stats without returning errors. [VERIFIED: `builder.go:883-940`]
- `validateStagedPaths` currently replays only `numericValues`, so any future merge-time failure category outside numeric promotion must add a simulator before adding merge-layer code. [VERIFIED: `builder.go:724-740`; VERIFIED: `16-CONTEXT.md`]

### CI/Enforcement Audit

- `Makefile` `lint` currently runs only `golangci-lint run`. [VERIFIED: `Makefile:25-28`]
- GitHub CI `lint` currently invokes `golangci/golangci-lint-action@v9` directly and does not call `make lint`. [VERIFIED: `.github/workflows/ci.yml:56-72`]
- Therefore, a Makefile-only marker grep would satisfy local lint but not the roadmap wording that CI flags reintroduced merge-layer error returns. [VERIFIED: `.github/workflows/ci.yml:56-72`; VERIFIED: `.planning/ROADMAP.md`]

## Recommended Plan Decomposition and Dependency Order

| Plan | Scope | Depends On | Files / Functions |
|------|-------|------------|-------------------|
| 16-01 Merge Contract and Validator Completeness | Add `// MUST_BE_CHECKED_BY_VALIDATOR` markers; make `mergeStagedPaths`, `mergeNumericObservation`, and `promoteNumericPathToFloat` return no `error`; keep numeric promotion rejection in `validateStagedPaths`; add focused validator tests for int-only existing state + float observation and float-mixed existing state + non-exact int observation. [VERIFIED: `16-CONTEXT.md`; VERIFIED: `builder.go:724-880`] | None. [VERIFIED: `.planning/ROADMAP.md`] | `builder.go`, `gin_test.go` or new `atomicity_test.go`. [VERIFIED: code audit] |
| 16-02 Tragic Rename, Recovery, and Logging | Rename `poisonErr` to `tragicErr`; introduce `runMergeWithRecover(func()) error`; wrap only `mergeStagedPaths`; set `tragicErr` on recovered panic; emit one Error-level logger event. [VERIFIED: `16-CONTEXT.md`; VERIFIED: `builder.go:30-34`; VERIFIED: `logging/logger.go:42-45`] | 16-01 signatures simplify merge call sites. [VERIFIED: `builder.go:697-709`] | `builder.go`, `gin_test.go`, possibly `observability_policy_test.go` for logging attr policy. [VERIFIED: code audit] |
| 16-03 Public Failure Catalog and Atomicity Property | Add `atomicity_test.go` with determinism sanity test, gopter corpus, guaranteed failing docs, and `tragicErr` nil matrix. [VERIFIED: `16-CONTEXT.md`; VERIFIED: `property_test.go:14-40`] | 16-01 and 16-02. [VERIFIED: `.planning/REQUIREMENTS.md`] | `atomicity_test.go`, helper generators, existing transformer/parser test helpers as needed. [VERIFIED: code audit] |
| 16-04 Static Enforcement in Local and CI Lint | Add grep/awk check for marker signatures to `make lint`; make GitHub CI run the same check, either by calling `make lint` or by adding a separate step. [VERIFIED: `16-CONTEXT.md`; VERIFIED: `Makefile:25-28`; VERIFIED: `.github/workflows/ci.yml:56-72`] | 16-01 markers exist. [VERIFIED: `16-CONTEXT.md`] | `Makefile`, `.github/workflows/ci.yml`, optional `scripts/check-merge-validator-markers.sh` if shell complexity grows. [VERIFIED: code audit] |

## Architecture Patterns

### System Architecture Diagram

```text
AddDocument(docID, jsonDoc)
  -> tragic gate (`tragicErr` nil?)
  -> position lookup / numRGs gate
  -> parser.Parse(jsonDoc, pos, builder-as-parserSink)
      -> BeginDocument(pos) creates documentBuildState
      -> StageScalar / StageMaterialized / StageJSONNumber mutate stagedPathData only
      -> parser/stage errors return before shared state merge
  -> parser contract checks (BeginDocument count and rgID)
  -> validateStagedPaths(staged)
      -> preview documentBuildState
      -> replay numeric observations against real b.pathData via seedNumericSimulation
      -> non-tragic validation error returns before merge
  -> runMergeWithRecover(func { mergeStagedPaths(staged) })
      -> merge mutates pathData, bloom, HLL, trigrams, stats
      -> recovered panic becomes tragicErr + Error log
  -> docID bookkeeping updates only after successful merge
  -> Finalize builds immutable GINIndex from builder state
```

Diagram source: current `builder.go` control flow and Phase 16 decisions. [VERIFIED: `builder.go:304-348`; VERIFIED: `builder.go:697-779`; VERIFIED: `16-CONTEXT.md`]

### Recommended Project Structure

```text
.
├── builder.go          # keep core validate/merge/refusal helpers together for minimal diff
├── atomicity_test.go   # new phase-level property and failure-catalog tests
├── gin_test.go         # migrate existing poison/tragic gate test
├── Makefile            # local marker grep under lint
└── .github/workflows/ci.yml  # ensure CI runs marker grep, not only golangci-lint
```

Structure source: current file layout and Phase 16 success criteria. [VERIFIED: `rg --files`; VERIFIED: `.planning/ROADMAP.md`]

### Pattern 1: Validate Before Mutate

**What:** Stage document observations into `documentBuildState`, validate every user-reachable failure against existing builder state, then merge without returning validation errors. [VERIFIED: `builder.go:89-109`; VERIFIED: `builder.go:697-779`; VERIFIED: `16-CONTEXT.md`]

**When to use:** Use this pattern for all Phase 16 changes; do not add snapshot/restore. [VERIFIED: `.planning/REQUIREMENTS.md`; VERIFIED: `16-CONTEXT.md`]

**Example:**

```go
// Source: builder.go validate/merge pattern and Phase 16 context.
func (b *GINBuilder) mergeDocumentState(docID DocID, pos int, exists bool, state *documentBuildState) error {
    if err := b.validateStagedPaths(state); err != nil {
        return err
    }
    if err := b.runMergeWithRecover(func() {
        b.mergeStagedPaths(state)
    }); err != nil {
        b.tragicErr = err
        return err
    }
    // docID bookkeeping stays after successful merge.
    return nil
}
```

Example source: recommended adaptation of current code. [VERIFIED: `builder.go:697-720`; VERIFIED: `16-CONTEXT.md`]

### Pattern 2: Recovery at the Merge Boundary Only

**What:** Put `recover()` in a helper invoked around `mergeStagedPaths`, not around parser/stage/validation or all of `AddDocument`. [VERIFIED: `16-CONTEXT.md`]

**Why:** Go `recover` only regains control inside deferred functions during panics, and the standard library convention is to recover internally while exposing errors at package boundaries. [CITED: https://go.dev/blog/defer-panic-and-recover]

**Example:**

```go
// Source: Go panic/recover docs plus Phase 16 D-04/D-06.
func runMergeWithRecover(fn func()) (err error) {
    defer func() {
        if recovered := recover(); recovered != nil {
            err = errors.Errorf("builder tragic: recovered panic in merge: %v", recovered)
        }
    }()
    fn()
    return nil
}
```

Example source: recommended helper shape; exact name/message remains planner discretion. [CITED: https://go.dev/blog/defer-panic-and-recover; VERIFIED: `16-CONTEXT.md`]

### Pattern 3: Marker-Enforced Merge Functions

**What:** Annotate only merge-layer functions whose user-reachable failures must have validator coverage. [VERIFIED: `16-CONTEXT.md`]

**Example:**

```go
// MUST_BE_CHECKED_BY_VALIDATOR
func (b *GINBuilder) mergeNumericObservation(pd *pathBuildData, observation stagedNumericValue, rgID int, path string) {
    // No error return; caller relies on validateStagedPaths to pre-detect failures.
}
```

Example source: Phase 16 D-04. [VERIFIED: `16-CONTEXT.md`]

### Anti-Patterns to Avoid

- **Returning validation errors from merge:** This recreates the current partial-mutation poison path. [VERIFIED: `builder.go:701-708`; VERIFIED: `16-CONTEXT.md`]
- **Adding `recover()` around all of `AddDocument`:** Context explicitly rejects that broader net and would hide parser/stage programming errors in the wrong catalog. [VERIFIED: `16-CONTEXT.md`]
- **Using a fresh `pathData` clone for validation:** Context locks the shadow-field simulation pattern instead of typed preview cloning. [VERIFIED: `16-CONTEXT.md`]
- **Only updating `Makefile` for marker grep:** Current CI bypasses `make lint`, so CI would not enforce the invariant. [VERIFIED: `.github/workflows/ci.yml:56-72`; VERIFIED: `Makefile:25-28`]
- **Asserting only query-equivalence for atomicity:** Requirement demands byte-identical `Encode`, so query-only checks are insufficient. [VERIFIED: `.planning/REQUIREMENTS.md`]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Per-document rollback | Manual snapshot/restore of maps, bitmaps, HLL, trigrams, bloom, and doc ID structures. [VERIFIED: `builder.go` audit; VERIFIED: `16-CONTEXT.md`] | Existing `documentBuildState` + `validateStagedPaths` + infallible merge. [VERIFIED: `builder.go:89-109`; VERIFIED: `builder.go:724-779`] | Snapshot coverage is error-prone and explicitly deferred. [VERIFIED: `.planning/REQUIREMENTS.md`; VERIFIED: `16-CONTEXT.md`] |
| Property testing framework | A custom random corpus runner. [VERIFIED: repo already has gopter] | Existing gopter helpers and `propertyTestParameters()`. [VERIFIED: `property_test.go:14-40`; VERIFIED: `go.mod`] | The project already standardizes gopter budgets and shrink behavior. [VERIFIED: `go doc github.com/leanovate/gopter/prop.ForAll`; VERIFIED: `property_test.go`] |
| CI signature policy | A new golangci plugin or AST analyzer. [VERIFIED: `16-CONTEXT.md`] | Shell/grep/awk marker check wired into both local lint and CI. [VERIFIED: `16-CONTEXT.md`; VERIFIED: `.github/workflows/ci.yml`] | Context locks a grep-style check and the target invariant is simple. [VERIFIED: `16-CONTEXT.md`] |
| Failure taxonomy | Phase-16-only enum or exported error type. [VERIFIED: `.planning/REQUIREMENTS.md`] | Existing plain errors in Phase 16; defer taxonomy and structured errors. [VERIFIED: `16-CONTEXT.md`] | Phases 17 and 18 own those surfaces. [VERIFIED: `.planning/REQUIREMENTS.md`] |

**Key insight:** The hard part is not rollback; it is making `validateStagedPaths` the single complete catalog for user-reachable merge failures and then making merge signatures impossible to misuse. [VERIFIED: `builder.go` audit; VERIFIED: `16-CONTEXT.md`]

## Runtime State Inventory

| Category | Items Found | Action Required |
|----------|-------------|-----------------|
| Stored data | None found for `poisonErr`/`builder poisoned`; repository audit found only source/test references. [VERIFIED: `rg -n "poisonErr|builder poisoned|poisoned" .`] | No data migration. [VERIFIED: code audit] |
| Live service config | None identified; phase changes internal Go builder code and CI config only. [VERIFIED: `.planning/ROADMAP.md`; VERIFIED: code audit] | No service config action. [VERIFIED: code audit] |
| OS-registered state | None identified. [VERIFIED: project file audit; ASSUMED: no external services registered for this library checkout] | No OS re-registration. [ASSUMED] |
| Secrets/env vars | No `.env`, SQLite, or obvious datastore files found in repository depth-3 audit. [VERIFIED: `find . -maxdepth 3 ...`] | No secret/env var rename. [VERIFIED: repository audit] |
| Build artifacts | `ami-gin.test` exists and contains old `poisonErr`/`builder poisoned` strings; `coverage.out` exists but did not contain those strings. [VERIFIED: `find . -maxdepth 3 ...`; VERIFIED: `strings ami-gin.test | rg ...`; VERIFIED: `rg ... coverage.out`] | Delete or regenerate ignored build/test artifacts after implementation; do not commit them. [VERIFIED: build artifact audit] |

## Common Pitfalls

### Pitfall 1: Preview State That Does Not Represent Real `pathData`

**What goes wrong:** Validator passes even though existing builder numeric state would make merge fail. [VERIFIED: `builder.go:724-740`; VERIFIED: `builder.go:799-880`]

**Why it happens:** A fresh preview that does not seed from `b.pathData` would miss existing int-only or float-mixed state. [VERIFIED: `builder.go:671-691`]

**How to avoid:** Keep using `stageNumericObservation` during validation because it calls `seedNumericSimulation` on first path touch. [VERIFIED: `builder.go:597-600`; VERIFIED: `builder.go:671-691`]

**Warning signs:** New validator code builds independent `pathBuildData` clones or bypasses `stageNumericObservation`. [VERIFIED: `16-CONTEXT.md`; ASSUMED]

### Pitfall 2: Promotion Error Wording Drift

**What goes wrong:** Validator-hoisted errors no longer match current user-visible strings or tests. [VERIFIED: `builder.go:632-647`; VERIFIED: `builder.go:818-840`; VERIFIED: `builder.go:855-868`]

**Why it happens:** Current merge promotion can wrap `promoteNumericPathToFloat` with `"promote numeric path %s"`, while the simulator returns `"unsupported mixed numeric promotion at %s"`. [VERIFIED: `builder.go:818-821`; VERIFIED: `builder.go:632-647`]

**How to avoid:** Planner should add explicit tests for both promotion directions and lock the D-09 wording chosen in context. [VERIFIED: `16-CONTEXT.md`; VERIFIED: `gin_test.go:3142-3166`]

**Warning signs:** Tests only check substring `$.score` and not the exact Phase 16 wording. [VERIFIED: `gin_test.go:3149-3155`]

### Pitfall 3: Recover Test That Does Not Prove Builder Refusal

**What goes wrong:** `runMergeWithRecover` is tested in isolation but `AddDocument` does not refuse later calls. [VERIFIED: `16-CONTEXT.md`; VERIFIED: current refusal test at `gin_test.go:425-447`]

**Why it happens:** A helper can return an error without assigning `b.tragicErr`. [ASSUMED]

**How to avoid:** Test both the helper return and the builder-level gate after assigning tragedy. [VERIFIED: `16-CONTEXT.md`; VERIFIED: `builder.go:305-307`]

**Warning signs:** New tests assert only panic-to-error conversion and never call a subsequent `AddDocument`. [VERIFIED: `16-CONTEXT.md`]

### Pitfall 4: Marker Check Not Actually Running in CI

**What goes wrong:** Local `make lint` fails on a bad marker signature, but GitHub CI remains green. [VERIFIED: `Makefile:25-28`; VERIFIED: `.github/workflows/ci.yml:56-72`]

**Why it happens:** CI uses `golangci/golangci-lint-action@v9` directly. [VERIFIED: `.github/workflows/ci.yml:68-72`]

**How to avoid:** Add a CI step for the marker check or change CI lint to call `make lint`. [VERIFIED: `.github/workflows/ci.yml`; VERIFIED: `16-CONTEXT.md`]

**Warning signs:** `.github/workflows/ci.yml` is unchanged after adding the Makefile grep. [VERIFIED: `.github/workflows/ci.yml`]

### Pitfall 5: Atomicity Property Fails Due to Encode Nondeterminism

**What goes wrong:** A full-vs-clean corpus byte comparison fails for reasons unrelated to failed-document mutation. [VERIFIED: `16-CONTEXT.md`]

**Why it happens:** The property depends on deterministic `Finalize` and `Encode` ordering. [VERIFIED: `builder.go:951-980`; VERIFIED: `serialize.go:795-822`]

**How to avoid:** Add the clean-corpus determinism sanity test first, as locked in D-05. [VERIFIED: `16-CONTEXT.md`]

**Warning signs:** `atomicity_test.go` contains no separate determinism test. [VERIFIED: `16-CONTEXT.md`]

## Code Examples

### Validator Coverage Test Shape

```go
func TestValidateStagedPathsRejectsLossyPromotionBeforeMerge(t *testing.T) {
    b := mustNewBuilder(t, DefaultConfig(), 2)
    if err := b.AddDocument(0, []byte(`{"score":9007199254740993}`)); err != nil {
        t.Fatalf("seed: %v", err)
    }

    state := newDocumentBuildState(1)
    if err := b.stageJSONNumberLiteral("$.score", "1.5", state); err != nil {
        t.Fatalf("stage: %v", err)
    }

    err := b.validateStagedPaths(state)
    if err == nil || !strings.Contains(err.Error(), "unsupported mixed numeric promotion at $.score") {
        t.Fatalf("validateStagedPaths() = %v", err)
    }
}
```

Source: current test helpers and validation functions. [VERIFIED: `builder.go:541-550`; VERIFIED: `builder.go:724-740`; VERIFIED: `gin_test.go:3142-3166`]

### Atomicity Property Core Assertion

```go
properties.Property("failed documents do not change encoded index", prop.ForAll(
    func(corpus atomicityCorpus) string {
        fullBytes, fullErr := buildAtomicityIndex(corpus.All)
        cleanBytes, cleanErr := buildAtomicityIndex(corpus.CleanOnly)
        switch {
        case cleanErr != nil:
            return cleanErr.Error()
        case fullErr != nil:
            return fullErr.Error()
        case !bytes.Equal(fullBytes, cleanBytes):
            return "encoded index differs after failed documents"
        default:
            return ""
        }
    },
    genAtomicityCorpus(1000),
))
```

Source: gopter supports `prop.ForAll`, fixed-size slices via `gen.SliceOfN`, and string-returning properties where empty means pass. [VERIFIED: `go doc github.com/leanovate/gopter/prop.ForAll`; VERIFIED: `go doc github.com/leanovate/gopter/gen.SliceOfN`]

### Marker Check Sketch

```sh
awk '
  /MUST_BE_CHECKED_BY_VALIDATOR/ { marker=1; next }
  marker && /^func / {
    if ($0 ~ /\) *\([^)]*error/ || $0 ~ /\) *error/) {
      print "merge validator marker function returns error: " $0
      bad=1
    }
    marker=0
  }
  END { exit bad }
' builder.go
```

Source: context requires marker signatures without `error` returns; exact grep/awk invocation is discretionary. [VERIFIED: `16-CONTEXT.md`]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Treat merge errors as builder-poisoning user-visible failures. [VERIFIED: `builder.go:701-708`] | Treat user-input failures as per-document validation errors and reserve tragedy for internal invariant/panic paths. [VERIFIED: `.planning/REQUIREMENTS.md`; VERIFIED: `16-CONTEXT.md`; CITED: https://lucene.apache.org/core/10_1_0/core/org/apache/lucene/index/IndexWriter.html] | Phase 16 planning date 2026-04-23. [VERIFIED: `16-CONTEXT.md`; VERIFIED: `date +%F`] | Planner must prioritize correctness over performance and avoid snapshot/restore unless future validation becomes impossible. [VERIFIED: `.planning/PROJECT.md`; VERIFIED: `16-CONTEXT.md`] |
| Merge functions return `error`. [VERIFIED: `builder.go:743`; VERIFIED: `builder.go:799`; VERIFIED: `builder.go:855`] | Merge functions marked `MUST_BE_CHECKED_BY_VALIDATOR` have no `error` return. [VERIFIED: `16-CONTEXT.md`] | Phase 16 target. [VERIFIED: `.planning/ROADMAP.md`] | Compiler and grep enforce the contract. [VERIFIED: `16-CONTEXT.md`] |
| Poison terminology. [VERIFIED: `builder.go:30-34`; VERIFIED: `gin_test.go:425-447`] | Tragic terminology aligned with Lucene's tragic exception separation. [VERIFIED: `16-CONTEXT.md`; CITED: https://lucene.apache.org/core/10_1_0/core/org/apache/lucene/index/IndexWriter.html] | Phase 16 target. [VERIFIED: `.planning/ROADMAP.md`] | Public failure catalog tests must prove user errors do not set tragedy. [VERIFIED: `.planning/REQUIREMENTS.md`] |

**Deprecated/outdated:**
- `poisonErr` field name and `"builder poisoned"` wording are obsolete after Phase 16. [VERIFIED: `16-CONTEXT.md`; VERIFIED: `builder.go:30-34`; VERIFIED: `gin_test.go:435-442`]
- Merge-layer `error` returns are obsolete after Phase 16. [VERIFIED: `16-CONTEXT.md`; VERIFIED: `builder.go:743`; VERIFIED: `builder.go:799`; VERIFIED: `builder.go:855`]

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `rg` is available in CI or a POSIX `grep`/`awk` fallback can be used. [ASSUMED] | Standard Stack / Plan 16-04 | Marker check implementation may fail in CI if the chosen command is unavailable. [ASSUMED] |
| A2 | No OS-registered state embeds the internal `poisonErr` name. [ASSUMED] | Runtime State Inventory | A local service or generated artifact outside the repo could retain stale wording. [ASSUMED] |
| A3 | Plan sizing into four plans is appropriate. [ASSUMED] | Recommended Plan Decomposition | Planner may need to split tests from CI or merge them depending on implementation diff size. [ASSUMED] |

## Open Questions (RESOLVED)

1. **Should the CI lint job call `make lint` or add a separate marker-check step?** [VERIFIED: `.github/workflows/ci.yml:56-72`]
   - RESOLVED: CI uses a separate explicit marker-check step in `.github/workflows/ci.yml` in addition to local `make lint`.
   - What we know: CI currently uses the golangci action directly. [VERIFIED: `.github/workflows/ci.yml:68-72`]
   - What's unclear: Whether the project prefers action-native golangci execution or Makefile-as-source-of-truth. [ASSUMED]
   - Recommendation: Add a separate explicit marker-check step to minimize behavior change to the existing golangci action. [ASSUMED]

2. **Should `runMergeWithRecover` include raw `panic_value` in logs?** [VERIFIED: `16-CONTEXT.md`; VERIFIED: `logging/attrs.go:5-23`]
   - RESOLVED: Recovery logging omits the raw panic value and does not emit a `panic_value` attr.
   - What we know: Context asks for attrs beyond `panic_value` at discretion, and the logging package has exported `Attr` fields but a frozen helper vocabulary. [VERIFIED: `16-CONTEXT.md`; VERIFIED: `logging/attrs.go:5-23`]
   - What's unclear: Whether Error-level logs may use raw attr keys outside the INFO-level frozen vocabulary. [ASSUMED]
   - Recommendation: Use safe existing attrs such as `logging.AttrErrorType("other")` and omit raw recovered values entirely. [VERIFIED: `logging/attrs.go:55-59`; ASSUMED]

3. **Should validator tests assert the exact old wrapped promotion message or the D-09 normalized message?** [VERIFIED: `builder.go:818-821`; VERIFIED: `16-CONTEXT.md`]
   - RESOLVED: Validator tests assert normalized `unsupported mixed numeric promotion at $.score` wording.
   - What we know: Existing merge wraps one promotion path differently from `stageNumericObservation`. [VERIFIED: `builder.go:632-647`; VERIFIED: `builder.go:818-821`]
   - What's unclear: Whether D-09's wording stability refers to the unwrapped validator message or the older wrapped merge path. [ASSUMED]
   - Recommendation: Lock `"unsupported mixed numeric promotion at $.score"` for Phase 16 tests because it is the context's named string and includes path context. [VERIFIED: `16-CONTEXT.md`]

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|-------------|-----------|---------|----------|
| Go | Build/test/compiler signature checks. [VERIFIED: `Makefile`; VERIFIED: `.github/workflows/ci.yml`] | Yes. [VERIFIED: `command -v go`] | Local `go1.26.2`; module declares `1.25.5`; CI tests `1.25` and `1.26`. [VERIFIED: `go version`; VERIFIED: `go.mod`; VERIFIED: `.github/workflows/ci.yml`] | Use CI matrix for final compatibility if local differs. [VERIFIED: `.github/workflows/ci.yml`] |
| `gotestsum` | Full suite command. [VERIFIED: `Makefile:11-20`] | Yes. [VERIFIED: `command -v gotestsum`] | `v1.13.0`. [VERIFIED: `gotestsum --version`] | `make test` installs it if missing. [VERIFIED: `Makefile:7-20`] |
| `golangci-lint` | Lint command. [VERIFIED: `Makefile:25-28`] | Yes. [VERIFIED: `command -v golangci-lint`] | `2.11.4`. [VERIFIED: `golangci-lint --version`] | CI action pins same version. [VERIFIED: `.github/workflows/ci.yml:68-72`] |
| `rg` | Search and possible marker check. [VERIFIED: environment audit] | Yes. [VERIFIED: `command -v rg`] | `15.1.0`. [VERIFIED: `rg --version`] | POSIX `grep`/`awk` for CI portability. [ASSUMED] |

**Missing dependencies with no fallback:** None identified for research/planning. [VERIFIED: environment audit]

**Missing dependencies with fallback:** `gsd-sdk query` is unavailable in this checkout; research used direct `.planning/` file reads instead. [VERIFIED: `gsd-sdk query init.phase-op "16"` command output]

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` plus `github.com/leanovate/gopter` `v0.2.11`. [VERIFIED: `go.mod`; VERIFIED: `property_test.go:1-40`; VERIFIED: `go list -m -json github.com/leanovate/gopter`] |
| Config file | `.golangci.yml` for lint; no separate Go test config file. [VERIFIED: `.golangci.yml`; VERIFIED: repo audit] |
| Quick run command | `go test -run 'Test(AddDocument|ValidateStagedPaths|RunMerge|Atomicity)' ./...` after tests are named. [ASSUMED] |
| Full suite command | `make test` or CI-equivalent `gotestsum --format short-verbose --packages="./..." --junitfile unit.xml -- -short -race -coverprofile=coverage.out -timeout=30m ./...`. [VERIFIED: `Makefile:11-20`; VERIFIED: `.github/workflows/ci.yml:35-36`] |

### Phase Requirements -> Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| ATOMIC-01 | Full corpus with guaranteed failing docs encodes byte-identically to clean subset; clean corpus encodes deterministically across two builds. [VERIFIED: `.planning/REQUIREMENTS.md`; VERIFIED: `16-CONTEXT.md`] | property + unit sanity | `go test -run TestAddDocumentAtomicity ./...` [ASSUMED test name] | No; create `atomicity_test.go`. [VERIFIED: `rg --files | rg '^atomicity_test\\.go$'` returned no output] |
| ATOMIC-02 | Merge functions have no `error` returns and validator catches numeric promotion failures before merge. [VERIFIED: `.planning/REQUIREMENTS.md`; VERIFIED: `builder.go:724-880`] | unit + static grep | `go test -run TestValidateStagedPaths ./...` and `make lint` [ASSUMED test name; VERIFIED: `Makefile:25-28`] | Partial; existing tests cover rejected promotion but not marker/signature policy. [VERIFIED: `gin_test.go:3142-3166`; VERIFIED: `Makefile:25-28`] |
| ATOMIC-03 | `tragicErr` is nil for public user-input failures, recovered merge panic sets `tragicErr`, and subsequent `AddDocument` is refused. [VERIFIED: `.planning/REQUIREMENTS.md`; VERIFIED: `16-CONTEXT.md`] | unit matrix + recovery test | `go test -run 'Test(AddDocumentPublicFailuresDoNotSetTragicErr|RunMergeWithRecover|AddDocumentRefusesAfterTragic)' ./...` [ASSUMED test names] | Partial; existing poison refusal test must migrate and expand. [VERIFIED: `gin_test.go:425-447`] |

### Sampling Rate

- **Per task commit:** Run the relevant focused `go test -run ... ./...` plus the marker-check command after marker work lands. [ASSUMED; VERIFIED: Phase requirements]
- **Per wave merge:** Run `go test ./...` or `make test` depending on time budget; run `make lint` after static enforcement changes. [VERIFIED: `Makefile`; ASSUMED]
- **Phase gate:** Run `make test`, `make lint`, and `go build ./...`; confirm GitHub CI lint has the marker grep path. [VERIFIED: `Makefile`; VERIFIED: `.github/workflows/ci.yml`]

### Wave 0 Gaps

- [ ] `atomicity_test.go` — covers ATOMIC-01 and parts of ATOMIC-03. [VERIFIED: `16-CONTEXT.md`]
- [ ] Focused validator tests — cover ATOMIC-02 numeric promotion directions against real `pathData`. [VERIFIED: `builder.go:724-880`]
- [ ] Public failure catalog test — covers ATOMIC-03 and asserts `tragicErr == nil` for all user-input failures. [VERIFIED: `16-CONTEXT.md`]
- [ ] Marker grep in `Makefile` and `.github/workflows/ci.yml` — covers ATOMIC-02 enforcement. [VERIFIED: `Makefile`; VERIFIED: `.github/workflows/ci.yml`]

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|------------------|
| V2 Authentication | No. [VERIFIED: project is a Go library with no auth surface in phase scope] | Not applicable. [VERIFIED: `.planning/ROADMAP.md`] |
| V3 Session Management | No. [VERIFIED: no session/web server code in phase scope] | Not applicable. [VERIFIED: `.planning/ROADMAP.md`] |
| V4 Access Control | No. [VERIFIED: no authorization boundary in phase scope] | Not applicable. [VERIFIED: `.planning/ROADMAP.md`] |
| V5 Input Validation | Yes. [VERIFIED: phase validates parser/stage/numeric failure handling] | Validate before merge; reject unsupported numeric promotion before state mutation. [VERIFIED: `builder.go:724-740`; VERIFIED: `16-CONTEXT.md`] |
| V6 Cryptography | No. [VERIFIED: no cryptographic code in phase scope] | Do not hand-roll crypto; no crypto work planned. [VERIFIED: `.planning/ROADMAP.md`] |
| V9 Error Handling and Logging | Yes. [VERIFIED: phase renames tragic error path and adds recovery logging] | Keep user-input failures non-tragic; log only recovered panic through existing Logger seam. [VERIFIED: `16-CONTEXT.md`; VERIFIED: `logging/logger.go`] |

### Known Threat Patterns for Go Builder Ingest

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Failed document partially mutates index and creates false positives/metadata drift. [VERIFIED: current poison path comments at `builder.go:701-706`] | Tampering | Validate all user-reachable failures before merge and assert byte-identical encoded output. [VERIFIED: `.planning/REQUIREMENTS.md`; VERIFIED: `16-CONTEXT.md`] |
| Panic during merge crashes caller process. [CITED: https://go.dev/blog/defer-panic-and-recover] | Denial of Service | Recover at merge boundary, set `tragicErr`, and refuse later writes. [VERIFIED: `16-CONTEXT.md`] |
| Logging panic values leaks raw document content. [ASSUMED] | Information Disclosure | Keep default logger noop and be conservative with Error-level attrs; avoid INFO-level raw values. [VERIFIED: `gin.go:655-683`; VERIFIED: `logging/attrs.go:14-23`] |
| CI misses invariant regression. [VERIFIED: `.github/workflows/ci.yml:56-72`] | Tampering / Process | Add marker grep to CI path, not only Makefile. [VERIFIED: `.github/workflows/ci.yml`; VERIFIED: `16-CONTEXT.md`] |

## Sources

### Primary (HIGH confidence)

- `.planning/phases/16-adddocument-atomicity-lucene-contract/16-CONTEXT.md` — locked Phase 16 decisions, deferred scope, and test strategy. [VERIFIED: file read]
- `.planning/REQUIREMENTS.md` — ATOMIC-01, ATOMIC-02, ATOMIC-03 requirements. [VERIFIED: file read]
- `.planning/ROADMAP.md` — Phase 16 success criteria and phase sequence. [VERIFIED: file read]
- `.planning/PROJECT.md` and `.planning/STATE.md` — milestone priorities and current state. [VERIFIED: file read]
- `builder.go`, `parser_sink.go`, `parser_stdlib.go`, `gin.go`, `logging/logger.go`, `logging/attrs.go`, `Makefile`, `.github/workflows/ci.yml` — current code audit. [VERIFIED: `nl`, `rg`, direct file reads]
- `go list -m -json` / `go doc` outputs for gopter and package versions. [VERIFIED: local Go toolchain]
- Apache Lucene `IndexWriter` 10.1.0 API docs — addDocument consistency contract and tragic exception APIs. [CITED: https://lucene.apache.org/core/10_1_0/core/org/apache/lucene/index/IndexWriter.html]
- Go Blog "Defer, Panic, and Recover" — panic/recover behavior and standard-library convention. [CITED: https://go.dev/blog/defer-panic-and-recover]

### Secondary (MEDIUM confidence)

- Prior phase contexts for Phase 13 parser seam, Phase 14 observability seam, and Phase 15 CLI failure handling. [VERIFIED: `.planning/phases/13-parser-seam-extraction/13-CONTEXT.md`; VERIFIED: `.planning/phases/14-observability-seams/14-CONTEXT.md`; VERIFIED: `.planning/phases/15-experimentation-cli/15-CONTEXT.md`]

### Tertiary (LOW confidence)

- Assumptions about OS-registered state and CI shell availability. [ASSUMED]

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — versions and tools verified from `go.mod`, `go list`, and local commands. [VERIFIED: `go.mod`; VERIFIED: `go list -m -json`; VERIFIED: environment audit]
- Architecture: HIGH — current control flow and mutation points verified from source. [VERIFIED: `builder.go` audit]
- Pitfalls: MEDIUM-HIGH — CI bypass pitfall is verified; log-attr and exact wording risks need planner/user confirmation only if implementation choices diverge. [VERIFIED: `.github/workflows/ci.yml`; ASSUMED]

**Research date:** 2026-04-23 [VERIFIED: `date +%F`]
**Valid until:** 2026-05-23 for code audit if Phase 16 remains unimplemented; sooner if `builder.go`, `Makefile`, or `.github/workflows/ci.yml` changes. [ASSUMED]
