# Phase 5: Release - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in `05-CONTEXT.md` — this log preserves the alternatives considered.

**Date:** 2026-04-13
**Phase:** 05-release
**Areas discussed:** Release cut path, Public release surface, Release notes style, Known limitations presentation

---

## Release cut path

| Option | Description | Selected |
|--------|-------------|----------|
| Production tag-push flow | Cut `v0.1.0` through the same tag-triggered GitHub Actions/GoReleaser automation used for future releases | ✓ |
| Same path with explicit rehearsal emphasis | Rehearse on the target SHA, then cut the real tag through the same production flow | |
| One-time bootstrap/manual release | Use a special first-release path, then automate later tags | |

**User's choice:** `1A`
**Notes:** The user locked the production tag-push flow as the real release path, then pointed to `/Users/tazarov/experiments/telia/tclr/tclr-v2` as a reference for release-process rigor. That reference was used to carry forward safety ideas such as clean-worktree checks, commit review, and dry-run/preflight behavior without adopting release branches.

---

## Public release surface

| Option | Description | Selected |
|--------|-------------|----------|
| Library-first release contract | Keep the public release story centered on the Go module and do not treat `cmd/gin-index` as a supported release artifact | ✓ |
| Library-first plus source-install CLI mention | Mention the CLI as a helper install surface without making it a packaged artifact | |
| Dual-surface release | Treat the library and CLI as equally supported release products | |

**User's choice:** `2A`
**Notes:** The CLI exists and is useful, but the user kept Phase 5 scoped to a library-first public release. The discussion explicitly rejected broadening this phase into packaged CLI distribution.

---

## Release notes style

| Option | Description | Selected |
|--------|-------------|----------|
| Maintainer-written notes only | Handwritten release text with little or no generated grouping | |
| Hybrid preface + grouped generated changelog | Short human intro followed by grouped, generated GoReleaser changelog sections | ✓ |
| Pure generated grouped changelog | Fully generated grouped notes with no human framing | |
| GitHub native auto-notes | Use GitHub's generated release-notes system instead of GoReleaser grouping | |

**User's choice:** `3B`
**Notes:** The user explicitly accepted a small amount of release-process research and agreed with a hybrid structure: a concise human preface for first-release context, then grouped generated notes for repeatable automation.

---

## Known limitations presentation

| Option | Description | Selected |
|--------|-------------|----------|
| Brief explanatory section in README | One framing sentence plus three direct bullets in `README.md` | ✓ |
| Blunt bullet list only | Minimal bullets with no framing context | |
| Separate limitations doc | Keep README short and move caveats elsewhere | |
| Collapsible/details block | Hide the limitations inside an expandable section | |

**User's choice:** `4A`
**Notes:** The user chose a visible but concise README treatment. The three limitations remain exactly the pre-identified deferred capabilities: OR/AND composites, index merge, and query-time transformers.

---

## the agent's Discretion

- Exact GoReleaser changelog grouping regexes and workflow naming.
- Exact implementation of the release preflight/dry-run before the real tag push.
- Exact wording and placement of the short `v0.1.0` preface and README limitations section.

## Deferred Ideas

None.
