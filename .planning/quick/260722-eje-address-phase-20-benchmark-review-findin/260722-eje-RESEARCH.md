# Phase 20 benchmark review follow-up research

## Current behavior and failure modes

- `benchmark_test.go` sets `phase20MaxExternalDocumentBytes` to 1 MiB. The upstream `jsonexamples/citm_catalog.json` is currently 1,727,204 bytes (GitHub contents API, 2026-07-22), so `phase20LoadExternalDocuments` rejects a named input that the docs invite contributors to use. The 8 MiB aggregate cap is otherwise sufficient for the current top-level simdjson examples.
- `phase20BenchmarkQuery` only rejects a warm-up result of zero. `GINIndex.resolvePredicatePath` returns `-1` for an unresolved path, and `evaluatePredicate` deliberately turns that into `AllRGs`; therefore an unresolved query passes the warm-up and can be timed as a cheap fail-open fallback.
- The path risk comes from two different construction rules. `builder.go:stageMaterializedValue` appends object keys as `path+"."+key`, while `phase20JSONPathChild` uses bracket notation for keys outside its identifier subset. `resolvePredicatePath` normalizes supported query syntax before looking in `idx.pathLookup`. A key such as `user-name` currently canonicalizes compatibly under `ojg`, but keys containing path separators (for example `a.b`) do not necessarily identify the same indexed path. The benchmark must not rely on incidental normalization behavior.
- `phase20FirstScalarPath` reports only `(path, bool)`: nulls, depth rejection, invalid syntax, and valid-but-unresolved indexed paths all collapse into “no supported non-null scalar leaf.” Its traversal is deterministic through `sortedObjectKeys`, but the actual “first key is null, next key is scalar” branch is not directly covered.
- `phase20ExternalBenchmarkPaths` treats every value other than `1`, including nonempty `true`, as silently disabled. This contradicts the recorded exact-gate decision. The file also duplicates SCREAMING_SNAKE constants and camelCase aliases for the same two environment variable names.
- `phase20ScanLineWithDelimiter` correctly returns a final unterminated record and keeps LF/CRLF bytes in tokens, but neither behavior is explicit in tests/comments. The scanner token-too-long branch and the cross-file decrement in `phase20DiscoverExternalDocuments` are also not directly covered.
- `testdata/phase20/generate_test.go` is a valid package test, but Go intentionally excludes directories named `testdata` from `./...`. Thus the current `Makefile:test` never runs it. The generator has no automated comparison against committed fixtures, creates errors with deprecated `fmt.Errorf(... %w ...)`, and panics from `main` on an ordinary command failure.
- `.planning/STATE.md` conflicts with `.planning/ROADMAP.md`: Phase 21 is the next planned phase, while Phase 999.7 remains an unpromoted backlog item. The v1.3 plan count is 1 (Phase 19) + 2 (Phase 20) = 3, not 34.

## Recommended minimal implementation

### 1. Harden external loading without widening the trust boundary

In `benchmark_test.go`, raise only `phase20MaxExternalDocumentBytes` to `2 * 1024 * 1024`; keep 64 files, 8 MiB aggregate, and depth 64 unchanged. Two MiB accepts the current 1,727,204-byte `citm_catalog.json` with modest headroom and does not turn the local tier into an unbounded loader. Update the numeric limit in `.planning/phases/20-realistic-benchmark-dataset-foundation/20-SECURITY.md` and describe the limits in both Phase 20 README sections.

Change `phase20ExternalBenchmarkPaths` to use only camelCase constants and implement this gate:

- empty/unset: disabled, no directory access;
- exactly `1`: enabled;
- any other nonempty value, including whitespace-padded `1`: configuration error.

Keep regular top-level files only. Improve the empty-discovery message to say that no supported **regular** top-level files were found and symlinks are not followed; this fixes the misleading symlink-only case without adding symlink traversal. Add the requested why-comment above `phase20ScanLineWithDelimiter`: it preserves delimiters so aggregate byte accounting includes LF and CRLF.

### 2. Make unresolved predicates impossible to time

Do not change the builder's global path model in this quick task; that would be a broader compatibility/collision decision. Instead:

1. Let an external `phase20BenchmarkFixture` carry documents with an empty predicate until the Query action has built its index.
2. Change `phase20DeriveScalarPathPredicate` (and the recursive helper) to accept the finished `*GINIndex`. Traverse scalar candidates deterministically, canonicalize supported JSONPath syntax, call `idx.findPath`, and return the resolved `PathEntry.PathName`. Skip null, syntactically unsupported, and valid-but-unresolved candidates while retaining the first rejection reason for the final error. This both derives an actual indexed canonical path and can continue to a later safe scalar.
3. Before `b.ResetTimer`, have `phase20BenchmarkQuery` call `idx.resolvePredicatePath` for both smoke and external predicates and fail if the returned path ID/entry is invalid. Keep the nonzero warm-up assertion, but do not require fewer than all row groups: an `IsNotNull` field may legitimately occur in every row group.

This double check is intentional: derivation selects a resolvable external candidate, while the Query preflight guarantees no future fixture can bypass resolution and time `AllRGs` fallback.

Return a contextual final error when no candidate works: distinguish “only null/no scalar,” “depth exceeded 64,” invalid path syntax, and “candidate did not resolve in the built index.” A simple `(path, ok, firstRejection error)` recursive result is enough; no new public type is needed.

### 3. Make fixture generation self-verifying and part of the normal test target

Refactor `generate.go` just enough to expose one ordered `generatedFixtures()` result used by `main` and tests. An ordered slice also makes failure order stable. Add a test that renders each generated record set with the same final newline and byte-compares it with the committed JSONL file; report the fixture name and tell the contributor to rerun the documented generator. This is deterministic drift detection without subprocesses or hashes that must be refreshed manually.

In `Makefile:test`, use gotestsum's space-separated package list:

```text
--packages="./... ./testdata/phase20"
```

This syntax was verified locally and runs `TestWriteFixtureToDirRejectsSymlink`; `./...` alone does not discover that package.

Keep `fmt` for record formatting, add `github.com/pkg/errors`, use `errors.Errorf` for new formatted errors and `errors.Wrapf` for OS failures. Replace `panic(err)` with a small `run() error`/`main` boundary that writes one contextual error to stderr and exits nonzero. No subprocess test is needed because write failures and deterministic output are covered below the boundary.

### 4. Focused tests and documentation cleanup

Extend the existing Phase 20 tests rather than adding a new framework:

- gate table: empty is disabled; `0`, `true`, `yes`, and ` 1 ` are errors; exactly `1` proceeds;
- external discovery: symlink-only supported filename yields the explicit regular-file/symlink message (skip only if the platform cannot create a symlink);
- JSONL loader: a valid final record without newline is returned and its exact bytes are charged; an over-2-MiB JSONL token reaches the scanner-too-long error path; LF and CRLF accounting remains covered;
- aggregate budget: exercise two or more files with a deliberately small injected budget by extracting the existing path loop into a tiny budget-parameter helper; this avoids an 8+ MiB unit fixture while testing the real decrement logic;
- scalar traversal: sorted first scalar is null and the next scalar is chosen; `user-name` resolves to the actual indexed path; an unresolved separator-containing key is skipped in favor of a later scalar; unresolved-only and over-depth inputs return the specific rejection; an explicitly missing benchmark predicate fails preflight;
- generator package: symlink protection plus exact generated-vs-committed fixture bytes.

In `testdata/phase20/README.md`, replace “A future optional local run uses” with present tense and state that the enable flag must be exactly `1`. Mirror the exact-`1` statement in the root `README.md`. Remove the `Current bytes` column rather than parsing or separately drift-testing prose metadata: exact content is enforced by the generator drift test, while document counts and caps are already executable contracts in `TestPhase20SmokeFixtures`.

Finally, update only the inconsistent current-position fields in `.planning/STATE.md`: `stopped_at`/Current focus/Phase should point to Phase 21, and `progress.completed_plans` should be 3. Leave Phase 999.7 in the ROADMAP backlog.

## Risks and verification

- Raising the per-document cap also raises the scanner token ceiling; the unchanged 8 MiB total cap bounds aggregate memory/input. Document the 2 MiB value everywhere that currently says 1 MiB.
- Do not “fix” special-key indexing globally here. Explicit resolution plus candidate skipping makes the benchmark honest without changing serialized/index semantics.
- Tests should assert contextual wrapping rather than the exact standard-library scanner error string, which is not a stable exported sentinel.

Run after implementation:

```bash
go test ./... -run '^TestPhase20' -count=1
go test ./testdata/phase20 -count=1
make test
go test ./... -run '^$' -bench '^BenchmarkPhase20RealisticJSON/tier=smoke' -benchtime=1x -count=1 -benchmem
GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL=1 GIN_PHASE20_SIMDJSON_DIR=/path/to/simdjson/jsonexamples go test ./... -run '^$' -bench 'BenchmarkPhase20RealisticJSON/tier=external/fixture=local-example' -benchtime=1x -count=1 -benchmem
go run ./testdata/phase20/generate.go && git diff --exit-code -- testdata/phase20/*.jsonl
```

Current focused Phase 20 tests and the explicit generator-package test pass before changes; the missing coverage is discovery by the standard target and the review branches above.
