# Deferred Items

## Plan 16-04

- `make lint` is currently blocked by an existing `goconst` finding in `gin_test.go` (`unsupported mixed numeric promotion at $.score` appears three times). This file is outside plan 16-04 scope and is owned by another executor, so plan 16-04 did not modify it.
