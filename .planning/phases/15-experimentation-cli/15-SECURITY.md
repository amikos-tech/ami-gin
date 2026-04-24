---
phase: 15
artifact: security
verified: 2026-04-22T00:00:00Z
status: secured
asvs_level: 1
block_on: open
threats_total: 11
threats_open: 0
---

# Phase 15 Security Verification

## Scope

Threat verification for Phase 15 `gin-index experiment`, based only on the declared threat register in:

- `15-01-PLAN.md`
- `15-02-PLAN.md`
- `15-03-PLAN.md`

All `## Threat Flags` sections in the phase summaries were reviewed. No unregistered flags were present.

## Threat Verification

| Threat ID | Category | Disposition | Status | Evidence |
|-----------|----------|-------------|--------|----------|
| T-15-01 | Denial of Service | mitigate | CLOSED | `cmd/gin-index/experiment.go:309-326` and `cmd/gin-index/experiment.go:346-352` use `bufio.Reader.ReadBytes('\n')` in both passes; `cmd/gin-index/experiment_test.go:547-565` locks >64 KiB input with `TestRunExperimentLargeLineNoTruncation`. |
| T-15-02 | Tampering | mitigate | CLOSED | `cmd/gin-index/experiment.go:39-42` rejects `--rg-size <= 0` before source preparation; `cmd/gin-index/experiment_test.go:590-601` covers `TestRunExperimentRejectsInvalidRGSize`. |
| T-15-03 | Information Disclosure | mitigate | CLOSED | `cmd/gin-index/experiment_output.go:120-153` writes summary/path output to `stdout`; `cmd/gin-index/experiment.go:40-42`, `cmd/gin-index/experiment.go:62`, `cmd/gin-index/experiment.go:68`, `cmd/gin-index/experiment.go:86`, `cmd/gin-index/experiment.go:115`, `cmd/gin-index/experiment.go:124`, `cmd/gin-index/experiment.go:151`, and `cmd/gin-index/experiment.go:360` route diagnostics to `stderr`; `cmd/gin-index/experiment_test.go:312-347` verifies log output stays off `stdout`. |
| T-15-04a | Denial of Service | mitigate | CLOSED | `cmd/gin-index/experiment.go:180-188` selects the two-pass path when sampling is not enabled; `cmd/gin-index/experiment.go:258-292` spools stdin to a temp file during counting; `cmd/gin-index/experiment.go:304-326` counts by streaming bytes, not full-file buffering; `cmd/gin-index/experiment_test.go:502-525` covers stdin ingest. |
| T-15-04b | Denial of Service | mitigate | CLOSED | Inferred from `cmd/gin-index/experiment.go:66-71` plus `cmd/gin-index/experiment.go:288-290`: `runExperiment(...)` defers `source.cleanup()`, and the stdin cleanup removes the temp spool file after the build pass; `cmd/gin-index/experiment.go:264-273` closes the temp file before reopen; stdin behavior remains covered by `cmd/gin-index/experiment_test.go:502-525`. |
| T-15-04 | Information Disclosure | mitigate | CLOSED | `cmd/gin-index/experiment.go:406-419` uses the repo-owned `logging/slogadapter`; `logging/attrs.go:14-23` freezes INFO-level keys and forbids raw user content; `query.go:63-72` emits only `operation`, `status`, and `error.type`; `query.go:245-248` emits bounded `predicate_op` and `path_mode`; `cmd/gin-index/experiment_test.go:312-347` verifies stderr-only logging. |
| T-15-05 | Tampering | mitigate | CLOSED | `cmd/gin-index/experiment_output.go:13-53` defines explicit report structs, and `cmd/gin-index/experiment_output.go:155-159` marshals them directly with `encoding/json`; `cmd/gin-index/experiment_test.go:103-166` locks schema and omission behavior in `TestRunExperimentJSONGolden`. |
| T-15-06 | Tampering | mitigate | CLOSED | `cmd/gin-index/experiment.go:425-445` requires `.gin`, encodes with `gin.Encode(...)`, calls `writeLocalIndexFile(...)`, and derives mode from stdin vs local input; `cmd/gin-index/main.go:658-674` defines `localOutputMode(...)` and `writeLocalIndexFile(...)`; `cmd/gin-index/experiment_test.go:249-309` covers roundtrip and non-`.gin` rejection; `cmd/gin-index/main_test.go:727-767` covers output mode derivation. |
| T-15-07 | Denial of Service | mitigate | CLOSED | `cmd/gin-index/experiment.go:170-176` keeps only counters in `experimentBuildResult`; `cmd/gin-index/experiment.go:360-365` streams each continue-mode diagnostic directly to `stderr` and increments counters, with no retained error list; `cmd/gin-index/experiment_test.go:369-423` covers continue-mode reporting. |
| T-15-08 | Repudiation | mitigate | CLOSED | `cmd/gin-index/experiment.go:354-365` emits exact `line N:` diagnostics before both abort and continue branches; `cmd/gin-index/experiment_test.go:350-367` covers abort mode and `cmd/gin-index/experiment_test.go:369-423` covers continue mode. |
| T-15-09 | Tampering | mitigate | CLOSED | `cmd/gin-index/experiment_policy_test.go:13-115` guards forbidden dependencies, imports, TTY/ANSI logic, and `--parser` exposure; `go test ./cmd/gin-index -count=1` passed during this audit. |

## Threat Flags

None. `15-01-SUMMARY.md`, `15-02-SUMMARY.md`, and `15-03-SUMMARY.md` each report `## Threat Flags` as `None`.

## Verification Commands

- `go test ./cmd/gin-index -count=1`
- `go test ./... -count=1`

## Result

Phase 15 is secured against the declared threat register at ASVS Level 1.
