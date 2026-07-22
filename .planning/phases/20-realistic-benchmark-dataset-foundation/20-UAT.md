---
status: complete
phase: 20-realistic-benchmark-dataset-foundation
source: [.planning/phases/20-realistic-benchmark-dataset-foundation/20-01-SUMMARY.md, .planning/phases/20-realistic-benchmark-dataset-foundation/20-02-SUMMARY.md]
started: 2026-07-21T11:06:33Z
updated: 2026-07-21T12:43:07Z
---

## Current Test
<!-- OVERWRITE each test - shows where we are -->

[testing complete]

## Tests

### 1. Governed Smoke Fixture Integrity
expected: Run `go test ./... -run '^TestPhase20SmokeFixtures$' -count=1`. The test passes, confirming the four checked-in JSONL fixtures are valid, within their documented size and record-count limits, cover the required realistic shapes, preserve the combined stream byte-for-byte, and include the required provenance and offline-use policy.
result: pass
note: "Automated run exited 0; the root package and all applicable repository packages passed."

### 2. Deterministic Offline Fixture Refresh
expected: Run `go run ./testdata/phase20/generate.go`, then `git diff --exit-code -- testdata/phase20`. The generator completes without downloading data and the diff command exits successfully with no fixture changes.
result: pass
note: "Generator exited 0 and the scoped git diff reported no changes."

### 3. Offline Realistic JSON Benchmarks
expected: Run `go test ./... -run '^$' -bench '^BenchmarkPhase20RealisticJSON/tier=smoke' -benchtime=1x -count=1 -benchmem`. The command succeeds offline and reports Build, Encode, and Query benchmark leaves for nested-high-cardinality, mixed-type-arrays, number-heavy, and combined fixtures.
result: pass
note: "Automated run exited 0 and reported all 12 expected Build, Encode, and Query leaves across the four smoke fixtures."

### 4. Disabled External Benchmark Gate
expected: Run `go test ./... -run '^$' -bench 'BenchmarkPhase20RealisticJSON/tier=external/fixture=local-example' -benchtime=1x -count=1 -benchmem` without Phase 20 environment variables. The external tier remains visible, skips with setup guidance, and does not try to read local data.
result: pass
note: "Automated run exited 0; a verbose confirmation showed Build, Encode, and Query all skipped with the documented environment-variable guidance."

### 5. Explicitly Enabled Local Benchmark Tier
expected: Run `GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL=1 GIN_PHASE20_SIMDJSON_DIR=testdata/phase20 go test ./... -run '^$' -bench 'BenchmarkPhase20RealisticJSON/tier=external/fixture=local-example' -benchtime=1x -count=1 -benchmem`. The command discovers the top-level JSONL inputs and successfully runs the registered Build, Encode, and schema-independent Query leaves.
result: pass
note: "Automated run exited 0 and the enabled tier discovered the checked-in top-level JSONL files and ran Build, Encode, and Query successfully."

## Summary

total: 5
passed: 5
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]
