package gin

import (
	"github.com/pkg/errors"
)

// parserLifecycleError marks a parser failure after which the parser cannot
// safely accept another document. cleanupErr is the lifecycle failure and
// concurrentErr retains any walk or Stage callback failure observed before
// cleanup completed.
type parserLifecycleError struct {
	cleanupErr    error
	concurrentErr error
}

func newParserLifecycleError(cleanupErr, concurrentErr error) *parserLifecycleError {
	if cleanupErr == nil {
		cleanupErr = errors.New("parser lifecycle failure without cleanup error")
	}
	return &parserLifecycleError{
		cleanupErr:    cleanupErr,
		concurrentErr: concurrentErr,
	}
}

func (e *parserLifecycleError) Error() string {
	if e.concurrentErr == nil {
		return e.cleanupErr.Error()
	}
	return e.cleanupErr.Error() + "; concurrent parser failure: " + e.concurrentErr.Error()
}

func (e *parserLifecycleError) Unwrap() []error {
	if e.concurrentErr == nil {
		return []error{e.cleanupErr}
	}
	return []error{e.cleanupErr, e.concurrentErr}
}

func isParserLifecycleError(err error) bool {
	var lifecycleErr *parserLifecycleError
	return errors.As(err, &lifecycleErr)
}

func panicValueAsError(value any) error {
	if err, ok := value.(error); ok {
		return err
	}
	return errors.Errorf("%v", value)
}

// Parser translates one JSON document into staged per-path observations,
// writing them through the supplied sink. Implementations MUST preserve the
// source number's integer or float classification without coercing exact
// integers through float64. Raw-text parsers use sink.StageJSONNumber so the
// builder classifies the source lexeme. Exact typed parsers may use
// sink.StageInt64 for signed integers, sink.StageUint64 for unsigned integers,
// and sink.StageFloat64 for lexeme-classified floats. Unsigned integer values
// above math.MaxInt64 fail at the numeric layer.
//
// Parse may add parser-local context to errors it creates. Errors returned by
// sink Stage callbacks must retain their layer. AddDocument preserves staged
// error provenance and applies parser hard/soft classification only to errors
// that did not originate from a Stage callback. A parser that cannot safely
// parse another document must return a parserLifecycleError; AddDocument then
// terminates the builder regardless of ParserFailureMode.
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

	// Parse walks jsonDoc and stages observations for rgID via sink. The
	// parser's first sink call MUST be sink.BeginDocument(rgID), and Parse
	// MUST call BeginDocument exactly once. After Parse returns successfully,
	// AddDocument verifies the call count and rgID; call ordering and error
	// paths remain the parser implementation's responsibility.
	//
	// Present-marking contract: for object and array roots, Parse MUST
	// call sink.MarkPresent for the container's canonicalPath before
	// staging children; otherwise IsNull / IsNotNull queries will return
	// wrong results for that path. All Stage* sink methods (StageScalar,
	// StageInt64, StageUint64, StageFloat64, StageJSONNumber,
	// StageNativeNumeric, and StageMaterialized) implicitly mark their path
	// present.
	Parse(jsonDoc []byte, rgID int, sink parserSink) error
}

// CloseableParser is a Parser that owns resources requiring deterministic
// release. The caller owns a CloseableParser and must call Close after every
// builder using it has stopped parsing. GINBuilder never closes a supplied
// parser because parser ownership remains with the caller.
type CloseableParser interface {
	Parser
	Close() error
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
//
// If p also implements CloseableParser, the caller retains the obligation to
// call Close: GINBuilder never closes a supplied parser.
func WithParser(p Parser) BuilderOption {
	return func(b *GINBuilder) error {
		if p == nil {
			return errors.New("parser cannot be nil")
		}
		b.parser = p
		return nil
	}
}
