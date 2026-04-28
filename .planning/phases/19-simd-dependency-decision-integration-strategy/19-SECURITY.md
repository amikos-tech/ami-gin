---
phase: 19
slug: simd-dependency-decision-integration-strategy
status: verified
threats_open: 0
asvs_level: phase-local
created: 2026-04-27
verified: 2026-04-27
---

# Phase 19 - Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Future dependency introduction -> project license posture | Phase 19 chooses an optional external SIMD dependency that Phase 21 will later add to `go.mod`, README, and NOTICE materials. | Dependency identity, version pin, tag commit, license, and NOTICE obligations |
| Default build consumers -> optional SIMD runtime | Consumers should not receive native-library requirements or behavior changes unless they explicitly opt into the SIMD path. | Build tags, parser constructor choice, native shared-library availability, and parser selection |
| Native-library bootstrap -> public parser API | Upstream shared-library loading can fail because of missing assets, environment configuration, or platform support. | Construction errors, environment variable guidance, and caller fallback policy |
| SIMD correctness evidence -> phase advancement | Later SIMD implementation must not proceed or ship if encoded bytes or query results diverge from stdlib behavior. | Parity results, stop/fallback decisions, and phase pause/deferral records |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T19-01 | Information Disclosure / Compliance | optional `pure-simdjson` dependency | mitigate | Closed: `19-SIMD-STRATEGY.md` records `github.com/amikos-tech/pure-simdjson v0.1.4`, tag commit `0f53f3f2e8bb9608d6b79211ffc5fc7b53298617`, `License: MIT`, root `NOTICE.md` plus README credit ownership in Phase 21, and manual LICENSE/NOTICE review on every bump PR. | closed |
| T19-02 | Denial of Service | default builds and native-library loading | mitigate | Closed: `19-SIMD-STRATEGY.md` keeps SIMD behind `//go:build simdjson`, requires `NewSIMDParser() (Parser, error)` plus explicit `WithParser` selection, and states default stdlib builds remain dependency-free and native-library-free. | closed |
| T19-03 | Repudiation / Denial of Service | SIMD parser construction failure | mitigate | Closed: `19-SIMD-STRATEGY.md` makes construction errors hard, requires the wrapped guidance `initialize pure-simdjson SIMD parser; set PURE_SIMDJSON_LIB_PATH or see docs/simd-deployment.md`, and documents an explicit caller-owned stdlib fallback recipe instead of silent fallback. | closed |
| T19-04 | Tampering | SIMD parity and stop/fallback policy | mitigate | Closed: `19-SIMD-STRATEGY.md` marks non-parity encoded bytes or query results as HARD blockers, requires Phase 21/22 pause and v1.4 deferral recording on HARD triggers, and assigns Phase 22 ownership for parity tests, benchmarks, SIMD CI, distribution guidance verification, and stop-table enforcement. | closed |

*Status: open / closed*
*Disposition: mitigate (implementation required) / accept (documented risk) / transfer (third-party)*

---

## Accepted Risks Log

No accepted risks.

*Accepted risks do not resurface in future audit runs.*

---

## Unregistered Flags

None. `19-01-SUMMARY.md` has no `## Threat Flags` section, and the summary reinforces the same documented controls: build-tagged explicit opt-in, hard construction failure with caller-owned fallback, parity/CI ownership, and documentation-only Phase 19 scope.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-04-27 | 4 | 4 | 0 | Codex inline `gsd-security-auditor` |

### Verification Evidence

- `rg -n "NOTICE|default stdlib|NewSIMDParser|WithParser|No silent fallback|parity|HARD|/gsd-pause-work|PURE_SIMDJSON" .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md` - PASS
- `rg -n "Threat Flags|T19-|Security|threat|HARD|No silent fallback|NOTICE|default stdlib|NewSIMDParser|WithParser|parity|PURE_SIMDJSON" .planning/phases/19-simd-dependency-decision-integration-strategy/19-01-SUMMARY.md .planning/phases/19-simd-dependency-decision-integration-strategy/19-SIMD-STRATEGY.md` - PASS
- Auditor result: `## SECURED` with threat-by-threat evidence in `19-SIMD-STRATEGY.md`; Phase 19 is documentation-only and changed no product code.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-04-27
