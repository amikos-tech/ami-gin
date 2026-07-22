---
quick_id: 260722-ke3
slug: fix-pr41-item2-stream-directory-entries-
date: 2026-07-22
status: complete
---

# Summary: Stream external corpus directory entries

## What changed

`phase20ExternalBenchmarkPaths` (`benchmark_test.go`) no longer calls
`os.ReadDir`, which eagerly materialized and sorted the entire directory listing
before the 64-file cap was enforced. It now opens the directory and streams
entries in batches of `phase20ExternalReadDirBatch` (128) via `File.ReadDir(n)`,
rejecting the moment supported files exceed `phase20MaxExternalFiles` — before the
rest of the directory is read.

- Bounded memory: never holds more than one batch of `DirEntry` plus the (capped)
  supported-paths slice, regardless of how many unrelated files live in the dir.
- Early rejection: over-cap directories fail after ~65 supported files instead of
  reading all N entries.
- Ordering preserved: the success path still `sort.Strings(paths)` explicitly, so
  `File.ReadDir`'s unsorted batches produce the same deterministic result as before.

## Behavior change

Early-exit rejection no longer knows the exact supported-file count, so the error
message changed from "contains %d supported files" to "contains more than %d
supported files". The `file-count` test asserts only `err != nil`, so it stayed
green.

## Verification

- `go build ./...` — OK
- `go vet ./...` — OK
- `go test -run 'TestPhase20External' -v` — all PASS (external tier, resource
  limits, depth validation)
- `golangci-lint run` — no new findings; the change introduces no duplicated
  string literals (pre-existing goconst findings in other test files are a
  local-vs-CI version discrepancy, unrelated to this change)

## Addresses

PR #41 review item 2 (automated `@claude` review, non-blocking perf observation).
