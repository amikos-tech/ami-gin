package gin

import (
	"math"

	"github.com/pkg/errors"

	"github.com/amikos-tech/ami-gin/logging"
)

// parserSink is the narrow write contract a Parser uses to publish
// observations. It is intentionally package-private so alternative
// parsers cannot reach into the builder's internals. *documentBuildState
// is exposed as an OPAQUE handle; parsers MUST NOT read its fields.
//
// Path argument convention (per method):
//   - canonicalPath: already-normalized path (via normalizeWalkPath).
//     Parser MUST pre-normalize before calling.
//   - path: raw, un-normalized path. The sink impl normalizes internally
//     (matches today's stageMaterializedValue behavior).
//
// Scalar staging: use StageScalar for nil, bool, string, and json.Number
// tokens.
//
// Numeric staging: prefer StageJSONNumber when the parser still has raw source
// text. Exact typed parsers should use StageInt64 for signed integers,
// StageUint64 only for values no larger than math.MaxInt64, and StageFloat64
// for lexeme-classified floats. Larger integers fail under NumericFailureMode.
// StageFloat64 always preserves float classification; StageNativeNumeric is for
// already-materialized Go values and may fold whole float64 values to integers.
type parserSink interface {
	BeginDocument(rgID int) *documentBuildState
	MarkPresent(state *documentBuildState, canonicalPath string)
	StageScalar(state *documentBuildState, canonicalPath string, token any) error
	StageInt64(state *documentBuildState, canonicalPath string, v int64) error
	StageUint64(state *documentBuildState, canonicalPath string, v uint64) error
	StageFloat64(state *documentBuildState, canonicalPath string, v float64) error
	StageJSONNumber(state *documentBuildState, canonicalPath, raw string) error
	StageNativeNumeric(state *documentBuildState, canonicalPath string, v any) error
	StageMaterialized(state *documentBuildState, path string, value any, allowTransform bool) error
	ShouldBufferForTransform(canonicalPath string) bool
	// Logger returns the builder's configured logger for parser-side
	// structured logging (e.g. SIMD close-error reporting).
	Logger() logging.Logger
}

func (b *GINBuilder) BeginDocument(rgID int) *documentBuildState {
	s := newDocumentBuildState(rgID)
	b.currentDocState = s
	b.beginDocumentCalls++
	return s
}

func (b *GINBuilder) MarkPresent(state *documentBuildState, canonicalPath string) {
	state.getOrCreatePath(canonicalPath).present = true
}

func (b *GINBuilder) StageScalar(state *documentBuildState, canonicalPath string, token any) error {
	return tagStageError(b.stageScalarToken(canonicalPath, token, state))
}

func (b *GINBuilder) StageInt64(state *documentBuildState, canonicalPath string, v int64) error {
	return tagStageError(b.stageNativeNumeric(canonicalPath, v, state))
}

// StageUint64 requires v no larger than math.MaxInt64; see StageFloat64 and
// StageNativeNumeric for related numeric-staging contracts.
func (b *GINBuilder) StageUint64(state *documentBuildState, canonicalPath string, v uint64) error {
	return tagStageError(b.stageNativeNumeric(canonicalPath, v, state))
}

// StageFloat64 preserves the source's float classification. Unlike
// StageNativeNumeric, it never folds a whole float64 value to an integer.
func (b *GINBuilder) StageFloat64(state *documentBuildState, canonicalPath string, v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		if b.config.NumericFailureMode == IngestFailureSoft {
			return tagStageError(newSoftSkipNumericDocumentError(canonicalPath))
		}
		return tagStageError(newIngestError(
			IngestLayerNumeric,
			canonicalPath,
			v,
			errors.New("non-finite numeric value"),
		))
	}
	return tagStageError(b.stageNumericObservation(canonicalPath, stagedNumericValue{
		isInt:    false,
		floatVal: v,
	}, state))
}

func (b *GINBuilder) StageJSONNumber(state *documentBuildState, canonicalPath, raw string) error {
	return tagStageError(b.stageJSONNumberLiteral(canonicalPath, raw, state))
}

// StageNativeNumeric stages an already-materialized Go number. Unlike
// StageFloat64, it may fold a whole float64 value to the integer domain.
func (b *GINBuilder) StageNativeNumeric(state *documentBuildState, canonicalPath string, v any) error {
	return tagStageError(b.stageNativeNumeric(canonicalPath, v, state))
}

func (b *GINBuilder) StageMaterialized(state *documentBuildState, path string, value any, allowTransform bool) error {
	return tagStageError(b.stageMaterializedValue(path, value, state, allowTransform))
}

func (b *GINBuilder) ShouldBufferForTransform(canonicalPath string) bool {
	return len(b.config.representations(canonicalPath)) > 0
}

func (b *GINBuilder) Logger() logging.Logger {
	return configLogger(&b.config)
}

var _ parserSink = (*GINBuilder)(nil)
