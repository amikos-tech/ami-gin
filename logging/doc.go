// Package logging defines the repo-owned, backend-neutral logging contract
// used throughout github.com/amikos-tech/ami-gin.
//
// # Contract
//
// The Logger interface is intentionally context-free: it does not accept or
// store a context.Context. Callers own trace correlation externally. The
// library never injects trace IDs, span IDs, or request IDs into log output.
//
// # Adapter Split
//
// The core logging package has no backend dependency. Callers who want to
// route log output to a specific backend import the appropriate sub-package:
//
//   - logging/slogadapter  — adapts *slog.Logger
//   - logging/stdadapter   — adapts *log.Logger
//
// Importing logging alone does not pull either adapter into the build graph.
//
// # Safe-Metadata Restrictions
//
// INFO-level log attributes are limited to the frozen allowlist:
//
//   - operation       — coarse operation name (e.g. "evaluate", "encode")
//   - predicate_op    — operator kind (e.g. "EQ", "GT")
//   - path_mode       — bounded enum: "exact", "bloom-only", "adaptive-hybrid"
//   - status          — outcome: "ok" or "error"
//   - error.type      — normalized error category; unknown values collapse to "other"
//
// Predicate values, field path names, document IDs, row-group IDs, term IDs,
// and any raw user content must never appear in INFO-level log attributes.
// Those details may appear only in trace events or behind a LevelDebug guard.
package logging
