//go:build simdjson

package gin

import (
	stderrors "errors"
	"math"
	"strings"
	"testing"

	"github.com/pkg/errors"
)

func TestSIMDDocumentLifecycleCleanupRunsExactlyOnce(t *testing.T) {
	walkCause := errors.New("walk sentinel")
	softCause := newSoftSkipNumericDocumentError("$.score")
	tests := []struct {
		name    string
		walkErr error
	}{
		{name: "success"},
		{name: "walk-error", walkErr: walkCause},
		{name: "soft-stage-error", walkErr: softCause},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			walkCalls := 0
			closeCalls := 0
			got := finishSIMDDocument(
				func() error {
					walkCalls++
					return tc.walkErr
				},
				func() error {
					closeCalls++
					return nil
				},
			)

			if got != tc.walkErr {
				t.Fatalf("finishSIMDDocument() = %v, want unchanged %v", got, tc.walkErr)
			}
			if walkCalls != 1 || closeCalls != 1 {
				t.Fatalf("callback calls = (walk=%d, close=%d), want (1, 1)", walkCalls, closeCalls)
			}
		})
	}
}

func TestSIMDDocumentLifecycleCleanupRunsWhenWalkPanics(t *testing.T) {
	closeCalls := 0
	var recovered any
	func() {
		defer func() {
			recovered = recover()
		}()
		_ = finishSIMDDocument(
			func() error { panic("walk panic") },
			func() error {
				closeCalls++
				return nil
			},
		)
	}()

	if recovered != "walk panic" {
		t.Fatalf("recovered panic = %v, want walk panic", recovered)
	}
	if closeCalls != 1 {
		t.Fatalf("close calls after walk panic = %d, want 1", closeCalls)
	}
}

type simdDocumentLifecycleTestParser struct {
	parseCalls int
	walkCalls  int
	closeCalls int
	walk       func(parserSink, *documentBuildState) error
	closeErr   error
}

func (*simdDocumentLifecycleTestParser) Name() string { return "simd-lifecycle-test" }

func (p *simdDocumentLifecycleTestParser) Parse(_ []byte, rgID int, sink parserSink) error {
	p.parseCalls++
	return finishSIMDDocument(
		func() error {
			p.walkCalls++
			state := sink.BeginDocument(rgID)
			return p.walk(sink, state)
		},
		func() error {
			p.closeCalls++
			return p.closeErr
		},
	)
}

func TestSIMDDocumentLifecycleCloseFailurePoisonsBuilder(t *testing.T) {
	closeCause := errors.New("close sentinel")
	parser := &simdDocumentLifecycleTestParser{
		walk: func(sink parserSink, state *documentBuildState) error {
			return sink.StageString(state, "$.name", "not-committed")
		},
		closeErr: closeCause,
	}
	config, err := NewConfig(WithParserFailureMode(IngestFailureSoft))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	builder, err := NewBuilder(config, 2, WithParser(parser))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	firstErr := builder.AddDocument(DocID(9), []byte(`{"name":"first"}`))
	if firstErr == nil {
		t.Fatal("first AddDocument error = nil, want fatal close failure")
	}
	if !stderrors.Is(firstErr, closeCause) {
		t.Fatalf("errors.Is(%v, closeCause) = false", firstErr)
	}
	var lifecycleErr *parserLifecycleError
	if !stderrors.As(firstErr, &lifecycleErr) {
		t.Fatalf("errors.As(%v, *parserLifecycleError) = false", firstErr)
	}
	if !strings.Contains(firstErr.Error(), "close pure-simdjson document") {
		t.Fatalf("first AddDocument error = %q, want cleanup context", firstErr.Error())
	}
	if parser.parseCalls != 1 || parser.walkCalls != 1 || parser.closeCalls != 1 {
		t.Fatalf(
			"callback calls after first AddDocument = (parse=%d, walk=%d, close=%d), want (1, 1, 1)",
			parser.parseCalls,
			parser.walkCalls,
			parser.closeCalls,
		)
	}
	requireSIMDLifecycleBuilderUncommitted(t, builder)

	storedErr := builder.Err()
	secondErr := builder.AddDocument(DocID(10), []byte(`{"name":"second"}`))
	if secondErr == nil {
		t.Fatal("second AddDocument error = nil, want prior tragic refusal")
	}
	if !stderrors.Is(secondErr, storedErr) {
		t.Fatalf("errors.Is(%v, storedErr) = false", secondErr)
	}
	if parser.parseCalls != 1 || parser.walkCalls != 1 || parser.closeCalls != 1 {
		t.Fatalf(
			"callback calls after second AddDocument = (parse=%d, walk=%d, close=%d), want (1, 1, 1)",
			parser.parseCalls,
			parser.walkCalls,
			parser.closeCalls,
		)
	}
	if builder.Err() != storedErr {
		t.Fatalf("builder.Err() changed from %p to %p", storedErr, builder.Err())
	}
}

func TestSIMDDocumentLifecycleCombinedErrorsPreserveProvenance(t *testing.T) {
	tests := []struct {
		name        string
		numericMode IngestFailureMode
		assertCause func(*testing.T, error)
	}{
		{
			name:        "hard-numeric",
			numericMode: IngestFailureHard,
			assertCause: func(t *testing.T, err error) {
				t.Helper()
				var stageErr *stageCallbackError
				if !stderrors.As(err, &stageErr) {
					t.Fatalf("errors.As(%v, *stageCallbackError) = false", err)
				}
				var ingestErr *IngestError
				if !stderrors.As(err, &ingestErr) {
					t.Fatalf("errors.As(%v, *IngestError) = false", err)
				}
				if ingestErr.Layer() != IngestLayerNumeric || ingestErr.Path() != "$.score" {
					t.Fatalf(
						"IngestError = (layer=%q, path=%q), want (numeric, $.score)",
						ingestErr.Layer(),
						ingestErr.Path(),
					)
				}
			},
		},
		{
			name:        "soft-numeric",
			numericMode: IngestFailureSoft,
			assertCause: func(t *testing.T, err error) {
				t.Helper()
				var skipErr *softSkipDocumentError
				if !stderrors.As(err, &skipErr) {
					t.Fatalf("errors.As(%v, *softSkipDocumentError) = false", err)
				}
				if !stderrors.Is(err, errSkipDocument) {
					t.Fatalf("errors.Is(%v, errSkipDocument) = false", err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			closeCause := errors.New("close sentinel")
			parser := &simdDocumentLifecycleTestParser{
				walk: func(sink parserSink, state *documentBuildState) error {
					return sink.StageFloat64(state, "$.score", math.NaN())
				},
				closeErr: closeCause,
			}
			config, err := NewConfig(
				WithParserFailureMode(IngestFailureSoft),
				WithNumericFailureMode(tc.numericMode),
			)
			if err != nil {
				t.Fatalf("NewConfig: %v", err)
			}
			builder, err := NewBuilder(config, 1, WithParser(parser))
			if err != nil {
				t.Fatalf("NewBuilder: %v", err)
			}

			addErr := builder.AddDocument(DocID(11), []byte(`{"score":0}`))
			if addErr == nil {
				t.Fatal("AddDocument error = nil, want fatal combined failure")
			}
			if !stderrors.Is(addErr, closeCause) {
				t.Fatalf("errors.Is(%v, closeCause) = false", addErr)
			}
			tc.assertCause(t, addErr)
			tc.assertCause(t, builder.Err())
			if !strings.Contains(addErr.Error(), "close pure-simdjson document") ||
				!strings.Contains(addErr.Error(), "numeric") {
				t.Fatalf("AddDocument error = %q, want cleanup and numeric context", addErr.Error())
			}
			if parser.parseCalls != 1 || parser.walkCalls != 1 || parser.closeCalls != 1 {
				t.Fatalf(
					"callback calls = (parse=%d, walk=%d, close=%d), want (1, 1, 1)",
					parser.parseCalls,
					parser.walkCalls,
					parser.closeCalls,
				)
			}
			requireSIMDLifecycleBuilderUncommitted(t, builder)
		})
	}
}

func requireSIMDLifecycleBuilderUncommitted(t *testing.T, builder *GINBuilder) {
	t.Helper()
	if builder.Err() == nil {
		t.Fatal("builder.Err() = nil, want fatal lifecycle error")
	}
	if builder.NumSoftSkippedDocuments() != 0 {
		t.Fatalf("NumSoftSkippedDocuments() = %d, want 0", builder.NumSoftSkippedDocuments())
	}
	if builder.numDocs != 0 || builder.nextPos != 0 {
		t.Fatalf("builder advanced after lifecycle failure: numDocs=%d nextPos=%d", builder.numDocs, builder.nextPos)
	}
	if len(builder.docIDToPos) != 0 || len(builder.posToDocID) != 0 || len(builder.pathData) != 0 {
		t.Fatalf(
			"builder committed state after lifecycle failure: docIDToPos=%d posToDocID=%d pathData=%d",
			len(builder.docIDToPos),
			len(builder.posToDocID),
			len(builder.pathData),
		)
	}
	if builder.Finalize() != nil {
		t.Fatal("Finalize() after lifecycle failure is non-nil")
	}
}
