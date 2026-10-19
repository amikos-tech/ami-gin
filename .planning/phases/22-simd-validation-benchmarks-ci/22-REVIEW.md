---
phase: 22-simd-validation-benchmarks-ci
reviewed: 2026-10-19T09:36:17Z
depth: standard
files_reviewed: 14
files_reviewed_list:
  - .github/workflows/ci.yml
  - .goreleaser.yml
  - Makefile
  - benchmark_simd_test.go
  - benchmark_test.go
  - docs/simd-deployment.md
  - parser_parity_fuzz_simd_test.go
  - parser_parity_phase20_simd_test.go
  - parser_parity_simd_test.go
  - parser_parity_test.go
  - parser_simd_integration_test.go
  - simd_docs_guard_test.go
  - simd_example_test.go
  - simd_workflow_contract_test.go
findings:
  critical: 0
  warning: 4
  info: 10
  total: 14
status: issues_found
---

# Phase 22: Code Review Report

**Reviewed:** 2026-10-19T09:36:17Z
**Depth:** standard
**Files Reviewed:** 14
**Status:** issues_found

## Summary

Phase 22 added SIMD parity tests, a differential fuzz harness with a seed corpus, a documentation drift guard, paired stdlib/SIMD benchmarks, a workflow contract test, and a five-leg SIMD CI matrix. The scope was reviewed at HEAD (`4d573e5`), which includes later commits layered on top of the Phase 22 commit (`c7f9b58`).

What was verified, not assumed:

- `go vet .` and `go vet -tags simdjson .` are clean.
- The untagged guard tests (`TestSIMDWorkflowContract`, `TestSIMDDocumentationContract` with `AMI_GIN_REQUIRE_SIMD_DOCS_UPSTREAM=1`, stdlib parity and golden tests) pass locally.
- The tagged parity, fuzz-seed, policy, and golden tests pass locally and actually execute (not skip) on darwin/arm64.
- The CI library path `${cache}/${version}/${goos}-${goarch}/${library}` matches upstream `artifactCachePath` (`<cacheDir>/v<Version>/<goos>-<goarch>/<libname>` with `Version = "0.1.7"` and `go list -m` yielding `v0.1.7`).
- The docs guard pin `v0.1.7` in `docs/simd-deployment.md:105` matches `go.mod`.
- Fuzz seed files match the fixture documents they claim to mirror.

No correctness defect in the parity or fuzz logic was found. The main concern is a CI quality-gate gap: golangci-lint never compiles the `simdjson` build graph, so the whole set of tagged files added in this phase (and `parser_simd.go`) is unlinted. Running the configured linter with `--build-tags simdjson` reports 6 issues today, one of them inside a Phase 22 file. The remaining findings are robustness nits in the workflow and test harnesses.

## Warnings

### WR-01: golangci-lint never covers the `simdjson` build graph; tagged lint fails today

**File:** `.github/workflows/ci.yml:79-83`, `Makefile:168-169`, `.golangci.yml` (`run:` has no `build-tags`)
**Issue:** The lint job runs `golangci-lint --config=.golangci.yml` with no build tags. Every file added by this phase under `//go:build simdjson` (`benchmark_simd_test.go`, `parser_parity_fuzz_simd_test.go`, `parser_parity_phase20_simd_test.go`, `parser_parity_simd_test.go`, `parser_simd_integration_test.go`, `simd_example_test.go`) plus `parser_simd.go` is excluded from the lint gate while the same configuration is enforced on the stdlib graph. This is asymmetric with `security-scan`, which the phase deliberately extended to run `govulncheck -tags simdjson`. Running `GOTOOLCHAIN=go1.25.5 golangci-lint@v2.11.4 run --build-tags simdjson ./...` locally reports 6 issues (2 `errorlint`, 1 `goconst`, 1 `gocritic`, 2 `nilerr`). One is in a reviewed file: `parser_simd_integration_test.go:144` (`gocritic` unlambda on the `func() (CloseableParser, error) { return NewSIMDParser() }` closure). Note that the obvious unlambda fix (`NewSIMDParser` as a function value) would break `validateSIMDConstructorCalls`, which counts `CallExpr` nodes and requires exactly one raw helper call, so that site needs a scoped `//nolint:gocritic // policy requires a call expression` rather than the mechanical fix.
**Fix:** Add a tagged lint pass alongside the default one, and resolve the 6 reported issues (or add scoped `nolint` with a reason where the constructor-call policy requires the current shape):
```yaml
      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v9
        with:
          version: v2.11.4
          args: --config=.golangci.yml

      - name: Run golangci-lint (simdjson)
        uses: golangci/golangci-lint-action@v9
        with:
          version: v2.11.4
          args: --config=.golangci.yml --build-tags simdjson
```
and in the Makefile:
```make
lint: check-validator-markers check-notice-version check-simd-docs
	$(GOLANGCI_LINT_CMD) run
	$(GOLANGCI_LINT_CMD) run --build-tags simdjson
```

### WR-02: Explicit-path smoke step uses `always()`, so it runs on cancelled jobs

**File:** `.github/workflows/ci.yml:205`
**Issue:** `if: ${{ always() && steps.simd-library.outcome == 'success' }}` makes the step run even when the workflow run is cancelled. The intent (run the smoke after a tagged-suite failure) only needs "not cancelled". With `-timeout=30m` and `-race` on required legs, a cancelled run keeps a runner busy for a full compile-and-test cycle. GitHub's own guidance is to use `!cancelled()` for this pattern. `simd_workflow_contract_test.go:124` pins the exact `always()` string, so the contract test must change with it.
**Fix:**
```yaml
        if: ${{ !cancelled() && steps.simd-library.outcome == 'success' }}
```
and update the anchor at `simd_workflow_contract_test.go:124` to match.

### WR-03: Docs-guard drift subtests rely on single-occurrence `strings.Replace`; a second link makes them fail spuriously

**File:** `simd_docs_guard_test.go:100,107,114,121,128,135`
**Issue:** Each "rejects ... drift" subtest mutates one occurrence (`strings.Replace(..., 1)`) and then asserts that `validateSIMDDocumentationInputs` fails. The validator only checks `strings.Contains`. If `README.md` or `CHANGELOG.md` ever gains a second `](docs/simd-deployment.md)` link (a very likely future edit for the changelog), the mutation removes only the first occurrence, validation still passes, and the subtest fails with "documentation mutation error = <nil>" even though nothing drifted. The same applies to the parser-source, release, bootstrap-pin and example-body mutations. This is a test-reliability trap that will present as an unrelated CI failure on a docs PR.
**Fix:** Use `strings.ReplaceAll` in every mutation so the test asserts "no occurrence survives", e.g.:
```go
in.readme = strings.ReplaceAll(in.readme, "](docs/simd-deployment.md)", "](docs/simd-moved.md)")
in.changelog = strings.ReplaceAll(in.changelog, "](docs/simd-deployment.md)", "](docs/simd-moved.md)")
```
Apply the same to lines 82-86, 100, 121, 128 and 135.

### WR-04: `buildFuzzParserOutcome` depends on composite-literal evaluation order for correctness

**File:** `parser_parity_fuzz_simd_test.go:94-97`
**Issue:**
```go
outcome := fuzzBuildOutcome{
    err:         builder.AddDocument(DocID(0), doc),
    softSkipped: builder.NumSoftSkippedDocuments(),
}
```
The `softSkipped` value is only correct because Go evaluates function calls in a composite literal left-to-right in source order. Swapping the two fields (or a `gofmt`/`gci`-style struct-field reorder) silently reads the counter before the document is ingested, which turns every soft-skip into `softSkipped: 0` and reclassifies known asymmetries as `rejection_agreement`/`unexpected_one_sided_commit`. The harness would then fail on its own seed corpus, or worse, hide real divergences. This is the central classification input of the fuzz harness and should not depend on literal ordering.
**Fix:**
```go
addErr := builder.AddDocument(DocID(0), doc)
outcome := fuzzBuildOutcome{
    err:         addErr,
    softSkipped: builder.NumSoftSkippedDocuments(),
}
```

## Info

### IN-01: `release` matrix key is never consumed by the workflow

**File:** `.github/workflows/ci.yml:120,128,136,144,152`
**Issue:** `matrix.release` is declared on all five rows but no step references it (`grep matrix.release` returns nothing). It is only read by `simd_workflow_contract_test.go:41-45`, which pins values the workflow does not use. Dead configuration that drifts silently.
**Fix:** Either use it (for example, in the job name or the cache key) or remove the key from both the workflow and the contract test.

### IN-02: Support-policy table introduces an undefined "tier 2"

**File:** `docs/simd-deployment.md:268`
**Issue:** `darwin/amd64` is labelled "Advisory (tier 2)" while the other advisory rows say "Advisory". The workflow has exactly two tiers (`required`/`advisory`) and the contract test asserts a 2/3 split; "tier 2" is not defined anywhere.
**Fix:** Use "Advisory" consistently, or define the term in the surrounding text.

### IN-03: `loadingMutations.field` is dead

**File:** `simd_docs_guard_test.go:66-87`
**Issue:** Each `loadingMutations` entry sets `field: &inputs.libraryLoading` etc., but the loop ignores `mutation.field` and re-dispatches on `mutation.name` in a `switch`. The pointer is never dereferenced, and it would be wrong anyway because `mutated := inputs` copies the struct, so the pointer targets the unmutated original.
**Fix:** Drop `field` and keep the `switch`, or replace the `switch` with a `mutate func(*simdDocumentationInputs)` like the second table at lines 92-139.

### IN-04: Pure-unit subtests are skipped when the upstream module is absent

**File:** `simd_docs_guard_test.go:156-179,216-223`
**Issue:** `loadSIMDDocumentationInputs` is called before any subtest, so `t.Skipf` at line 222 skips the whole test, including "versioned replacement is effective" and "unversioned local replacement fails closed", which only exercise `effectiveSIMDModule` and need no upstream source.
**Fix:** Move the two `effectiveSIMDModule` subtests into their own `TestEffectiveSIMDModule`, or call `loadSIMDDocumentationInputs` inside the subtests that need it.

### IN-05: Unexplained split string literal

**File:** `simd_docs_guard_test.go:314`
**Issue:** `"https://github.com/amikos-tech/" + "pure-simdjson/blob/"` mirrors the trick in `notice_version_guard_test.go:14` that dodges the `simd-isolation-check` grep for the quoted import path. That grep only matches `"github.com/amikos-tech/pure-simdjson"` with surrounding quotes, which a URL literal never produces, so the split here has no effect and reads as an accident. Where the split is load-bearing (the module constant) it has no comment either.
**Fix:** Join the literal here, and add a one-line comment on `pureSIMDJSONModule` explaining why it is split.

### IN-06: Stale `help` text for `lint`

**File:** `Makefile:197`
**Issue:** `lint - Run validator marker checks and golangci-lint` no longer describes the target; it also runs `check-notice-version` and `check-simd-docs` (line 168).
**Fix:** `lint - Run validator marker, NOTICE and SIMD docs checks, then golangci-lint`.

### IN-07: Redundant loop-variable copies under Go 1.25

**File:** `benchmark_simd_test.go:37,48`; `parser_parity_phase20_simd_test.go:11,66`; `parser_parity_test.go:66,78,146,396`; `parser_simd_integration_test.go` (`fx := fx` in `TestSIMDParserGoldenAuthoredFixtures`)
**Issue:** `go.mod` declares Go 1.25.5, so per-iteration loop variables are the default and `arm := arm` / `fx := fx` / `fixture := fixture` / `testCase := testCase` / `c := c` are no-ops that suggest a capture hazard that no longer exists.
**Fix:** Remove the shadowing copies.

### IN-08: Silent double-drop is reported as `rejection_disagreement`

**File:** `parser_parity_fuzz_simd_test.go:154-157,175-181,215-218`
**Issue:** When both arms return `committed=false, err=nil, softSkipped=0` (a document that vanished without a trace), `classifyFuzzParserOutcomes` yields `rejection_agreement`, `fuzzRejectionsAgree` returns `false`, and the failure line reads `class=rejection_disagreement`. The harness correctly fails, but the recorded class is misleading when triaging fuzz crashers because the arms did agree; neither rejected.
**Fix:** Add a `fuzzOutcomeSilentDrop` class (or an explicit message) for the "both arms produced nothing" case before falling through to the signature comparison.

### IN-09: Property tests swallow error details

**File:** `parser_parity_test.go:165-173,183-191,201-209,227-235,245-253,263-271`
**Issue:** Every property returns `false` on `encodeDocs`/`encodeDocsWithParser` error, so a gopter failure reports only the shrunk input and not the wrapped error that caused it.
**Fix:** Use `prop.ForAll` with a `func(...) (bool, error)` or `gopter.NewPropResult`-style return so the error text is part of the failure report.

### IN-10: Windows leg composes a backslash `runner.temp` with forward-slash segments

**File:** `.github/workflows/ci.yml:176,182,188-192,196,207`
**Issue:** On `windows-latest`, `${{ runner.temp }}` expands to `D:\a\_temp`, so `PURE_SIMDJSON_CACHE_DIR` and the resolved `PURE_SIMDJSON_LIB_PATH` are mixed-separator paths (`D:\a\_temp/pure-simdjson/v0.1.7/windows-amd64/...`). Git Bash and Go generally accept mixed separators, and this leg is advisory, but the `actions/cache` `path:` and the `test -f` check are the two places where a conversion mismatch would surface as a silent cache miss or a failed resolve step.
**Fix:** Normalise once in the bash steps, e.g. `cache_dir="${RUNNER_TEMP//\\//}/pure-simdjson"`, and reuse it for `PURE_SIMDJSON_CACHE_DIR` and the resolved path; keep `${{ runner.temp }}/pure-simdjson` only in the `actions/cache` input.

---

_Reviewed: 2026-10-19T09:36:17Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
