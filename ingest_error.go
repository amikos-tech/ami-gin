package gin

import (
	"fmt"
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
)

// IngestError reports a hard per-document ingest failure.
//
// Path() returns the source JSONPath that rejected the document. Parser-level
// failures that are not attributable to a path report the empty string.
//
// Layer() identifies the ingest stage that rejected the document. Callers must
// tolerate future layer strings in addition to the built-in parser,
// transformer, numeric, and schema values.
//
// Value() returns a verbatim string representation of the offending input or
// value. The library does not redact or truncate it; callers that log
// untrusted documents own their redaction and output-size policy.
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

// Value returns the verbatim offending input or transformed value.
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
	return &IngestError{
		path:  path,
		layer: layer,
		value: value,
		err:   err,
	}
}
