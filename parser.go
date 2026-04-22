package gin

import (
	"github.com/pkg/errors"
)

// Parser translates one JSON document into staged per-path observations,
// writing them through the supplied sink. Implementations MUST preserve
// exact-int64 semantics: integers outside the float64-exact range
// [-2^53, 2^53] must be reported via sink.StageJSONNumber (raw source
// text) so the builder's classifier stays the single source of truth
// for numeric type.
//
// Parse MUST NOT wrap errors on behalf of the builder. The caller
// (AddDocument) returns Parse errors verbatim.
//
// External implementability: the sink type referenced by Parse (parserSink)
// is package-private; third-party Parser implementations outside package gin
// cannot satisfy this interface. The Parser name is exported today so
// WithParser remains a stable entry point; exporting the sink and enabling
// external parsers is deferred.
type Parser interface {
	// Name returns a stable identifier for telemetry (e.g. "stdlib").
	// MUST NOT return the empty string; NewBuilder rejects an empty name.
	Name() string

	// Parse walks jsonDoc and stages observations for rgID via sink.
	// The parser's first sink call MUST be sink.BeginDocument(rgID), and
	// Parse MUST call BeginDocument exactly once. AddDocument enforces
	// this with a post-Parse runtime guard.
	//
	// Present-marking contract: for object and array roots, Parse MUST
	// call sink.MarkPresent for the container's canonicalPath before
	// staging children; otherwise IsNull / IsNotNull queries will return
	// wrong results for that path. All Stage* sink methods (StageScalar,
	// StageJSONNumber, StageNativeNumeric, StageMaterialized) implicitly
	// mark their path present.
	Parse(jsonDoc []byte, rgID int, sink parserSink) error
}

// WithParser installs a custom JSON parser. The default is stdlibParser
// (encoding/json.Decoder with UseNumber), which preserves v1.0 behavior
// byte-identically. Supplying nil returns an error. If supplied multiple
// times, the last WithParser wins (BuilderOption convention).
//
// NOTE: external (out-of-package) implementations of Parser are not
// currently possible because parserSink is package-private. WithParser
// exists today as a forward-compat entry point and a seam for testing and
// internal telemetry.
func WithParser(p Parser) BuilderOption {
	return func(b *GINBuilder) error {
		if p == nil {
			return errors.New("parser cannot be nil")
		}
		b.parser = p
		return nil
	}
}
