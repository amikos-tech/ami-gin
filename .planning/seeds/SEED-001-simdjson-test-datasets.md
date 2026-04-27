---
id: SEED-001
status: dormant
planted: 2026-04-15
planted_during: v1.0 / Phase 08 (adaptive-high-cardinality-indexing)
trigger_when: performance milestone or benchmark-focused phase
scope: Medium
---

# SEED-001: Use simdjson JSON example datasets for testing and benchmarking

## Why This Matters

Current tests use hand-crafted JSON documents. Real-world datasets from the
simdjson project expose edge cases in nesting depth, type diversity, key
cardinality, and document size that synthetic fixtures miss. These datasets are
well-known in the JSON tooling ecosystem and would strengthen both correctness
coverage and benchmark realism.

## When to Surface

**Trigger:** When a milestone or phase focuses on performance validation, benchmarking, or test infrastructure.

This seed should be presented during `/gsd-new-milestone` when the milestone
scope matches any of these conditions:
- Milestone includes benchmark or performance validation work
- Milestone adds test data infrastructure or fixture management
- Milestone targets index quality metrics on realistic data

## Scope Estimate

**Medium** — Needs a test fixture management approach (download/vendor, size
limits), new benchmark suites wired to the datasets, and possibly golden-file
regression tests for index stats across the corpus.

## Breadcrumbs

Related code and decisions found in the current codebase:

- `benchmark_test.go` — existing benchmark suite (currently uses synthetic data)
- `generators_test.go` — property-based test generators (hand-crafted distributions)
- `integration_property_test.go` — integration property tests
- `parquet_test.go` — Parquet integration tests
- No `testdata/` directory exists yet — fixture infrastructure would be new

## Notes

Source: https://github.com/simdjson/simdjson/tree/master/jsonexamples

Notable datasets in the simdjson repo include twitter.json (nested, high-cardinality),
github_events.json (mixed types, arrays), and several others covering geographic data,
number-heavy payloads, and deeply nested structures. These exercise different index
code paths (trigram, numeric, null, high-cardinality string) in a single test run.
