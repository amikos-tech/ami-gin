# Parser Parity Goldens

Byte-level goldens pinning `Encode()` output for the `stdlibParser` path.
They are the Phase 13 merge gate: if `parser_parity_test.go` fails with a
byte-mismatch, the refactor drifted from v1.0 behavior.

Regenerated only when the serialization format bumps (v10+) or a
legitimate behavior change is approved and audited.

## How these were initially captured

The goldens in this directory were generated during Phase 13 Plan 02,
AFTER `AddDocument` was wired through the new parser seam and the Plan 02
benchmark gate confirmed no representative performance drift against the
focused v1.0 baseline (`GOMAXPROCS=1`, three-run probes, equal allocs/op,
and flat wall-clock medians on the seam-path benchmarks). Because
`stdlibParser.Parse` is a code-move of the pre-refactor
`parseAndStageDocument` + `stageStreamValue` + `decodeTransformedValue`
logic, the encoded bytes captured from this branch are the authoritative
pins for the seam path without needing a brittle v1.0 cherry-pick.

## Regenerate (future format bumps)

```bash
# From the same commit that owns the behavior change:
go test -tags regenerate_goldens -run TestRegenerateParityGoldens -count=1 .
git add testdata/parity-golden/*.bin
git commit -m "chore(parity): refresh goldens to v<N>"
```

## Format

Each `.bin` is a full v9-encoded index blob as emitted by `Encode()`. The
files in this directory are compressed payloads, so they start with the
transport wrapper magic `GINc` (`serialize.go:101`); the wrapped inner index
header still carries `MagicBytes = "GIN\x01"` and `Version = 9`. One file per
authored fixture; names match `authoredParityFixtures()` in
`parser_parity_fixtures_test.go`.

## Fixture list

| File | Coverage |
|------|----------|
| `int64-boundaries.bin` | MaxInt64, -MaxInt64, 2^53+1, 0 (BUILD-03 / Pitfall #1) |
| `nulls-and-missing.bin` | Explicit null vs. absent paths |
| `deep-nested.bin` | Object/array recursion |
| `unicode-keys.bin` | Non-ASCII keys (NormalizePath exercise) |
| `empty-arrays.bin` | `[]` and `[[], []]` edges |
| `large-strings.bin` | Trigram-index stress |
| `transformers-iso-date-and-lower.bin` | WithISODateTransformer + WithToLowerTransformer (D-05 dim #4 / Pitfall #2) |
