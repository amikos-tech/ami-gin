package gin

import (
	"fmt"
	"strconv"
)

// IngestLayer identifies the ingest layer that rejected a document.
type IngestLayer string

const (
	IngestLayerParser      IngestLayer = "parser"
	IngestLayerTransformer IngestLayer = "transformer"
	IngestLayerNumeric     IngestLayer = "numeric"
	IngestLayerSchema      IngestLayer = "schema"
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

func (e *IngestError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Path == "" {
		return fmt.Sprintf("ingest %s failure: %v", e.Layer, e.Err)
	}
	return fmt.Sprintf("ingest %s failure at %s: %v", e.Layer, e.Path, e.Err)
}

func (e *IngestError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

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
	if value.isInt {
		return strconv.FormatInt(value.intVal, 10)
	}
	return strconv.FormatFloat(value.floatVal, 'g', -1, 64)
}
