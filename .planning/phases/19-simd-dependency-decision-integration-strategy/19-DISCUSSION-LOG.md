# Phase 19: SIMD Dependency Decision & Integration Strategy - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-27
**Phase:** 19-simd-dependency-decision-integration-strategy
**Areas discussed:** Upstream coordination, Shared-lib distribution, API + platform fallback, Stop/fallback condition

---

## Pre-discussion finding (load-bearing)

The 2026-04-21 STACK.md research flagged three blockers: (1) no LICENSE on `pure-simdjson`, (2) no tags, (3) no shipped binaries. Verified live via `gh api` on 2026-04-27 — **all three blockers are resolved**. Upstream now has MIT LICENSE, four published tags (v0.1.0/v0.1.2/v0.1.3/v0.1.4), and pre-built signed binaries for 5 platforms in every release. This collapsed the original "block-or-proceed" axis and reframed every gray area around pinning discipline and integration polish rather than dependency adoption viability.

`pure-simdjson` v0.1.4's `docs/bootstrap.md` revealed a complete built-in distribution story: auto-download on first `NewParser()` from CloudFlare R2 + GitHub fallback, SHA-256 verified, OS-cached with flock, `PURE_SIMDJSON_LIB_PATH` for air-gapped, `PURE_SIMDJSON_BINARY_MIRROR` for corporate, `pure-simdjson-bootstrap` CLI for offline pre-fetch. This further simplified Area 2 to a "delegate or layer" question.

---

## Upstream coordination

### A1.1 — Pin style

| Option | Description | Selected |
|--------|-------------|----------|
| Tag v0.1.4 | `go get github.com/amikos-tech/pure-simdjson@v0.1.4` — readable tag in go.mod, SHA frozen in go.sum. Standard Go convention. | ✓ |
| Pseudo-version SHA | Pin to a specific commit SHA via go.mod replace or pseudo-version. | |
| You decide | Defer to Claude's recommendation. | |

**User's choice:** Tag v0.1.4 (Recommended)
**Notes:** Standard Go convention; `go.sum` freezes the SHA, so immutability is preserved without sacrificing readability.

### A1.2 — Bump cadence

| Option | Description | Selected |
|--------|-------------|----------|
| Pin-and-hold | Stay on v0.1.4 until ami-gin needs a specific fix/feature, or upstream cuts a major-shape change. PR-reviewed. | ✓ |
| Track upstream patch releases | Bump within a minor series (v0.1.x) automatically via Dependabot/Renovate. | |
| You decide | Defer to Claude. | |

**User's choice:** Pin-and-hold (Recommended)
**Notes:** Avoids parity-test churn (Phase 22 goldens recompute on every bump).

### A1.3 — NOTICE policy

| Option | Description | Selected |
|--------|-------------|----------|
| NOTICE.md + README mention | Add a NOTICE.md to ami-gin root mirroring/citing pure-simdjson's NOTICE (covers simdjson Apache-2.0). One-line credit + links in README under Dependencies. | ✓ |
| README/CHANGELOG only | Skip dedicated NOTICE.md; rely on Go module graph + README mention. | |
| Vendor LICENSE+NOTICE files | Copy to ami-gin/third_party/. | |

**User's choice:** NOTICE.md + README mention (Recommended)
**Notes:** Clean OSS-conventional acknowledgement of the Apache-2.0 chain from upstream simdjson C++; doesn't go stale on upstream changes.

### A1.4 — Drift check

| Option | Description | Selected |
|--------|-------------|----------|
| Manual review on bump | Pin-and-hold cadence already gates bumps through PRs; reviewer manually verifies LICENSE+NOTICE haven't changed. | ✓ |
| CI license-check job | Add a GitHub Action that runs go-licenses or similar. | |
| Spot-check only — no guard | Trust the upstream sibling-org coordination. | |

**User's choice:** Manual review on bump (Recommended)
**Notes:** Matches the human-in-the-loop bump policy from A1.2.

---

## Shared-lib distribution

### A2.1 — Layering over upstream bootstrap

| Option | Description | Selected |
|--------|-------------|----------|
| Pure delegation | NewSIMDParser is a thin wrapper around purejson.NewParser(). Docs link to upstream bootstrap.md. | |
| Wrap with friendlier errors | Pure delegation + ami-gin-specific error guidance (PURE_SIMDJSON_LIB_PATH hint) on construction failure. | ✓ |
| Add ami-gin distribution helpers | Reimplement or shadow some of pure-simdjson's bootstrap (own pre-flight CLI, extra env vars). | |

**User's choice:** Wrap with friendlier errors
**Notes:** Pure delegation in spirit, but ami-gin's error message points the developer at the right env var/docs without forcing them to read the upstream traceback first.

### A2.2 — CI bootstrap

| Option | Description | Selected |
|--------|-------------|----------|
| actions/cache + auto-download | GitHub Actions cache keyed on pure-simdjson tag; auto-download exercises the user-path. | ✓ |
| Pre-flight via pure-simdjson-bootstrap CLI | CI step pre-fetches via the upstream CLI then sets PURE_SIMDJSON_LIB_PATH. | |
| Just auto-download every time | No cache, no pre-flight. | |

**User's choice:** Cache via actions/cache + auto-download (Recommended)
**Notes:** Exercises the same code path consumers hit; cache makes warm runs near-zero overhead.

### A2.3 — Consumer documentation

| Option | Description | Selected |
|--------|-------------|----------|
| Tiered: happy-path + ops section | README happy path; separate docs/simd-deployment.md for air-gapped, corporate mirrors, CI, hermetic. Cross-links upstream bootstrap.md. | ✓ |
| Single doc section with all env vars | One README section listing all env vars inline. | |
| Link to upstream only | ami-gin docs say "see pure-simdjson bootstrap docs". | |

**User's choice:** Tiered: happy-path + ops section (Recommended)
**Notes:** Doesn't front-load complexity onto every reader; ops scenarios stay accessible.

### A2.4 — Release-time check

| Option | Description | Selected |
|--------|-------------|----------|
| No release-time check | Bump-PR CI run is sufficient evidence; no extra release step. | ✓ |
| Add a release-time smoke check | Release workflow runs a fresh-cache NewSIMDParser smoke before tagging. | |
| Document a manual pre-release checklist item | Lightweight manual bullet in the release runbook. | |

**User's choice:** No release-time check (Recommended)
**Notes:** Pin-and-hold + bump-PR CI already covers binary reachability.

---

## API + platform fallback

### A3.1 — Constructor shape

User initially declined the question to clarify; Claude provided a 7-axis DX comparison (error locality, actionability, inspection, reuse, test substitution, future extensibility, pattern consistency) showing Option A wins on every axis except line count.

| Option | Description | Selected |
|--------|-------------|----------|
| A: NewSIMDParser() (Parser, error) + WithParser | Two-step. Matches Phase 13 D-02 + existing New* constructors. | ✓ |
| B: WithSIMDParser() one-step option | Single line; loses on every other DX axis. | |
| Both — primitive + sugar | Adds API surface; goes against PROJECT.md "avoid gratuitous API churn". | |

**User's choice:** A: NewSIMDParser() + WithParser (Recommended)
**Notes:** Matches Phase 13 D-02 exactly; max DX wins.

### A3.2 — Load-failure behavior

User asked whether silent fallback to stdlib is good DX or a "surprise" risk. Claude responded with a "where does the surprise actually land" analysis: hard error relocates surprise to construction time (developer can fix in 30 seconds); warn-and-fallback relocates surprise to production observability (developer notices missing perf gain days later via dashboards).

| Option | Description | Selected |
|--------|-------------|----------|
| Hard error + documented fallback recipe | NewSIMDParser returns (nil, err). Caller chooses fallback explicitly via 4-line recipe in docs/simd-deployment.md. | ✓ |
| Warn-and-fallback (silent degradation) | NewSIMDParser logs WARN and returns a stdlib parser. Forces one-size-fits-all. | |
| Configurable per call — default hard error | NewSIMDParser(WithFallbackOnLoadFailure()) opts into fallback. | |

**User's choice:** Hard error + documented fallback recipe (Recommended)
**Notes:** Loud signal at construction; preserves audit trail; explicit caller policy. Matches "fail loudly, recover explicitly" Go-library pattern.

### A3.3 — CI matrix scope

| Option | Description | Selected |
|--------|-------------|----------|
| linux/amd64 + linux/arm64 only | Two simdjson jobs on cheap Linux runners. | |
| All 5 supported platforms | linux-amd64/arm64, darwin-amd64/arm64, windows-amd64. ~3-5x runner cost. | ✓ |
| linux/amd64 only | Single simdjson job. | |

**User's choice:** All 5 supported platforms
**Notes:** User chose maximum coverage over runner cost — strong v1.3 acceptance evidence; catches platform-specific numeric/binding regressions.

### A3.4 — Constructor options surface

| Option | Description | Selected |
|--------|-------------|----------|
| Parameter-free for v1.3 | func NewSIMDParser() (Parser, error). No options. | ✓ |
| Reserve a variadic options slot now | func NewSIMDParser(opts ...SIMDOption) (Parser, error) with zero options defined. | |
| Pass through pure-simdjson's options | Mirror purejson.NewParser(...) options 1:1. | |

**User's choice:** Parameter-free for v1.3 (Recommended)
**Notes:** Ship minimal; extend additively in v1.4 if real callers ask.

---

## Stop / fallback condition

### A4.1 — Stop policy shape

| Option | Description | Selected |
|--------|-------------|----------|
| Tiered by blocker type | Hard blockers halt and defer; soft blockers narrow scope and ship. Switch table in CONTEXT.md. | ✓ |
| Single 'all or nothing' | Any unrecoverable blocker = halt SIMD entirely. | |
| Best-effort — ship whatever lands | Ship partial SIMD with documented limitations. | |

**User's choice:** Tiered by blocker type (Recommended)

### A4.2 — Hard vs soft classification

| Option | Description | Selected |
|--------|-------------|----------|
| Numeric parity fails | HARD: violates Phase 07 BUILD-03; cannot ship correctness-broken. | ✓ |
| Upstream archives or unrecoverable v0.2 break | HARD: defer SIMD to v1.4. | ✓ |
| Speedup below 1.5x on realistic fixtures | SOFT: ship with disclaimer. | (not selected) |
| One platform fails CI, others pass | SOFT: ship with that platform documented as tier 2. | ✓ |

**User's choice:** numeric parity (HARD), upstream archive (HARD), single-platform CI flake (SOFT). Skipped the 1.5x perf threshold option.

### A4.3 — Perf-threshold policy

Re-asked after the user skipped the 1.5x option in A4.2.

| Option | Description | Selected |
|--------|-------------|----------|
| No fixed threshold — decide on Phase 22 evidence | Don't pre-commit. Phase 22 measures, reports, and decides with real numbers. | ✓ |
| Below 1.5x = SOFT | Lock the 1.5x threshold now. | |
| Below 1.5x = HARD | Defer to v1.4 if SIMD doesn't beat stdlib by 1.5x. | |

**User's choice:** No fixed threshold (Recommended)
**Notes:** Avoids hard-coding a threshold without measurement.

### A4.4 — Runbook depth

User pushed back on this question, calling the runbook concept potentially YAGNI/overengineering. Claude re-evaluated and agreed: the original ROADMAP success criterion #3 was conservative when LICENSE/tags/binaries were missing; with those resolved, the switch table from A4.1 + A4.2 is sufficient. No speculative deferral plans or per-blocker rollback steps.

| Option | Description | Selected |
|--------|-------------|----------|
| Switch table is the entire stop condition | A4.1+A4.2 satisfy criterion #3 minimally. No runbook authored. | ✓ |
| Add a one-paragraph 'if HARD: halt and re-plan' note | Sentence-level insurance against ambiguity. | |
| Author the v1.4 deferral plan now | Reverse the pushback; lock concrete steps. | |

**User's choice:** Yes — switch table is the entire stop condition (Recommended)
**Notes:** User correctly applied the "do exactly what is asked" guidance from their developer profile; Claude had drifted into speculative authorship.

---

## Claude's Discretion

Areas where Claude was given latitude (captured in CONTEXT.md `<decisions>` "Claude's Discretion"):

- `Name()` exact value: `"pure-simdjson"` (no version suffix in v1.3)
- Friendlier-error wrap message text format (inline string constant)
- `docs/simd-deployment.md` exact section structure (top-level sections suggested but not mandated)
- CHANGELOG wording for SIMD landing (Phase 21 drafts)
- File location of `parser_simd.go` (root, per Phase 13 D-02 — not revisited)
- Whether to emit an INFO log on SIMD parser construction success (planner's call; not required)

---

## Deferred Ideas

Captured in CONTEXT.md `<deferred>` for future reference:

- `NewSIMDParser(opts ...SIMDOption)` variadic options — defer until real caller asks
- `Name()` versioned format (`"pure-simdjson/v0.1.4"`) — defer until telemetry has concrete need
- CI release-time binary-reachability check — bump-PR CI already covers
- License-drift CI tooling — manual review on bump is sufficient
- Stop-condition concrete deferral runbook — author only if a HARD trigger fires
- `WithSIMDParser()` one-step convenience option — duplicates Phase 13 D-02 seam
- Configurable warn-and-fallback policy — default is hard error, callers wrap
- Per-document SIMD/stdlib failover — out of scope; v1.3 is parser selection at builder construction
