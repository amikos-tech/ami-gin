package gin

import (
	"encoding/json"
	"sort"

	"github.com/pkg/errors"
)

const (
	MagicBytes = "GIN\x01"
	// Version is the binary format version. Decode rejects mismatches with
	// ErrVersionMismatch; the only migration path is to rebuild the index
	// with the target binary. Version history:
	//   v6: PathEntry.Mode byte + FlagTrigramIndex bit reassignment
	//       (phase 08 adaptive high-cardinality indexing)
	//   v5: never released; payloads are always rejected. Was an in-tree
	//       iteration of the adaptive string index section before the wire
	//       format was finalised in v6.
	//   v4: earlier pre-OSS format
	Version = uint16(6)
)

const (
	FlagHasDocIDMap uint16 = 1 << iota
)

const (
	TypeString uint8 = 1 << iota
	TypeInt
	TypeFloat
	TypeBool
	TypeNull
)

const (
	FlagTrigramIndex uint8 = 1 << iota // path has trigram index for CONTAINS queries
)

// PathMode is the exclusive storage mode for a path entry.
// The zero value is the classic exact mode.
type PathMode uint8

const (
	// PathModeClassic keeps the full exact string index for a path.
	// Its user-facing string label remains "exact" because that describes the
	// query semantics more clearly than the internal mode name.
	PathModeClassic PathMode = iota
	// PathModeBloomOnly stores no exact term index and answers via bloom-filter fallback.
	PathModeBloomOnly
	// PathModeAdaptiveHybrid stores promoted exact terms plus lossy tail buckets.
	PathModeAdaptiveHybrid
)

type GINIndex struct {
	// GINIndex is immutable after `Finalize()` or `Decode()`; pathLookup is
	// derived, non-serialized state rebuilt once and then treated as read-only.
	Header                Header
	PathDirectory         []PathEntry
	GlobalBloom           *BloomFilter
	StringIndexes         map[uint16]*StringIndex
	AdaptiveStringIndexes map[uint16]*AdaptiveStringIndex
	NumericIndexes        map[uint16]*NumericIndex
	NullIndexes           map[uint16]*NullIndex
	TrigramIndexes        map[uint16]*TrigramIndex
	StringLengthIndexes   map[uint16]*StringLengthIndex
	PathCardinality       map[uint16]*HyperLogLog
	DocIDMapping          []DocID
	Config                *GINConfig
	pathLookup            map[string]uint16
}

type Header struct {
	Magic             [4]byte
	Version           uint16
	Flags             uint16
	NumRowGroups      uint32
	NumDocs           uint64
	NumPaths          uint32
	CardinalityThresh uint32
}

type PathEntry struct {
	PathID        uint16
	PathName      string
	ObservedTypes uint8
	Cardinality   uint32
	// Mode is the exclusive string-evaluation mode for this path.
	Mode  PathMode
	Flags uint8
	// AdaptivePromotedTerms and AdaptiveBucketCount are derived metadata
	// populated from the adaptive section at decode time. They are not
	// persisted in the path directory; encoders must not rely on them.
	AdaptivePromotedTerms uint16
	AdaptiveBucketCount   uint16
}

type StringIndex struct {
	Terms     []string
	RGBitmaps []*RGSet
}

// AdaptiveStringIndex stores promoted exact terms plus lossy tail buckets.
// Terms must be sorted lexically; RGBitmaps is parallel to Terms. Values that
// are not promoted fall into one of len(BucketRGBitmaps) hash buckets, which
// may return false-positive row groups.
type AdaptiveStringIndex struct {
	// Terms holds the promoted exact-match values in sorted order.
	Terms []string
	// RGBitmaps[i] lists the row groups that contain Terms[i].
	RGBitmaps []*RGSet
	// BucketRGBitmaps partitions the long-tail terms by xxhash; len must be a
	// non-zero power of two. A bucket hit is a superset match (may include
	// row groups that do not actually contain the queried term).
	BucketRGBitmaps []*RGSet
}

// String returns the user-facing label used in CLI output and diagnostics.
func (m PathMode) String() string {
	switch m {
	case PathModeClassic:
		return "exact"
	case PathModeBloomOnly:
		return "bloom-only"
	case PathModeAdaptiveHybrid:
		return "adaptive-hybrid"
	default:
		return "unknown"
	}
}

// IsValid reports whether m is one of the declared PathMode constants.
// Decoders should call this on every byte read from disk before trusting
// the value; values outside the declared range indicate a corrupt payload.
func (m PathMode) IsValid() bool {
	switch m {
	case PathModeClassic, PathModeBloomOnly, PathModeAdaptiveHybrid:
		return true
	default:
		return false
	}
}

// NewAdaptiveStringIndex validates and constructs an adaptive string index.
func NewAdaptiveStringIndex(terms []string, rgBitmaps []*RGSet, bucketBitmaps []*RGSet) (*AdaptiveStringIndex, error) {
	if len(terms) != len(rgBitmaps) {
		return nil, errors.Errorf("adaptive rgbitmap count %d does not match term count %d", len(rgBitmaps), len(terms))
	}
	if !sort.StringsAreSorted(terms) {
		return nil, errors.New("adaptive terms must be sorted")
	}
	if len(bucketBitmaps) == 0 {
		return nil, errors.New("adaptive bucket count must be greater than 0")
	}
	if !isPowerOfTwo(len(bucketBitmaps)) {
		return nil, errors.Errorf("adaptive bucket count %d must be a power of two", len(bucketBitmaps))
	}

	for i, rgSet := range rgBitmaps {
		if rgSet == nil {
			return nil, errors.Errorf("adaptive promoted bitmap %d is nil", i)
		}
	}
	for i, rgSet := range bucketBitmaps {
		if rgSet == nil {
			return nil, errors.Errorf("adaptive bucket bitmap %d is nil", i)
		}
	}

	return &AdaptiveStringIndex{
		Terms:           terms,
		RGBitmaps:       rgBitmaps,
		BucketRGBitmaps: bucketBitmaps,
	}, nil
}

type NumericValueType uint8

const (
	NumericValueTypeIntOnly NumericValueType = iota
	NumericValueTypeFloatMixed
)

type NumericIndex struct {
	// ValueType is the numeric storage mode: int-only or float/mixed.
	ValueType    NumericValueType
	IntGlobalMin int64
	IntGlobalMax int64
	GlobalMin    float64
	GlobalMax    float64
	RGStats      []RGNumericStat
}

type RGNumericStat struct {
	IntMin   int64
	IntMax   int64
	Min      float64
	Max      float64
	HasValue bool
}

type NullIndex struct {
	NullRGBitmap    *RGSet
	PresentRGBitmap *RGSet
}

type StringLengthIndex struct {
	GlobalMin uint32
	GlobalMax uint32
	RGStats   []RGStringLengthStat
}

type RGStringLengthStat struct {
	Min      uint32
	Max      uint32
	HasValue bool
}

type Operator uint8

const (
	OpEQ Operator = iota
	OpNE
	OpGT
	OpLT
	OpGTE
	OpLTE
	OpIN
	OpNIN
	OpIsNull
	OpIsNotNull
	OpContains
	OpRegex
)

type Predicate struct {
	Path     string
	Operator Operator
	Value    any
}

// FieldTransformer transforms a value before indexing.
// Returns (transformedValue, ok). If ok=false, original value is indexed.
type FieldTransformer func(value any) (any, bool)

type GINConfig struct {
	CardinalityThreshold    uint32
	BloomFilterSize         uint32
	BloomFilterHashes       uint8
	EnableTrigrams          bool
	TrigramMinLength        int
	HLLPrecision            uint8
	PrefixBlockSize         int
	AdaptiveMinRGCoverage   int
	AdaptivePromotedTermCap int
	AdaptiveCoverageCeiling float64
	AdaptiveBucketCount     int
	ftsPaths                []string                    // paths to enable FTS on; empty means all paths
	fieldTransformers       map[string]FieldTransformer // path -> transformer
	transformerSpecs        map[string]TransformerSpec  // path -> spec for serialization
}

type ConfigOption func(*GINConfig) error

func WithFTSPaths(paths ...string) ConfigOption {
	return func(c *GINConfig) error {
		seen := make(map[string]string, len(paths))
		canonicalPaths := make([]string, 0, len(paths))
		for _, path := range paths {
			canonicalPath, err := canonicalizeSupportedPath(path)
			if err != nil {
				return err
			}
			if firstPath, exists := seen[canonicalPath]; exists {
				return errors.Errorf("duplicate canonical FTS path %q from %q and %q", canonicalPath, firstPath, path)
			}
			seen[canonicalPath] = path
			canonicalPaths = append(canonicalPaths, canonicalPath)
		}
		c.ftsPaths = canonicalPaths
		return nil
	}
}

func WithFieldTransformer(path string, fn FieldTransformer) ConfigOption {
	return func(c *GINConfig) error {
		canonicalPath, err := canonicalizeSupportedPath(path)
		if err != nil {
			return err
		}
		if c.fieldTransformers == nil {
			c.fieldTransformers = make(map[string]FieldTransformer)
		}
		c.fieldTransformers[canonicalPath] = fn
		return nil
	}
}

func WithRegisteredTransformer(path string, id TransformerID, params []byte) ConfigOption {
	return func(c *GINConfig) error {
		canonicalPath, err := canonicalizeSupportedPath(path)
		if err != nil {
			return err
		}
		if c.fieldTransformers == nil {
			c.fieldTransformers = make(map[string]FieldTransformer)
		}
		if c.transformerSpecs == nil {
			c.transformerSpecs = make(map[string]TransformerSpec)
		}
		fn, err := ReconstructTransformer(id, params)
		if err != nil {
			return err
		}
		c.fieldTransformers[canonicalPath] = fn
		c.transformerSpecs[canonicalPath] = NewTransformerSpec(canonicalPath, id, params)
		return nil
	}
}

func WithISODateTransformer(path string) ConfigOption {
	return WithRegisteredTransformer(path, TransformerISODateToEpochMs, nil)
}

func WithDateTransformer(path string) ConfigOption {
	return WithRegisteredTransformer(path, TransformerDateToEpochMs, nil)
}

func WithCustomDateTransformer(path, layout string) ConfigOption {
	params, _ := jsonMarshal(CustomDateParams{Layout: layout})
	return WithRegisteredTransformer(path, TransformerCustomDateToEpochMs, params)
}

func WithToLowerTransformer(path string) ConfigOption {
	return WithRegisteredTransformer(path, TransformerToLower, nil)
}

func WithIPv4Transformer(path string) ConfigOption {
	return WithRegisteredTransformer(path, TransformerIPv4ToInt, nil)
}

func WithSemVerTransformer(path string) ConfigOption {
	return WithRegisteredTransformer(path, TransformerSemVerToInt, nil)
}

func WithRegexExtractTransformer(path, pattern string, group int) ConfigOption {
	params, _ := jsonMarshal(RegexParams{Pattern: pattern, Group: group})
	return WithRegisteredTransformer(path, TransformerRegexExtract, params)
}

func WithRegexExtractIntTransformer(path, pattern string, group int) ConfigOption {
	params, _ := jsonMarshal(RegexParams{Pattern: pattern, Group: group})
	return WithRegisteredTransformer(path, TransformerRegexExtractInt, params)
}

func WithDurationTransformer(path string) ConfigOption {
	return WithRegisteredTransformer(path, TransformerDurationToMs, nil)
}

func WithEmailDomainTransformer(path string) ConfigOption {
	return WithRegisteredTransformer(path, TransformerEmailDomain, nil)
}

func WithURLHostTransformer(path string) ConfigOption {
	return WithRegisteredTransformer(path, TransformerURLHost, nil)
}

func WithNumericBucketTransformer(path string, size float64) ConfigOption {
	params, _ := jsonMarshal(NumericBucketParams{Size: size})
	return WithRegisteredTransformer(path, TransformerNumericBucket, params)
}

func WithBoolNormalizeTransformer(path string) ConfigOption {
	return WithRegisteredTransformer(path, TransformerBoolNormalize, nil)
}

// WithAdaptiveMinRGCoverage sets the minimum number of row groups a term must
// cover to be eligible for promotion to the exact adaptive index.
// Terms below this threshold fall into the bucket layer.
func WithAdaptiveMinRGCoverage(minCoverage int) ConfigOption {
	return func(c *GINConfig) error {
		if minCoverage < 0 {
			return errors.New("adaptive min RG coverage must be non-negative")
		}
		c.AdaptiveMinRGCoverage = minCoverage
		return nil
	}
}

// WithAdaptivePromotedTermCap caps the number of terms promoted to the exact
// adaptive index per high-cardinality path. Zero disables adaptive mode.
func WithAdaptivePromotedTermCap(cap int) ConfigOption {
	return func(c *GINConfig) error {
		if cap < 0 {
			return errors.New("adaptive promoted term cap must be non-negative")
		}
		c.AdaptivePromotedTermCap = cap
		return nil
	}
}

// WithAdaptiveCoverageCeiling sets the maximum fraction of row groups a term
// may cover and still be eligible for promotion. Terms above the ceiling are
// treated as too-ubiquitous and fall through to the bucket layer.
// Must be in the open interval (0, 1).
func WithAdaptiveCoverageCeiling(ceiling float64) ConfigOption {
	return func(c *GINConfig) error {
		if ceiling <= 0 || ceiling >= 1 {
			return errors.New("adaptive coverage ceiling must be greater than 0 and less than 1")
		}
		c.AdaptiveCoverageCeiling = ceiling
		return nil
	}
}

// WithAdaptiveBucketCount sets the fan-out of the long-tail bucket layer.
// Must be a positive power of two. To disable adaptive mode, omit this
// option (and WithAdaptivePromotedTermCap) or build a GINConfig literal
// with AdaptiveBucketCount/AdaptivePromotedTermCap set to 0; this option
// rejects 0 to keep the builder path explicit.
func WithAdaptiveBucketCount(bucketCount int) ConfigOption {
	return func(c *GINConfig) error {
		if bucketCount <= 0 {
			return errors.New("adaptive bucket count must be greater than 0")
		}
		if !isPowerOfTwo(bucketCount) {
			return errors.New("adaptive bucket count must be a power of two")
		}
		c.AdaptiveBucketCount = bucketCount
		return nil
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

func DefaultConfig() GINConfig {
	return GINConfig{
		CardinalityThreshold:    10000,
		BloomFilterSize:         65536,
		BloomFilterHashes:       5,
		EnableTrigrams:          true,
		TrigramMinLength:        3,
		HLLPrecision:            12,
		PrefixBlockSize:         16,
		AdaptiveMinRGCoverage:   2,
		AdaptivePromotedTermCap: 64,
		AdaptiveCoverageCeiling: 0.80,
		AdaptiveBucketCount:     128,
	}
}

// AdaptiveEnabled reports whether adaptive high-cardinality indexing is enabled.
func (c GINConfig) AdaptiveEnabled() bool {
	return c.AdaptivePromotedTermCap > 0 && c.AdaptiveBucketCount > 0
}

func NewGINIndex() *GINIndex {
	return &GINIndex{
		Header: Header{
			Magic:   [4]byte{'G', 'I', 'N', 0x01},
			Version: Version,
		},
		PathDirectory:         make([]PathEntry, 0),
		StringIndexes:         make(map[uint16]*StringIndex),
		AdaptiveStringIndexes: make(map[uint16]*AdaptiveStringIndex),
		NumericIndexes:        make(map[uint16]*NumericIndex),
		NullIndexes:           make(map[uint16]*NullIndex),
		TrigramIndexes:        make(map[uint16]*TrigramIndex),
		StringLengthIndexes:   make(map[uint16]*StringLengthIndex),
		PathCardinality:       make(map[uint16]*HyperLogLog),
		pathLookup:            make(map[string]uint16),
	}
}

func (c GINConfig) validate() error {
	// Zero is the disable sentinel for AdaptivePromotedTermCap and
	// AdaptiveBucketCount; AdaptiveEnabled() reports false when either is 0.
	// The functional options reject 0 to keep the builder path explicit, but
	// validate() must accept 0 so struct-literal callers can disable adaptive
	// mode without invoking the options.
	if c.AdaptiveMinRGCoverage < 0 {
		return errors.New("adaptive min RG coverage must be non-negative")
	}
	if c.AdaptivePromotedTermCap < 0 {
		return errors.New("adaptive promoted term cap must be non-negative")
	}
	if c.AdaptiveBucketCount < 0 {
		return errors.New("adaptive bucket count must be non-negative")
	}

	if c.AdaptiveEnabled() {
		if c.AdaptiveCoverageCeiling <= 0 || c.AdaptiveCoverageCeiling >= 1 {
			return errors.New("adaptive coverage ceiling must be greater than 0 and less than 1")
		}
		if !isPowerOfTwo(c.AdaptiveBucketCount) {
			return errors.New("adaptive bucket count must be a power of two")
		}
		if c.AdaptivePromotedTermCap > maxAdaptiveTermsPerPath {
			return errors.Errorf("adaptive promoted term cap must be <= %d", maxAdaptiveTermsPerPath)
		}
		if c.AdaptiveBucketCount > maxAdaptiveBucketsPerPath {
			return errors.Errorf("adaptive bucket count must be <= %d", maxAdaptiveBucketsPerPath)
		}
	}
	return nil
}

func (idx *GINIndex) rebuildPathLookup() error {
	canonicalDirectory := append([]PathEntry(nil), idx.PathDirectory...)
	lookup := make(map[string]uint16, len(idx.PathDirectory))
	originals := make(map[string]string, len(idx.PathDirectory))

	for i := range canonicalDirectory {
		entry := &canonicalDirectory[i]
		// Keep the explicit range guard ahead of the ordering check so corrupt
		// decodes report a precise out-of-range failure instead of a generic
		// out-of-order error.
		if int(entry.PathID) >= len(idx.PathDirectory) {
			return errors.Wrapf(ErrInvalidFormat, "path id %d out of range for %q", entry.PathID, entry.PathName)
		}
		if entry.PathID != uint16(i) {
			return errors.Wrapf(ErrInvalidFormat, "path id %d out of order at directory position %d for %q", entry.PathID, i, entry.PathName)
		}

		rawPath := entry.PathName
		canonical := NormalizePath(rawPath)
		if firstPath, exists := originals[canonical]; exists {
			return errors.Wrapf(ErrInvalidFormat, "duplicate canonical path %q from %q and %q", canonical, firstPath, rawPath)
		}
		entry.PathName = canonical
		lookup[canonical] = entry.PathID
		originals[canonical] = rawPath
	}

	if err := idx.validatePathReferences(); err != nil {
		return err
	}

	idx.PathDirectory = canonicalDirectory
	idx.pathLookup = lookup
	return nil
}

func (idx *GINIndex) validatePathReferences() error {
	for _, pathID := range sortedPathIDs(idx.StringIndexes) {
		if err := idx.validatePathReference("string index", pathID); err != nil {
			return err
		}
	}
	for _, pathID := range sortedPathIDs(idx.AdaptiveStringIndexes) {
		if err := idx.validatePathReference("adaptive string index", pathID); err != nil {
			return err
		}
	}
	for _, pathID := range sortedPathIDs(idx.StringLengthIndexes) {
		if err := idx.validatePathReference("string length index", pathID); err != nil {
			return err
		}
	}
	for _, pathID := range sortedPathIDs(idx.NumericIndexes) {
		if err := idx.validatePathReference("numeric index", pathID); err != nil {
			return err
		}
	}
	for _, pathID := range sortedPathIDs(idx.NullIndexes) {
		if err := idx.validatePathReference("null index", pathID); err != nil {
			return err
		}
	}
	for _, pathID := range sortedPathIDs(idx.TrigramIndexes) {
		if err := idx.validatePathReference("trigram index", pathID); err != nil {
			return err
		}
	}
	for _, pathID := range sortedPathIDs(idx.PathCardinality) {
		if err := idx.validatePathReference("path cardinality", pathID); err != nil {
			return err
		}
	}

	for i := range idx.PathDirectory {
		entry := idx.PathDirectory[i]
		_, hasStringIdx := idx.StringIndexes[entry.PathID]
		_, hasAdaptiveIdx := idx.AdaptiveStringIndexes[entry.PathID]

		switch entry.Mode {
		case PathModeClassic:
			if hasAdaptiveIdx {
				return errors.Wrapf(ErrInvalidFormat, "exact path %d must not have adaptive section", entry.PathID)
			}
		case PathModeBloomOnly:
			if hasStringIdx {
				return errors.Wrapf(ErrInvalidFormat, "bloom-only path %d must not have string index", entry.PathID)
			}
			if hasAdaptiveIdx {
				return errors.Wrapf(ErrInvalidFormat, "bloom-only path %d must not have adaptive section", entry.PathID)
			}
		case PathModeAdaptiveHybrid:
			if !hasAdaptiveIdx {
				return errors.Wrapf(ErrInvalidFormat, "adaptive path %d missing adaptive section", entry.PathID)
			}
			if hasStringIdx {
				return errors.Wrapf(ErrInvalidFormat, "adaptive path %d must not have exact string index", entry.PathID)
			}
		default:
			return errors.Wrapf(ErrInvalidFormat, "path %d has unknown mode %d", entry.PathID, entry.Mode)
		}
	}

	return nil
}

func sortedPathIDs[T any](m map[uint16]T) []uint16 {
	pathIDs := make([]uint16, 0, len(m))
	for pathID := range m {
		pathIDs = append(pathIDs, pathID)
	}
	sort.Slice(pathIDs, func(i, j int) bool {
		return pathIDs[i] < pathIDs[j]
	})
	return pathIDs
}

func (idx *GINIndex) validatePathReference(kind string, pathID uint16) error {
	if int(pathID) >= len(idx.PathDirectory) {
		return errors.Wrapf(ErrInvalidFormat, "%s path id %d out of range", kind, pathID)
	}
	return nil
}

func jsonMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func isPowerOfTwo(v int) bool {
	return v > 0 && (v&(v-1)) == 0
}
