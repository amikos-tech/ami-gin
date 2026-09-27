# Deferred items (out of scope for 260925-kcs)

Discovered while verifying the final gate (`go build ./... && go vet ./... &&
make lint && go test -race -short . ./cmd/...`). Neither item was caused by
this plan's changes; both are logged here per the executor scope-boundary
rule instead of being fixed.

## 1. `make lint` fails on a local toolchain/golangci-lint version mismatch

`make lint` shells out to `go run github.com/golangci/golangci-lint/v2/...@v2.11.4`,
which compiles against whatever Go toolchain is first on `PATH`. In this
environment that is Homebrew's `go1.27.1` (`/opt/homebrew/bin/go`), while
`go.mod` pins `go 1.25.5`. The mismatch produces:

```
logging/attrs.go:3:8: could not import strings (...: could not load export
data: internal error in importing "internal/goarch" (cannot decode
"internal/goarch", export data version 4 is greater than maximum supported
version 2); please report an issue))) (typecheck)
```

**Verified pre-existing:** reproduced identically on the unmodified PR #80
base commit (`6cbd8b0`) in a throwaway `git worktree`, before any of this
plan's changes were applied. Not caused by this task.

**Workaround used for this task's verification:** the pre-installed
`golangci-lint` binary (`/opt/homebrew/bin/golangci-lint`, v2.13.2, prebuilt
— not compiled via `go run` against the local toolchain) does not hit the
export-data mismatch. Ran `golangci-lint run ./...` directly instead, per the
task's documented fallback ("If make lint is unavailable, run
`golangci-lint run ./...` and report").

**Suggested follow-up (not part of this task):** pin a `go` toolchain
directive in `go.mod` (or document a required local Go version) so `make
lint`'s `go run` invocation is reproducible across contributor machines
regardless of the Homebrew-installed Go version.

## 2. Pre-existing `goconst` debt across the test suite (172 findings)

`golangci-lint run ./... --max-issues-per-linter=0 --max-same-issues=0`
reports 172 `goconst` findings (repeated string literals like `$.email`,
`2024-01-15T10:30:00Z`, `Alice@Example.COM`, etc.) spread across many
pre-existing test files (`gin_test.go`, `transformers_test.go`,
`transformer_registry_test.go`, `serialize_security_test.go`,
`cmd/gin-index/main_test.go`, `cmd/gin-index/experiment_test.go`, and more).

**Verified pre-existing:** the exact same 172 string values are flagged on
the unmodified PR #80 base commit (`6cbd8b0`), confirmed via a value-only
diff (`diff base_goconst_vals.txt branch_goconst_vals.txt` → no output) in a
throwaway `git worktree`. This plan's test additions introduced one new
occurrence (`{"status":"warn"}` tipped past the goconst threshold in
`cmd/gin-index/main_test.go`) and one new `unparam` finding
(`sharedZstdEncoderCached` in `serialize_profile_test.go`); both were fixed
in this plan (commit `d29f714`) so the final branch state matches the base
branch's pre-existing lint debt exactly — zero net new findings.

**Not fixed here:** de-duplicating 172 pre-existing string literals into
named constants across unrelated test files is well outside this task's
scope (PR #80 review findings I1-I4, S1-S11 on the encoder-profile feature
only). Left for a dedicated follow-up quick task if the project wants
`golangci-lint run ./...` to exit 0 with zero findings.
