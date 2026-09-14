package gin

import (
	"fmt"

	"github.com/pkg/errors"
)

// IngestLayer identifies the ingest layer that rejected a document.
type IngestLayer string

const (
	// IngestLayerParser identifies JSON parser failures for a document.
	IngestLayerParser IngestLayer = "parser"

	// IngestLayerTransformer identifies field transformer failures for a document.
	IngestLayerTransformer IngestLayer = "transformer"

	// IngestLayerNumeric identifies numeric coercion or promotion failures for a document.
	IngestLayerNumeric IngestLayer = "numeric"

	// IngestLayerSchema identifies unsupported value-shape failures for a document.
	IngestLayerSchema IngestLayer = "schema"

	// IngestLayerResource identifies a builder resource limit that rejected an
	// otherwise valid document. Callers can rebuild with a higher limit.
	IngestLayerResource IngestLayer = "resource"
)

// IngestError reports a hard per-document ingest failure.
//
// Path() returns the source JSONPath that rejected the document. Parser-level
// failures that are not attributable to a path report the empty string.
//
// Layer() identifies the ingest stage that rejected the document. Callers must
// tolerate future layer strings in addition to the built-in parser,
// transformer, numeric, schema, and resource values.
//
// Value() returns a verbatim string representation of the offending input or
// value for document-data failures. It is empty for resource failures, which
// have no offending document value; their diagnostics are available from
// Cause(). The library does not redact or truncate document-data values, so
// callers that log untrusted documents own their redaction and output-size
// policy.
type IngestError struct {
	path  string
	layer IngestLayer
	value string
	err   error
}

// Path returns the source JSONPath that rejected the document.
func (e *IngestError) Path() string {
	if e == nil {
		return ""
	}
	return e.path
}

// Layer returns the ingest stage that rejected the document.
func (e *IngestError) Layer() IngestLayer {
	if e == nil {
		return ""
	}
	return e.layer
}

// Value returns the verbatim offending input or transformed value. It is empty
// for resource failures, which do not have an offending document value.
func (e *IngestError) Value() string {
	if e == nil {
		return ""
	}
	return e.value
}

// Error returns a stable human-readable message for the hard ingest failure.
// The "ingest <layer> failure [at <path>]: <cause>" format is API-stable.
func (e *IngestError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.path == "" {
		return fmt.Sprintf("ingest %s failure: %v", e.layer, e.err)
	}
	return fmt.Sprintf("ingest %s failure at %s: %v", e.layer, e.path, e.err)
}

// Unwrap returns the underlying cause for stdlib error unwrapping.
func (e *IngestError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// Cause returns the underlying cause for github.com/pkg/errors compatibility.
func (e *IngestError) Cause() error {
	if e == nil {
		return nil
	}
	return e.err
}

func newIngestError(layer IngestLayer, path string, value any, err error) error {
	if err == nil {
		return nil
	}
	return newIngestErrorString(layer, path, fmt.Sprint(value), err)
}

func newIngestErrorString(layer IngestLayer, path string, value string, err error) error {
	if err == nil {
		return nil
	}
	if layer == IngestLayerResource && value != "" {
		panic(errors.Errorf("newIngestErrorString: resource IngestError must carry no value, got %q", value))
	}
	return &IngestError{
		path:  path,
		layer: layer,
		value: value,
		err:   err,
	}
}
