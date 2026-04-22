package telemetry

// Frozen operation name constants for coarse boundary instrumentation.
// These are the only operation strings emitted at INFO level.
// New operation names must be added here; callers must not construct
// free-form operation strings at call sites.
const (
	OperationEvaluate         = "query.evaluate"
	OperationEncode           = "serialize.encode"
	OperationDecode           = "serialize.decode"
	OperationBuildFromParquet = "parquet.build"
)

// ErrorTypeOther is the canonical fallback value for the frozen
// error.type vocabulary. Classifiers in parent packages should reuse this
// constant rather than declaring their own "other" literal to avoid drift.
const ErrorTypeOther = "other"
