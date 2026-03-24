# GIN Index — Open Source Readiness

## What This Is

GIN Index is a Generalized Inverted Index library for JSON data, designed for row-group pruning in columnar storage (Parquet). It enables fast predicate evaluation to determine which row groups may contain matching documents — filling a gap between full-scan and standing up a database. This project tracks the work needed to take it from a private repo to a credible public open-source release.

## Core Value

A credible first impression: anyone who finds the repo can immediately understand, build, test, and contribute — with no internal artifacts leaking through.

## Requirements

### Validated

- ✓ Core index structures (string, numeric, null, trigram, bloom, HLL) — existing
- ✓ Query evaluation with 12 operators (EQ, NE, GT, GTE, LT, LTE, IN, NIN, IsNull, IsNotNull, Contains, Regex) — existing
- ✓ Binary serialization with zstd compression — existing
- ✓ Field transformers for pre-index value transformation — existing
- ✓ CLI tool for Parquet file operations — existing
- ✓ Property-based tests (gopter) and benchmarks — existing
- ✓ 10 runnable examples covering every feature — existing
- ✓ Functional options pattern throughout — existing

### Active

- [ ] Add MIT LICENSE file
- [ ] Change module path from `github.com/amikos-tech/gin-index` to `github.com/amikos-tech/ami-gin`
- [ ] Remove or scrub internal references (PRD with "Kiba Team" mentions)
- [ ] Add CI pipeline (GitHub Actions: test, lint, build)
- [ ] Fix deserialization security issues (unbounded allocations, version validation)
- [ ] Add CONTRIBUTING.md with build/test instructions
- [ ] Review `.planning/` contents for any sensitive internal references before going public
- [ ] Add CI badge and GoDoc badge to README
- [ ] Document known limitations (OR/AND composites, index merge, query-time transformers)

### Out of Scope

- OR/AND composite queries — future feature work, not blocking OSS launch
- Index merge across multiple indices — future feature, document as known limitation
- Query-time transformers — future feature, document as known limitation
- Web UI or dashboard — this is a library, not a service
- Package manager distribution beyond Go modules — `go get` is sufficient

## Context

- The repo is currently private at `https://github.com/amikos-tech/ami-gin`
- Module path in go.mod is `github.com/amikos-tech/gin-index` — needs to match the actual repo
- All dependencies are permissively licensed (Apache-2.0, MIT, BSD-3) — no conflicts with MIT
- Existing CI is limited to Claude code review workflows — no Go test/lint pipeline
- `gin-index-prd.md` contains internal "Kiba Team" references that need scrubbing
- Codebase has 1.35:1 test-to-source ratio — strong testing foundation
- CONCERNS.md flagged deserialization security issues: unbounded allocations and no version validation

## Constraints

- **License**: MIT — simple, permissive, compatible with all dependency licenses
- **Module path**: Must be `github.com/amikos-tech/ami-gin` to match the GitHub repo URL
- **Go version**: 1.25.5 (already current)
- **No breaking API changes**: Existing API surface is clean — preserve it through the OSS transition

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| MIT license | Simple, permissive, compatible with all dependency licenses | — Pending |
| Module path = repo path (`ami-gin`) | Standard Go convention, reduces confusion | — Pending |
| Remove PRD rather than scrub | Internal planning doc has no value for external contributors | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd:transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-03-24 after initialization*
