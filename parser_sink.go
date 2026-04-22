package gin

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
// Numeric staging: prefer StageJSONNumber when the parser still has the
// raw source text — the builder's classifier is the single source of
// truth for numeric type and raw text preserves
// exact-int64 semantics outside [-2^53, 2^53]. Use StageNativeNumeric
// only when the parser has already decoded to a Go numeric inside the
// float64-exact range (e.g. SIMD / streaming parsers that skip the
// json.Number intermediate).
type parserSink interface {
	BeginDocument(rgID int) *documentBuildState
	MarkPresent(state *documentBuildState, canonicalPath string)
	StageScalar(state *documentBuildState, canonicalPath string, token any) error
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
	return b.stageScalarToken(canonicalPath, token, state)
}

func (b *GINBuilder) StageJSONNumber(state *documentBuildState, canonicalPath, raw string) error {
	return b.stageJSONNumberLiteral(canonicalPath, raw, state)
}

func (b *GINBuilder) StageNativeNumeric(state *documentBuildState, canonicalPath string, v any) error {
	return b.stageNativeNumeric(canonicalPath, v, state)
}

func (b *GINBuilder) StageMaterialized(state *documentBuildState, path string, value any, allowTransform bool) error {
	return b.stageMaterializedValue(path, value, state, allowTransform)
}

func (b *GINBuilder) ShouldBufferForTransform(canonicalPath string) bool {
	return len(b.config.representations(canonicalPath)) > 0
}

var _ parserSink = (*GINBuilder)(nil)
