//go:build simdjson

package gin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"

	purejson "github.com/amikos-tech/pure-simdjson"
	"github.com/pkg/errors"
)

const simdParserName = "pure-simdjson"

type simdParser struct {
	parser *purejson.Parser
}

func (*simdParser) Name() string { return simdParserName }

// NewSIMDParser constructs the optional pure-simdjson parser. Callers select
// it explicitly through WithParser; construction never falls back to stdlib.
// The caller owns the returned parser and must call Close after all builders
// using it have stopped parsing.
func NewSIMDParser() (CloseableParser, error) {
	const initializationContext = "initialize pure-simdjson SIMD parser; set PURE_SIMDJSON_LIB_PATH or see docs/simd-deployment.md"

	parser, err := purejson.NewParser()
	if err != nil {
		return nil, errors.Wrap(err, initializationContext)
	}
	return &simdParser{parser: parser}, nil
}

func (s *simdParser) Close() error {
	return errors.Wrap(s.parser.Close(), "close pure-simdjson SIMD parser")
}

func (s *simdParser) Parse(jsonDoc []byte, rgID int, sink parserSink) (err error) {
	doc, err := s.parser.Parse(jsonDoc)
	if err != nil {
		if errors.Is(err, purejson.ErrInvalidJSON) {
			if numericErr, routed := routeSIMDNumericParseFailure(jsonDoc, rgID, sink); routed {
				if numericErr != nil {
					return numericErr
				}
			}
		}
		return errors.Wrap(err, "failed to parse JSON")
	}
	return finishSIMDDocument(
		func() error {
			state := sink.BeginDocument(rgID)
			return s.walkElement(doc.Root(), "$", state, sink)
		},
		doc.Close,
	)
}

// routeSIMDNumericParseFailure distinguishes malformed JSON from valid numeric
// literals that the native parser rejects before it can expose a typed element.
// It performs no indexing work: only the first unstageable number is sent
// through StageJSONNumber so NumericFailureMode and path reporting remain
// identical to the stdlib path.
func routeSIMDNumericParseFailure(
	jsonDoc []byte,
	rgID int,
	sink parserSink,
) (err error, routed bool) {
	decoder := json.NewDecoder(bytes.NewReader(jsonDoc))
	decoder.UseNumber()

	value, err := decodeAny(decoder)
	if err != nil {
		return nil, false
	}
	if err := ensureDecoderEOF(decoder); err != nil {
		return nil, false
	}

	path, raw, ok := findUnstageableJSONNumber("$", value)
	if !ok {
		return nil, false
	}

	state := sink.BeginDocument(rgID)
	return sink.StageJSONNumber(state, normalizeWalkPath(path), raw), true
}

func findUnstageableJSONNumber(path string, value any) (string, string, bool) {
	switch v := value.(type) {
	case json.Number:
		if _, _, _, err := parseJSONNumberLiteral(v.String()); err != nil {
			return path, v.String(), true
		}
	case []any:
		for i, item := range v {
			if invalidPath, raw, ok := findUnstageableJSONNumber(
				fmt.Sprintf("%s[%d]", path, i),
				item,
			); ok {
				return invalidPath, raw, true
			}
		}
	case map[string]any:
		for _, key := range sortedObjectKeys(v) {
			if invalidPath, raw, ok := findUnstageableJSONNumber(
				path+"."+key,
				v[key],
			); ok {
				return invalidPath, raw, true
			}
		}
	}
	return "", "", false
}

// finishSIMDDocument runs a document walk and releases the native document
// exactly once. A walk panic always remains the primary outcome; a concurrent
// close error is logged before the original panic value is re-raised.
func finishSIMDDocument(walk func() error, closeDocument func() error) (err error) {
	defer func() {
		recovered := recover()
		closeErr := closeDocument()
		if recovered != nil {
			if closeErr != nil {
				log.Printf("gin: close pure-simdjson document after walk panic: %v", closeErr)
			}
			panic(recovered)
		}
		if closeErr == nil {
			return
		}

		err = newParserLifecycleError(
			errors.Wrap(closeErr, "close pure-simdjson document"),
			err,
		)
	}()

	return walk()
}

func (s *simdParser) walkElement(
	element purejson.Element,
	rawPath string,
	state *documentBuildState,
	sink parserSink,
) error {
	canonicalPath := normalizeWalkPath(rawPath)
	if sink.ShouldBufferForTransform(canonicalPath) {
		value, err := materializeElement(element, rawPath)
		if err != nil {
			return errors.Wrapf(err, "materialize transformed subtree at %s", canonicalPath)
		}
		return sink.StageMaterialized(state, rawPath, value, true)
	}

	elementType := element.Type()
	switch elementType {
	case purejson.TypeNull:
		return sink.StageScalar(state, canonicalPath, nil)
	case purejson.TypeBool:
		value, err := element.GetBool()
		if err != nil {
			return errors.Wrapf(err, "read pure-simdjson bool at %s", canonicalPath)
		}
		return sink.StageScalar(state, canonicalPath, value)
	case purejson.TypeInt64:
		value, err := element.GetInt64()
		if err != nil {
			return errors.Wrapf(err, "read pure-simdjson int64 at %s", canonicalPath)
		}
		return sink.StageInt64(state, canonicalPath, value)
	case purejson.TypeUint64:
		value, err := element.GetUint64()
		if err != nil {
			return errors.Wrapf(err, "read pure-simdjson uint64 at %s", canonicalPath)
		}
		return sink.StageUint64(state, canonicalPath, value)
	case purejson.TypeFloat64:
		value, err := element.GetFloat64()
		if err != nil {
			return errors.Wrapf(err, "read pure-simdjson float64 at %s", canonicalPath)
		}
		return sink.StageFloat64(state, canonicalPath, value)
	case purejson.TypeString:
		value, err := element.GetString()
		if err != nil {
			return errors.Wrapf(err, "read pure-simdjson string at %s", canonicalPath)
		}
		return sink.StageScalar(state, canonicalPath, value)
	case purejson.TypeObject:
		sink.MarkPresent(state, canonicalPath)
		object, err := element.AsObject()
		if err != nil {
			return errors.Wrapf(err, "read pure-simdjson object at %s", canonicalPath)
		}

		entries := make(map[string]purejson.Element)
		iterator := object.Iter()
		for iterator.Next() {
			entries[iterator.Key()] = iterator.Value()
		}
		if err := iterator.Err(); err != nil {
			return errors.Wrapf(err, "iterate pure-simdjson object at %s", canonicalPath)
		}

		keys := make([]string, 0, len(entries))
		for key := range entries {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if err := s.walkElement(entries[key], rawPath+"."+key, state, sink); err != nil {
				return err
			}
		}
		return nil
	case purejson.TypeArray:
		sink.MarkPresent(state, canonicalPath)
		array, err := element.AsArray()
		if err != nil {
			return errors.Wrapf(err, "read pure-simdjson array at %s", canonicalPath)
		}

		iterator := array.Iter()
		for i := 0; iterator.Next(); i++ {
			value := iterator.Value()
			if err := s.walkElement(value, fmt.Sprintf("%s[%d]", rawPath, i), state, sink); err != nil {
				return err
			}
			if err := s.walkElement(value, rawPath+"[*]", state, sink); err != nil {
				return err
			}
		}
		if err := iterator.Err(); err != nil {
			return errors.Wrapf(err, "iterate pure-simdjson array at %s", canonicalPath)
		}
		return nil
	case purejson.TypeInvalid:
		return errors.Errorf("invalid pure-simdjson element at %s", canonicalPath)
	default:
		return errors.Errorf("unsupported pure-simdjson element type %d at %s", elementType, canonicalPath)
	}
}

func materializeElement(element purejson.Element, rawPath string) (any, error) {
	canonicalPath := normalizeWalkPath(rawPath)
	elementType := element.Type()
	switch elementType {
	case purejson.TypeNull:
		return nil, nil
	case purejson.TypeBool:
		value, err := element.GetBool()
		if err != nil {
			return nil, errors.Wrapf(err, "materialize pure-simdjson bool at %s", canonicalPath)
		}
		return value, nil
	case purejson.TypeInt64:
		value, err := element.GetInt64()
		if err != nil {
			return nil, errors.Wrapf(err, "materialize pure-simdjson int64 at %s", canonicalPath)
		}
		return json.Number(strconv.FormatInt(value, 10)), nil
	case purejson.TypeUint64:
		value, err := element.GetUint64()
		if err != nil {
			return nil, errors.Wrapf(err, "materialize pure-simdjson uint64 at %s", canonicalPath)
		}
		return json.Number(strconv.FormatUint(value, 10)), nil
	case purejson.TypeFloat64:
		value, err := element.GetFloat64()
		if err != nil {
			return nil, errors.Wrapf(err, "materialize pure-simdjson float64 at %s", canonicalPath)
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, errors.Errorf("materialize pure-simdjson float64 at %s: non-finite numeric value", canonicalPath)
		}
		raw := strconv.FormatFloat(value, 'g', -1, 64)
		if !strings.ContainsAny(raw, ".eE") {
			raw += ".0"
		}
		return json.Number(raw), nil
	case purejson.TypeString:
		value, err := element.GetString()
		if err != nil {
			return nil, errors.Wrapf(err, "materialize pure-simdjson string at %s", canonicalPath)
		}
		return value, nil
	case purejson.TypeArray:
		array, err := element.AsArray()
		if err != nil {
			return nil, errors.Wrapf(err, "materialize pure-simdjson array at %s", canonicalPath)
		}

		values := make([]any, 0)
		iterator := array.Iter()
		for i := 0; iterator.Next(); i++ {
			value, err := materializeElement(iterator.Value(), fmt.Sprintf("%s[%d]", rawPath, i))
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		if err := iterator.Err(); err != nil {
			return nil, errors.Wrapf(err, "materialize pure-simdjson array iterator at %s", canonicalPath)
		}
		return values, nil
	case purejson.TypeObject:
		object, err := element.AsObject()
		if err != nil {
			return nil, errors.Wrapf(err, "materialize pure-simdjson object at %s", canonicalPath)
		}

		values := make(map[string]any)
		iterator := object.Iter()
		for iterator.Next() {
			key := iterator.Key()
			value, err := materializeElement(iterator.Value(), rawPath+"."+key)
			if err != nil {
				return nil, err
			}
			values[key] = value
		}
		if err := iterator.Err(); err != nil {
			return nil, errors.Wrapf(err, "materialize pure-simdjson object iterator at %s", canonicalPath)
		}
		return values, nil
	case purejson.TypeInvalid:
		return nil, errors.Errorf("cannot materialize invalid pure-simdjson element at %s", canonicalPath)
	default:
		return nil, errors.Errorf(
			"cannot materialize unsupported pure-simdjson element type %d at %s",
			elementType,
			canonicalPath,
		)
	}
}

var _ CloseableParser = (*simdParser)(nil)
