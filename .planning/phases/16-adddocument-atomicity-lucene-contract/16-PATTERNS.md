# Phase 16: AddDocument Atomicity (Lucene contract) - Pattern Map

**Mapped:** 2026-04-23
**Files analyzed:** 6 new/modified files
**Analogs found:** 6 / 6

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `builder.go` | service | request-response, transform | `builder.go` | exact |
| `atomicity_test.go` | test | batch, property | `property_test.go`, `generators_test.go` | role+data-flow match |
| `gin_test.go` | test | request-response | `gin_test.go` | exact |
| `observability_policy_test.go` | test | event-driven, policy | `observability_policy_test.go`, `query_observability_test.go` | exact |
| `Makefile` | config | batch | `Makefile` | exact |
| `.github/workflows/ci.yml` | config | batch | `.github/workflows/ci.yml` | exact |

## Pattern Assignments

### `builder.go` (service, request-response/transform)

**Analog:** `builder.go`

**Imports pattern** (`builder.go` lines 3-14):

```go
import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/cespare/xxhash/v2"
	"github.com/pkg/errors"
)
```

Copy this import grouping. Add the logging package in the project-prefix group only if the implementation calls `logging.Error(...)` directly from `builder.go`.

**Builder tragic gate pattern** (`builder.go` lines 19-34):

```go
type GINBuilder struct {
	config     GINConfig
	numRGs     int
	numDocs    uint64
	maxRGID    int
	pathData   map[string]*pathBuildData
	bloom      *BloomFilter
	codec      DocIDCodec
	docIDToPos map[DocID]int
	posToDocID []DocID
	nextPos    int
	// poisonErr is non-nil once a merge step has failed partway through
	// mutating shared state. Subsequent AddDocument calls refuse to compound
	// corruption; Finalize remains callable so callers can discard the
	// builder gracefully.
	poisonErr error
```

Rename `poisonErr` to `tragicErr` in this same field location. Preserve the shape: one builder-owned terminal error, checked before document parsing and write bookkeeping.

**Staging shadow-field pattern** (`builder.go` lines 73-87):

```go
type stagedPathData struct {
	observedTypes uint8
	present       bool
	isNull        bool
	stringTerms   map[string]struct{}
	numericValues []stagedNumericValue

	numericSeeded       bool
	numericSimHasValue  bool
	numericSimValueType NumericValueType
	numericSimIntMin    int64
	numericSimIntMax    int64
	numericSimFloatMin  float64
	numericSimFloatMax  float64
}
```

Any validator simulator state added in this phase should copy this shadow-field style onto `stagedPathData`; do not introduce a separate preview clone type.

**AddDocument gate and parser handoff pattern** (`builder.go` lines 304-348):

```go
func (b *GINBuilder) AddDocument(docID DocID, jsonDoc []byte) error {
	if b.poisonErr != nil {
		return errors.Wrap(b.poisonErr, "builder poisoned by prior merge failure; discard and rebuild")
	}
	pos, exists := b.docIDToPos[docID]
	if !exists {
		pos = b.nextPos
		if pos >= b.numRGs {
			return errors.Errorf("position %d exceeds numRGs %d", pos, b.numRGs)
		}
	}

	// Return parser errors verbatim; do not wrap here. Reset the handoff
	// fields before dispatch so AddDocument can verify Parse called
	// BeginDocument exactly once with the expected row-group id.
	b.currentDocState = nil
	b.beginDocumentCalls = 0
	defer func() {
		b.currentDocState = nil
		b.beginDocumentCalls = 0
	}()

	if err := b.parser.Parse(jsonDoc, pos, b); err != nil {
		return err
	}

	if b.beginDocumentCalls == 0 {
		return errors.Errorf("parser %q did not call BeginDocument", b.parserName)
	}
	if b.beginDocumentCalls != 1 {
		return errors.Errorf(
			"parser %q called BeginDocument %d times; want exactly 1",
			b.parserName,
			b.beginDocumentCalls,
		)
	}
	if b.currentDocState.rgID != pos {
		return errors.Errorf(
			"parser %q BeginDocument rgID mismatch: got %d, want %d",
			b.parserName,
			b.currentDocState.rgID,
			pos,
		)
	}
	return b.mergeDocumentState(docID, pos, exists, b.currentDocState)
}
```

Keep parser errors verbatim and keep doc bookkeeping after merge success. The tragic gate wording changes, but the gate remains the first statement inside `AddDocument`.

**Validate-before-merge pattern** (`builder.go` lines 697-709 and 724-740):

```go
func (b *GINBuilder) mergeDocumentState(docID DocID, pos int, exists bool, state *documentBuildState) error {
	if err := b.validateStagedPaths(state); err != nil {
		return err
	}
	if err := b.mergeStagedPaths(state); err != nil {
		// mergeStagedPaths mutates pathData, bloom, and presentRGs path-by-path
		// in a sorted loop. A mid-loop failure leaves earlier paths merged for
		// this document while later ones are untouched, so the builder's state
		// no longer reflects any single consistent document set. Flag it so
		// subsequent AddDocument calls can't compound the corruption.
		b.poisonErr = err
		return err
	}
```

```go
func (b *GINBuilder) validateStagedPaths(state *documentBuildState) error {
	preview := newDocumentBuildState(state.rgID)
	paths := make([]string, 0, len(state.paths))
	for path := range state.paths {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		staged := state.paths[path]
		for _, observation := range staged.numericValues {
			if err := b.stageNumericObservation(path, observation, preview); err != nil {
				return err
			}
		}
	}
	return nil
}
```

Update `mergeDocumentState` so `mergeStagedPaths` is called through the recovery helper and no longer returns ordinary validation errors. Preserve the sorted path walk and validation-first ordering.

**Merge mutation pattern** (`builder.go` lines 743-778):

```go
func (b *GINBuilder) mergeStagedPaths(state *documentBuildState) error {
	paths := make([]string, 0, len(state.paths))
	for path := range state.paths {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		staged := state.paths[path]
		pd := b.getOrCreatePath(path)
		pd.observedTypes |= staged.observedTypes
		if staged.present {
			pd.presentRGs.Set(state.rgID)
		}
		if staged.isNull {
			pd.nullRGs.Set(state.rgID)
		}

		if len(staged.stringTerms) > 0 {
			terms := make([]string, 0, len(staged.stringTerms))
			for term := range staged.stringTerms {
				terms = append(terms, term)
			}
			sort.Strings(terms)
			for _, term := range terms {
				b.addStringTerm(pd, term, state.rgID, path)
			}
		}

		for _, observation := range staged.numericValues {
			if err := b.mergeNumericObservation(pd, observation, state.rgID, path); err != nil {
				return err
			}
		}
	}
	return nil
}
```

For Phase 16, place `// MUST_BE_CHECKED_BY_VALIDATOR` above this function, drop the `error` return, and remove the inner error branch after `mergeNumericObservation` also becomes infallible.

**Numeric simulator pattern** (`builder.go` lines 597-647 and 671-691):

```go
func (b *GINBuilder) stageNumericObservation(path string, observation stagedNumericValue, state *documentBuildState) error {
	pathState := state.getOrCreatePath(path)
	pathState.present = true
	b.seedNumericSimulation(path, pathState)

	if !pathState.numericSimHasValue {
		pathState.numericSimHasValue = true
		if observation.isInt {
			pathState.numericSimValueType = NumericValueTypeIntOnly
			pathState.numericSimIntMin = observation.intVal
			pathState.numericSimIntMax = observation.intVal
			pathState.observedTypes |= TypeInt
		} else {
			pathState.numericSimValueType = NumericValueTypeFloatMixed
			pathState.numericSimFloatMin = observation.floatVal
			pathState.numericSimFloatMax = observation.floatVal
			pathState.observedTypes |= TypeFloat
		}
		pathState.numericValues = append(pathState.numericValues, observation)
		return nil
	}

	if pathState.numericSimValueType == NumericValueTypeIntOnly {
		if observation.isInt {
			if observation.intVal < pathState.numericSimIntMin {
				pathState.numericSimIntMin = observation.intVal
			}
			if observation.intVal > pathState.numericSimIntMax {
				pathState.numericSimIntMax = observation.intVal
			}
			pathState.observedTypes |= TypeInt
			pathState.numericValues = append(pathState.numericValues, observation)
			return nil
		}

		if !canRepresentIntAsExactFloat(pathState.numericSimIntMin) || !canRepresentIntAsExactFloat(pathState.numericSimIntMax) {
			return errors.Errorf("unsupported mixed numeric promotion at %s", path)
		}
```

```go
func (b *GINBuilder) seedNumericSimulation(path string, pathState *stagedPathData) {
	if pathState.numericSeeded {
		return
	}
	pathState.numericSeeded = true

	pd, ok := b.pathData[path]
	if !ok || !pd.hasNumericValues {
		return
	}

	pathState.numericSimHasValue = true
	pathState.numericSimValueType = pd.numericValueType
	if pd.numericValueType == NumericValueTypeIntOnly {
		pathState.numericSimIntMin = pd.intGlobalMin
		pathState.numericSimIntMax = pd.intGlobalMax
		return
	}
	pathState.numericSimFloatMin = pd.floatGlobalMin
	pathState.numericSimFloatMax = pd.floatGlobalMax
}
```

The validator extension should continue to route through `stageNumericObservation`, because it seeds from real `b.pathData`. Do not bypass `seedNumericSimulation`.

**Merge numeric failure sites to hoist** (`builder.go` lines 799-880):

```go
func (b *GINBuilder) mergeNumericObservation(pd *pathBuildData, observation stagedNumericValue, rgID int, path string) error {
	if !pd.hasNumericValues {
		pd.hasNumericValues = true
		if observation.isInt {
			pd.numericValueType = NumericValueTypeIntOnly
			pd.intGlobalMin = observation.intVal
			pd.intGlobalMax = observation.intVal
			b.addIntNumericValue(pd, observation.intVal, rgID)
			b.bloom.AddString(path + "=" + strconv.FormatInt(observation.intVal, 10))
			return nil
		}
```

```go
	if pd.numericValueType == NumericValueTypeIntOnly && !observation.isInt {
		if err := b.promoteNumericPathToFloat(pd); err != nil {
			return errors.Wrapf(err, "promote numeric path %s", path)
		}
	}
```

```go
	if observation.isInt {
		if !canRepresentIntAsExactFloat(observation.intVal) {
			return errors.Errorf("unsupported mixed numeric promotion at %s", path)
		}
		floatVal = float64(observation.intVal)
	}
```

```go
func (b *GINBuilder) promoteNumericPathToFloat(pd *pathBuildData) error {
	if pd.numericValueType == NumericValueTypeFloatMixed {
		return nil
	}
	if !canRepresentIntAsExactFloat(pd.intGlobalMin) || !canRepresentIntAsExactFloat(pd.intGlobalMax) {
		return errors.New("unsupported mixed numeric promotion")
	}
	for _, stat := range pd.numericStats {
		if !stat.HasValue {
			continue
		}
		if !canRepresentIntAsExactFloat(stat.IntMin) || !canRepresentIntAsExactFloat(stat.IntMax) {
			return errors.New("unsupported mixed numeric promotion")
		}
	}
	pd.numericValueType = NumericValueTypeFloatMixed
```

These are the exact merge-layer error returns that must be pre-detected by `validateStagedPaths`. After hoisting, keep the mutation body but remove the error returns.

**Auth/guard pattern:** Not applicable. This package has no auth surface.

**Recovery helper gap:** No existing repo helper uses `recover()` for this path. Use the standard `defer recover()` helper shape from `16-RESEARCH.md`, then emit through the shared logger seam below.

---

### `atomicity_test.go` (test, batch/property)

**Analogs:** `property_test.go`, `generators_test.go`, `gin_test.go`

**Imports pattern** (`property_test.go` lines 3-12, plus `gin_test.go` lines 3-12 for `bytes`):

```go
import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)
```

```go
import (
	"bytes"
	"encoding/binary"
	stderrors "errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)
```

For `atomicity_test.go`, combine these styles: standard imports first, then gopter imports. Use `bytes.Equal` for encoded byte comparison.

**Property budget pattern** (`property_test.go` lines 14-40):

```go
const (
	propertyTestDefaultMinSuccessfulTests  = 1000
	propertyTestShortMinSuccessfulTests    = 100
	propertyTestHLLMinSuccessfulTests      = 250
	propertyTestHLLShortMinSuccessfulTests = 50
	propertyTestHLLEstimateSampleSize      = 256
)

func propertyTestMinSuccessfulTestsForMode(short bool, normal, shortBudget int) int {
	if !short {
		return normal
	}
	if shortBudget <= 0 || shortBudget > normal {
		return normal
	}
	return shortBudget
}

func propertyTestParametersWithBudgets(normal, shortBudget int) *gopter.TestParameters {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = propertyTestMinSuccessfulTestsForMode(testing.Short(), normal, shortBudget)
	return params
}

func propertyTestParameters() *gopter.TestParameters {
	return propertyTestParametersWithBudgets(propertyTestDefaultMinSuccessfulTests, propertyTestShortMinSuccessfulTests)
}
```

Use `propertyTestParameters()` so the atomicity property runs 1000 successful tests normally and 100 in short mode.

**Property shape pattern** (`property_test.go` lines 266-321):

```go
func TestPropertySerializationRoundTrip(t *testing.T) {
	properties := gopter.NewProperties(propertyTestParameters())

	properties.Property("encode/decode produces equivalent index", prop.ForAll(
		func(docs [][]byte) bool {
			validDocs := make([][]byte, 0)
			for _, doc := range docs {
				if len(doc) > 2 {
					validDocs = append(validDocs, doc)
				}
			}
			if len(validDocs) == 0 {
				return true
			}

			numRGs := len(validDocs)
			if numRGs > 100 {
				numRGs = 100
			}
			builder, _ := NewBuilder(DefaultConfig(), numRGs)
			for i, doc := range validDocs {
				if i >= numRGs {
					break
				}
				_ = builder.AddDocument(DocID(i), doc)
			}
			idx := builder.Finalize()

			encoded, err := Encode(idx)
			if err != nil {
				return true
			}

			decoded, err := Decode(encoded)
			if err != nil {
				return false
			}
```

Copy the `gopter.NewProperties` / `prop.ForAll` / `TestingRun(t)` structure, but return a diagnostic string or bool consistently. The research sketch uses string-returning properties; existing repo properties mostly return bool.

**Generator convention** (`generators_test.go` lines 87-109 and 280-305):

```go
func GenJSONDocument(maxDepth int) gopter.Gen {
	return genFlatJSONObject()
}

func genFlatJSONObject() gopter.Gen {
	return gopter.CombineGens(
		gen.AlphaString(),
		gen.AlphaString(),
		gen.Float64Range(-1e6, 1e6),
		gen.Bool(),
	).Map(func(vals []interface{}) []byte {
		m := map[string]any{
			"str1": vals[0].(string),
			"str2": vals[1].(string),
			"num":  vals[2].(float64),
			"flag": vals[3].(bool),
		}
		if m["str1"] == "" {
			m["str1"] = "default"
		}
		data, _ := json.Marshal(m)
		return data
	})
}
```

```go
return gopter.CombineGens(
	gen.IntRange(0, len(names)-1),
	gen.IntRange(18, 65),
	gen.Bool(),
	gen.IntRange(0, len(statuses)-1),
).Map(func(vals []interface{}) TestDoc {
	name := names[vals[0].(int)]
	age := vals[1].(int)
	active := vals[2].(bool)
	status := statuses[vals[3].(int)]

	data := map[string]any{
		"name":   name,
		"age":    float64(age),
		"active": active,
		"status": status,
	}
	jsonBytes, _ := json.Marshal(data)
	return TestDoc{JSON: jsonBytes, Data: data}
})
```

New failure-intent generators should be local to `atomicity_test.go` unless they become broadly reusable. Use `gopter.CombineGens(...).Map(...)` and constrained constants to guarantee the intended failure class.

**Encode determinism assertion source** (`property_test.go` lines 292-300):

```go
idx := builder.Finalize()

encoded, err := Encode(idx)
if err != nil {
	return true
}

decoded, err := Decode(encoded)
if err != nil {
	return false
}
```

Atomicity needs two independent build+encode calls and `bytes.Equal` between full-vs-clean corpora. Add a separate deterministic clean-corpus sanity test before the property.

---

### `gin_test.go` (test, request-response/failure catalog)

**Analog:** `gin_test.go`

**Imports/helper pattern** (`gin_test.go` lines 3-21):

```go
import (
	"bytes"
	"encoding/binary"
	stderrors "errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
)

func mustNewBuilder(t *testing.T, config GINConfig, numRGs int) *GINBuilder {
	t.Helper()
	builder, err := NewBuilder(config, numRGs)
	if err != nil {
		t.Fatalf("failed to create builder: %v", err)
	}
	return builder
}
```

Keep package `gin` for tests that need unexported fields such as `tragicErr`, `numDocs`, and internal helper functions.

**Existing poison/tragic gate migration target** (`gin_test.go` lines 425-447):

```go
func TestAddDocumentRefusesAfterMergeFailurePoisonsBuilder(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)
	if err := builder.AddDocument(0, []byte(`{"name":"alice"}`)); err != nil {
		t.Fatalf("AddDocument(seed) failed: %v", err)
	}

	// Simulate a mid-loop mergeStagedPaths failure by poisoning the builder
	// directly. The natural trigger path (mixed numeric promotion) is caught
	// by validateStagedPaths' preview before mergeStagedPaths runs, so poison
	// is defensive; we still need to prove the refusal contract.
	builder.poisonErr = stderrors.New("simulated merge failure")

	err := builder.AddDocument(1, []byte(`{"name":"bob"}`))
	if err == nil {
		t.Fatal("AddDocument after poison = nil, want wrapped poison error")
	}
	if !strings.Contains(err.Error(), "builder poisoned") {
		t.Fatalf("AddDocument error = %q, want 'builder poisoned' context", err.Error())
	}
	if !strings.Contains(err.Error(), "simulated merge failure") {
		t.Fatalf("AddDocument error = %q, want original cause preserved", err.Error())
	}
}
```

Rename this test and field to tragic wording. Preserve the direct field-level shape for the gate test.

**No partial mutation test pattern** (`gin_test.go` lines 2904-2957):

```go
func TestAddDocumentRejectsUnsupportedNumberWithoutPartialMutation(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 4)

	if err := builder.AddDocument(0, []byte(`{"name":"stable","score":10}`)); err != nil {
		t.Fatalf("seed AddDocument failed: %v", err)
	}

	err := builder.AddDocument(1, []byte(`{"name":"leak","nested":{"label":"should-not-stick"},"score":9223372036854775808}`))
	if err == nil {
		t.Fatal("expected unsupported numeric literal to fail")
	}
	if !strings.Contains(err.Error(), "$.score") {
		t.Fatalf("error should contain path context, got %v", err)
	}

	if builder.numDocs != 1 {
		t.Fatalf("numDocs = %d, want 1", builder.numDocs)
	}
	if _, exists := builder.docIDToPos[DocID(1)]; exists {
		t.Fatalf("docIDToPos contains rejected document: %+v", builder.docIDToPos)
	}
	if len(builder.posToDocID) != 1 {
		t.Fatalf("posToDocID len = %d, want 1", len(builder.posToDocID))
	}
	if builder.nextPos != 1 {
		t.Fatalf("nextPos = %d, want 1", builder.nextPos)
	}
	if _, exists := builder.pathData["$.nested.label"]; exists {
		t.Fatal("rejected document leaked nested path into builder state")
	}
```

Use this assertion style for public failure catalog cases: check returned error, `tragicErr == nil`, doc bookkeeping unchanged, and no staged paths leaked.

**Mixed numeric promotion test pattern** (`gin_test.go` lines 3142-3166):

```go
func TestMixedNumericPathRejectsLossyPromotion(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 2)

	if err := builder.AddDocument(0, []byte(`{"score":9007199254740993}`)); err != nil {
		t.Fatalf("seed AddDocument failed: %v", err)
	}

	err := builder.AddDocument(1, []byte(`{"score":1.5}`))
	if err == nil {
		t.Fatal("expected lossy mixed numeric promotion to fail")
	}
	if !strings.Contains(err.Error(), "$.score") {
		t.Fatalf("error should contain path context, got %v", err)
	}

	if builder.numDocs != 1 {
		t.Fatalf("numDocs = %d, want 1", builder.numDocs)
	}

	idx := builder.Finalize()
	result := idx.Evaluate([]Predicate{EQ("$.score", int64(9007199254740993))})
	if result.Count() != 1 || !result.IsSet(0) || result.IsSet(1) {
		t.Fatalf("exact int64 EQ after rejected promotion = %v, want [0]", result.ToSlice())
	}
}
```

Extend this coverage in Phase 16 to assert validator rejection before merge and `tragicErr == nil`.

---

### `observability_policy_test.go` (test, event-driven/policy)

**Analogs:** `observability_policy_test.go`, `query_observability_test.go`

**External package policy-test pattern** (`observability_policy_test.go` lines 1-33):

```go
package gin_test

// observability_policy_test.go - Phase 14 policy and regression gate tests.
//
// This file enforces the frozen observability vocabulary and guards against
// slipping back to legacy logger conventions or leaking backend-specific types.

import (
	"bufio"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gin "github.com/amikos-tech/ami-gin"
	"github.com/amikos-tech/ami-gin/logging"
	"github.com/amikos-tech/ami-gin/telemetry"
)
```

Use package `gin_test` for public observability policy tests. If the tragic recovery test needs unexported `runMergeWithRecover` or `tragicErr`, keep that test in package `gin` in `gin_test.go` or `atomicity_test.go` instead.

**INFO allowlist policy pattern** (`observability_policy_test.go` lines 42-68):

```go
func TestInfoLevelAttrAllowlist(t *testing.T) {
	// Canonical frozen allowlist - must match logging/attrs.go exactly.
	// To add a new INFO-level attr, update logging/attrs.go AND this list.
	const (
		keyOperation   = "operation"
		keyPredicateOp = "predicate_op"
		keyPathMode    = "path_mode"
		keyStatus      = "status"
		keyErrorType   = "error.type"
	)

	frozenAllowlist := []string{
		keyOperation,
		keyPredicateOp,
		keyPathMode,
		keyStatus,
		keyErrorType,
	}

	// Verify that the constructor helpers produce attrs with exactly these keys.
	attrsUnderTest := []logging.Attr{
		logging.AttrOperation("query.evaluate"),
		logging.AttrPredicateOp("EQ"),
		logging.AttrPathMode(logging.PathModeExact),
		logging.AttrStatus("ok"),
		logging.AttrErrorType("io"),
	}
```

Phase 16 tragic logging is Error-level, not INFO-level. Do not add raw document values to INFO attrs; avoid broad raw panic attributes unless a policy test explicitly bounds them.

**Emission capture pattern** (`observability_policy_test.go` lines 112-147 and 513-524):

```go
func TestInfoLevelEmissionsUseOnlyAllowlistedAttrs(t *testing.T) {
	frozenAllowlist := map[string]bool{
		"operation":    true,
		"predicate_op": true,
		"path_mode":    true,
		"status":       true,
		"error.type":   true,
	}

	var captured []logging.Attr
	capLogger := &policyCapLogger{attrs: &captured}

	idx, err := buildSmallIndex()
	if err != nil {
		t.Fatalf("build index: %v", err)
	}

	cfg, err := gin.NewConfig(gin.WithLogger(capLogger))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	idx.Config = &cfg

	// Run an evaluation to trigger INFO-level emission.
	_ = idx.EvaluateContext(context.Background(), []gin.Predicate{gin.EQ("$.name", "alice")})
```

```go
type policyCapLogger struct {
	attrs  *[]logging.Attr
	called bool
}

func (c *policyCapLogger) Enabled(_ logging.Level) bool { return true }
func (c *policyCapLogger) Log(_ logging.Level, _ string, attrs ...logging.Attr) {
	c.called = true
	*c.attrs = append(*c.attrs, attrs...)
}
```

Copy this capture logger shape when asserting the recovered tragic path emits exactly one Error-level log event.

**Logger seam usage pattern** (`query_observability_test.go` lines 81-110 and 117-137):

```go
var captured []logging.Attr
capLogger := &captureLogger{attrs: &captured}
cfg, err := gin.NewConfig(gin.WithLogger(capLogger))
if err != nil {
	t.Fatalf("NewConfig: %v", err)
}
idx.Config = &cfg

ctx, cancel := context.WithCancel(context.Background())
cancel()

got := idx.EvaluateContext(ctx, []gin.Predicate{gin.EQ("$.status", "active")})
if got.Count() != int(idx.Header.NumRowGroups) {
	t.Fatalf("EvaluateContext canceled count=%d; want fail-open count=%d", got.Count(), idx.Header.NumRowGroups)
}
if !capLogger.called {
	t.Fatal("expected canceled evaluation to emit a completion log entry")
}
if value, ok := attrValue(captured, "status"); !ok || value != "error" {
	t.Fatalf("status attr = %q, %v; want %q, true", value, ok, "error")
}
if value, ok := attrValue(captured, "error.type"); !ok || value != "other" {
	t.Fatalf("error.type attr = %q, %v; want %q, true", value, ok, "other")
}
```

```go
func TestAdaptiveInvariantViolationUsesLoggerSeam(t *testing.T) {
	var captured []logging.Attr
	capLogger := &captureLogger{attrs: &captured}

	idx := buildAdaptiveInvariantIndex(t, 3)
	cfg, err := gin.NewConfig(gin.WithLogger(capLogger))
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	idx.Config = &cfg

	got := idx.Evaluate([]gin.Predicate{gin.EQ("$.field", "hot")})
	// Must still fail open to all row groups.
	if got.Count() != 3 {
		t.Fatalf("invariant violation: count=%d; want 3 (fail-open)", got.Count())
	}
	// Logger seam must have received a message.
	if !capLogger.called {
		t.Fatal("expected the repo-owned logger to receive the invariant violation message")
	}
}
```

Use this pattern to prove the tragic recovery log goes through `WithLogger` and remains silent under `DefaultConfig`.

---

### `Makefile` (config, batch/static enforcement)

**Analog:** `Makefile`

**Test target pattern** (`Makefile` lines 11-20):

```make
.PHONY: test
test: gotestsum-bin
	gotestsum \
		--format short-verbose \
		--packages="./..." \
		--junitfile unit.xml \
		-- \
		-v \
		-coverprofile=coverage.out \
		-timeout=30m
```

Keep multi-line shell commands tab-indented. Do not change `test` unless needed by the phase.

**Lint target pattern** (`Makefile` lines 25-31):

```make
.PHONY: lint
lint:
	golangci-lint run

.PHONY: lint-fix
lint-fix:
	golangci-lint run --fix ./...
```

Add the marker/signature grep under `lint` after or before `golangci-lint run`. Keep it a normal shell command so `make lint` fails non-zero if any marked function returns `error`.

**Help target pattern** (`Makefile` lines 41-51):

```make
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build     - Build all packages"
	@echo "  test      - Run tests with coverage"
	@echo "  integration-test - Run integration test suite"
	@echo "  lint      - Run golangci-lint"
	@echo "  lint-fix  - Run golangci-lint with auto-fix"
	@echo "  security-scan - Run govulncheck against all packages"
	@echo "  clean     - Remove generated files"
	@echo "  help      - Show this help"
```

If the lint description changes, update this echo line in the same commit.

---

### `.github/workflows/ci.yml` (config, batch/CI)

**Analog:** `.github/workflows/ci.yml`

**Test matrix pattern** (`.github/workflows/ci.yml` lines 15-36):

```yaml
  test:
    name: test (${{ matrix.go-version }})
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        go-version: ["1.25", "1.26"]
    steps:
      - name: Checkout repository
        uses: actions/checkout@v6

      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version: ${{ matrix.go-version }}
          cache-dependency-path: go.sum

      - name: Install gotestsum
        run: go install gotest.tools/gotestsum@v1.13.0

      - name: Run tests
        run: gotestsum --format short-verbose --packages="./..." --junitfile unit.xml -- -short -race -coverprofile=coverage.out -timeout=30m ./...
```

Do not put the marker grep only in the test matrix. The phase requires lint/static enforcement too.

**Lint job pattern** (`.github/workflows/ci.yml` lines 56-72):

```yaml
  lint:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout repository
        uses: actions/checkout@v6

      - name: Set up Go
        uses: actions/setup-go@v6
        with:
          go-version: "1.26"
          cache-dependency-path: go.sum

      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v9
        with:
          version: v2.11.4
          args: --config=.golangci.yml
```

Add a separate `run:` step in this `lint` job for the marker check, or change this job to call `make lint` in addition to the existing action. A Makefile-only check will not satisfy the CI requirement because this job currently bypasses `make lint`.

## Shared Patterns

### Error Handling

**Source:** `builder.go`, `parser_test.go`
**Apply to:** `builder.go`, `gin_test.go`, `atomicity_test.go`

Use `github.com/pkg/errors` in implementation code and `strings.Contains` in tests where exact wrapped text is intentionally not locked.

```go
return errors.Errorf("unsupported JSON token type %T at %s", token, canonicalPath)
return errors.Wrapf(err, "parse numeric at %s", path)
return errors.Wrap(err, "unsupported integer literal")
```

Sources: `builder.go` lines 436, 544, 567.

### Parser Failure Catalog Tests

**Source:** `parser_test.go`
**Apply to:** `atomicity_test.go`, `gin_test.go`

```go
func TestAddDocumentReturnsParserErrorVerbatim(t *testing.T) {
	sentinel := errors.New("sentinel parse error")
	b, err := NewBuilder(DefaultConfig(), 4, WithParser(failingParser{err: sentinel}))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	got := b.AddDocument(DocID(0), []byte(`{"a": 1}`))
	if got == nil {
		t.Fatal("expected error from AddDocument, got nil")
	}
	if got.Error() != "sentinel parse error" {
		t.Fatalf("AddDocument err = %q, want %q", got.Error(), "sentinel parse error")
	}
}
```

Source: `parser_test.go` lines 282-295.

```go
cases := []struct {
	name       string
	jsonDoc    string
	wantSubstr string
}{
	{name: "garbage", jsonDoc: "garbage", wantSubstr: "read JSON token"},
	{name: "bad-object-value", jsonDoc: `{"a":}`, wantSubstr: "parse object value at $.a"},
	{name: "bad-object-key", jsonDoc: `{1:true}`, wantSubstr: "read object key at $"},
	{name: "unterminated-object", jsonDoc: `{"a":1`, wantSubstr: "close object at $"},
	{name: "unterminated-array", jsonDoc: `[1,`, wantSubstr: "parse array element at $[1]"},
	{name: "trailing-json", jsonDoc: `{"a":1} []`, wantSubstr: "unexpected trailing JSON content"},
}
```

Source: `parser_test.go` lines 297-309.

### Parser Contract Misuse Tests

**Source:** `parser_test.go`
**Apply to:** public failure catalog in `atomicity_test.go`

```go
type skipBeginDocumentParser struct{}

func (skipBeginDocumentParser) Name() string { return "skip-begin" }

func (skipBeginDocumentParser) Parse([]byte, int, parserSink) error {
	return nil
}
```

```go
type doubleBeginDocumentParser struct{}

func (doubleBeginDocumentParser) Name() string { return "double-begin" }

func (doubleBeginDocumentParser) Parse(_ []byte, rgID int, sink parserSink) error {
	_ = sink.BeginDocument(rgID)
	_ = sink.BeginDocument(rgID)
	return nil
}
```

Sources: `parser_test.go` lines 329-359.

### Transformer Failure Tests

**Source:** `transformers_test.go`
**Apply to:** `atomicity_test.go`, `gin_test.go`

```go
func TestBuilderFailsWhenCompanionTransformFails(t *testing.T) {
	config, err := NewConfig(
		WithCustomTransformer("$.email", "strict", func(value any) (any, bool) {
			s, ok := value.(string)
			if !ok || !strings.Contains(s, "@") {
				return nil, false
			}
			return s, true
		}),
	)
	if err != nil {
		t.Fatalf("NewConfig failed: %v", err)
	}

	builder, err := NewBuilder(config, 2)
	if err != nil {
		t.Fatalf("NewBuilder failed: %v", err)
	}

	err = builder.AddDocument(0, []byte(`{"email":42}`))
	if err == nil {
		t.Fatal("AddDocument(0) error = nil, want strict companion failure")
	}
	if !strings.Contains(err.Error(), "$.email") || !strings.Contains(err.Error(), "strict") {
		t.Fatalf("AddDocument(0) error = %v, want source path and alias context", err)
	}

	if err := builder.AddDocument(1, []byte(`{"email":"bob@example.com"}`)); err != nil {
		t.Fatalf("AddDocument(1) failed after rejected document: %v", err)
	}
```

Source: `transformers_test.go` lines 481-510.

### Logger Seam

**Source:** `gin.go`, `logging/logger.go`, `logging/noop.go`, `logging/attrs.go`
**Apply to:** `builder.go`, `observability_policy_test.go`

```go
// Runtime-only observability fields. These are never serialized into the
// on-wire config payload (see SerializedConfig and writeConfig/readConfig).
Logger  logging.Logger    // noop by default; set via WithLogger
Signals telemetry.Signals // disabled by default; set via WithSignals
```

Source: `gin.go` lines 368-371.

```go
func WithLogger(logger logging.Logger) ConfigOption {
	return func(c *GINConfig) error {
		if logger == nil {
			return errors.New("logger cannot be nil")
		}
		c.Logger = logger
		return nil
	}
}
```

Source: `gin.go` lines 685-694.

```go
// Error emits an error-level message.
func Error(logger Logger, msg string, attrs ...Attr) {
	Default(logger).Log(LevelError, msg, attrs...)
}
```

Source: `logging/logger.go` lines 42-45.

```go
func (noopLogger) Enabled(Level) bool         { return false }
func (noopLogger) Log(Level, string, ...Attr) {}
```

Source: `logging/noop.go` lines 23-24.

### Static Policy / CI Pattern

**Source:** `Makefile`, `.github/workflows/ci.yml`
**Apply to:** marker grep enforcement

The local check belongs under `make lint`; CI currently runs the golangci action directly, so the same marker check must be added to `.github/workflows/ci.yml`.

```make
.PHONY: lint
lint:
	golangci-lint run
```

Source: `Makefile` lines 25-27.

```yaml
      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v9
        with:
          version: v2.11.4
          args: --config=.golangci.yml
```

Source: `.github/workflows/ci.yml` lines 68-72.

## No Analog Found

All scoped files have usable analogs. One internal helper pattern has no direct codebase analog:

| File / Helper | Role | Data Flow | Reason |
|---------------|------|-----------|--------|
| `runMergeWithRecover(func())` inside `builder.go` | utility | event-driven panic recovery | No existing repo helper recovers panics and converts them to builder terminal errors. Use the defer/recover sketch from `16-RESEARCH.md` and the logger seam excerpts above. |

## Metadata

**Analog search scope:** module root Go files, root `_test.go` files, `logging/`, `Makefile`, `.github/workflows/ci.yml`, `.golangci.yml`
**Files scanned:** 92
**Pattern extraction date:** 2026-04-23
**Project guidance applied:** `AGENTS.md`, `CLAUDE.md`; no project-local `.claude/skills` or `.agents/skills` directories found.
