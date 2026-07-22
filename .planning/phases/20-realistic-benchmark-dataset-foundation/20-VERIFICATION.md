---
phase: 20-realistic-benchmark-dataset-foundation
verified: 2026-07-21T12:06:42+03:00
status: passed
score: 10/10 must-haves verified
overrides_applied: 0
---

# Phase 20: Realistic Benchmark Dataset Foundation Verification Report

**Phase Goal:** Provide governed, realistic JSONL fixture inputs and an offline-default benchmark workflow for later SIMD evaluation.
**Verified:** 2026-07-21T12:06:42+03:00
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

| # | Observable truth | Status | Evidence |
|---|---|---|---|
| 1 | Fixture governance specifies synthesized provenance, byte caps, offline behavior, and future license/NOTICE handling. | VERIFIED | `testdata/phase20/README.md` records four files, counts, current sizes, caps, no copied upstream rows/text, no-download behavior, and a pinned-revision LICENSE/NOTICE review rule. |
| 2 | Fixtures are deterministic, committed, bounded local inputs. | VERIFIED | `testdata/phase20/generate.go` writes only the four named files; running it left all four fixture files unchanged in `git diff --exit-code`. |
| 3 | Nested/high-cardinality shape is present. | VERIFIED | The nested fixture has 96 records with repeated categories and varied nested actors/users, repositories, URLs, timestamps, and searchable text; `TestPhase20SmokeFixtures` asserts this. |
| 4 | Mixed-type wildcard-array shape is present. | VERIFIED | The mixed fixture has 96 records whose `wildcard_values` covers strings, integer and decimal numbers, booleans, null, objects, and empty arrays; integrity coverage asserts every kind. |
| 5 | Number-heavy shape preserves required numeric lexemes and range data. | VERIFIED | The number fixture has 96 records and retains unquoted `9223372036854775807`, `-9223372036854775807`, and `9007199254740993`, plus decimals, RFC3339 timestamps, epochs, and low/mid/high cohorts. |
| 6 | The combined fixture is a raw, prescribed three-stream round-robin rather than a fourth schema or concatenation. | VERIFIED | `TestPhase20SmokeFixtures` compares all 288 raw lines byte-for-byte against 96 nested/mixed/number cycles and checks source-kind order. |
| 7 | Default benchmark smoke leaves use only checked-in data and expose Build, Encode, and Query work. | VERIFIED | `BenchmarkPhase20RealisticJSON` registers four `tier=smoke` fixtures, each with Build/Encode/Query leaves, all of which passed with both Phase-20 environment variables unset. |
| 8 | External input is opt-in exactly when `GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL == "1"`. | VERIFIED | `phase20ExternalBenchmarkPaths` returns before reading `GIN_PHASE20_SIMDJSON_DIR` unless the exact enable value is `1`; focused tests cover unset and `true`, and the disabled benchmark command skipped all three leaves without the directory variable. |
| 9 | Enabled external input is local-only, deterministic, schema-independent, and actionable on invalid configuration. | VERIFIED | Discovery accepts only sorted top-level `.ndjson`, `.jsonl`, and `.json`; focused tests cover missing, empty, unsupported, invalid, and valid inputs. The query is derived as `IsNotNull` from the first supported observed scalar path. |
| 10 | Contributor documentation matches the implemented offline and manual-local workflows. | VERIFIED | Root `README.md` has the Phase 20 workflow, exact smoke and local benchmark selectors, accepted formats, data-derived query explanation, no-download rule, and pinned-revision LICENSE/NOTICE guidance. |

**Score:** 10/10 must-haves verified

## Requirement Traceability

| Requirement | Status | Evidence |
|---|---|---|
| DATA-01 | SATISFIED | Fixture README and root README document synthesized provenance, source inspiration, no copied upstream data, counts/caps, offline default, exact external variables, and the pinned-revision LICENSE/NOTICE rule. |
| DATA-02 | SATISFIED | Four committed JSONL fixtures meet the 96/96/96/288 record contract, caps, raw-interleaving contract, multi-row-group packing, and three required realistic shapes. |
| DATA-03 | SATISFIED | Offline smoke benchmark passed with external variables removed. External discovery is gated before directory access, external leaves remain registered when disabled, and tests cover exact gate behavior. |

No orphaned Phase 20 requirement IDs were found: `DATA-01`, `DATA-02`, and `DATA-03` are mapped to Phase 20 in `.planning/REQUIREMENTS.md` and checked by the Phase 20 plans and implementation.

## Behavioral Evidence

| Check | Result | Status |
|---|---|---|
| `go run ./testdata/phase20/generate.go` then `git diff --exit-code -- testdata/phase20/*.jsonl` | Generator exited 0; no fixture drift. | PASS |
| `go test ./... -run '^TestPhase20' -count=1` | Fixture integrity, malformed-line diagnostics, external-gate, discovery, ordering, and derived-predicate tests passed. | PASS |
| External variables unset: `go test ./... -run '^$' -bench '^BenchmarkPhase20RealisticJSON/tier=smoke' -benchtime=1x -count=1 -benchmem` | All 12 smoke leaves (four fixtures × Build/Encode/Query) passed. | PASS |
| `GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL=true` with `GIN_PHASE20_SIMDJSON_DIR` unset | External Build, Encode, and Query leaves were registered and each skipped with both-variable setup guidance. | PASS |
| `go test ./... -count=1` | Full regression suite passed. | PASS |
| `go build ./...` | All packages built successfully. | PASS |

## Gating and Offline Review

The smoke branch calls only the committed raw-JSONL loader. It does not call the external discovery helper. In the external branch, `phase20ExternalBenchmarkPaths` first reads `GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL`; every value other than the exact string `"1"` returns `enabled=false` before the code reads, cleans, stats, or lists `GIN_PHASE20_SIMDJSON_DIR`. The disabled benchmark evidence confirms that a non-`1` value succeeds without the directory variable.

No downloader, network client, SIMD parser-selection change, build tag, CI change, or Phase-13 golden refresh was introduced by the Phase 20 change set. The only simdjson URLs are documentation review links; fixture and benchmark execution uses local files.

## Human Verification Required

None for Phase 20 acceptance. A real local simdjson checkout remains intentionally optional; the documented manual command can exercise it when a contributor has one. A future direct redistribution of upstream rows also remains conditional on the documented pinned-revision license/NOTICE review.

## Gaps Summary

No gaps found. Phase 20 achieves the roadmap success criteria: governed offline fixture policy and provenance, all three required realistic JSON shapes, and default smoke benchmarks that require neither network access nor external data.

---

_Verifier: Codex (independent Phase 20 verification)_
