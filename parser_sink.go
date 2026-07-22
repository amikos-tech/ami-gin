package gin

import (
	"math"

	"github.com/pkg/errors"
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
// Scalar staging: StageScalar is the stdlib token path. Parsers with typed
// scalar accessors should use the exact StageString, StageBool, StageInt64,
// StageUint64, and StageFloat64 routes instead.
//
// Numeric staging: prefer StageJSONNumber when the parser still has raw source
// text. Exact typed parsers should use StageInt64 or StageUint64 for integers
// and StageFloat64 for lexeme-classified floats. StageNativeNumeric remains for
// already-materialized Go values and may fold whole float64 values to integers.
type parserSink interface {
	BeginDocument(rgID int) *documentBuildState
	MarkPresent(state *documentBuildState, canonicalPath string)
	StageScalar(state *documentBuildState, canonicalPath string, token any) error
	StageString(state *documentBuildState, canonicalPath string, v string) error
	StageBool(state *documentBuildState, canonicalPath string, v bool) error
	StageInt64(state *documentBuildState, canonicalPath string, v int64) error
	StageUint64(state *documentBuildState, canonicalPath string, v uint64) error
	StageFloat64(state *documentBuildState, canonicalPath string, v float64) error
	StageJSONNumber(state *documentBuildState, canonicalPath, raw string) error
	StageNativeNumeric(state *documentBuildState, canonicalPath string, v any) error
	StageMaterialized(state *documentBuildState, path string, value any, allowTransform bool) error
	ShouldBufferForTransform(canonicalPath string) bool
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

func (b *GINBuilder) StageString(state *documentBuildState, canonicalPath string, v string) error {
	return tagStageError(b.stageScalarToken(canonicalPath, v, state))
}

func (b *GINBuilder) StageBool(state *documentBuildState, canonicalPath string, v bool) error {
	return tagStageError(b.stageScalarToken(canonicalPath, v, state))
}

func (b *GINBuilder) StageInt64(state *documentBuildState, canonicalPath string, v int64) error {
	return tagStageError(b.stageNativeNumeric(canonicalPath, v, state))
}

func (b *GINBuilder) StageUint64(state *documentBuildState, canonicalPath string, v uint64) error {
	return tagStageError(b.stageNativeNumeric(canonicalPath, v, state))
}

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

func (b *GINBuilder) StageNativeNumeric(state *documentBuildState, canonicalPath string, v any) error {
	return tagStageError(b.stageNativeNumeric(canonicalPath, v, state))
}

func (b *GINBuilder) StageMaterialized(state *documentBuildState, path string, value any, allowTransform bool) error {
	return tagStageError(b.stageMaterializedValue(path, value, state, allowTransform))
}

func (b *GINBuilder) ShouldBufferForTransform(canonicalPath string) bool {
	return len(b.config.representations(canonicalPath)) > 0
}

var _ parserSink = (*GINBuilder)(nil)
