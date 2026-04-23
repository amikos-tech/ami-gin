# Phase 17: Failure-Mode Taxonomy Unification - Pattern Map

**Mapped:** 2026-04-23
**Files analyzed:** 12
**Analogs found:** 11 / 12

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `gin.go` | config / model | transform, request-response validation | `gin.go` | exact |
| `builder.go` | service | streaming, transform, request-response | `builder.go` | exact |
| `transformer_registry.go` | model / registry | transform, serialization metadata | `transformer_registry.go` | exact |
| `serialize.go` | utility / service | file-I/O, transform | `serialize.go` | exact |
| `failure_modes_test.go` | test | request-response, transform | `atomicity_test.go` + layer tests | role-match |
| `parser_test.go` | test | streaming, request-response | `parser_test.go` | exact |
| `transformers_test.go` | test | transform, request-response | `transformers_test.go` | exact |
| `atomicity_test.go` | test | batch, transform | `atomicity_test.go` | exact |
| `serialize_security_test.go` | test | file-I/O, transform | `serialize_security_test.go` | exact |
| `gin_test.go` | test | transform, validation | `gin_test.go` | exact |
| `examples/failure-modes/main.go` | example / CLI | batch, request-response | `examples/basic/main.go` + `examples/transformers/main.go` | role-match |
| `CHANGELOG.md` | documentation | documentation | none | no-source-analog |

## Pattern Assignments

### `gin.go` (config / model, transform validation)

**Analog:** `gin.go`

**Imports pattern** (lines 3-13):
```go
import (
	"encoding/json"
	"math"
	"sort"
	"strings"

	"github.com/pkg/errors"

	"github.com/amikos-tech/ami-gin/logging"
	"github.com/amikos-tech/ami-gin/telemetry"
)
```

**Public enum and option pattern to replace** (lines 276-330):
```go
type FieldTransformer func(value any) (any, bool)

type TransformerFailureMode string

const (
	TransformerFailureStrict TransformerFailureMode = "strict"
	TransformerFailureSoft   TransformerFailureMode = "soft_fail"
)

func normalizeTransformerFailureMode(mode TransformerFailureMode) TransformerFailureMode {
	if mode == "" {
		return TransformerFailureStrict
	}
	return mode
}

func validateTransformerFailureMode(mode TransformerFailureMode) error {
	switch normalizeTransformerFailureMode(mode) {
	case TransformerFailureStrict, TransformerFailureSoft:
		return nil
	default:
		return errors.Errorf("invalid transformer failure mode %q", mode)
	}
}

func WithTransformerFailureMode(mode TransformerFailureMode) TransformerOption {
	return func(options *transformerRegistrationOptions) error {
		if err := validateTransformerFailureMode(mode); err != nil {
			return err
		}
		options.failureMode = normalizeTransformerFailureMode(mode)
		return nil
	}
}
```

**Config field pattern** (lines 352-372):
```go
type GINConfig struct {
	CardinalityThreshold       uint32
	BloomFilterSize            uint32
	BloomFilterHashes          uint8
	EnableTrigrams             bool
	TrigramMinLength           int
	HLLPrecision               uint8
	PrefixBlockSize            int
	AdaptiveMinRGCoverage      int
	AdaptivePromotedTermCap    int
	AdaptiveCoverageCeiling    float64
	AdaptiveBucketCount        int
	ftsPaths                   []string
	representationSpecs        map[string][]RepresentationSpec
	representationTransformers map[string][]registeredRepresentation

	Logger  logging.Logger
	Signals telemetry.Signals
}
```

**Registration validation and normalization pattern** (lines 421-449):
```go
func (c *GINConfig) addRepresentation(canonicalPath, alias string, transformerSpec TransformerSpec, serializable bool, failureMode TransformerFailureMode, fn FieldTransformer) error {
	if err := validateRepresentationAlias(alias); err != nil {
		return errors.Wrapf(err, "transformer alias invalid for %s", canonicalPath)
	}
	if fn == nil {
		return errors.Errorf("transformer alias %q for %s requires a function", alias, canonicalPath)
	}
	if err := validateTransformerFailureMode(failureMode); err != nil {
		return errors.Wrapf(err, "transformer alias %q for %s", alias, canonicalPath)
	}
	// ...
	failureMode = normalizeTransformerFailureMode(failureMode)
	transformerSpec.FailureMode = failureMode
```

**Functional option and constructor pattern** (lines 472-512, 642-652):
```go
func WithCustomTransformer(path, alias string, fn FieldTransformer, opts ...TransformerOption) ConfigOption {
	return func(c *GINConfig) error {
		options, err := resolveTransformerOptions(opts...)
		if err != nil {
			return err
		}
		canonicalPath, err := canonicalizeSupportedPath(path)
		if err != nil {
			return err
		}
		spec := NewTransformerSpec(canonicalPath, TransformerUnknown, nil)
		return c.addRepresentation(canonicalPath, alias, spec, false, options.failureMode, fn)
	}
}

func NewConfig(opts ...ConfigOption) (GINConfig, error) {
	cfg := DefaultConfig()
	for _, opt := range opts {
		if err := opt(&cfg); err != nil {
			return GINConfig{}, err
		}
	}
	if err := cfg.validate(); err != nil {
		return GINConfig{}, err
	}
	return cfg, nil
}
```

**Defaults and validation pattern** (lines 655-670, 750-817):
```go
func DefaultConfig() GINConfig {
	cfg := GINConfig{
		CardinalityThreshold:    10000,
		BloomFilterSize:         65536,
		BloomFilterHashes:       5,
		EnableTrigrams:          true,
		TrigramMinLength:        3,
		HLLPrecision:            12,
		PrefixBlockSize:         defaultPrefixBlockSize,
		AdaptiveMinRGCoverage:   2,
		AdaptivePromotedTermCap: 64,
		AdaptiveCoverageCeiling: 0.80,
		AdaptiveBucketCount:     128,
	}
	normalizeObservability(&cfg)
	return cfg
}

func (c GINConfig) validate() error {
	// ...
	for canonicalPath, specs := range c.representationSpecs {
		seenAliases := make(map[string]struct{}, len(specs))
		for _, spec := range specs {
			if spec.SourcePath != canonicalPath {
				return errors.Errorf("transformer source path %q stored under %q", spec.SourcePath, canonicalPath)
			}
			if err := validateRepresentationAlias(spec.Alias); err != nil {
				return errors.Wrapf(err, "transformer alias invalid for %s", canonicalPath)
			}
			if err := validateTransformerFailureMode(spec.Transformer.FailureMode); err != nil {
				return errors.Wrapf(err, "transformer failure mode invalid for %s alias %q", canonicalPath, spec.Alias)
			}
		}
	}
	return nil
}
```

**Apply to Phase 17:**
- Rename the public type and constants here: `IngestFailureMode`, `IngestFailureHard`, `IngestFailureSoft`.
- Keep `WithTransformerFailureMode`, but change its parameter type to `IngestFailureMode`.
- Add `WithParserFailureMode` and `WithNumericFailureMode` as `ConfigOption` functions using the same fail-fast option pattern.
- Add config fields beside other build-time config fields. Do not serialize parser/numeric modes.
- Normalize empty mode to hard in default/config validation and use sites so struct literals remain valid.

---

### `builder.go` (service, streaming / transform)

**Analog:** `builder.go`

**Imports and tragic state pattern** (lines 3-16, 23-40):
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

	"github.com/amikos-tech/ami-gin/logging"
)

type GINBuilder struct {
	// ...
	tragicErr error
}
```

**Builder constructor validation pattern** (lines 123-168):
```go
type BuilderOption func(*GINBuilder) error

func NewBuilder(config GINConfig, numRGs int, opts ...BuilderOption) (*GINBuilder, error) {
	if numRGs <= 0 {
		return nil, errors.New("numRGs must be greater than 0")
	}
	if err := config.validate(); err != nil {
		return nil, err
	}
	// ...
	for _, opt := range opts {
		if err := opt(b); err != nil {
			return nil, err
		}
	}
	if b.parser == nil {
		b.parser = stdlibParser{}
	}
	name := b.parser.Name()
	if name == "" {
		return nil, errors.New("parser name cannot be empty")
	}
	b.parserName = name
	return b, nil
}
```

**AddDocument parser and contract boundary** (lines 315-359):
```go
func (b *GINBuilder) AddDocument(docID DocID, jsonDoc []byte) error {
	if b.tragicErr != nil {
		return errors.Wrap(b.tragicErr, "builder closed by prior tragic failure; discard and rebuild")
	}
	pos, exists := b.docIDToPos[docID]
	if !exists {
		pos = b.nextPos
		if pos >= b.numRGs {
			return errors.Errorf("position %d exceeds numRGs %d", pos, b.numRGs)
		}
	}

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
		return errors.Errorf("parser %q called BeginDocument %d times; want exactly 1", b.parserName, b.beginDocumentCalls)
	}
	if b.currentDocState.rgID != pos {
		return errors.Errorf("parser %q BeginDocument rgID mismatch: got %d, want %d", b.parserName, b.currentDocState.rgID, pos)
	}
	return b.mergeDocumentState(docID, pos, exists, b.currentDocState)
}
```

**Transformer failure routing pattern to change** (lines 533-547):
```go
func (b *GINBuilder) stageCompanionRepresentations(canonicalPath string, value any, state *documentBuildState) error {
	registrations := b.config.representations(canonicalPath)
	if len(registrations) == 0 {
		return nil
	}

	prepared := prepareTransformerValue(value)
	for _, registration := range registrations {
		transformed, ok := registration.FieldTransformer(prepared)
		if !ok {
			if normalizeTransformerFailureMode(registration.Transformer.FailureMode) == TransformerFailureSoft {
				continue
			}
			return errors.Errorf("companion transformer %q on %s failed to produce a value", registration.Alias, canonicalPath)
		}
```

**Numeric staging pattern** (lines 556-592, 612-684):
```go
func (b *GINBuilder) stageJSONNumberLiteral(path, raw string, state *documentBuildState) error {
	isInt, intVal, floatVal, err := parseJSONNumberLiteral(raw)
	if err != nil {
		return errors.Wrapf(err, "parse numeric at %s", path)
	}
	return b.stageNumericObservation(path, stagedNumericValue{
		isInt:    isInt,
		intVal:   intVal,
		floatVal: floatVal,
	}, state)
}

func (b *GINBuilder) stageNativeNumeric(path string, value any, state *documentBuildState) error {
	obs, err := stagedNumericFromValue(value)
	if err != nil {
		return errors.Wrapf(err, "parse numeric at %s", path)
	}
	return b.stageNumericObservation(path, obs, state)
}

func (b *GINBuilder) stageNumericObservation(path string, observation stagedNumericValue, state *documentBuildState) error {
	pathState := state.getOrCreatePath(path)
	pathState.present = true
	b.seedNumericSimulation(path, pathState)
	// ...
	if !canRepresentIntAsExactFloat(pathState.numericSimIntMin) || !canRepresentIntAsExactFloat(pathState.numericSimIntMax) {
		return errors.Errorf("unsupported mixed numeric promotion at %s", path)
	}
```

**Commit boundary and tragic recovery pattern** (lines 712-792):
```go
func (b *GINBuilder) mergeDocumentState(docID DocID, pos int, exists bool, state *documentBuildState) error {
	if err := b.commitStagedPaths(state); err != nil {
		return err
	}

	if !exists {
		b.docIDToPos[docID] = pos
		b.posToDocID = append(b.posToDocID, docID)
		b.nextPos++
	}

	if pos > b.maxRGID {
		b.maxRGID = pos
	}
	b.numDocs++
	return nil
}

func (b *GINBuilder) commitStagedPaths(state *documentBuildState) error {
	if err := b.validateStagedPaths(state); err != nil {
		return err
	}
	if err := runMergeWithRecover(b.config.Logger, func() { b.mergeStagedPaths(state) }); err != nil {
		b.tragicErr = err
		return err
	}
	return nil
}

func (b *GINBuilder) validateStagedPaths(state *documentBuildState) error {
	preview := newDocumentBuildState(state.rgID)
	// replay numeric observations into preview
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

**Apply to Phase 17:**
- Parser soft mode belongs only in the direct `b.parser.Parse(...)` error branch.
- Parser contract checks after `Parse` remain hard.
- Transformer soft mode must no longer `continue`; it must signal whole-document skip.
- Numeric soft mode must cover staging errors and `validateStagedPaths` errors before merge.
- Do not route `runMergeWithRecover` through soft mode. It remains tragic and closes the builder.
- Soft skip must return before `mergeDocumentState` mutates `docIDToPos`, `posToDocID`, `nextPos`, `maxRGID`, or `numDocs`.

---

### `transformer_registry.go` (model / registry, transform metadata)

**Analog:** `transformer_registry.go`

**Imports pattern** (lines 3-9):
```go
import (
	"encoding/json"
	"regexp"
	"time"

	"github.com/pkg/errors"
)
```

**Metadata struct pattern** (lines 47-55):
```go
type TransformerSpec struct {
	Path        string                 `json:"path"`
	Alias       string                 `json:"alias,omitempty"`
	TargetPath  string                 `json:"target_path,omitempty"`
	FailureMode TransformerFailureMode `json:"failure_mode,omitempty"`
	ID          TransformerID          `json:"id"`
	Name        string                 `json:"name"`
	Params      json.RawMessage        `json:"params,omitempty"`
}
```

**Spec constructor pattern** (lines 227-234):
```go
func NewTransformerSpec(path string, id TransformerID, params json.RawMessage) TransformerSpec {
	return TransformerSpec{
		Path:   path,
		ID:     id,
		Name:   transformerNames[id],
		Params: params,
	}
}
```

**Apply to Phase 17:**
- Change `FailureMode` to `IngestFailureMode`.
- Keep the JSON field name `failure_mode` for representation metadata.
- Preserve legacy decode support for serialized tokens even though public symbols are renamed.

---

### `serialize.go` (utility / service, file-I/O)

**Analog:** `serialize.go`

**Imports and error style** (lines 3-18, 76-88):
```go
import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	stderrors "errors"
	"io"
	"math"
	"sort"

	"github.com/RoaringBitmap/roaring/v2"
	"github.com/klauspost/compress/zstd"
	"github.com/pkg/errors"

	"github.com/amikos-tech/ami-gin/telemetry"
)

var (
	ErrVersionMismatch        = errors.New("version mismatch")
	ErrInvalidFormat          = errors.New("invalid format")
	ErrDecodedSizeExceedsLimit = errors.New("decoded size exceeds configured limit")
)
```

**SerializedConfig boundary** (lines 112-125):
```go
type SerializedConfig struct {
	BloomFilterSize         uint32            `json:"bloom_filter_size"`
	BloomFilterHashes       uint8             `json:"bloom_filter_hashes"`
	EnableTrigrams          bool              `json:"enable_trigrams"`
	TrigramMinLength        int               `json:"trigram_min_length"`
	HLLPrecision            uint8             `json:"hll_precision"`
	PrefixBlockSize         int               `json:"prefix_block_size"`
	AdaptiveMinRGCoverage   int               `json:"adaptive_min_rg_coverage"`
	AdaptivePromotedTermCap int               `json:"adaptive_promoted_term_cap"`
	AdaptiveCoverageCeiling float64           `json:"adaptive_coverage_ceiling"`
	AdaptiveBucketCount     int               `json:"adaptive_bucket_count"`
	FTSPaths                []string          `json:"fts_paths,omitempty"`
	Transformers            []TransformerSpec `json:"transformers,omitempty"`
}
```

**Encode/decode wrapping pattern** (lines 320-324, 468-475):
```go
if err := writeConfig(&buf, idx.Config); err != nil {
	return nil, errors.Wrap(err, "write config")
}
if err := writeRepresentations(&buf, idx); err != nil {
	return nil, errors.Wrap(err, "write representations")
}

cfg, err := readConfig(buf)
if err != nil {
	return nil, errors.Wrap(err, "read config")
}
idx.Config = cfg
representations, err := readRepresentations(buf)
```

**Config write pattern** (lines 1590-1629):
```go
func writeConfig(w io.Writer, cfg *GINConfig) error {
	if cfg == nil {
		return binary.Write(w, binary.LittleEndian, uint32(0))
	}

	sc := SerializedConfig{
		BloomFilterSize:         cfg.BloomFilterSize,
		BloomFilterHashes:       cfg.BloomFilterHashes,
		EnableTrigrams:          cfg.EnableTrigrams,
		TrigramMinLength:        cfg.TrigramMinLength,
		HLLPrecision:            cfg.HLLPrecision,
		PrefixBlockSize:         cfg.PrefixBlockSize,
		AdaptiveMinRGCoverage:   cfg.AdaptiveMinRGCoverage,
		AdaptivePromotedTermCap: cfg.AdaptivePromotedTermCap,
		AdaptiveCoverageCeiling: cfg.AdaptiveCoverageCeiling,
		AdaptiveBucketCount:     cfg.AdaptiveBucketCount,
		FTSPaths:                cfg.ftsPaths,
	}
	// ...
	transformer := representation.Transformer
	transformer.FailureMode = normalizeTransformerFailureMode(transformer.FailureMode)
	sc.Transformers = append(sc.Transformers, transformer)
```

**Config read and registration pattern** (lines 1666-1735):
```go
var sc SerializedConfig
if err := json.Unmarshal(data, &sc); err != nil {
	return nil, errors.Wrap(err, "unmarshal config")
}

cfg := &GINConfig{
	BloomFilterSize:         sc.BloomFilterSize,
	BloomFilterHashes:       sc.BloomFilterHashes,
	EnableTrigrams:          sc.EnableTrigrams,
	TrigramMinLength:        sc.TrigramMinLength,
	HLLPrecision:            sc.HLLPrecision,
	PrefixBlockSize:         sc.PrefixBlockSize,
	AdaptiveMinRGCoverage:   sc.AdaptiveMinRGCoverage,
	AdaptivePromotedTermCap: sc.AdaptivePromotedTermCap,
	AdaptiveCoverageCeiling: sc.AdaptiveCoverageCeiling,
	AdaptiveBucketCount:     sc.AdaptiveBucketCount,
}

for _, spec := range sc.Transformers {
	canonicalPath, err := canonicalizeSupportedPath(spec.Path)
	// ...
	spec.FailureMode = normalizeTransformerFailureMode(spec.FailureMode)
	fn, err := ReconstructTransformer(spec.ID, spec.Params)
	if err != nil {
		return nil, errors.Wrapf(err, "reconstruct transformer for path %s", spec.Path)
	}
	if err := cfg.addRepresentation(canonicalPath, alias, spec, true, spec.FailureMode, fn); err != nil {
		return nil, errors.Wrapf(ErrInvalidFormat, "register transformer for %s alias %q: %v", canonicalPath, alias, err)
	}
}
```

**Representation metadata validation pattern** (lines 1777-1845):
```go
func readRepresentations(r io.Reader) ([]RepresentationSpec, error) {
	var sectionLen uint32
	if err := binary.Read(r, binary.LittleEndian, &sectionLen); err != nil {
		if stderrors.Is(err, io.EOF) || stderrors.Is(err, io.ErrUnexpectedEOF) {
			return nil, errors.Wrap(ErrInvalidFormat, "missing representation metadata length")
		}
		return nil, err
	}
	// ...
	representation.Transformer.FailureMode = normalizeTransformerFailureMode(representation.Transformer.FailureMode)
	if err := validateTransformerFailureMode(representation.Transformer.FailureMode); err != nil {
		return nil, errors.Wrapf(ErrInvalidFormat, "invalid representation failure mode for %s alias %q: %v", canonicalPath, representation.Alias, err)
	}
	return representations, nil
}
```

**Apply to Phase 17:**
- Do not add parser/numeric failure modes to `SerializedConfig`.
- Preserve `Version = 9` unless the on-wire transformer token spelling intentionally changes.
- Prefer private wire-token helpers if public `IngestFailureMode` literals are `hard` / `soft`.
- Decode both legacy `strict` and `soft_fail` transformer tokens into the new type.

---

### `failure_modes_test.go` (test, request-response / transform)

**Primary analog:** `atomicity_test.go`
**Secondary analogs:** `parser_test.go`, `transformers_test.go`, `gin_test.go`

**Shared fixture/import pattern** (atomicity_test.go lines 1-15):
```go
package gin

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/pkg/errors"
)
```

**Non-tragic failure oracle** (atomicity_test.go lines 205-219):
```go
func requireAddDocumentNonTragicFailure(t *testing.T, builder *GINBuilder, err error, wantDocs uint64, wantNextPos int) {
	t.Helper()
	if err == nil {
		t.Fatal("AddDocument error = nil, want non-tragic failure")
	}
	if builder.tragicErr != nil {
		t.Fatalf("builder.tragicErr != nil after public failure: %v", builder.tragicErr)
	}
	if builder.numDocs != wantDocs {
		t.Fatalf("numDocs = %d, want %d", builder.numDocs, wantDocs)
	}
	if builder.nextPos != wantNextPos {
		t.Fatalf("nextPos = %d, want %d", builder.nextPos, wantNextPos)
	}
}
```

**Public failure catalog pattern** (atomicity_test.go lines 230-305):
```go
func TestAddDocumentPublicFailuresDoNotSetTragicErr(t *testing.T) {
	t.Run("malformed-json", func(t *testing.T) {
		builder := mustNewBuilder(t, DefaultConfig(), 4)
		err := builder.AddDocument(0, []byte("garbage"))
		requireAddDocumentNonTragicFailure(t, builder, err, 0, 0)
		requireSubsequentValidDocument(t, builder)
	})

	t.Run("strict-transformer", func(t *testing.T) {
		config, err := strictEmailAtomicityConfig()
		if err != nil {
			t.Fatalf("strictEmailAtomicityConfig: %v", err)
		}
		builder := mustNewBuilder(t, config, 4)
		err = builder.AddDocument(0, []byte(`{"email":42}`))
		requireAddDocumentNonTragicFailure(t, builder, err, 0, 0)
		if err := builder.AddDocument(1, []byte(`{"email":"valid@example.com"}`)); err != nil {
			t.Fatalf("valid email after transformer failure: %v", err)
		}
	})

	t.Run("validator-rejected-numeric-promotion", func(t *testing.T) {
		builder := mustNewBuilder(t, DefaultConfig(), 4)
		if err := builder.AddDocument(0, []byte(`{"score":9007199254740993}`)); err != nil {
			t.Fatalf("seed AddDocument failed: %v", err)
		}
		err := builder.AddDocument(1, []byte(`{"score":1.5}`))
		requireAddDocumentNonTragicFailure(t, builder, err, 1, 1)
	})
}
```

**Parser contract stays hard pattern** (parser_test.go lines 337-397):
```go
func TestAddDocumentRejectsParserSkippingBeginDocument(t *testing.T) {
	b, err := NewBuilder(DefaultConfig(), 4, WithParser(skipBeginDocumentParser{}))
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	err = b.AddDocument(DocID(0), []byte(`{"a":1}`))
	if err == nil {
		t.Fatal("expected error from AddDocument when parser skips BeginDocument, got nil")
	}
	if !strings.Contains(err.Error(), "did not call BeginDocument") {
		t.Fatalf("want error containing %q, got %q", "did not call BeginDocument", err.Error())
	}
}

func TestAddDocumentRejectsBeginDocumentRGIDMismatch(t *testing.T) {
	// same pattern: NewBuilder, AddDocument, require non-nil error and substring
}
```

**Transformer hard/soft analog to rewrite** (transformers_test.go lines 481-576):
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
	// hard mode expects AddDocument error and only later valid document indexed
}

func TestBuilderSoftFailSkipsCompanionWhenConfigured(t *testing.T) {
	config, err := NewConfig(
		WithCustomTransformer("$.email", "strict", func(value any) (any, bool) {
			s, ok := value.(string)
			if !ok || !strings.Contains(s, "@") {
				return nil, false
			}
			return s, true
		}, WithTransformerFailureMode(TransformerFailureSoft)),
	)
	// current expectations are obsolete: Phase 17 soft should skip the whole document
}
```

**Numeric validation analog** (gin_test.go lines 3328-3395):
```go
func TestValidateStagedPathsRejectsLossyPromotionBeforeMerge(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)
	if err := builder.AddDocument(0, []byte(`{"score":9007199254740993}`)); err != nil {
		t.Fatalf("seed AddDocument failed: %v", err)
	}

	state := newDocumentBuildState(1)
	state.getOrCreatePath("$.score").numericValues = append(
		state.getOrCreatePath("$.score").numericValues,
		stagedNumericValue{floatVal: 1.5},
	)

	err := builder.validateStagedPaths(state)
	if err == nil {
		t.Fatal("validateStagedPaths() = nil, want mixed numeric promotion error")
	}
}
```

**Apply to Phase 17:**
- A new `failure_modes_test.go` should own the cross-layer hard/soft matrix if the planner keeps tests centralized.
- Soft cases should assert `err == nil`, `builder.tragicErr == nil`, unchanged `numDocs`, unchanged `nextPos`, no rejected docID mapping, and subsequent valid document dense packing.
- Hard cases should preserve existing error-return behavior and error substrings where current tests already assert them.

---

### `parser_test.go` (test, streaming / request-response)

**Analog:** `parser_test.go`

**Imports and parser seam assertions** (lines 1-9, 29-44):
```go
import (
	"fmt"
	"strings"
	"testing"

	"github.com/pkg/errors"
)

var (
	_ parserSink = (*GINBuilder)(nil)
	_ Parser     = stdlibParser{}
)

func TestBuilderHasParserFields(t *testing.T) {
	b, err := NewBuilder(DefaultConfig(), 4)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	b.parserName = "xyz"
	b.currentDocState = newDocumentBuildState(0)
	if b.parserName != "xyz" || b.currentDocState == nil {
		t.Fatal("parser fields not writable/readable")
	}
}
```

**Contract failure pattern** (lines 337-397): use the contract tests quoted above for missing, duplicate, and mismatched `BeginDocument`.

**Apply to Phase 17:**
- Add parser failure-mode tests here only if not using `failure_modes_test.go`.
- Ordinary `stdlibParser` parse errors can be soft skipped; parser contract errors must continue to fail under soft parser mode.

---

### `transformers_test.go` (test, transform)

**Analog:** `transformers_test.go`

**Helper pattern** (lines 11-30):
```go
func requirePathID(t *testing.T, idx *GINIndex, path string) uint16 {
	t.Helper()
	pathID, ok := idx.pathLookup[path]
	if !ok {
		t.Fatalf("pathLookup[%q] missing", path)
	}
	return pathID
}

func mustRoundTripIndex(t *testing.T, idx *GINIndex) *GINIndex {
	t.Helper()
	encoded, err := Encode(idx)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	return decoded
}
```

**Hard transformer rejection pattern** (lines 481-536): copy the `NewConfig`, `NewBuilder`, failing `AddDocument`, valid follow-up, and finalized index assertions.

**Current soft transformer test to rewrite** (lines 538-576):
```go
if err := builder.AddDocument(0, []byte(`{"email":42}`)); err != nil {
	t.Fatalf("AddDocument(0) error = %v, want soft-fail success", err)
}
if err := builder.AddDocument(1, []byte(`{"email":"bob@example.com"}`)); err != nil {
	t.Fatalf("AddDocument(1) failed: %v", err)
}

idx := builder.Finalize()
if idx.Header.NumDocs != 2 {
	t.Fatalf("Header.NumDocs = %d, want 2", idx.Header.NumDocs)
}
```

**Apply to Phase 17:**
- Replace the `NumDocs == 2` and raw `42` expectations with whole-document skip expectations.
- Keep the custom transformer fixture style.
- Keep path/result assertions through `idx.Evaluate` after finalization.

---

### `atomicity_test.go` (test, batch / transform)

**Analog:** `atomicity_test.go`

**Atomicity build oracle** (lines 36-54):
```go
func buildAtomicityIndex(config GINConfig, docs []atomicityDoc, numRGs int) ([]byte, error) {
	if numRGs < 1 {
		numRGs = 1
	}
	builder, err := NewBuilder(config, numRGs)
	if err != nil {
		return nil, errors.Wrap(err, "new atomicity builder")
	}
	for _, doc := range docs {
		err := builder.AddDocument(doc.docID, doc.doc)
		switch {
		case doc.shouldFail && err == nil:
			return nil, errors.Errorf("AddDocument(%d) succeeded for expected failure", doc.docID)
		case !doc.shouldFail && err != nil:
			return nil, errors.Wrapf(err, "AddDocument(%d) failed for clean document", doc.docID)
		}
	}
	return Encode(builder.Finalize())
}
```

**Determinism and DocID mapping pattern** (lines 129-167):
```go
first, err := buildAtomicityIndex(config, docs, numRGs)
if err != nil {
	t.Fatalf("first build: %v", err)
}
second, err := buildAtomicityIndex(config, docs, numRGs)
if err != nil {
	t.Fatalf("second build: %v", err)
}
if !bytes.Equal(first, second) {
	t.Fatal("encoded index differs for identical clean corpus")
}
// DocIDMapping preserves accepted non-contiguous IDs
```

**Property oracle** (lines 489-526):
```go
properties.Property("failed documents do not change encoded index", prop.ForAll(
	func(corpus atomicityCorpus) string {
		fullBytes, err := buildAtomicityIndex(config, corpus.all, corpus.numRGs)
		if err != nil {
			return err.Error()
		}
		cleanBytes, err := buildAtomicityIndex(config, corpus.cleanOnly, corpus.numRGs)
		if err != nil {
			return err.Error()
		}
		if !bytes.Equal(fullBytes, cleanBytes) {
			return "full corpus and clean corpus encoded bytes differ"
		}
		return ""
	},
	genAtomicityCorpus(1000),
))
```

**Apply to Phase 17:**
- Reuse this shape for the soft-skip regression: failed soft documents should leave the same encoded bytes as a corpus where those documents were never attempted.
- A smaller table covering one parser failure, one transformer failure, and one numeric failure is enough for this phase.

---

### `serialize_security_test.go` (test, file-I/O)

**Analog:** `serialize_security_test.go`

**Representation section locator** (lines 69-125):
```go
func locateRepresentationSection(t *testing.T, data []byte) ([]RepresentationSpec, int) {
	t.Helper()
	payload := data[4:]
	buf := bytes.NewReader(payload)
	idx := NewGINIndex()

	if err := readHeader(buf, idx); err != nil {
		t.Fatalf("readHeader() error = %v", err)
	}
	// read all sections up to config
	if _, err := readConfig(buf); err != nil {
		t.Fatalf("readConfig() error = %v", err)
	}

	offset := len(data) - buf.Len()
	representations, err := readRepresentations(buf)
	if err != nil {
		t.Fatalf("readRepresentations() error = %v", err)
	}
	return representations, offset
}
```

**Version mismatch test pattern** (lines 294-327):
```go
tests := []struct {
	name    string
	version uint16
}{
	{name: "future version", version: 99},
	{name: "zero version", version: 0},
	{name: "previous phase version", version: 8},
}

for _, tt := range tests {
	t.Run(tt.name, func(t *testing.T) {
		binary.LittleEndian.PutUint16(data[8:10], tt.version)
		_, err = Decode(data)
		if err == nil {
			t.Fatalf("expected error for version %d, got nil", tt.version)
		}
		if !stderrors.Is(err, ErrVersionMismatch) {
			t.Fatalf("expected ErrVersionMismatch for version %d, got: %v", tt.version, err)
		}
	})
}
```

**Representation failure mode round-trip pattern** (lines 587-614):
```go
func TestRepresentationFailureModeRoundTrip(t *testing.T) {
	config, err := NewConfig(
		WithToLowerTransformer("$.email", "lower", WithTransformerFailureMode(TransformerFailureSoft)),
	)
	if err != nil {
		t.Fatalf("NewConfig() error = %v", err)
	}

	builder := mustNewBuilder(t, config, 1)
	if err := builder.AddDocument(0, []byte(`{"email":"Alice@Example.COM"}`)); err != nil {
		t.Fatalf("AddDocument() error = %v", err)
	}

	decoded := mustRoundTripIndex(t, builder.Finalize())
	specs := decoded.Config.representationSpecs["$.email"]
	if specs[0].Transformer.FailureMode != TransformerFailureSoft {
		t.Fatalf("decoded failure mode = %q, want %q", specs[0].Transformer.FailureMode, TransformerFailureSoft)
	}
}
```

**Hand-built config payload pattern** (lines 2244-2280):
```go
sc := SerializedConfig{
	Transformers: []TransformerSpec{
		lower,
		duplicateAlias,
	},
}

data, err := json.Marshal(sc)
if err != nil {
	t.Fatalf("json.Marshal() error = %v", err)
}

var buf bytes.Buffer
if err := binary.Write(&buf, binary.LittleEndian, uint32(len(data))); err != nil {
	t.Fatalf("binary.Write() error = %v", err)
}
if _, err := buf.Write(data); err != nil {
	t.Fatalf("buf.Write() error = %v", err)
}

_, err = readConfig(&buf)
if err == nil {
	t.Fatal("expected duplicate transformer alias error, got nil")
}
if !stderrors.Is(err, ErrInvalidFormat) {
	t.Fatalf("expected ErrInvalidFormat, got %v", err)
}
```

**Apply to Phase 17:**
- Add a legacy-token decode test by hand-building `SerializedConfig` and/or representation metadata with `failure_mode: "strict"` and `"soft_fail"`.
- Assert `Version` remains 9 if on-wire tokens remain unchanged.
- If tokens change, copy the version mismatch pattern and explicitly bump version/history.

---

### `gin_test.go` (test, validation / tragic boundary)

**Analog:** `gin_test.go`

**Shared helper pattern** (lines 18-27):
```go
const mixedNumericPromotionScoreErr = "unsupported mixed numeric promotion at $.score"

func mustNewBuilder(t *testing.T, config GINConfig, numRGs int) *GINBuilder {
	t.Helper()
	builder, err := NewBuilder(config, numRGs)
	if err != nil {
		t.Fatalf("failed to create builder: %v", err)
	}
	return builder
}
```

**Struct-literal compatibility pattern** (lines 266-280):
```go
func TestNewBuilderAllowsLegacyConfigLiteralWhenAdaptiveDisabled(t *testing.T) {
	config := GINConfig{
		CardinalityThreshold: 128,
		BloomFilterSize:      1 << 20,
		BloomFilterHashes:    7,
		EnableTrigrams:       true,
		TrigramMinLength:     3,
		HLLPrecision:         12,
		PrefixBlockSize:      16,
	}

	if _, err := NewBuilder(config, 2); err != nil {
		t.Fatalf("NewBuilder() error = %v, want legacy struct literal to remain valid", err)
	}
}
```

**Tragic recovery boundary pattern** (lines 540-570):
```go
func TestAddDocumentRefusesAfterRecoveredMergePanic(t *testing.T) {
	builder := mustNewBuilder(t, DefaultConfig(), 3)
	builder.testHooks.mergeStagedPathsPanicHook = func() { panic("simulated merge panic") }

	err := builder.AddDocument(0, []byte(`{"name":"alice"}`))
	if err == nil {
		t.Fatal("AddDocument() error = nil, want recovered merge panic")
	}
	if builder.tragicErr == nil {
		t.Fatal("builder.tragicErr = nil, want recovered merge panic error")
	}
	if builder.numDocs != 0 {
		t.Fatalf("numDocs = %d, want 0", builder.numDocs)
	}
	err = builder.AddDocument(1, []byte(`{"name":"bob"}`))
	if err == nil {
		t.Fatal("AddDocument after tragedy = nil, want refusal")
	}
}
```

**Apply to Phase 17:**
- Keep zero-value config compatibility by normalizing empty failure modes to hard.
- Keep merge panic tests hard even when all public failure modes are soft.
- Use existing numeric validation tests when covering validator-rejected promotion.

---

### `examples/failure-modes/main.go` (example / CLI, batch)

**Analogs:** `examples/basic/main.go`, `examples/transformers/main.go`

**Example main/run structure** (examples/basic/main.go lines 13-25):
```go
func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	builder, err := gin.NewBuilder(gin.DefaultConfig(), 4)
	if err != nil {
		return errors.Wrap(err, "create builder")
	}
```

**Config + builder pattern** (examples/transformers/main.go lines 21-35):
```go
func run() error {
	config, err := gin.NewConfig(
		gin.WithISODateTransformer("$.created_at", "epoch_ms"),
		gin.WithDateTransformer("$.birth_date", "epoch_ms"),
		gin.WithCustomDateTransformer("$.custom_ts", "epoch_ms", "2006/01/02 15:04"),
	)
	if err != nil {
		return errors.Wrap(err, "create config")
	}

	builder, err := gin.NewBuilder(config, 5)
	if err != nil {
		return errors.Wrap(err, "create builder")
	}
```

**Document ingestion helper** (examples/basic/main.go lines 88-100):
```go
type exampleDocument struct {
	rgID gin.DocID
	body string
}

func addDocuments(builder *gin.GINBuilder, docs ...exampleDocument) error {
	for _, doc := range docs {
		if err := builder.AddDocument(doc.rgID, []byte(doc.body)); err != nil {
			return errors.Wrapf(err, "add document to row group %d", doc.rgID)
		}
	}
	return nil
}
```

**Output pattern** (examples/basic/main.go lines 41-49):
```go
fmt.Printf("Index built: %d docs, %d row groups, %d paths\n",
	idx.Header.NumDocs, idx.Header.NumRowGroups, idx.Header.NumPaths)

result := idx.Evaluate([]gin.Predicate{
	gin.EQ("$.department", "engineering"),
})
fmt.Printf("Matching row groups: %v\n", result.ToSlice())
```

**Apply to Phase 17:**
- Keep `main` and `run` exactly like existing examples.
- Use `github.com/pkg/errors` for wrapping.
- Use fixed documents and deterministic `fmt.Printf` output.
- Demonstrate one hard configuration that returns an error on the first bad document.
- Demonstrate one soft configuration where bad parser/transformer/numeric inputs are skipped and accepted documents pack densely.

---

### `CHANGELOG.md` (documentation)

**Analog:** No root changelog exists.

**Planner guidance:**
- Create a new root `CHANGELOG.md`.
- Use a short `## Unreleased` section.
- Include the breaking rename in one line:
```markdown
- Breaking: `TransformerFailureMode`, `TransformerFailureStrict`, and `TransformerFailureSoft` were replaced by `IngestFailureMode`, `IngestFailureHard`, and `IngestFailureSoft`.
```
- A before/after snippet is acceptable if kept small.
- Do not include repository-hosting or company references.

## Shared Patterns

### Functional Options And Validation

**Source:** `gin.go` lines 642-652 and 750-817  
**Apply to:** `gin.go`, tests using `NewConfig`, examples using custom config

```go
cfg := DefaultConfig()
for _, opt := range opts {
	if err := opt(&cfg); err != nil {
		return GINConfig{}, err
	}
}
if err := cfg.validate(); err != nil {
	return GINConfig{}, err
}
return cfg, nil
```

Use this for `WithParserFailureMode`, `WithNumericFailureMode`, and the retargeted `WithTransformerFailureMode`.

### Staged Ingest Before Durable Commit

**Source:** `builder.go` lines 315-359 and 712-727  
**Apply to:** `builder.go`, `failure_modes_test.go`, `atomicity_test.go`

```go
if err := b.parser.Parse(jsonDoc, pos, b); err != nil {
	return err
}
// parser contract checks
return b.mergeDocumentState(docID, pos, exists, b.currentDocState)
```

```go
if !exists {
	b.docIDToPos[docID] = pos
	b.posToDocID = append(b.posToDocID, docID)
	b.nextPos++
}
b.numDocs++
```

Soft skips must return before the second block runs.

### Tragic Boundary

**Source:** `builder.go` lines 730-761 and `gin_test.go` lines 540-570  
**Apply to:** `builder.go`, numeric soft-mode tests

```go
if err := runMergeWithRecover(b.config.Logger, func() { b.mergeStagedPaths(state) }); err != nil {
	b.tragicErr = err
	return err
}
```

Do not classify recovered merge panics as parser, transformer, or numeric soft skips.

### Serialization Compatibility

**Source:** `serialize.go` lines 112-125, 1590-1629, 1777-1845  
**Apply to:** `serialize.go`, `transformer_registry.go`, `serialize_security_test.go`

```go
type SerializedConfig struct {
	// build/query config fields
	FTSPaths     []string          `json:"fts_paths,omitempty"`
	Transformers []TransformerSpec `json:"transformers,omitempty"`
}
```

Parser and numeric modes stay out of serialized config. Transformer failure mode remains representation metadata and must accept legacy wire tokens.

### Atomicity Test Oracle

**Source:** `atomicity_test.go` lines 205-219 and 489-526  
**Apply to:** `failure_modes_test.go`, `atomicity_test.go`

```go
if builder.tragicErr != nil {
	t.Fatalf("builder.tragicErr != nil after public failure: %v", builder.tragicErr)
}
if builder.numDocs != wantDocs {
	t.Fatalf("numDocs = %d, want %d", builder.numDocs, wantDocs)
}
if builder.nextPos != wantNextPos {
	t.Fatalf("nextPos = %d, want %d", builder.nextPos, wantNextPos)
}
```

For soft skips, invert only the error assertion: `err` should be nil, while durable state remains unchanged.

### Example Shape

**Source:** `examples/basic/main.go` lines 13-25 and 88-100  
**Apply to:** `examples/failure-modes/main.go`

```go
func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
```

Keep example output deterministic and wrap errors with `github.com/pkg/errors`.

## No Analog Found

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `CHANGELOG.md` | documentation | documentation | No root changelog exists in the repo. Use the Phase 17 context and validation wording. |

## Metadata

**Analog search scope:** repo root Go files, `examples/*/main.go`, phase docs, `AGENTS.md`, `CLAUDE.md`  
**Files scanned:** 75 Go files, 6 planning/project instruction markdown files  
**Strong analogs used:** `gin.go`, `builder.go`, `transformer_registry.go`, `serialize.go`, `atomicity_test.go`, `parser_test.go`, `transformers_test.go`, `serialize_security_test.go`, `gin_test.go`, `examples/basic/main.go`, `examples/transformers/main.go`  
**Pattern extraction date:** 2026-04-23
