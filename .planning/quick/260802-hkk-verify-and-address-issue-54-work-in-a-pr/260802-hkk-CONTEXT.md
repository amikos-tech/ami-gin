# Quick Task 260802-hkk: verify and address issue #54 (work in a PR branch) - Context

**Gathered:** 2026-08-02
**Status:** Ready for planning

<domain>
## Task Boundary

Verify and address issue #54: deeply nested arrays cause exponential work in `stageMaterializedValue`. Work on a dedicated PR branch.

</domain>

<decisions>
## Implementation Decisions

### Wildcard behavior
- Preserve all supported public wildcard-query behavior, such as `$.orders[*].id`.
- Internal numbered/wildcard path combinations do not need to be retained when they are unnecessary to provide that public behavior.

### Safety boundary
- Add an opt-in, configurable staged-path budget as defense in depth.
- The default must preserve existing behavior; callers handling untrusted JSON can configure a limit and receive a clear ingest error rather than risking excessive resource use.

### Scope
- Keep the PR focused on the amplification fix plus regression and wildcard-behavior tests.
- Do not broaden it into parser nesting-limit alignment or unrelated documentation changes.

### the agent's Discretion
- Select the smallest compatible implementation and precise configuration/API shape.

</decisions>

<specifics>
## Specific Ideas

- Tests should demonstrate that deeply nested arrays no longer grow exponentially in work or allocations.
- Tests should demonstrate that wildcard queries continue to return the expected row groups.

</specifics>

<canonical_refs>
## Canonical References

- GitHub issue #54: `[PERF] stageMaterializedValue is O(2^depth) for nested arrays: 33-byte document allocates 1.41 GB`.

</canonical_refs>
