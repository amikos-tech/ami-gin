package gin

import (
	stderrors "errors"
	"strings"
	"testing"

	"github.com/pkg/errors"
)

func TestParserLifecycleErrorCleanupOnly(t *testing.T) {
	cleanupCause := errors.New("cleanup sentinel")
	lifecycleErr := newParserLifecycleError(
		errors.Wrap(cleanupCause, "close parser document"),
		nil,
	)
	outer := errors.Wrap(lifecycleErr, "outer context")

	var extracted *parserLifecycleError
	if !stderrors.As(outer, &extracted) {
		t.Fatal("errors.As failed to extract *parserLifecycleError")
	}
	if !stderrors.Is(outer, cleanupCause) {
		t.Fatalf("errors.Is(%v, cleanupCause) = false", outer)
	}
	if !strings.Contains(outer.Error(), "close parser document") {
		t.Fatalf("error = %q, want cleanup context", outer.Error())
	}
}

func TestParserLifecycleErrorPreservesConcurrentStageError(t *testing.T) {
	cleanupCause := errors.New("cleanup sentinel")
	stageCause := errors.New("stage sentinel")
	lifecycleErr := newParserLifecycleError(
		errors.Wrap(cleanupCause, "close parser document"),
		tagStageError(stageCause),
	)
	outer := errors.Wrap(lifecycleErr, "outer context")

	var extractedLifecycle *parserLifecycleError
	if !stderrors.As(outer, &extractedLifecycle) {
		t.Fatal("errors.As failed to extract *parserLifecycleError")
	}
	var extractedStage *stageCallbackError
	if !stderrors.As(outer, &extractedStage) {
		t.Fatal("errors.As failed to extract *stageCallbackError")
	}
	if !stderrors.Is(outer, cleanupCause) {
		t.Fatalf("errors.Is(%v, cleanupCause) = false", outer)
	}
	if !stderrors.Is(outer, stageCause) {
		t.Fatalf("errors.Is(%v, stageCause) = false", outer)
	}
	if !strings.Contains(outer.Error(), "close parser document") ||
		!strings.Contains(outer.Error(), "stage sentinel") {
		t.Fatalf("error = %q, want cleanup and stage context", outer.Error())
	}
}

func TestParserLifecycleErrorPreservesSoftSkipCause(t *testing.T) {
	cleanupCause := errors.New("cleanup sentinel")
	lifecycleErr := newParserLifecycleError(
		errors.Wrap(cleanupCause, "close parser document"),
		newSoftSkipNumericDocumentError("$.score"),
	)
	outer := errors.Wrap(lifecycleErr, "outer context")

	var extractedLifecycle *parserLifecycleError
	if !stderrors.As(outer, &extractedLifecycle) {
		t.Fatal("errors.As failed to extract *parserLifecycleError")
	}
	var extractedSkip *softSkipDocumentError
	if !stderrors.As(outer, &extractedSkip) {
		t.Fatal("errors.As failed to extract *softSkipDocumentError")
	}
	if !stderrors.Is(outer, cleanupCause) {
		t.Fatalf("errors.Is(%v, cleanupCause) = false", outer)
	}
	if !stderrors.Is(outer, errSkipDocument) {
		t.Fatalf("errors.Is(%v, errSkipDocument) = false", outer)
	}
}

func TestParserLifecycleErrorMissingCleanupCauseRemainsTerminal(t *testing.T) {
	concurrentCause := errors.New("concurrent sentinel")
	err := newParserLifecycleError(nil, concurrentCause)

	var lifecycleErr *parserLifecycleError
	if !stderrors.As(err, &lifecycleErr) {
		t.Fatal("errors.As failed to extract *parserLifecycleError")
	}
	if !stderrors.Is(err, concurrentCause) {
		t.Fatalf("errors.Is(%v, concurrentCause) = false", err)
	}
	if !strings.Contains(err.Error(), "without cleanup error") {
		t.Fatalf("error = %q, want missing-cleanup context", err.Error())
	}
}

type parserLifecycleFailureParser struct {
	calls int
	err   error
}

func (*parserLifecycleFailureParser) Name() string { return "lifecycle-failure" }

func (p *parserLifecycleFailureParser) Parse(_ []byte, rgID int, sink parserSink) error {
	p.calls++
	state := sink.BeginDocument(rgID)
	if err := sink.StageScalar(state, "$.name", "not-committed"); err != nil {
		return err
	}
	return p.err
}

type parserPanicTestParser struct {
	calls      int
	panicValue any
}

func (*parserPanicTestParser) Name() string { return "panic-test" }

func (p *parserPanicTestParser) Parse(_ []byte, rgID int, sink parserSink) error {
	p.calls++
	state := sink.BeginDocument(rgID)
	if err := sink.StageScalar(state, "$.name", "not-committed"); err != nil {
		return err
	}
	panic(p.panicValue)
}

func TestParserPanicPoisonsBuilderAfterCallerRecovery(t *testing.T) {
	panicCause := errors.New("parser panic sentinel")
	parser := &parserPanicTestParser{panicValue: panicCause}
	builder, err := NewBuilder(DefaultConfig(), 2, WithParser(parser))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	var recovered any
	func() {
		defer func() {
			recovered = recover()
		}()
		_ = builder.AddDocument(DocID(1), []byte(`{"name":"first"}`))
	}()

	recoveredErr, ok := recovered.(error)
	if !ok || !stderrors.Is(recoveredErr, panicCause) {
		t.Fatalf("recovered panic = %v, want error wrapping %v", recovered, panicCause)
	}
	if builder.Err() == nil {
		t.Fatal("builder.Err() = nil, want tragic parser panic")
	}
	if !stderrors.Is(builder.Err(), panicCause) {
		t.Fatalf("errors.Is(%v, panicCause) = false", builder.Err())
	}
	if !strings.Contains(builder.Err().Error(), "builder tragic: parser panic") {
		t.Fatalf("builder.Err() = %q, want parser panic context", builder.Err())
	}
	if builder.numDocs != 0 || builder.nextPos != 0 || len(builder.pathData) != 0 {
		t.Fatalf(
			"builder committed state after parser panic: numDocs=%d nextPos=%d pathData=%d",
			builder.numDocs,
			builder.nextPos,
			len(builder.pathData),
		)
	}

	storedErr := builder.Err()
	secondErr := builder.AddDocument(DocID(2), []byte(`{"name":"second"}`))
	if !stderrors.Is(secondErr, storedErr) {
		t.Fatalf("second AddDocument error = %v, want stored tragic error %v", secondErr, storedErr)
	}
	if parser.calls != 1 {
		t.Fatalf("parser calls = %d, want 1", parser.calls)
	}
}

func TestParserLifecycleFailurePoisonsBuilderOnce(t *testing.T) {
	for _, mode := range []IngestFailureMode{IngestFailureHard, IngestFailureSoft} {
		t.Run(string(mode), func(t *testing.T) {
			testParserLifecycleFailurePoisonsBuilderOnce(t, mode)
		})
	}
}

func testParserLifecycleFailurePoisonsBuilderOnce(t *testing.T, mode IngestFailureMode) {
	t.Helper()
	cleanupCause := errors.New("cleanup sentinel")
	parser := &parserLifecycleFailureParser{
		err: newParserLifecycleError(
			errors.Wrap(cleanupCause, "close parser document"),
			nil,
		),
	}
	config, err := NewConfig(WithParserFailureMode(mode))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	builder, err := NewBuilder(config, 2, WithParser(parser))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	firstErr := builder.AddDocument(DocID(7), []byte(`{"name":"first"}`))
	if firstErr == nil {
		t.Fatal("first AddDocument error = nil, want fatal lifecycle error")
	}
	if parser.calls != 1 {
		t.Fatalf("parser calls after first AddDocument = %d, want 1", parser.calls)
	}
	if builder.Err() == nil {
		t.Fatal("builder.Err() = nil, want fatal lifecycle error")
	}
	if !stderrors.Is(firstErr, builder.Err()) {
		t.Fatalf("first AddDocument error = %v, want builder.Err() = %v", firstErr, builder.Err())
	}
	if !stderrors.Is(firstErr, cleanupCause) {
		t.Fatalf("errors.Is(%v, cleanupCause) = false", firstErr)
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

	storedErr := builder.Err()
	secondErr := builder.AddDocument(DocID(8), []byte(`{"name":"second"}`))
	if secondErr == nil {
		t.Fatal("second AddDocument error = nil, want prior tragic refusal")
	}
	if parser.calls != 1 {
		t.Fatalf("parser calls after second AddDocument = %d, want 1", parser.calls)
	}
	if !stderrors.Is(builder.Err(), storedErr) {
		t.Fatalf("builder.Err() = %v, want stored error %v", builder.Err(), storedErr)
	}
	if !stderrors.Is(secondErr, storedErr) {
		t.Fatalf("errors.Is(%v, storedErr) = false", secondErr)
	}
	if !strings.Contains(secondErr.Error(), "builder closed by prior tragic failure; discard and rebuild") {
		t.Fatalf("second AddDocument error = %q, want prior tragic refusal context", secondErr.Error())
	}
}
