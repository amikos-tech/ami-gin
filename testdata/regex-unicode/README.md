# Regex Unicode goldens (#115)

These files test how `RegexExtract` and `RegexExtractInt` companions behave
across Go toolchains with different Unicode tables. Go 1.26 knows Unicode
15.0.0. Go 1.27 knows Unicode 17.0.0.

## Files

| File | Written by | Toolchain | Content |
|------|------------|-----------|---------|
| `detector-corpus.golden` | current code | go1.26.9 and go1.27.3 | Frozen text of 3,000 generated patterns, one column per Unicode version |
| `legacy-regex-go1.26.9.gin` | pre-#115 builder (29a3e0c) | go1.26.9 | Shared fixture docs: `upper`, `n2` (unfrozen, Unicode-sensitive) and `num` (safe) |
| `legacy-regex-go1.27.3.gin` | pre-#115 builder (29a3e0c) | go1.27.3 | Same as above |
| `legacy-safe-go1.26.9.gin` | pre-#115 builder (29a3e0c) | go1.26.9 | Shared fixture with only the safe `num` companion |
| `legacy-garay-go1.27.3.gin` | pre-#115 builder (29a3e0c) | go1.27.3 | Unfrozen `(\p{Garay}+)` companion. Go 1.26 cannot compile it |
| `frozen-garay-go1.27.3.gin` | current builder | go1.27.3 | Same Garay docs, frozen pattern |

All indexes are encoded without compression.

The `gen` and `oldreader` directories hold test files. Go tooling ignores
`testdata`, so the Makefile copies them into a git worktree of an old commit.

## Regenerate

Detector corpus (two steps, one per toolchain):

```sh
GOTOOLCHAIN=go1.26.9 go test -tags regenerate_goldens -run TestRegenerateRegexUnicodeGoldens .
GOTOOLCHAIN=go1.27.3 go test -tags regenerate_goldens -run TestRegenerateRegexUnicodeGoldens .
```

Legacy indexes (pre-#115 builder in a temporary worktree of 29a3e0c):

```sh
make regex-unicode-legacy-goldens
```

Frozen Garay index (needs Unicode 17.0.0):

```sh
GOTOOLCHAIN=go1.27.3 go test -tags regenerate_goldens -run TestRegenerateRegexUnicodeGarayGolden .
```

## Old readers

`make old-reader-check` is a local target. It is not part of CI. It builds
`regex.gin` with the current code on both toolchains. Then the pre-#115 reader
(29a3e0c, on both toolchains) and the v1.6.0 reader (2fbbd97, on go1.26.9)
decode it and the frozen Garay golden, and check exact row groups. A last
negative control cell runs the pre-#115 reader on go1.26.9 against
`legacy-regex-go1.27.3.gin`. It must return the known wrong row groups, which
shows that the exact checks detect the drift.
