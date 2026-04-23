package gin

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

const maxExactFloatInt = int64(1 << 53)
const maxInt64AsFloat64 = float64(1 << 63) // upper bound for float64→int64; math.MaxInt64 rounds up to this

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
	// tragicErr is non-nil once an internal invariant violation or recovered
	// merge panic has closed the builder. Subsequent AddDocument calls refuse
	// to compound corruption; Finalize remains callable so callers can discard
	// the builder gracefully. Partial merges committed before a tragic panic are
	// not rolled back; Finalize after tragedy reflects an undefined subset of the
	// failing document's paths.
	tragicErr error

	// parser defaults to stdlibParser{} at NewBuilder; parserName is the
	// cached Parser.Name() result. During AddDocument, parserSink.BeginDocument
	// hands the staged state back through currentDocState and increments
	// beginDocumentCalls so AddDocument can enforce the sink contract after
	// Parse returns.
	parser             Parser
	parserName         string
	currentDocState    *documentBuildState
	beginDocumentCalls int
	testHooks          builderTestHooks
}

type builderTestHooks struct {
	mergeStagedPathsPanicHook func()
}

type pathBuildData struct {
	pathID            uint16
	observedTypes     uint8
	uniqueValues      map[string]struct{}
	stringTerms       map[string]*RGSet
	numericStats      map[int]*RGNumericStat
	stringLengthStats map[int]*RGStringLengthStat
	nullRGs           *RGSet
	presentRGs        *RGSet
	hll               *HyperLogLog
	trigrams          *TrigramIndex

	hasNumericValues bool
	numericValueType NumericValueType
	intGlobalMin     int64
	intGlobalMax     int64
	floatGlobalMin   float64
	floatGlobalMax   float64
}

type stagedNumericValue struct {
	isInt    bool
	intVal   int64
	floatVal float64
}

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

type documentBuildState struct {
	rgID  int
	paths map[string]*stagedPathData
}

func newDocumentBuildState(rgID int) *documentBuildState {
	return &documentBuildState{
		rgID:  rgID,
		paths: make(map[string]*stagedPathData),
	}
}

func (s *documentBuildState) getOrCreatePath(path string) *stagedPathData {
	if pd, ok := s.paths[path]; ok {
		return pd
	}
	pd := &stagedPathData{
		stringTerms: make(map[string]struct{}),
	}
	s.paths[path] = pd
	return pd
}

type BuilderOption func(*GINBuilder) error

func WithCodec(codec DocIDCodec) BuilderOption {
	return func(b *GINBuilder) error {
		if codec == nil {
			return errors.New("codec cannot be nil")
		}
		b.codec = codec
		return nil
	}
}

func NewBuilder(config GINConfig, numRGs int, opts ...BuilderOption) (*GINBuilder, error) {
	if numRGs <= 0 {
		return nil, errors.New("numRGs must be greater than 0")
	}
	if err := config.validate(); err != nil {
		return nil, err
	}
	bloom, err := NewBloomFilter(config.BloomFilterSize, config.BloomFilterHashes)
	if err != nil {
		return nil, errors.Wrap(err, "create bloom filter")
	}
	b := &GINBuilder{
		config:     config,
		numRGs:     numRGs,
		pathData:   make(map[string]*pathBuildData),
		bloom:      bloom,
		codec:      NewIdentityCodec(),
		docIDToPos: make(map[DocID]int),
		posToDocID: make([]DocID, 0),
	}
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

func adaptiveBucketIndex(term string, bucketCount int) int {
	if bucketCount <= 0 {
		panic("adaptive bucket count must be greater than 0")
	}
	return int(xxhash.Sum64String(term) & uint64(bucketCount-1))
}

func buildStringIndex(stringTerms map[string]*RGSet) *StringIndex {
	si := &StringIndex{
		Terms:     make([]string, 0, len(stringTerms)),
		RGBitmaps: make([]*RGSet, 0, len(stringTerms)),
	}
	terms := make([]string, 0, len(stringTerms))
	for term := range stringTerms {
		terms = append(terms, term)
	}
	sort.Strings(terms)
	for _, term := range terms {
		si.Terms = append(si.Terms, term)
		si.RGBitmaps = append(si.RGBitmaps, stringTerms[term])
	}
	return si
}

type adaptivePromotionCandidate struct {
	term     string
	coverage int
}

func (b *GINBuilder) selectAdaptivePromotedTerms(pd *pathBuildData) map[string]struct{} {
	if b.config.AdaptivePromotedTermCap == 0 || len(pd.stringTerms) == 0 {
		return map[string]struct{}{}
	}

	candidates := make([]adaptivePromotionCandidate, 0, len(pd.stringTerms))
	for term, rgSet := range pd.stringTerms {
		coverage := rgSet.Count()
		if coverage < b.config.AdaptiveMinRGCoverage {
			continue
		}
		if float64(coverage)/float64(b.numRGs) > b.config.AdaptiveCoverageCeiling {
			continue
		}
		candidates = append(candidates, adaptivePromotionCandidate{
			term:     term,
			coverage: coverage,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].coverage == candidates[j].coverage {
			return candidates[i].term < candidates[j].term
		}
		return candidates[i].coverage > candidates[j].coverage
	})

	if len(candidates) > b.config.AdaptivePromotedTermCap {
		candidates = candidates[:b.config.AdaptivePromotedTermCap]
	}

	promoted := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		promoted[candidate.term] = struct{}{}
	}
	return promoted
}

func (b *GINBuilder) buildAdaptiveStringIndex(pd *pathBuildData) *AdaptiveStringIndex {
	promoted := b.selectAdaptivePromotedTerms(pd)
	terms := make([]string, 0, len(promoted))
	rgBitmaps := make([]*RGSet, 0, len(promoted))
	bucketBitmaps := make([]*RGSet, b.config.AdaptiveBucketCount)

	for bucketID := range bucketBitmaps {
		bucketBitmaps[bucketID] = MustNewRGSet(b.numRGs)
	}

	for term := range promoted {
		terms = append(terms, term)
	}
	sort.Strings(terms)
	for _, term := range terms {
		rgBitmaps = append(rgBitmaps, pd.stringTerms[term].Clone())
	}

	for term, rgSet := range pd.stringTerms {
		if _, ok := promoted[term]; ok {
			continue
		}
		bucketID := adaptiveBucketIndex(term, len(bucketBitmaps))
		bucketBitmaps[bucketID].UnionWith(rgSet)
	}

	adaptive, err := NewAdaptiveStringIndex(terms, rgBitmaps, bucketBitmaps)
	if err != nil {
		panic(err)
	}
	return adaptive
}

func (b *GINBuilder) shouldEnableTrigrams(path string) bool {
	if !b.config.EnableTrigrams {
		return false
	}
	if len(b.config.ftsPaths) == 0 {
		return true
	}
	for _, pattern := range b.config.ftsPaths {
		if matchFTSPath(pattern, path) {
			return true
		}
	}
	return false
}

func matchFTSPath(pattern, path string) bool {
	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		return path == prefix || strings.HasPrefix(path, prefix+".")
	}
	return pattern == path
}

func (b *GINBuilder) getOrCreatePath(path string) *pathBuildData {
	if pd, ok := b.pathData[path]; ok {
		return pd
	}
	pd := &pathBuildData{
		pathID:            uint16(len(b.pathData)),
		uniqueValues:      make(map[string]struct{}),
		stringTerms:       make(map[string]*RGSet),
		numericStats:      make(map[int]*RGNumericStat),
		stringLengthStats: make(map[int]*RGStringLengthStat),
		nullRGs:           MustNewRGSet(b.numRGs),
		presentRGs:        MustNewRGSet(b.numRGs),
		hll:               MustNewHyperLogLog(b.config.HLLPrecision),
	}
	if b.shouldEnableTrigrams(path) {
		pd.trigrams = MustNewTrigramIndex(b.numRGs)
	}
	b.pathData[path] = pd
	return pd
}

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

func normalizeWalkPath(path string) string {
	if !strings.Contains(path, "['") && !strings.Contains(path, `["`) {
		return path
	}
	return NormalizePath(path)
}

func (b *GINBuilder) walkJSON(path string, value any, rgID int) error {
	if b.tragicErr != nil {
		return errors.Wrap(b.tragicErr, "builder closed by prior tragic failure; discard and rebuild")
	}

	state := newDocumentBuildState(rgID)
	if err := b.stageMaterializedValue(path, value, state, true); err != nil {
		return err
	}
	if err := b.validateStagedPaths(state); err != nil {
		return err
	}
	if err := runMergeWithRecover(b.config.Logger, func() { b.mergeStagedPaths(state) }); err != nil {
		b.tragicErr = err
		return err
	}
	return nil
}

func ensureDecoderEOF(decoder *json.Decoder) error {
	if _, err := decoder.Token(); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("unexpected trailing JSON content")
}

func decodeAny(decoder *json.Decoder) (any, error) {
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func sortedObjectKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func prepareTransformerValue(value any) any {
	switch v := value.(type) {
	case json.Number:
		if floatVal, err := strconv.ParseFloat(v.String(), 64); err == nil {
			return floatVal
		}
		return v.String()
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = prepareTransformerValue(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = prepareTransformerValue(item)
		}
		return out
	default:
		return value
	}
}

func (b *GINBuilder) stageScalarToken(canonicalPath string, token any, state *documentBuildState) error {
	pathState := state.getOrCreatePath(canonicalPath)
	pathState.present = true

	switch v := token.(type) {
	case nil:
		pathState.observedTypes |= TypeNull
		pathState.isNull = true
		return nil
	case bool:
		pathState.observedTypes |= TypeBool
		pathState.stringTerms[strconv.FormatBool(v)] = struct{}{}
		return nil
	case string:
		pathState.observedTypes |= TypeString
		pathState.stringTerms[v] = struct{}{}
		return nil
	case json.Number:
		return b.stageJSONNumberLiteral(canonicalPath, v.String(), state)
	default:
		return errors.Errorf("unsupported JSON token type %T at %s", token, canonicalPath)
	}
}

func (b *GINBuilder) stageMaterializedValue(path string, value any, state *documentBuildState, allowTransform bool) error {
	canonicalPath := normalizeWalkPath(path)
	if allowTransform {
		if err := b.stageCompanionRepresentations(canonicalPath, value, state); err != nil {
			return err
		}
	}

	pathState := state.getOrCreatePath(canonicalPath)
	pathState.present = true

	switch v := value.(type) {
	case nil:
		pathState.observedTypes |= TypeNull
		pathState.isNull = true
		return nil
	case bool:
		pathState.observedTypes |= TypeBool
		pathState.stringTerms[strconv.FormatBool(v)] = struct{}{}
		return nil
	case string:
		pathState.observedTypes |= TypeString
		pathState.stringTerms[v] = struct{}{}
		return nil
	case json.Number:
		return b.stageJSONNumberLiteral(canonicalPath, v.String(), state)
	case float64:
		return b.stageNativeNumeric(canonicalPath, v, state)
	case float32:
		return b.stageNativeNumeric(canonicalPath, float64(v), state)
	case int:
		return b.stageNativeNumeric(canonicalPath, int64(v), state)
	case int8:
		return b.stageNativeNumeric(canonicalPath, int64(v), state)
	case int16:
		return b.stageNativeNumeric(canonicalPath, int64(v), state)
	case int32:
		return b.stageNativeNumeric(canonicalPath, int64(v), state)
	case int64:
		return b.stageNativeNumeric(canonicalPath, v, state)
	case uint:
		if v > math.MaxInt64 {
			return errors.Errorf("unsupported integer at %s", canonicalPath)
		}
		return b.stageNativeNumeric(canonicalPath, int64(v), state)
	case uint8:
		return b.stageNativeNumeric(canonicalPath, int64(v), state)
	case uint16:
		return b.stageNativeNumeric(canonicalPath, int64(v), state)
	case uint32:
		return b.stageNativeNumeric(canonicalPath, int64(v), state)
	case uint64:
		if v > math.MaxInt64 {
			return errors.Errorf("unsupported integer at %s", canonicalPath)
		}
		return b.stageNativeNumeric(canonicalPath, int64(v), state)
	case []any:
		for i, item := range v {
			if err := b.stageMaterializedValue(fmt.Sprintf("%s[%d]", path, i), item, state, true); err != nil {
				return err
			}
			if err := b.stageMaterializedValue(path+"[*]", item, state, true); err != nil {
				return err
			}
		}
		return nil
	case map[string]any:
		for _, key := range sortedObjectKeys(v) {
			if err := b.stageMaterializedValue(path+"."+key, v[key], state, true); err != nil {
				return err
			}
		}
		return nil
	default:
		return errors.Errorf("unsupported transformed value type %T at %s", value, canonicalPath)
	}
}

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
		if err := b.stageMaterializedValue(registration.TargetPath, transformed, state, false); err != nil {
			return err
		}
	}

	return nil
}

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

func parseJSONNumberLiteral(raw string) (bool, int64, float64, error) {
	if strings.ContainsAny(raw, ".eE") {
		floatVal, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return false, 0, 0, err
		}
		if math.IsNaN(floatVal) || math.IsInf(floatVal, 0) {
			return false, 0, 0, errors.New("non-finite numeric value")
		}
		return false, 0, floatVal, nil
	}

	intVal, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return false, 0, 0, errors.Wrap(err, "unsupported integer literal")
	}
	return true, intVal, 0, nil
}

func (b *GINBuilder) stageNativeNumeric(path string, value any, state *documentBuildState) error {
	obs, err := stagedNumericFromValue(value)
	if err != nil {
		return errors.Wrapf(err, "parse numeric at %s", path)
	}
	return b.stageNumericObservation(path, obs, state)
}

func stagedNumericFromValue(value any) (stagedNumericValue, error) {
	switch v := value.(type) {
	case int64:
		return stagedNumericValue{isInt: true, intVal: v}, nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return stagedNumericValue{}, errors.New("non-finite numeric value")
		}
		if v == math.Trunc(v) && v >= math.MinInt64 && v < maxInt64AsFloat64 {
			return stagedNumericValue{isInt: true, intVal: int64(v)}, nil
		}
		return stagedNumericValue{floatVal: v}, nil
	default:
		return stagedNumericValue{}, errors.Errorf("unsupported numeric type %T", value)
	}
}

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

		pathState.numericSimValueType = NumericValueTypeFloatMixed
		pathState.numericSimFloatMin = math.Min(float64(pathState.numericSimIntMin), observation.floatVal)
		pathState.numericSimFloatMax = math.Max(float64(pathState.numericSimIntMax), observation.floatVal)
		pathState.observedTypes |= TypeFloat
		pathState.numericValues = append(pathState.numericValues, observation)
		return nil
	}

	if observation.isInt {
		if !canRepresentIntAsExactFloat(observation.intVal) {
			return errors.Errorf("unsupported mixed numeric promotion at %s", path)
		}
		floatVal := float64(observation.intVal)
		if floatVal < pathState.numericSimFloatMin {
			pathState.numericSimFloatMin = floatVal
		}
		if floatVal > pathState.numericSimFloatMax {
			pathState.numericSimFloatMax = floatVal
		}
		pathState.observedTypes |= TypeInt
		pathState.numericValues = append(pathState.numericValues, observation)
		return nil
	}

	if observation.floatVal < pathState.numericSimFloatMin {
		pathState.numericSimFloatMin = observation.floatVal
	}
	if observation.floatVal > pathState.numericSimFloatMax {
		pathState.numericSimFloatMax = observation.floatVal
	}
	pathState.observedTypes |= TypeFloat
	pathState.numericValues = append(pathState.numericValues, observation)
	return nil
}

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

func canRepresentIntAsExactFloat(value int64) bool {
	return value >= -maxExactFloatInt && value <= maxExactFloatInt
}

func (b *GINBuilder) mergeDocumentState(docID DocID, pos int, exists bool, state *documentBuildState) error {
	if err := b.validateStagedPaths(state); err != nil {
		return err
	}
	if err := runMergeWithRecover(b.config.Logger, func() { b.mergeStagedPaths(state) }); err != nil {
		b.tragicErr = err
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

func runMergeWithRecover(logger logging.Logger, fn func()) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			attrs := []logging.Attr{
				logging.AttrErrorType("other"),
				logging.Attr{Key: "panic_type", Value: fmt.Sprintf("%T", recovered)},
			}
			if message, ok := safeMergePanicMessage(recovered); ok {
				attrs = append(attrs, logging.Attr{Key: "panic_message", Value: message})
			}
			logging.Error(logger, "builder tragic: recovered panic in merge", attrs...)
			if e, ok := recovered.(error); ok {
				err = errors.Wrap(e, "builder tragic: recovered panic in merge")
				return
			}
			err = errors.Errorf("builder tragic: recovered panic in merge: %v", recovered)
		}
	}()
	fn()
	return nil
}

func safeMergePanicMessage(recovered any) (string, bool) {
	e, ok := recovered.(error)
	if !ok {
		return "", false
	}
	message := e.Error()
	if strings.HasPrefix(message, "validator missed unsupported mixed numeric promotion") {
		return message, true
	}
	return "", false
}

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

// MUST_BE_CHECKED_BY_VALIDATOR
func (b *GINBuilder) mergeStagedPaths(state *documentBuildState) {
	paths := make([]string, 0, len(state.paths))
	for path := range state.paths {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		staged := state.paths[path]
		if b.testHooks.mergeStagedPathsPanicHook != nil {
			b.testHooks.mergeStagedPathsPanicHook()
		}
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
			b.mergeNumericObservation(pd, observation, state.rgID, path)
		}
	}
}

func (b *GINBuilder) addStringTerm(pd *pathBuildData, term string, rgID int, path string) {
	pd.uniqueValues[term] = struct{}{}
	pd.hll.AddString(term)

	if _, ok := pd.stringTerms[term]; !ok {
		pd.stringTerms[term] = MustNewRGSet(b.numRGs)
	}
	pd.stringTerms[term].Set(rgID)

	b.bloom.AddString(path + "=" + term)

	b.addStringLengthStat(pd, len(term), rgID)

	if pd.trigrams != nil && len(term) >= b.config.TrigramMinLength {
		pd.trigrams.Add(term, rgID)
	}
}

// MUST_BE_CHECKED_BY_VALIDATOR
func (b *GINBuilder) mergeNumericObservation(pd *pathBuildData, observation stagedNumericValue, rgID int, path string) {
	if !pd.hasNumericValues {
		pd.hasNumericValues = true
		if observation.isInt {
			pd.numericValueType = NumericValueTypeIntOnly
			pd.intGlobalMin = observation.intVal
			pd.intGlobalMax = observation.intVal
			b.addIntNumericValue(pd, observation.intVal, rgID)
			b.bloom.AddString(path + "=" + strconv.FormatInt(observation.intVal, 10))
			return
		}
		pd.numericValueType = NumericValueTypeFloatMixed
		pd.floatGlobalMin = observation.floatVal
		pd.floatGlobalMax = observation.floatVal
		b.addFloatNumericValue(pd, observation.floatVal, rgID)
		b.bloom.AddString(path + "=" + strconv.FormatFloat(observation.floatVal, 'f', -1, 64))
		return
	}

	if pd.numericValueType == NumericValueTypeIntOnly && !observation.isInt {
		b.promoteNumericPathToFloat(pd)
	}

	if pd.numericValueType == NumericValueTypeIntOnly {
		if observation.intVal < pd.intGlobalMin {
			pd.intGlobalMin = observation.intVal
		}
		if observation.intVal > pd.intGlobalMax {
			pd.intGlobalMax = observation.intVal
		}
		b.addIntNumericValue(pd, observation.intVal, rgID)
		b.bloom.AddString(path + "=" + strconv.FormatInt(observation.intVal, 10))
		return
	}

	floatVal := observation.floatVal
	if observation.isInt {
		if !canRepresentIntAsExactFloat(observation.intVal) {
			panic(errors.Errorf("validator missed unsupported mixed numeric promotion at %s", path))
		}
		floatVal = float64(observation.intVal)
	}

	if floatVal < pd.floatGlobalMin {
		pd.floatGlobalMin = floatVal
	}
	if floatVal > pd.floatGlobalMax {
		pd.floatGlobalMax = floatVal
	}
	b.addFloatNumericValue(pd, floatVal, rgID)
	b.bloom.AddString(path + "=" + strconv.FormatFloat(floatVal, 'f', -1, 64))
}

// MUST_BE_CHECKED_BY_VALIDATOR
func (b *GINBuilder) promoteNumericPathToFloat(pd *pathBuildData) {
	if pd.numericValueType == NumericValueTypeFloatMixed {
		return
	}
	if !canRepresentIntAsExactFloat(pd.intGlobalMin) || !canRepresentIntAsExactFloat(pd.intGlobalMax) {
		panic(errors.New("validator missed unsupported mixed numeric promotion"))
	}
	for _, stat := range pd.numericStats {
		if !stat.HasValue {
			continue
		}
		if !canRepresentIntAsExactFloat(stat.IntMin) || !canRepresentIntAsExactFloat(stat.IntMax) {
			panic(errors.New("validator missed unsupported mixed numeric promotion"))
		}
	}
	pd.numericValueType = NumericValueTypeFloatMixed
	pd.floatGlobalMin = float64(pd.intGlobalMin)
	pd.floatGlobalMax = float64(pd.intGlobalMax)
	for _, stat := range pd.numericStats {
		if !stat.HasValue {
			continue
		}
		stat.Min = float64(stat.IntMin)
		stat.Max = float64(stat.IntMax)
	}
}

func (b *GINBuilder) addIntNumericValue(pd *pathBuildData, val int64, rgID int) {
	stat, ok := pd.numericStats[rgID]
	if !ok {
		pd.numericStats[rgID] = &RGNumericStat{
			IntMin: val,
			IntMax: val,
			// Int-only query logic uses IntMin/IntMax; Min/Max are kept for exported stats.
			Min:      float64(val),
			Max:      float64(val),
			HasValue: true,
		}
		return
	}
	if val < stat.IntMin {
		stat.IntMin = val
		stat.Min = float64(val)
	}
	if val > stat.IntMax {
		stat.IntMax = val
		stat.Max = float64(val)
	}
}

func (b *GINBuilder) addFloatNumericValue(pd *pathBuildData, val float64, rgID int) {
	stat, ok := pd.numericStats[rgID]
	if !ok {
		pd.numericStats[rgID] = &RGNumericStat{
			Min:      val,
			Max:      val,
			HasValue: true,
		}
		return
	}
	if val < stat.Min {
		stat.Min = val
	}
	if val > stat.Max {
		stat.Max = val
	}
}

func (b *GINBuilder) addStringLengthStat(pd *pathBuildData, length int, rgID int) {
	stat, ok := pd.stringLengthStats[rgID]
	if !ok {
		pd.stringLengthStats[rgID] = &RGStringLengthStat{
			Min:      uint32(length),
			Max:      uint32(length),
			HasValue: true,
		}
		return
	}
	if uint32(length) < stat.Min {
		stat.Min = uint32(length)
	}
	if uint32(length) > stat.Max {
		stat.Max = uint32(length)
	}
}

func (b *GINBuilder) Finalize() *GINIndex {
	idx := NewGINIndex()
	idx.GlobalBloom = b.bloom
	idx.Header.NumRowGroups = uint32(b.numRGs)
	idx.Header.NumDocs = b.numDocs
	idx.Header.CardinalityThresh = b.config.CardinalityThreshold
	idx.DocIDMapping = b.posToDocID
	idx.Config = &b.config

	paths := make([]string, 0, len(b.pathData))
	for p := range b.pathData {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for i, path := range paths {
		pd := b.pathData[path]
		pd.pathID = uint16(i)

		cardinality := uint32(pd.hll.Estimate())
		mode := PathModeClassic
		flags := uint8(0)
		adaptivePromotedTerms := uint16(0)
		adaptiveBucketCount := uint16(0)
		highCardinality := cardinality > b.config.CardinalityThreshold
		adaptiveEligible := b.config.AdaptiveEnabled() &&
			(pd.observedTypes&(TypeString|TypeBool) != 0) &&
			len(pd.stringTerms) > 0
		if highCardinality && !adaptiveEligible {
			mode = PathModeBloomOnly
		} else if highCardinality {
			mode = PathModeAdaptiveHybrid
			adaptiveBucketCount = uint16(b.config.AdaptiveBucketCount)
		}
		if pd.trigrams != nil && pd.trigrams.TrigramCount() > 0 {
			flags |= FlagTrigramIndex
		}

		entry := PathEntry{
			PathID:                pd.pathID,
			PathName:              path,
			ObservedTypes:         pd.observedTypes,
			Cardinality:           cardinality,
			Mode:                  mode,
			Flags:                 flags,
			AdaptivePromotedTerms: adaptivePromotedTerms,
			AdaptiveBucketCount:   adaptiveBucketCount,
		}
		if pd.observedTypes&TypeString != 0 || pd.observedTypes&TypeBool != 0 {
			switch {
			case mode == PathModeAdaptiveHybrid:
				adaptive := b.buildAdaptiveStringIndex(pd)
				idx.AdaptiveStringIndexes[pd.pathID] = adaptive
				entry.AdaptivePromotedTerms = uint16(len(adaptive.Terms))
			case mode == PathModeClassic && len(pd.stringTerms) > 0:
				idx.StringIndexes[pd.pathID] = buildStringIndex(pd.stringTerms)
			}

			if len(pd.stringLengthStats) > 0 {
				sli := &StringLengthIndex{
					RGStats: make([]RGStringLengthStat, b.numRGs),
				}
				first := true
				for rgID, stat := range pd.stringLengthStats {
					if rgID < len(sli.RGStats) {
						sli.RGStats[rgID] = *stat
					}
					if first {
						sli.GlobalMin = stat.Min
						sli.GlobalMax = stat.Max
						first = false
					} else {
						if stat.Min < sli.GlobalMin {
							sli.GlobalMin = stat.Min
						}
						if stat.Max > sli.GlobalMax {
							sli.GlobalMax = stat.Max
						}
					}
				}
				idx.StringLengthIndexes[pd.pathID] = sli
			}
		}

		idx.PathDirectory = append(idx.PathDirectory, entry)
		idx.pathLookup[path] = pd.pathID

		idx.PathCardinality[pd.pathID] = pd.hll

		if pd.observedTypes&(TypeInt|TypeFloat) != 0 && len(pd.numericStats) > 0 {
			ni := &NumericIndex{
				ValueType: pd.numericValueType,
				RGStats:   make([]RGNumericStat, b.numRGs),
			}
			if pd.numericValueType == NumericValueTypeIntOnly {
				ni.IntGlobalMin = pd.intGlobalMin
				ni.IntGlobalMax = pd.intGlobalMax
				ni.GlobalMin = float64(pd.intGlobalMin)
				ni.GlobalMax = float64(pd.intGlobalMax)
			} else {
				ni.GlobalMin = pd.floatGlobalMin
				ni.GlobalMax = pd.floatGlobalMax
			}
			for rgID, stat := range pd.numericStats {
				if rgID < len(ni.RGStats) {
					ni.RGStats[rgID] = *stat
				}
			}
			idx.NumericIndexes[pd.pathID] = ni
		}

		if pd.nullRGs.Count() > 0 || pd.presentRGs.Count() > 0 {
			idx.NullIndexes[pd.pathID] = &NullIndex{
				NullRGBitmap:    pd.nullRGs,
				PresentRGBitmap: pd.presentRGs,
			}
		}

		if pd.trigrams != nil && pd.trigrams.TrigramCount() > 0 {
			idx.TrigramIndexes[pd.pathID] = pd.trigrams
		}
	}

	idx.Header.NumPaths = uint32(len(idx.PathDirectory))
	idx.representations = collectMaterializedRepresentationsFromConfig(idx.Config, idx.pathLookup)
	if err := idx.rebuildRepresentationLookup(); err != nil {
		// Finalize() only panics here on broken builder invariants. User-driven
		// cases such as missing derived values are filtered out above when we
		// collect only materialized representations.
		panic(err)
	}
	return idx
}
