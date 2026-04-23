# Deferred Items

## Plan 16-04

- Resolved during the Wave 2 integration gate: the existing `goconst` finding in `gin_test.go` for `unsupported mixed numeric promotion at $.score` was fixed by extracting a shared test constant, allowing `make lint` to pass.
