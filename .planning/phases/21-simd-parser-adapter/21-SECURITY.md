---
phase: 21
slug: simd-parser-adapter
status: verified
threats_open: 0
asvs_level: 1
block_on: high
register_authored_at_plan_time: true
created: 2026-07-23
audited: 2026-07-23
---

# Phase 21 — SIMD Parser Adapter Security

> Per-phase security contract for the optional `pure-simdjson` adapter.

## Audit Summary

| Metric | Count |
|--------|------:|
| Unique plan-time threats | 21 |
| Mitigations verified | 19 |
| Accepted risks documented | 2 |
| Transferred risks | 0 |
| Open threats | 0 |

`T-21-SC` appeared in Plans 03–05 and is consolidated here as one threat. Its
combined dependency-pin, checksum, build-tag, isolation-gate, bootstrap
integrity, and explicit-path trust controls were all verified.

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Untrusted JSON → native parser | Caller-controlled document bytes enter the optional runtime-loaded parser. | Raw JSON bytes |
| Native document → Go adapter | Typed native views are valid only until document cleanup. | Typed elements and native lifetime state |
| Adapter → builder sink | Typed values, paths, transform subtrees, and stage errors determine committed index state. | Scalar values, canonical paths, staged errors |
| Parser lifecycle → `AddDocument` policy | Cleanup failures must bypass otherwise recoverable hard/soft ingest routing. | Lifecycle, stage, and skip error chains |
| Build graph → optional module | Default builds must exclude the optional wrapper and native bootstrap path. | Go imports and dependency graph |
| Module/bootstrap/operator → process | Wrapper, downloaded native assets, and explicit-path assets have separate integrity owners. | Go module and native shared library |
| Error context → caller telemetry | Operation names and canonical paths may cross into caller-visible errors. | Failure metadata; no adapter-appended raw document bytes |

## Threat Register

| Threat ID | Category | Component | Disposition | Verification Evidence | Status |
|-----------|----------|-----------|-------------|-----------------------|--------|
| T-21-01 | Tampering | `StageFloat64` numeric classification | mitigate | `parser_sink.go:74-90` routes finite values directly through `stageNumericObservation` with `isInt:false`; `parser_simd_test.go:183-221` commits and byte-compares both `1.0` and `1e18` against stdlib while proving the native whole-float route differs. | closed |
| T-21-02 | Tampering | Stage error provenance | mitigate | Every hard `StageFloat64` exit passes through `tagStageError` at `parser_sink.go:74-90`; `parser_simd_test.go:97-109,267-300` requires a numeric-layer `IngestError` for all hard non-finite cases. | closed |
| T-21-03 | Denial of Service | Oversized/non-finite numerics | mitigate | Hard/soft policy is enforced at `parser_sink.go:66-90` and `builder.go:697-768`; `parser_simd_test.go:78-95,224-300` proves oversized `uint64` and NaN/±Inf never commit document, mapping, position, or path state. | closed |
| T-21-04 | Tampering | License/NOTICE provenance | mitigate | `NOTICE.md:3-24` matches the exact cached `pure-simdjson@v0.1.4` `LICENSE:1-3` and `NOTICE:1-8`: MIT, 2026 Amikos Tech, and bundled simdjson Apache-2.0/MIT attribution. `go mod verify` passed. | closed |
| T-21-05 | Tampering | Native library supply chain | mitigate | `docs/simd-deployment.md:153-164` separately assigns `go.sum` wrapper verification, upstream SHA-256 verification for downloaded/mirrored assets, and operator checksum/provenance ownership for `PURE_SIMDJSON_LIB_PATH`; route details are also explicit at `docs/simd-deployment.md:63-114`. | closed |
| T-21-06 | Repudiation | Silent parser fallback | mitigate | `parser_simd.go:25-35` returns construction failure and implements no fallback; `docs/simd-deployment.md:17-26,58-61,116-151` requires explicit caller branching and telemetry for degraded stdlib selection. | closed |
| T-21-07 | Information Disclosure | Error/document examples | accept | Accepted risk `AR-21-07` below. The SIMD guidance at `docs/simd-deployment.md:39-61,81-151,166-183` and `README.md:80-112` contains variable names, approved-path placeholders, and failure-layer descriptions, but no credentials, file contents, or raw document values. | closed (accepted) |
| T-21-08 | Denial of Service | Leaked upstream document / `ErrParserBusy` | mitigate | Every successful upstream parse enters `finishSIMDDocument` at `parser_simd.go:37-48`; its defer closes on success, returned error, or panic and propagates failed close as a lifecycle error at `parser_simd.go:51-78`. Exactly-once and builder-poisoning regressions are at `parser_simd_lifecycle_test.go:14-74,223-283`. | closed |
| T-21-09 | Tampering | Transform numeric coercion | mitigate | Transform buffering precedes typed dispatch at `parser_simd.go:86-93`. `materializeElement` emits `json.Number` for int64, uint64, and float64 and preserves float class at `parser_simd.go:184-220`; stdlib-compatible transformer preparation is at `builder.go:506-527,559-584`. | closed |
| T-21-10 | Tampering | Duplicate object keys | mitigate | Direct walking collects entries into a last-write-wins map before sorted traversal at `parser_simd.go:129-155`; transform materialization also assigns iterator entries into a last-write-wins map at `parser_simd.go:246-265`. Both object entry points were checked. | closed |
| T-21-11 | Tampering | Default build isolation | mitigate | The only product importer is exactly tagged on `parser_simd.go:1,13`. `Makefile:7-31` enforces the exact first-line tag for every tracked product importer, requires a non-vacuous importer, rejects the exact module from `go list -deps -test ./...`, then builds and vets untagged. `make simd-isolation-check` passed; the default graph was absent and the tagged graph present. | closed |
| T-21-12 | Information Disclosure | Native/parser errors | accept | Accepted risk `AR-21-12` below. Adapter wrappers at `parser_simd.go:37-40,80-181,184-274` add only operation/type and canonical-path context; no wrapper formats or appends `jsonDoc`. | closed (accepted) |
| T-21-13 | Tampering | `AddDocument` parser soft-mode routing | mitigate | `builder.go:412-437` checks `isParserLifecycleError` before skip, stage, and `ParserFailureMode` branches. `parser_lifecycle_test.go:100-168` and `parser_simd_lifecycle_test.go:285-398` require a hard result, zero soft skips, and zero durable state. | closed |
| T-21-14 | Denial of Service | Failed close / `ErrParserBusy` reuse | mitigate | The prior-tragic guard rejects before parser dispatch at `builder.go:389-392`, and the first lifecycle failure is stored once at `builder.go:412-415`. Retry/count assertions at `parser_lifecycle_test.go:151-168` and `parser_simd_lifecycle_test.go:223-283` keep parse, walk, and cleanup calls at one. | closed |
| T-21-15 | Repudiation | Combined walk/stage and cleanup error | mitigate | `parserLifecycleError.Unwrap() []error` exposes cleanup and concurrent errors as peer causes at `parser.go:7-38`. Generic traversal tests cover cleanup, stage, soft-skip, and sentinels at `parser_lifecycle_test.go:11-82`; real numeric hard/soft combinations cover `IngestError` and skip provenance at `parser_simd_lifecycle_test.go:285-373`. | closed |
| T-21-16 | Denial of Service | Duplicate or missed cleanup | mitigate | `finishSIMDDocument` owns one deferred `closeDocument()` call at `parser_simd.go:51-78`; `parser_simd_lifecycle_test.go:14-49` asserts exactly one walk and close for success, walk error, and soft-stage error. Panic tests independently require one close. | closed |
| T-21-17 | Denial of Service | Panic plus failed close | mitigate | The defer recovers before one close and converts failed close into `parserLifecycleError` at `parser_simd.go:51-74`; error-valued and non-error panic regressions at `parser_simd_lifecycle_test.go:117-220` prove terminal routing without native loading. | closed |
| T-21-18 | Tampering | `ParserFailureMode` soft retry routing | mitigate | `parser_simd_lifecycle_test.go:117-185` performs a real second `AddDocument` under soft mode with a simulated busy retry result, then proves no second dispatch, no busy-cause exposure, zero soft skips, and zero committed state. The enforcing guard is `builder.go:389-415`. | closed |
| T-21-19 | Repudiation | Concurrent panic and cleanup provenance | mitigate | `parser_simd.go:62-74` preserves error-valued panics directly and gives non-error values stable `panic while walking SIMD document` context. `parser_simd_lifecycle_test.go:117-220` verifies both cleanup and error-panic identities through `errors.Is` and stable non-error context. | closed |
| T-21-20 | Denial of Service | Successful-close panic semantics | mitigate | When close succeeds, `parser_simd.go:53-59` re-panics the identical recovered value. `parser_simd_lifecycle_test.go:51-74` asserts identity-preserving re-panic and exactly one close. | closed |
| T-21-SC | Tampering | Optional pure-simdjson Go/native supply chain | mitigate | Exact `v0.1.4` pin: `go.mod:12`; module and go.mod checksums: `go.sum:50-51`; exact tagged importer boundary: `parser_simd.go:1,13`; isolation gate: `Makefile:7-31`; SHA-256 bootstrap and explicit-path operator trust: `docs/simd-deployment.md:63-114,153-164`. `go mod verify` and `make simd-isolation-check` passed. | closed |

*Status: `closed` · `closed (accepted)` · `open`*

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-21-07 | T-21-07 | Deployment examples necessarily disclose public environment-variable names and failure layers so operators can diagnose activation. The reviewed examples contain no credentials, file contents, or raw JSON values. Any caller-selected logging of construction failures remains caller-owned. | Phase 21 plan-time threat model | 2026-07-23 |
| AR-21-12 | T-21-12 | Parser diagnostics may disclose an operation/type and canonical JSON path, which is necessary for diagnosis. The adapter does not append raw document bytes. This acceptance is limited to adapter-added error text; callers remain responsible for redacting the existing verbatim `IngestError.Value()` API before logging it. | Phase 21 plan-time threat model | 2026-07-23 |

## Unregistered Threat Flags

None. None of the five plan summaries contains a formal `## Threat Flags`
section. Plans 04 and 05 explicitly report no new surface outside the
plan-time model; because the register was authored at plan time, this audit did
not perform a retroactive scan for undeclared threats.

## Verification Run

| Check | Result |
|-------|--------|
| `go test -run '^TestTypedSink' -count=1 .` | passed |
| `go test -run '^TestParserParity_AuthoredFixtures/simd-numeric-parity$' -count=1 .` | passed |
| `go test -run '^TestParserLifecycle' -count=1 .` | passed |
| `go test -tags simdjson -run '^TestSIMDDocumentLifecycle' -count=1 .` | passed |
| `go test -count=1 ./...` | passed |
| `go test -tags simdjson -count=1 ./...` | passed |
| `go build -tags simdjson ./... && go vet -tags simdjson ./...` | passed |
| `make simd-isolation-check` | passed |
| `go mod verify` | passed — all modules verified |
| Default/tagged graph probe | passed — dependency absent by default and present with `-tags simdjson` |
| Pinned upstream attribution comparison | passed — `NOTICE.md` matches v0.1.4 `LICENSE`/`NOTICE` facts |

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|--------------:|-------:|-----:|--------|
| 2026-07-23 | 21 | 21 | 0 | Codex `gsd-security-auditor` |

## Sign-Off

- [x] All threats have a disposition.
- [x] All 19 declared mitigations are present in implementation, tests, dependency metadata, or the specifically required operator documentation.
- [x] Both planned accepted risks are documented with verified disclosure boundaries.
- [x] No transfer dispositions require external evidence.
- [x] `threats_open: 0` confirmed.
- [x] `status: verified` set in frontmatter.

**Approval:** verified 2026-07-23
