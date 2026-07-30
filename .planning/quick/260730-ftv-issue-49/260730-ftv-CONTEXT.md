# Quick Task 260730-ftv: Resolve issue #49 - Context

**Gathered:** 2026-07-30
**Status:** Ready for planning

<domain>
## Task Boundary

Resolve issue #49 by eliminating the operational `pure-simdjson` bootstrap
version pin outside `go.mod`, and review the remaining version-specific
nesting-limit statement in `docs/simd-deployment.md`.

</domain>

<decisions>
## Implementation Decisions

### CI version source
- Derive the bootstrap command version from the `pure-simdjson` module version
  recorded in `go.mod`, using `go list`.
- Do not add a `tools.go` file or retain a duplicate pin with a drift guard.
- Prefer the smallest change that makes `go.mod` the operational source of
  truth.

### Single-source scope
- The "one file per dependency bump" requirement applies to operational pins
  that must remain synchronized for builds or CI.
- Intentional release-specific legal attribution and verified behavioral
  documentation may remain versioned when the version itself carries meaning.
- Any documentation or other reference that contradicts the selected
  `go.mod` version or the actual dependency behavior must be fixed.

### Existing documentation work
- Treat the general stale-reference cleanup already merged in change #50 as
  complete.
- Revisit only the remaining version-specific nesting-limit statement in
  `docs/simd-deployment.md`.
- Verify that statement against the current dependency before deciding whether
  it should remain release-specific or be corrected.

### the agent's Discretion
- Exact shell variable naming and quoting in the CI workflow.
- Verification coverage needed to prove that the bootstrap version is derived
  correctly without adding redundant production code.
- Final wording of the nesting-limit statement, subject to the decisions above.

</decisions>

<specifics>
## Specific Ideas

Use the issue's minimal derivation pattern: obtain the module version with
`go list -m -f '{{.Version}}' github.com/amikos-tech/pure-simdjson`, then invoke
the bootstrap command at that exact version.

</specifics>

<canonical_refs>
## Canonical References

- Issue #49 defines the drift problem and acceptance criteria.
- Change #50 is the existing documentation cleanup and should not be repeated.
- `go.mod` is the authority for the operational `pure-simdjson` version.

</canonical_refs>
