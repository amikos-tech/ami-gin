# Deferred Items — quick-260730-pcc

Out-of-scope discoveries found during execution. Not fixed (SCOPE BOUNDARY: only issues
directly caused by this task's changes are auto-fixed).

## 1. `make lint` fails on a pre-existing golangci-lint backlog

**Status:** Pre-existing, unrelated to this task. Verified by running `golangci-lint` against
the tree at the pre-change commit (`e5afb1e^`), which produced the **identical** 51 issues.

| Baseline (`e5afb1e^`) | After this task |
|-----------------------|-----------------|
| 51 issues             | 51 issues       |
| goconst: 50           | goconst: 50     |
| staticcheck: 1        | staticcheck: 1  |

**Breakdown:**

- **50 `goconst` issues** — repeated string literals in test files
  (`benchmark_test.go`, `cmd/gin-index/experiment_test.go`, `cmd/gin-index/main_test.go`,
  `gin_test.go`, `serialize_security_test.go`, `transformers_test.go`,
  `transformer_registry_test.go`, `telemetry/boundary_test.go`,
  `phase09_review_test.go`). None are in files touched by this task.

- **1 `staticcheck QF1001`** (`could apply De Morgan's law`) in
  `notice_version_guard_test.go`, inside the
  `CI runs the dedicated guard before golangci-lint` subtest. This task did **not** modify
  that subtest — the finding merely renumbered from line 79 to line 72 because the deletion
  removed 7 lines above it. The plan explicitly forbade touching other subtests.

**Recommendation:** Address as a separate `[CLN]` cleanup — either fix the literals/De Morgan
condition or tune `.golangci.yml` (e.g. exclude `goconst` for `_test.go`, consistent with the
existing `errcheck` test exclusion).

## 2. `LC_ALL=C` presence is unenforced on macOS/BSD (nuance, not a regression)

BSD `sed`'s `l` command octal-escapes multibyte characters **regardless of locale**, so on
macOS removing `export LC_ALL=C` from the guard does not fail any test. GNU `sed` on
`ubuntu-latest` (where CI runs) prints valid multibyte characters literally under a UTF-8
locale, so the behavioral `invisible version character is escaped` subtest does catch a
missing `LC_ALL=C` there.

Net effect: enforcement is retained at the CI gate but is not reproducible locally on macOS.
The `export LC_ALL=C` line itself was left untouched by this task. See the SUMMARY's
"Deviations" section for the full analysis.
