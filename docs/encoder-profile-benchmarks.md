# Encoder profile benchmark results (issue #79)

Source: `make bench-encoder-profile` on this branch, klauspost/compress v1.19.2, Go 1.25.5, linux/amd64, 4 physical CPUs (GOMAXPROCS forced per run; the zstd worker pool follows GOMAXPROCS, not physical CPUs). BENCHTIME=1s, COUNT=1.

Metrics: *retained* is heap still allocated after GC while the encoder is alive (what the shared cache keeps for the life of the process). *cold* constructs a fresh encoder per iteration and encodes the serialized payload once. *repeated* goes through `EncodeWithLevelContext` against the shared cache and includes index serialization time. Fixtures: `small` = 100 row groups (7 KB compressed), `highcard` = 500 row groups x 20 unique-string documents (206 KB compressed at level 15).

## Retained memory after first encode (MB)

| GOMAXPROCS | L15 default | L15 bounded | L3 default | L3 bounded |
|-----------:|------------:|------------:|-----------:|-----------:|
| 1 | 50.6 | 42.5 | 17.8 | 9.8 |
| 4 | 152.6 | 42.5 | 21.6 | 9.8 |
| 16 | 560.7 | 42.5 | 36.7 | 9.8 |

Bounded-memory retention is independent of GOMAXPROCS. Default retention scales linearly with it (about 36 MB per worker at level 15).

## Compressed size (identical across profiles)

| Fixture | L15 | L3 |
|---------|----:|---:|
| small | 7,157 B | 7,142 B |
| highcard | 206,396 B | 223,268 B |

## Full table

| GOMAXPROCS | Profile | Level | Fixture | Mode | Time/op | Alloc/op | Allocs/op | Retained | Compressed |
|-----------:|---------|------:|---------|------|--------:|---------:|----------:|---------:|-----------:|
| 1 | default | 15 | small | cold | 32.6 ms | 52.9 MB | 37 | 50.4 MB | 7,153 B |
| 1 | default | 15 | small | repeated | 1.8 ms | 443 KB | 6,378 | 50.4 MB | 7,157 B |
| 1 | default | 15 | highcard | cold | 47.7 ms | 54.4 MB | 54 | 50.6 MB | 206,392 B |
| 1 | default | 15 | highcard | repeated | 28.3 ms | 4.7 MB | 57,956 | 50.6 MB | 206,396 B |
| 1 | default | 3 | small | cold | 17.5 ms | 1.8 MB | 37 | 1.7 MB | 7,138 B |
| 1 | default | 3 | small | repeated | 0.6 ms | 443 KB | 6,378 | 1.7 MB | 7,142 B |
| 1 | default | 3 | highcard | cold | 26.3 ms | 20.1 MB | 55 | 17.8 MB | 223,264 B |
| 1 | default | 3 | highcard | repeated | 11.2 ms | 4.7 MB | 57,956 | 17.8 MB | 223,268 B |
| 1 | bounded-memory | 15 | small | cold | 21.4 ms | 44.4 MB | 60 | 42.2 MB | 7,153 B |
| 1 | bounded-memory | 15 | small | repeated | 2.0 ms | 382 KB | 6,382 | 42.2 MB | 7,157 B |
| 1 | bounded-memory | 15 | highcard | cold | 47.0 ms | 46.0 MB | 86 | 42.5 MB | 206,392 B |
| 1 | bounded-memory | 15 | highcard | repeated | 32.8 ms | 4.5 MB | 57,961 | 42.5 MB | 206,396 B |
| 1 | bounded-memory | 3 | small | cold | 19.1 ms | 1.5 MB | 58 | 1.3 MB | 7,138 B |
| 1 | bounded-memory | 3 | small | repeated | 0.6 ms | 385 KB | 6,383 | 1.3 MB | 7,142 B |
| 1 | bounded-memory | 3 | highcard | cold | 25.5 ms | 11.7 MB | 87 | 9.8 MB | 223,264 B |
| 1 | bounded-memory | 3 | highcard | repeated | 11.2 ms | 4.6 MB | 57,961 | 9.8 MB | 223,268 B |
| 4 | default | 15 | small | cold | 46.5 ms | 159.8 MB | 40 | 152.4 MB | 7,153 B |
| 4 | default | 15 | small | repeated | 1.8 ms | 527 KB | 6,378 | 152.4 MB | 7,157 B |
| 4 | default | 15 | highcard | cold | 57.5 ms | 161.4 MB | 57 | 152.6 MB | 206,392 B |
| 4 | default | 15 | highcard | repeated | 43.2 ms | 6.8 MB | 57,962 | 152.6 MB | 206,396 B |
| 4 | default | 3 | small | cold | 13.7 ms | 5.7 MB | 40 | 5.4 MB | 7,138 B |
| 4 | default | 3 | small | repeated | 0.5 ms | 444 KB | 6,378 | 5.4 MB | 7,142 B |
| 4 | default | 3 | highcard | cold | 20.1 ms | 24.1 MB | 58 | 21.6 MB | 223,264 B |
| 4 | default | 3 | highcard | repeated | 11.2 ms | 5.3 MB | 57,957 | 21.6 MB | 223,268 B |
| 4 | bounded-memory | 15 | small | cold | 15.5 ms | 44.4 MB | 60 | 42.2 MB | 7,153 B |
| 4 | bounded-memory | 15 | small | repeated | 1.8 ms | 382 KB | 6,382 | 42.2 MB | 7,157 B |
| 4 | bounded-memory | 15 | highcard | cold | 39.6 ms | 46.0 MB | 86 | 42.5 MB | 206,392 B |
| 4 | bounded-memory | 15 | highcard | repeated | 28.6 ms | 4.5 MB | 57,961 | 42.5 MB | 206,396 B |
| 4 | bounded-memory | 3 | small | cold | 13.0 ms | 1.5 MB | 58 | 1.3 MB | 7,138 B |
| 4 | bounded-memory | 3 | small | repeated | 0.5 ms | 385 KB | 6,383 | 1.3 MB | 7,142 B |
| 4 | bounded-memory | 3 | highcard | cold | 24.6 ms | 11.7 MB | 87 | 9.8 MB | 223,264 B |
| 4 | bounded-memory | 3 | highcard | repeated | 13.0 ms | 4.6 MB | 57,961 | 9.8 MB | 223,268 B |
| 16 | default | 15 | small | cold | 126.9 ms | 587.8 MB | 52 | 560.5 MB | 7,153 B |
| 16 | default | 15 | small | repeated | 2.9 ms | 1.0 MB | 6,379 | 560.5 MB | 7,157 B |
| 16 | default | 15 | highcard | cold | 122.9 ms | 589.3 MB | 68 | 560.7 MB | 206,392 B |
| 16 | default | 15 | highcard | repeated | 47.6 ms | 15.6 MB | 57,984 | 560.7 MB | 206,396 B |
| 16 | default | 3 | small | cold | 14.1 ms | 21.6 MB | 51 | 20.5 MB | 7,138 B |
| 16 | default | 3 | small | repeated | 0.6 ms | 446 KB | 6,378 | 20.5 MB | 7,142 B |
| 16 | default | 3 | highcard | cold | 21.6 ms | 39.9 MB | 69 | 36.7 MB | 223,264 B |
| 16 | default | 3 | highcard | repeated | 13.6 ms | 7.8 MB | 57,964 | 36.7 MB | 223,268 B |
| 16 | bounded-memory | 15 | small | cold | 16.2 ms | 44.4 MB | 59 | 42.2 MB | 7,153 B |
| 16 | bounded-memory | 15 | small | repeated | 1.7 ms | 382 KB | 6,382 | 42.2 MB | 7,157 B |
| 16 | bounded-memory | 15 | highcard | cold | 38.7 ms | 46.0 MB | 85 | 42.5 MB | 206,392 B |
| 16 | bounded-memory | 15 | highcard | repeated | 30.0 ms | 4.5 MB | 57,961 | 42.5 MB | 206,396 B |
| 16 | bounded-memory | 3 | small | cold | 13.4 ms | 1.5 MB | 57 | 1.3 MB | 7,138 B |
| 16 | bounded-memory | 3 | small | repeated | 0.9 ms | 385 KB | 6,383 | 1.4 MB | 7,142 B |
| 16 | bounded-memory | 3 | highcard | cold | 20.8 ms | 11.7 MB | 86 | 9.8 MB | 223,264 B |
| 16 | bounded-memory | 3 | highcard | repeated | 12.3 ms | 4.6 MB | 57,961 | 9.8 MB | 223,268 B |

## Reading the numbers

- Level 15 costs about 36 MB per worker whatever the profile; the bounded profile simply stops at one worker (~42 MB total) where the default keeps GOMAXPROCS of them.
- Cold construction of the default level-15 encoder is the expensive step at high GOMAXPROCS (hundreds of MB allocated and zeroed once). Repeated encodes through the shared cache allocate only the per-call serialization buffers under either profile.
- Repeated-encode time is within noise between profiles for a single caller. The bounded profile serializes concurrent same-level encodes on its one worker, so a process that encodes many indexes in parallel loses that parallelism.
- Level 3 retains a few MB under either profile at a larger output (about 8% larger on the high-cardinality fixture); dropping the level is a different trade-off from bounding the workers.
