---
created: 2026-08-02T06:33:58.366Z
title: Bind required SIMD checks in ruleset
area: planning
files:
  - .github/workflows/ci.yml
  - .planning/phases/22-simd-validation-benchmarks-ci/22-08-PLAN.md:15
  - .planning/phases/22-simd-validation-benchmarks-ci/22-08-PLAN.md:88
---

## Problem

Phase 22 Plan 22-08 is blocked because the active default-branch ruleset has no required status-check rule. PR #55 and CI run 30734887913 prove that the five-platform SIMD matrix runs at head `57972ab131360c4c5ad178c09cf74471cb3ab95c`, but neither required SIMD context currently blocks merge.

The follow-up must bind exactly these two checks:

- `SIMD (linux/amd64, required)`
- `SIMD (darwin/arm64, required)`

The three advisory SIMD contexts must remain non-required, unrelated rules and required checks must remain unchanged, and PR #55 must stay open and unmerged until Plan 22-08 completes.

## Solution

An authorized repository administrator should update the active default-branch ruleset to require exactly the two named SIMD contexts. Then inspect the rules read-only, confirm the binding, and resume `$gsd-execute-phase 22` so Plan 22-08 can complete Task 2 and begin the final controlled-evidence approval checkpoint.
