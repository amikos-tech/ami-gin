# Phase 20 Realistic JSONL Smoke Fixtures

These checked-in files are the offline default for Phase 20 tests and smoke benchmarks. They are `synthesized-from-shape`: simdjson `jsonexamples` provides structural inspiration, but no upstream rows or text are copied and tests/benchmarks do not download data.

| Fixture | Origin | Documents | Current bytes | Byte cap |
| --- | --- | ---: | ---: | ---: |
| `nested_high_cardinality.jsonl` | synthesized-from-shape | 96 | 46800 | 131072 |
| `mixed_type_arrays.jsonl` | synthesized-from-shape | 96 | 23558 | 131072 |
| `number_heavy.jsonl` | synthesized-from-shape | 96 | 33609 | 131072 |
| `combined.jsonl` | synthesized-from-shape | 288 | 103967 | 393216 |

Refresh the committed fixtures from the repository root with:

```bash
go run ./testdata/phase20/generate.go
```

The generator is local and deterministic; it does not download data. The committed JSONL files, rather than generator output at test time, are the consumer input.

## Optional local simdjson input

Exact upstream input is never downloaded or vendored. A future optional local run uses:

```bash
GIN_PHASE20_ENABLE_SIMDJSON_EXTERNAL=1 \
GIN_PHASE20_SIMDJSON_DIR=/path/to/simdjson/jsonexamples
```

If direct upstream rows are ever redistributed, first choose and record a pinned revision, then review its [LICENSE](https://github.com/simdjson/simdjson/blob/master/LICENSE) and [LICENSE-MIT](https://github.com/simdjson/simdjson/blob/master/LICENSE-MIT) plus any applicable NOTICE obligations. This synthesized fixture family does not vendor upstream license files or data.
