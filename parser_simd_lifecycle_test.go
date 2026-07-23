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
	panicValue := &struct{ message string }{message: "walk panic sentinel"}
	closeCalls := 0
	var recovered any
	func() {
		defer func() {
			recovered = recover()
		}()
		_ = finishSIMDDocument(
			func() error { panic(panicValue) },
			func() error {
				closeCalls++
				return nil
			},
		)
	}()

	if recovered != panicValue {
		t.Fatalf("recovered panic = %v, want identical value %v", recovered, panicValue)
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
	retryErr   error
}

func (*simdDocumentLifecycleTestParser) Name() string { return "simd-lifecycle-test" }

func (p *simdDocumentLifecycleTestParser) Parse(_ []byte, rgID int, sink parserSink) error {
	p.parseCalls++
	if p.parseCalls > 1 && p.retryErr != nil {
		return p.retryErr
	}
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

func addSIMDDocumentRecovering(
	builder *GINBuilder,
	docID DocID,
	jsonDoc []byte,
) (err error, recovered any) {
	defer func() {
		recovered = recover()
	}()
	err = builder.AddDocument(docID, jsonDoc)
	return err, nil
}

func TestSIMDDocumentLifecyclePanicCloseFailurePoisonsBuilder(t *testing.T) {
	panicCause := errors.New("walk panic sentinel")
	closeCause := errors.New("close sentinel")
	busyCause := errors.New("simulated ErrParserBusy")
	parser := &simdDocumentLifecycleTestParser{
		walk: func(_ parserSink, _ *documentBuildState) error {
			panic(panicCause)
		},
		closeErr: closeCause,
		retryErr: busyCause,
	}
	config, err := NewConfig(WithParserFailureMode(IngestFailureSoft))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	builder, err := NewBuilder(config, 2, WithParser(parser))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	firstErr, recovered := addSIMDDocumentRecovering(
		builder,
		DocID(12),
		[]byte(`{"name":"first"}`),
	)
	storedErr := builder.Err()
	secondErr := builder.AddDocument(DocID(13), []byte(`{"name":"second"}`))

	if recovered != nil {
		t.Fatalf("first AddDocument recovered panic = %v, want nil", recovered)
	}
	if firstErr == nil {
		t.Fatal("first AddDocument error = nil, want fatal panic-plus-close failure")
	}
	if storedErr == nil {
		t.Fatal("builder.Err() = nil, want stored tragic error")
	}
	if firstErr != storedErr {
		t.Fatalf("first AddDocument error = %p, builder.Err() = %p, want same stored error", firstErr, storedErr)
	}
	var lifecycleErr *parserLifecycleError
	if !stderrors.As(firstErr, &lifecycleErr) {
		t.Fatalf("errors.As(%v, *parserLifecycleError) = false", firstErr)
	}
	if !stderrors.Is(firstErr, closeCause) {
		t.Fatalf("errors.Is(%v, closeCause) = false", firstErr)
	}
	if !stderrors.Is(firstErr, panicCause) {
		t.Fatalf("errors.Is(%v, panicCause) = false", firstErr)
	}
	if secondErr == nil {
		t.Fatal("second AddDocument error = nil, want prior tragic refusal")
	}
	if !stderrors.Is(secondErr, storedErr) {
		t.Fatalf("errors.Is(%v, storedErr) = false", secondErr)
	}
	if stderrors.Is(secondErr, busyCause) {
		t.Fatalf("errors.Is(%v, busyCause) = true, want no retry dispatch", secondErr)
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
}

func TestSIMDDocumentLifecycleNonErrorPanicCloseFailureReturnsLifecycleError(t *testing.T) {
	closeCause := errors.New("close sentinel")
	closeCalls := 0
	var got error
	var recovered any
	func() {
		defer func() {
			recovered = recover()
		}()
		got = finishSIMDDocument(
			func() error { panic("non-error panic") },
			func() error {
				closeCalls++
				return closeCause
			},
		)
	}()

	if recovered != nil {
		t.Fatalf("recovered panic = %v, want nil", recovered)
	}
	var lifecycleErr *parserLifecycleError
	if !stderrors.As(got, &lifecycleErr) {
		t.Fatalf("errors.As(%v, *parserLifecycleError) = false", got)
	}
	if !stderrors.Is(got, closeCause) {
		t.Fatalf("errors.Is(%v, closeCause) = false", got)
	}
	if !strings.Contains(got.Error(), "panic while walking SIMD document: non-error panic") {
		t.Fatalf("error = %q, want stable panic context", got.Error())
	}
	if closeCalls != 1 {
		t.Fatalf("close calls = %d, want 1", closeCalls)
	}
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
