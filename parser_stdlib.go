package gin

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
)

const stdlibParserName = "stdlib"

// stdlibParser is the default Parser: wraps json.Decoder.UseNumber() and
// produces byte-identical staging calls to the pre-Phase-13 direct path.
// Zero-field struct + value receivers avoid heap allocation when boxed
// into the Parser interface.
type stdlibParser struct{}

func (stdlibParser) Name() string { return stdlibParserName }

func (s stdlibParser) Parse(jsonDoc []byte, rgID int, sink parserSink) error {
	decoder := json.NewDecoder(bytes.NewReader(jsonDoc))
	decoder.UseNumber()

	state := sink.BeginDocument(rgID)
	if err := s.streamValue(decoder, "$", state, sink); err != nil {
		return err
	}
	if err := ensureDecoderEOF(decoder); err != nil {
		return errors.Wrap(err, "failed to parse JSON")
	}
	return nil
}

func (s stdlibParser) streamValue(decoder *json.Decoder, path string, state *documentBuildState, sink parserSink) error {
	canonicalPath := normalizeWalkPath(path)
	if sink.ShouldBufferForTransform(canonicalPath) {
		value, err := decodeAny(decoder)
		if err != nil {
			return errors.Wrapf(err, "parse transformed subtree at %s", canonicalPath)
		}
		return sink.StageMaterialized(state, path, value, true)
	}

	token, err := decoder.Token()
	if err != nil {
		return errors.Wrap(err, "read JSON token")
	}

	switch tok := token.(type) {
	case json.Delim:
		sink.MarkPresent(state, canonicalPath)
		switch tok {
		case '{':
			objectValues := make(map[string]any)
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return errors.Wrapf(err, "read object key at %s", canonicalPath)
				}
				// encoding/json only emits string object keys here. Keep the
				// assertion defensive in case this walker is reused with a
				// different token source.
				key, ok := keyToken.(string)
				if !ok {
					return errors.Errorf("non-string object key at %s", canonicalPath)
				}
				value, err := decodeAny(decoder)
				if err != nil {
					return errors.Wrapf(err, "parse object value at %s.%s", canonicalPath, key)
				}
				objectValues[key] = value
			}
			for _, key := range sortedObjectKeys(objectValues) {
				if err := sink.StageMaterialized(state, path+"."+key, objectValues[key], true); err != nil {
					return err
				}
			}
			end, err := decoder.Token()
			if err != nil {
				return errors.Wrapf(err, "close object at %s", canonicalPath)
			}
			// encoding/json should only yield the matching closing
			// delimiter or an error here. Keep the check defensive.
			if delim, ok := end.(json.Delim); !ok || delim != '}' {
				return errors.Errorf("malformed object at %s", canonicalPath)
			}
			return nil
		case '[':
			for i := 0; decoder.More(); i++ {
				item, err := decodeAny(decoder)
				if err != nil {
					return errors.Wrapf(err, "parse array element at %s[%d]", canonicalPath, i)
				}
				if err := sink.StageMaterialized(state, fmt.Sprintf("%s[%d]", path, i), item, true); err != nil {
					return err
				}
				if err := sink.StageMaterialized(state, path+"[*]", item, true); err != nil {
					return err
				}
			}
			end, err := decoder.Token()
			if err != nil {
				return errors.Wrapf(err, "close array at %s", canonicalPath)
			}
			// encoding/json should only yield the matching closing
			// delimiter or an error here. Keep the check defensive.
			if delim, ok := end.(json.Delim); !ok || delim != ']' {
				return errors.Errorf("malformed array at %s", canonicalPath)
			}
			return nil
		default:
			// At a value boundary, encoding/json should only surface '{'
			// or '[' delimiters. Treat anything else as a parser bug.
			return errors.Errorf("unsupported delimiter %q at %s", tok, canonicalPath)
		}
	default:
		return sink.StageScalar(state, canonicalPath, token)
	}
}

var _ Parser = stdlibParser{}
