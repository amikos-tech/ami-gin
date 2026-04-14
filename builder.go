package gin

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

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
	return b, nil
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
		pd.trigrams, _ = NewTrigramIndex(b.numRGs)
	}
	b.pathData[path] = pd
	return pd
}

func (b *GINBuilder) AddDocument(docID DocID, jsonDoc []byte) error {
	pos, exists := b.docIDToPos[docID]
	if !exists {
		pos = b.nextPos
		if pos >= b.numRGs {
			return errors.Errorf("position %d exceeds numRGs %d", pos, b.numRGs)
		}
		b.docIDToPos[docID] = pos
		b.posToDocID = append(b.posToDocID, docID)
		b.nextPos++
	}

	if pos > b.maxRGID {
		b.maxRGID = pos
	}
	b.numDocs++

	var doc any
	if err := json.Unmarshal(jsonDoc, &doc); err != nil {
		return errors.Wrap(err, "failed to parse JSON")
	}

	b.walkJSON("$", doc, pos)
	return nil
}

func (b *GINBuilder) walkJSON(path string, value any, rgID int) {
	canonicalPath := NormalizePath(path)

	if b.config.fieldTransformers != nil {
		if transformer, ok := b.config.fieldTransformers[canonicalPath]; ok {
			if transformed, ok := transformer(value); ok {
				value = transformed
			}
		}
	}

	pd := b.getOrCreatePath(canonicalPath)
	pd.presentRGs.Set(rgID)

	switch v := value.(type) {
	case nil:
		pd.observedTypes |= TypeNull
		pd.nullRGs.Set(rgID)

	case bool:
		pd.observedTypes |= TypeBool
		term := strconv.FormatBool(v)
		b.addStringTerm(pd, term, rgID, canonicalPath)

	case float64:
		if v == math.Trunc(v) && v >= math.MinInt64 && v <= math.MaxInt64 {
			pd.observedTypes |= TypeInt
		} else {
			pd.observedTypes |= TypeFloat
		}
		b.addNumericValue(pd, v, rgID)
		b.bloom.AddString(canonicalPath + "=" + strconv.FormatFloat(v, 'f', -1, 64))

	case string:
		pd.observedTypes |= TypeString
		b.addStringTerm(pd, v, rgID, canonicalPath)

	case []any:
		for i, item := range v {
			arrayPath := fmt.Sprintf("%s[%d]", path, i)
			b.walkJSON(arrayPath, item, rgID)
		}
		wildcardPath := path + "[*]"
		for _, item := range v {
			b.walkJSON(wildcardPath, item, rgID)
		}

	case map[string]any:
		for key, val := range v {
			childPath := path + "." + key
			b.walkJSON(childPath, val, rgID)
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

func (b *GINBuilder) addNumericValue(pd *pathBuildData, val float64, rgID int) {
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
		flags := uint8(0)
		if cardinality > b.config.CardinalityThreshold {
			flags |= FlagBloomOnly
		}
		if pd.trigrams != nil && pd.trigrams.TrigramCount() > 0 {
			flags |= FlagTrigramIndex
		}

		entry := PathEntry{
			PathID:        pd.pathID,
			PathName:      path,
			ObservedTypes: pd.observedTypes,
			Cardinality:   cardinality,
			Flags:         flags,
		}
		idx.PathDirectory = append(idx.PathDirectory, entry)

		idx.PathCardinality[pd.pathID] = pd.hll

		if pd.observedTypes&TypeString != 0 || pd.observedTypes&TypeBool != 0 {
			if flags&FlagBloomOnly == 0 && len(pd.stringTerms) > 0 {
				si := &StringIndex{
					Terms:     make([]string, 0, len(pd.stringTerms)),
					RGBitmaps: make([]*RGSet, 0, len(pd.stringTerms)),
				}
				terms := make([]string, 0, len(pd.stringTerms))
				for t := range pd.stringTerms {
					terms = append(terms, t)
				}
				sort.Strings(terms)
				for _, t := range terms {
					si.Terms = append(si.Terms, t)
					si.RGBitmaps = append(si.RGBitmaps, pd.stringTerms[t])
				}
				idx.StringIndexes[pd.pathID] = si
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

		if pd.observedTypes&(TypeInt|TypeFloat) != 0 && len(pd.numericStats) > 0 {
			ni := &NumericIndex{
				RGStats: make([]RGNumericStat, b.numRGs),
			}
			if pd.observedTypes&TypeFloat != 0 {
				ni.ValueType = 1
			}
			first := true
			for rgID, stat := range pd.numericStats {
				if rgID < len(ni.RGStats) {
					ni.RGStats[rgID] = *stat
				}
				if first {
					ni.GlobalMin = stat.Min
					ni.GlobalMax = stat.Max
					first = false
				} else {
					if stat.Min < ni.GlobalMin {
						ni.GlobalMin = stat.Min
					}
					if stat.Max > ni.GlobalMax {
						ni.GlobalMax = stat.Max
					}
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
	if err := idx.rebuildPathLookup(); err != nil {
		panic(err)
	}
	return idx
}
