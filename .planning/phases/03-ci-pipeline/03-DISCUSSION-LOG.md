# Phase 3: CI Pipeline - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-11
**Phase:** 03-ci-pipeline
**Areas discussed:** CI entrypoints, Merge gates, Matrix scope, Coverage reporting

---

## CI entrypoints

| Option | Description | Selected |
|--------|-------------|----------|
| A | Makefile-first | |
| B | Workflow-inline | |
| C | Hybrid: `make` for local verbs, workflow YAML for matrix, SARIF, Codecov, and schedules | ✓ |

**User's choice:** `C`
**Notes:** Additional clarification recorded after selection: use the default GitHub runner group for CI jobs rather than any custom or self-hosted runner group.

---

## Merge gates

| Option | Description | Selected |
|--------|-------------|----------|
| A | Core-only required | |
| B | Balanced release gate: require lint, build, and test matrix; keep govulncheck and Codecov advisory | |
| C | Security-enforced gate | ✓ |
| D | Coverage-threshold gate | |

**User's choice:** `C`
**Notes:** User clarified that any `govulncheck` finding on a pull request should block merge.

---

## Matrix scope

| Option | Description | Selected |
|--------|-------------|----------|
| A | Test matrix only on Go `1.25` and `1.26`; build, lint, and security once on Go `1.26` | ✓ |
| B | Test + build matrix; lint and security once | |
| C | Broad matrix for everything | |

**User's choice:** `A`
**Notes:** Chosen to keep the PR signal clean while still covering cross-version test compatibility.

---

## Coverage reporting

| Option | Description | Selected |
|--------|-------------|----------|
| A | Codecov upload + badge only | |
| B | Codecov + GitHub artifacts (`coverage.out`, `unit.xml`) | ✓ |
| C | Codecov + artifacts + required `codecov/patch` | |
| D | Codecov + artifacts + hard `codecov/project` | |

**User's choice:** `B`
**Notes:** Coverage should be visible and inspectable, but not turned into a threshold gate in this phase.

---

## the agent's Discretion

- exact workflow file split
- exact job names
- exact artifact retention settings
- exact implementation of PR-blocking `govulncheck` and weekly Code Scanning upload

## Deferred Ideas

None.
