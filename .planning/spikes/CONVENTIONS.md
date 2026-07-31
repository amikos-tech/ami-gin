# Spike Conventions

Patterns established across spike sessions. New spikes follow these unless the question requires otherwise.

## Stack

- **Go** (matches the project). Spikes that exercise an external Go dependency live in their **own isolated module** (`module <name>spike` + local `go.mod`) inside the spike directory, so they never mutate the repo's `go.mod`-of-record — important when the dependency itself is a decision owned by an upcoming phase.
- When a spike needs **ami-gin itself**, keep the isolated module and add `replace github.com/amikos-tech/ami-gin => ../../..`. Verify afterwards that `git status --porcelain go.mod go.sum` is clean at the repo root. Note that the spike module resolves its own transitive versions, so state explicitly in the README why that does not affect the conclusions (Spike 002: both arms of every comparison use the same ami-gin build).
- Prefer driving the **public API** so the spike can stay in its own module. Reach for package-internal access only when the question genuinely cannot be asked from outside — and say so.

## Structure

- One directory per spike: `.planning/spikes/NNN-descriptive-name/`.
- Fact-finding spikes (does it classify X? does it load?) use stdout verification, not a UI — the answer is a fact, not a feeling.
- Multiple files per spike when the investigation forks: `harness_test.go` for the original questions, `followup_test.go` for reframed ones, `depth_test.go` for an unplanned discovery. The filenames themselves document the trail.
- Go-test-driven spikes (`TestQN_...` + `t.Logf` measurements) are preferred over `main.go` when the question is about the project's own code, because they get the toolchain's timeout, race, and fuzzing machinery for free.
- **Do not commit build artifacts.** Spike 001 committed an 8.6 MB binary; do not repeat that.

## Patterns

- **Capture verbatim API from the module cache** (`$GOMODCACHE/<mod>@<ver>/`) before writing code, so the spike compiles against reality and simultaneously documents the real signatures.
- Name sentinel errors explicitly in output (`errors.Is` against exported sentinels) rather than printing opaque error strings.
- **Distrust a test that passes on the first try.** Spike 002's Q2 passed *vacuously* — the chosen "poison" payload never poisoned anything, so the test proved something weaker than its own name claimed. Verify the precondition actually fired before believing the conclusion.
- **Measure, don't infer.** When something looks non-linear, sweep the parameter and print a table rather than reasoning from a stack trace. Spike 002's Q5 turned "this seems slow" into "O(2^depth), blows 3s at depth 18, 37 bytes".
- **A/B the parameters a spike recommends.** Recommending "add a guard" is weak; recommending "depth ≤ 8, because depth 12 starved the fuzzer to 0 execs/sec and depth 8 quadrupled corpus growth" is actionable.
- Record findings that are **out of scope for the commissioning phase** in the README's Results section and the MANIFEST requirements, explicitly flagged as backlog — so they are neither lost nor allowed to become scope creep.

## Tools & Libraries

- `github.com/amikos-tech/pure-simdjson` — loads its native lib at runtime via `github.com/ebitengine/purego` (no CGo); no build tag required by upstream. Pin currently `v0.1.7` (was `v0.1.4` at Spike 001). Always derive the version from `go list -m`, never a literal.
- `PURE_SIMDJSON_WARN_LEAKS=1` surfaces finalizer-based native handle leak warnings on stderr (`finalizer_prod.go:48`). Set it on any spike that churns parsers or documents.
- Go native fuzzing (`go test -fuzz`) works fine against an isolated spike module. Watch the `execs/sec` column — sustained `0/sec` means individual inputs are pathological, not that fuzzing is idle.
