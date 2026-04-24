package gin

import (
	"fmt"
	"strconv"
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
// Value is a verbatim string representation of the offending input or value;
// the library does not redact or truncate it. Callers that log untrusted
// documents own their redaction and output-size policy.
type IngestError struct {
	Path  string
	Layer IngestLayer
	Value string
	Err   error
}

// Error returns a human-readable message for the hard ingest failure.
func (e *IngestError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Path == "" {
		return fmt.Sprintf("ingest %s failure: %v", e.Layer, e.Err)
	}
	return fmt.Sprintf("ingest %s failure at %s: %v", e.Layer, e.Path, e.Err)
}

// Unwrap returns the underlying cause for stdlib error unwrapping.
func (e *IngestError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Cause returns the underlying cause for github.com/pkg/errors compatibility.
func (e *IngestError) Cause() error {
	if e == nil {
		return nil
	}
	return e.Err
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
		Path:  path,
		Layer: layer,
		Value: value,
		Err:   err,
	}
}

func formatStagedNumericValue(value stagedNumericValue) string {
	if value.raw != "" {
		return value.raw
	}
	if value.isInt {
		return strconv.FormatInt(value.intVal, 10)
	}
	return strconv.FormatFloat(value.floatVal, 'g', -1, 64)
}
