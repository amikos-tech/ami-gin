package logging

import "strings"

// Attr is a typed key-value pair used by the repo-owned logging contract.
// The typed surface (rather than variadic any) lets the compiler enforce
// the frozen vocabulary: callers must construct attrs through the named
// helpers below rather than passing raw strings or dynamic values.
type Attr struct {
	Key   string
	Value string
}

// Frozen INFO-level attribute keys. These are the only keys emitted at INFO
// level. Predicate values, path names, doc IDs, row-group IDs, term IDs, and
// raw user content must NOT appear in INFO-level log attrs.
const (
	keyOperation   = "operation"
	keyPredicateOp = "predicate_op"
	keyPathMode    = "path_mode"
	keyStatus      = "status"
	keyErrorType   = "error.type"
)

// Bounded path_mode values. Query evaluation uses PathMode.String() which
// maps to one of these labels only. Tests should enforce both the key list
// and the bounded value set.
const (
	PathModeExact           = "exact"
	PathModeBloomOnly       = "bloom-only"
	PathModeAdaptiveHybrid  = "adaptive-hybrid"
)

// AttrOperation returns an operation attr using the frozen key.
func AttrOperation(name string) Attr {
	return Attr{Key: keyOperation, Value: name}
}

// AttrPredicateOp returns a predicate_op attr using the frozen key.
func AttrPredicateOp(op string) Attr {
	return Attr{Key: keyPredicateOp, Value: op}
}

// AttrPathMode returns a path_mode attr using the frozen key.
// Callers must use one of the PathMode* constants as the value.
func AttrPathMode(mode string) Attr {
	return Attr{Key: keyPathMode, Value: mode}
}

// AttrStatus returns a status attr using the frozen key.
func AttrStatus(status string) Attr {
	return Attr{Key: keyStatus, Value: status}
}

// AttrErrorType returns an error.type attr using the frozen key.
// Unknown kinds collapse to "other".
func AttrErrorType(kind string) Attr {
	return Attr{Key: keyErrorType, Value: normalizeErrorType(kind)}
}

// normalizeErrorType collapses unknown error.type values to "other".
// The allowed set mirrors the telemetry package vocabulary.
func normalizeErrorType(kind string) string {
	normalized := strings.ToLower(strings.TrimSpace(kind))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")

	switch normalized {
	case "config", "io", "invalid_format", "deserialization", "integrity", "not_found", "other":
		return normalized
	default:
		return "other"
	}
}
