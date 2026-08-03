# Parser Parity Goldens

Byte-level goldens pinning the current `Encode()` output for the
`stdlibParser` path. They are a merge gate: an unexpected byte mismatch means
the current parser or serialization contract drifted.

They are not an unconditional v1.0 compatibility assertion. In particular,
the array canonicalization change records array elements only at wildcard
paths, so array-bearing fixtures intentionally differ from the old v1.0
encoding.

Regenerate only when the serialization format bumps (v10+) or an approved
behavior change requires it, and record the review in the audit trail below.

## How these were initially captured

The goldens in this directory were initially generated during Phase 13 Plan
02, after `AddDocument` was wired through the parser seam and its benchmark
gate found no representative performance drift. At that point,
`stdlibParser.Parse` was a code move of the direct staging path, so the blobs
were a useful baseline without a brittle historical cherry-pick. They now pin
the audited current behavior instead; later approved behavior changes must be
recorded in the audit trail.

## Regenerate (future format bumps)

```bash
# From the same commit that owns the behavior change:
go test -tags regenerate_goldens -run TestRegenerateParityGoldens -count=1 .
git add testdata/parity-golden/*.bin
git commit -m "chore(parity): refresh goldens to v<N>"
```

For a behavior change, the owning commit must include the regenerated blobs
and an audit note that names the expected fixture changes and the test used to
verify them. Do not put the golden refresh in a later test-only commit: that
separates the behavior change from the evidence required to review it.

## Audit trail

- 2026-08-02 — Array elements changed from private numeric paths to canonical
  wildcard paths in `0a9d371`. The four affected fixtures are
  `deep-nested`, `empty-arrays`, `single-rg-array-siblings`, and
  `transformer-buffered-container-numerics`; each becomes smaller because
  private numeric paths are removed. Their refresh landed separately in
  `b44ac8e`, which is a historical exception to the same-commit rule above.
  The intervening staging change only adds an array index to malformed-input
  error text, so it does not affect these valid fixtures. The current
  `TestStdlibParserGolden_AuthoredFixtures` check verifies the audited blobs.

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
| `simd-numeric-parity.bin` | Integer, whole-float, exponent, fraction, and exact-large-integer paths |
| `mixed-float-int.bin` | Integer/float widening on the same paths across row groups |
| `single-rg-array-siblings.bin` | Mixed numeric array siblings in one row group |
| `nulls-and-missing.bin` | Explicit null vs. absent paths |
| `deep-nested.bin` | Object/array recursion |
| `unicode-keys.bin` | Non-ASCII keys (NormalizePath exercise) |
| `empty-arrays.bin` | `[]` and `[[], []]` edges |
| `large-strings.bin` | Trigram-index stress |
| `transformer-buffered-container-numerics.bin` | Buffered object/array materialization with integers, whole floats, fractions, and nested numbers |
| `transformers-iso-date-and-lower.bin` | WithISODateTransformer + WithToLowerTransformer (D-05 dim #4 / Pitfall #2) |
| `transformers-soft-fail-wire.bin` | Full v9 payload pin for companion transformer `soft_fail` wire tokens |
