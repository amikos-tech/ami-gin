---
phase: 08
phase_name: "adaptive-high-cardinality-indexing"
project: "GIN Index — v1.0 Query & Index Quality"
generated: "2026-04-17T00:00:00Z"
counts:
  decisions: 5
  lessons: 4
  patterns: 5
  surprises: 4
missing_artifacts: []
---

# Phase 08 Learnings: adaptive-high-cardinality-indexing

## Decisions

### Adaptive Mode Uses Its Own Index Structure
Adaptive high-cardinality paths use a dedicated `AdaptiveStringIndex` map plus `FlagAdaptiveHybrid` instead of reusing bloom-only state.

**Rationale:** Keeping adaptive mode explicit makes the three string-path modes distinct and avoids overloading bloom-only behavior.
**Source:** 08-01-SUMMARY.md

---

### Promotion Is Based on Row-Group Coverage
Hot-term promotion is ranked by row-group coverage, while promoted terms are stored lexically for query-time binary search.

**Rationale:** Row-group coverage reflects pruning value better than raw document frequency, and lexical ordering keeps lookup efficient.
**Source:** 08-01-SUMMARY.md

---

### Adaptive State Gets an Explicit Wire-Format Section
Adaptive config and per-path metadata were persisted in an explicit version 5 format section between string indexes and string-length indexes.

**Rationale:** The format change needed to stay explicit, grouped with related string structures, and round-trip safely through encode/decode.
**Source:** 08-02-SUMMARY.md

---

### CLI Mode Reporting Comes From Path Flags and Metadata
`gin-index info` derives path mode from flags and appends compact adaptive counters from per-path metadata plus header/config thresholds.

**Rationale:** This keeps CLI output mode-aware without adding a separate diagnostics model or exposing oversized per-path detail.
**Source:** 08-02-SUMMARY.md

---

### Benchmarks Compare Modes on One Shared Fixture Family
The benchmark harness reuses one deterministic skewed fixture family across exact, bloom-only, and adaptive-hybrid modes and reports `candidate_rgs` and `encoded_bytes` directly.

**Rationale:** Using the same fixture isolates config-driven behavior changes and measures pruning recovery and size tradeoffs directly instead of inferring them from latency alone.
**Source:** 08-03-SUMMARY.md

---

## Lessons

### Threshold Properties Must Evolve With Mode Semantics
Once adaptive-hybrid mode exists, a cardinality-threshold property can no longer assume every threshold breach means bloom-only.

**Context:** Broader verification in Plan 01 exposed a stale pre-adaptive threshold property, and the planned property rewrite was required to bring verification back to green.
**Source:** 08-01-SUMMARY.md

---

### Verification Reliability Depends on Runtime Behavior
Full-suite verification may need a PTY-backed rerun in this runtime even when the command itself is correct.

**Context:** The exact `go test ./... -count=1` command returned no exit record in a silent non-PTY run, but completed green when rerun with a PTY.
**Source:** 08-02-SUMMARY.md

---

### Benchmark Work Needs a Clean Baseline
Stray RED-only tests from another plan can block unrelated verification even when the target benchmark work is correct.

**Context:** Plan 03 had to remove unfinished 08-02 serialization and CLI tests that were ahead of the requested base commit before benchmark verification could run against the intended scope.
**Source:** 08-03-SUMMARY.md

---

### Slow Test Suites Need Explicit Progress Signals
Long-running property and serialization tests can look hung unless verification captures visible progress.

**Context:** Repo-wide verification in Plan 03 was intentionally slow enough that JSON log capture was used to confirm forward movement rather than treating the run as stalled.
**Source:** 08-03-SUMMARY.md

---

## Patterns

### Three-Mode Finalize Pattern
When a string path breaches the cardinality threshold and adaptive mode is enabled, finalize builds promoted exact terms plus non-promoted hash buckets instead of collapsing directly to bloom-only.

**When to use:** Use this when high-cardinality string paths need to recover hot-value pruning while keeping a bounded fallback for the long tail.
**Source:** 08-01-SUMMARY.md

---

### Safe Adaptive Query Pattern
Adaptive query evaluation follows bloom reject, then string-length reject, then exact promoted lookup or lossy bucket fallback.

**When to use:** Use this when you need to preserve fast reject paths and keep positive lookups false-negative-free while still distinguishing exact from lossy results.
**Source:** 08-01-SUMMARY.md

---

### Split Global and Per-Path Serialization State
Persist global adaptive knobs in `SerializedConfig` and per-path adaptive state in a dedicated binary section.

**When to use:** Use this when adding a feature-specific wire-format extension that needs explicit format evolution without inflating the generic config payload.
**Source:** 08-02-SUMMARY.md

---

### Separate CLI Loading From Rendering
Move CLI info rendering into an `io.Writer`-based helper instead of coupling formatting to file-loading flow.

**When to use:** Use this when CLI output needs local tests that should not depend on S3, remote paths, or other environment plumbing.
**Source:** 08-02-SUMMARY.md

---

### Make Benchmark Labels and Claims Explicit
Use `mode=`, `shape=`, and `probe=` labels and fail benchmark setup if the core pruning claim regresses.

**When to use:** Use this when a benchmark is meant to defend a specific behavior claim over time, not just print performance numbers.
**Source:** 08-03-SUMMARY.md

---

## Surprises

### Old Threshold Coverage Broke the First Broad Verification Pass
The first broad verification in Plan 01 surfaced a stale threshold property that still reflected the pre-adaptive bloom-only cliff.

**Impact:** Task 3 had to rewrite the threshold property contract before the wider adaptive verification could pass cleanly.
**Source:** 08-01-SUMMARY.md

---

### Non-PTY Test Runs Were Not Trustworthy in This Runtime
The same full-suite verification command behaved differently depending on whether it ran under a PTY.

**Impact:** Verification procedure had to change even though the underlying code and command stayed the same.
**Source:** 08-02-SUMMARY.md

---

### The Benchmark Branch Was Ahead of Its Intended Base
Plan 03 unexpectedly inherited unfinished 08-02 RED-only tests from the branch baseline.

**Impact:** Two blocking cleanup commits were needed before the benchmark smoke command could reflect only the planned benchmark work.
**Source:** 08-03-SUMMARY.md

---

### Full-Suite Verification Looked Stalled Because It Was Genuinely Slow
Several property-based and serialization tests took long enough to resemble a hang during Plan 03 verification.

**Impact:** The verification approach shifted to JSON log capture so progress stayed observable during the slow repo-wide pass.
**Source:** 08-03-SUMMARY.md
