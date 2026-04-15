package gin

import (
	"fmt"
	"math"
	"sort"
	"strconv"
)

func (idx *GINIndex) Evaluate(predicates []Predicate) *RGSet {
	if len(predicates) == 0 {
		return AllRGs(int(idx.Header.NumRowGroups))
	}

	result := AllRGs(int(idx.Header.NumRowGroups))
	for _, p := range predicates {
		rgSet := idx.evaluatePredicate(p)
		result = result.Intersect(rgSet)
		if result.IsEmpty() {
			break
		}
	}
	return result
}

func (idx *GINIndex) evaluatePredicate(p Predicate) *RGSet {
	pathID, entry := idx.findPath(p.Path)
	if pathID < 0 {
		return AllRGs(int(idx.Header.NumRowGroups))
	}

	switch p.Operator {
	case OpEQ:
		return idx.evaluateEQ(pathID, entry, p.Value)
	case OpNE:
		return idx.evaluateNE(pathID, entry, p.Value)
	case OpGT:
		return idx.evaluateGT(pathID, p.Value)
	case OpGTE:
		return idx.evaluateGTE(pathID, p.Value)
	case OpLT:
		return idx.evaluateLT(pathID, p.Value)
	case OpLTE:
		return idx.evaluateLTE(pathID, p.Value)
	case OpIN:
		return idx.evaluateIN(pathID, entry, p.Value)
	case OpNIN:
		return idx.evaluateNIN(pathID, entry, p.Value)
	case OpIsNull:
		return idx.evaluateIsNull(pathID)
	case OpIsNotNull:
		return idx.evaluateIsNotNull(pathID)
	case OpContains:
		return idx.evaluateContains(pathID, entry, p.Value)
	case OpRegex:
		return idx.evaluateRegex(pathID, entry, p.Value)
	default:
		return AllRGs(int(idx.Header.NumRowGroups))
	}
}

func (idx *GINIndex) findPath(path string) (int, *PathEntry) {
	canonicalPath, err := canonicalizeSupportedPath(path)
	if err != nil {
		return -1, nil
	}

	pathID, ok := idx.pathLookup[canonicalPath]
	if !ok {
		return -1, nil
	}
	if int(pathID) >= len(idx.PathDirectory) {
		return -1, nil
	}
	return int(pathID), &idx.PathDirectory[pathID]
}

func (idx *GINIndex) lookupAdaptiveStringMatch(pathID uint16, term string) (bitmap *RGSet, exact bool, ok bool) {
	adaptive, ok := idx.AdaptiveStringIndexes[pathID]
	if !ok {
		return nil, false, false
	}

	termIdx := sort.SearchStrings(adaptive.Terms, term)
	if termIdx < len(adaptive.Terms) && adaptive.Terms[termIdx] == term {
		return adaptive.RGBitmaps[termIdx].Clone(), true, true
	}

	if adaptive.BucketCount <= 0 || len(adaptive.BucketRGBitmaps) == 0 {
		return NoRGs(int(idx.Header.NumRowGroups)), false, true
	}

	bucketID := adaptiveBucketIndex(term, adaptive.BucketCount)
	if bucketID < 0 || bucketID >= len(adaptive.BucketRGBitmaps) {
		return NoRGs(int(idx.Header.NumRowGroups)), false, true
	}
	if adaptive.BucketRGBitmaps[bucketID] == nil {
		return NoRGs(int(idx.Header.NumRowGroups)), false, true
	}
	return adaptive.BucketRGBitmaps[bucketID].Clone(), false, true
}

func (idx *GINIndex) evaluateAdaptiveStringTerm(pathID int, entry *PathEntry, term string) (bitmap *RGSet, exact bool, ok bool) {
	numRGs := int(idx.Header.NumRowGroups)
	bloomKey := entry.PathName + "=" + term
	if !idx.GlobalBloom.MayContainString(bloomKey) {
		return NoRGs(numRGs), true, true
	}

	if sli, ok := idx.StringLengthIndexes[uint16(pathID)]; ok {
		queryLen := uint32(len(term))
		if queryLen < sli.GlobalMin || queryLen > sli.GlobalMax {
			return NoRGs(numRGs), true, true
		}
	}

	return idx.lookupAdaptiveStringMatch(uint16(pathID), term)
}

func stringPredicateTerm(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case bool:
		return strconv.FormatBool(v), true
	default:
		return "", false
	}
}

func (idx *GINIndex) evaluateEQ(pathID int, entry *PathEntry, value any) *RGSet {
	numRGs := int(idx.Header.NumRowGroups)

	switch v := value.(type) {
	case string:
		if entry.Flags&FlagAdaptiveHybrid != 0 {
			if match, _, ok := idx.evaluateAdaptiveStringTerm(pathID, entry, v); ok {
				return match
			}
		}

		bloomKey := entry.PathName + "=" + v
		if !idx.GlobalBloom.MayContainString(bloomKey) {
			return NoRGs(numRGs)
		}

		if sli, ok := idx.StringLengthIndexes[uint16(pathID)]; ok {
			queryLen := uint32(len(v))
			if queryLen < sli.GlobalMin || queryLen > sli.GlobalMax {
				return NoRGs(numRGs)
			}
		}

		if entry.Flags&FlagBloomOnly != 0 {
			return AllRGs(numRGs)
		}
		if si, ok := idx.StringIndexes[uint16(pathID)]; ok {
			termIdx := sort.SearchStrings(si.Terms, v)
			if termIdx < len(si.Terms) && si.Terms[termIdx] == v {
				return si.RGBitmaps[termIdx].Clone()
			}
			return NoRGs(numRGs)
		}

	case float64:
		if ni, ok := idx.NumericIndexes[uint16(pathID)]; ok {
			if ni.ValueType == NumericValueTypeIntOnly {
				if queryInt, ok := toExactInt64(v); ok {
					return idx.evaluateIntOnlyEQ(ni, numRGs, queryInt)
				}
				return NoRGs(numRGs)
			}
			if v < ni.GlobalMin || v > ni.GlobalMax {
				return NoRGs(numRGs)
			}
			result := MustNewRGSet(numRGs)
			for rgID, stat := range ni.RGStats {
				if stat.HasValue && v >= stat.Min && v <= stat.Max {
					result.Set(rgID)
				}
			}
			return result
		}

	case int:
		return idx.evaluateEQ(pathID, entry, int64(v))

	case int64:
		if ni, ok := idx.NumericIndexes[uint16(pathID)]; ok && ni.ValueType == NumericValueTypeIntOnly {
			return idx.evaluateIntOnlyEQ(ni, numRGs, v)
		}
		return idx.evaluateEQ(pathID, entry, float64(v))

	case bool:
		term := strconv.FormatBool(v)
		return idx.evaluateEQ(pathID, entry, term)
	}

	return AllRGs(numRGs)
}

func (idx *GINIndex) evaluateNE(pathID int, entry *PathEntry, value any) *RGSet {
	if entry.Flags&FlagAdaptiveHybrid != 0 {
		if term, ok := stringPredicateTerm(value); ok {
			presentRGs := idx.evaluateIsNotNull(pathID)
			if eqResult, exact, handled := idx.evaluateAdaptiveStringTerm(pathID, entry, term); handled {
				if !exact {
					return presentRGs
				}
				return presentRGs.Intersect(eqResult.Invert())
			}
		}
	}

	eqResult := idx.evaluateEQ(pathID, entry, value)
	presentRGs := idx.evaluateIsNotNull(pathID)
	return presentRGs.Intersect(eqResult.Invert())
}

func (idx *GINIndex) evaluateGT(pathID int, value any) *RGSet {
	numRGs := int(idx.Header.NumRowGroups)
	ni, ok := idx.NumericIndexes[uint16(pathID)]
	if !ok {
		return AllRGs(numRGs)
	}

	if ni.ValueType == NumericValueTypeIntOnly {
		queryInt, ok := toRoundedInt64(value, math.Floor)
		if !ok {
			return AllRGs(numRGs)
		}
		if queryInt >= ni.IntGlobalMax {
			return NoRGs(numRGs)
		}
		result := MustNewRGSet(numRGs)
		for rgID, stat := range ni.RGStats {
			if stat.HasValue && stat.IntMax > queryInt {
				result.Set(rgID)
			}
		}
		return result
	}

	v := toFloat64(value)
	if v == nil {
		return AllRGs(numRGs)
	}

	if *v >= ni.GlobalMax {
		return NoRGs(numRGs)
	}

	result := MustNewRGSet(numRGs)
	for rgID, stat := range ni.RGStats {
		if stat.HasValue && stat.Max > *v {
			result.Set(rgID)
		}
	}
	return result
}

func (idx *GINIndex) evaluateGTE(pathID int, value any) *RGSet {
	numRGs := int(idx.Header.NumRowGroups)
	ni, ok := idx.NumericIndexes[uint16(pathID)]
	if !ok {
		return AllRGs(numRGs)
	}

	if ni.ValueType == NumericValueTypeIntOnly {
		queryInt, ok := toRoundedInt64(value, math.Ceil)
		if !ok {
			return AllRGs(numRGs)
		}
		if queryInt > ni.IntGlobalMax {
			return NoRGs(numRGs)
		}
		result := MustNewRGSet(numRGs)
		for rgID, stat := range ni.RGStats {
			if stat.HasValue && stat.IntMax >= queryInt {
				result.Set(rgID)
			}
		}
		return result
	}

	v := toFloat64(value)
	if v == nil {
		return AllRGs(numRGs)
	}

	if *v > ni.GlobalMax {
		return NoRGs(numRGs)
	}

	result := MustNewRGSet(numRGs)
	for rgID, stat := range ni.RGStats {
		if stat.HasValue && stat.Max >= *v {
			result.Set(rgID)
		}
	}
	return result
}

func (idx *GINIndex) evaluateLT(pathID int, value any) *RGSet {
	numRGs := int(idx.Header.NumRowGroups)
	ni, ok := idx.NumericIndexes[uint16(pathID)]
	if !ok {
		return AllRGs(numRGs)
	}

	if ni.ValueType == NumericValueTypeIntOnly {
		queryInt, ok := toRoundedInt64(value, math.Ceil)
		if !ok {
			return AllRGs(numRGs)
		}
		if queryInt <= ni.IntGlobalMin {
			return NoRGs(numRGs)
		}
		result := MustNewRGSet(numRGs)
		for rgID, stat := range ni.RGStats {
			if stat.HasValue && stat.IntMin < queryInt {
				result.Set(rgID)
			}
		}
		return result
	}

	v := toFloat64(value)
	if v == nil {
		return AllRGs(numRGs)
	}

	if *v <= ni.GlobalMin {
		return NoRGs(numRGs)
	}

	result := MustNewRGSet(numRGs)
	for rgID, stat := range ni.RGStats {
		if stat.HasValue && stat.Min < *v {
			result.Set(rgID)
		}
	}
	return result
}

func (idx *GINIndex) evaluateLTE(pathID int, value any) *RGSet {
	numRGs := int(idx.Header.NumRowGroups)
	ni, ok := idx.NumericIndexes[uint16(pathID)]
	if !ok {
		return AllRGs(numRGs)
	}

	if ni.ValueType == NumericValueTypeIntOnly {
		queryInt, ok := toRoundedInt64(value, math.Floor)
		if !ok {
			return AllRGs(numRGs)
		}
		if queryInt < ni.IntGlobalMin {
			return NoRGs(numRGs)
		}
		result := MustNewRGSet(numRGs)
		for rgID, stat := range ni.RGStats {
			if stat.HasValue && stat.IntMin <= queryInt {
				result.Set(rgID)
			}
		}
		return result
	}

	v := toFloat64(value)
	if v == nil {
		return AllRGs(numRGs)
	}

	if *v < ni.GlobalMin {
		return NoRGs(numRGs)
	}

	result := MustNewRGSet(numRGs)
	for rgID, stat := range ni.RGStats {
		if stat.HasValue && stat.Min <= *v {
			result.Set(rgID)
		}
	}
	return result
}

func (idx *GINIndex) evaluateIN(pathID int, entry *PathEntry, value any) *RGSet {
	numRGs := int(idx.Header.NumRowGroups)
	values, ok := value.([]any)
	if !ok {
		return AllRGs(numRGs)
	}

	result := NoRGs(numRGs)
	for _, v := range values {
		if entry.Flags&FlagAdaptiveHybrid != 0 {
			if term, ok := stringPredicateTerm(v); ok {
				if rgSet, _, handled := idx.evaluateAdaptiveStringTerm(pathID, entry, term); handled {
					result = result.Union(rgSet)
					continue
				}
			}
		}

		rgSet := idx.evaluateEQ(pathID, entry, v)
		result = result.Union(rgSet)
	}
	return result
}

func (idx *GINIndex) evaluateNIN(pathID int, entry *PathEntry, value any) *RGSet {
	if entry.Flags&FlagAdaptiveHybrid != 0 {
		values, ok := value.([]any)
		if !ok {
			return AllRGs(int(idx.Header.NumRowGroups))
		}

		presentRGs := idx.evaluateIsNotNull(pathID)
		inResult := NoRGs(int(idx.Header.NumRowGroups))
		allExact := true
		for _, v := range values {
			term, ok := stringPredicateTerm(v)
			if !ok {
				return presentRGs.Intersect(idx.evaluateIN(pathID, entry, value).Invert())
			}
			rgSet, exact, handled := idx.evaluateAdaptiveStringTerm(pathID, entry, term)
			if !handled {
				return presentRGs.Intersect(idx.evaluateIN(pathID, entry, value).Invert())
			}
			if !exact {
				allExact = false
			}
			inResult = inResult.Union(rgSet)
		}
		if !allExact {
			return presentRGs
		}
		return presentRGs.Intersect(inResult.Invert())
	}

	inResult := idx.evaluateIN(pathID, entry, value)
	presentRGs := idx.evaluateIsNotNull(pathID)
	return presentRGs.Intersect(inResult.Invert())
}

func (idx *GINIndex) evaluateIsNull(pathID int) *RGSet {
	numRGs := int(idx.Header.NumRowGroups)
	if ni, ok := idx.NullIndexes[uint16(pathID)]; ok {
		return ni.NullRGBitmap.Clone()
	}
	return NoRGs(numRGs)
}

func (idx *GINIndex) evaluateIsNotNull(pathID int) *RGSet {
	numRGs := int(idx.Header.NumRowGroups)
	if ni, ok := idx.NullIndexes[uint16(pathID)]; ok {
		return ni.PresentRGBitmap.Clone()
	}
	return AllRGs(numRGs)
}

func (idx *GINIndex) evaluateContains(pathID int, entry *PathEntry, value any) *RGSet {
	numRGs := int(idx.Header.NumRowGroups)

	pattern, ok := value.(string)
	if !ok {
		return AllRGs(numRGs)
	}

	if entry.Flags&FlagTrigramIndex == 0 {
		return AllRGs(numRGs)
	}

	ti, ok := idx.TrigramIndexes[uint16(pathID)]
	if !ok {
		return AllRGs(numRGs)
	}

	return ti.Search(pattern)
}

func (idx *GINIndex) evaluateRegex(pathID int, entry *PathEntry, value any) *RGSet {
	numRGs := int(idx.Header.NumRowGroups)

	pattern, ok := value.(string)
	if !ok {
		return AllRGs(numRGs)
	}

	// Check if trigram index exists for this path
	if entry.Flags&FlagTrigramIndex == 0 {
		return AllRGs(numRGs)
	}

	ti, ok := idx.TrigramIndexes[uint16(pathID)]
	if !ok {
		return AllRGs(numRGs)
	}

	// Extract literals from regex pattern
	info, err := AnalyzeRegex(pattern)
	if err != nil || len(info.Literals) == 0 {
		return AllRGs(numRGs)
	}

	// If all literals are too short for trigrams, can't prune
	if info.MinLength < ti.N {
		allTooShort := true
		for _, lit := range info.Literals {
			if len(lit) >= ti.N {
				allTooShort = false
				break
			}
		}
		if allTooShort {
			return AllRGs(numRGs)
		}
	}

	// For each literal alternative, find matching row groups using trigrams
	// Union the results (OR semantics for alternation)
	result := NoRGs(numRGs)
	for _, lit := range info.Literals {
		if len(lit) < ti.N {
			// Literal too short, can't use trigrams - must include all RGs
			return AllRGs(numRGs)
		}
		// Use trigram index to find candidate row groups
		litResult := ti.Search(lit)
		result = result.Union(litResult)
	}

	return result
}

func toFloat64(v any) *float64 {
	switch val := v.(type) {
	case float64:
		return &val
	case float32:
		f := float64(val)
		return &f
	case int:
		f := float64(val)
		return &f
	case int64:
		f := float64(val)
		return &f
	case int32:
		f := float64(val)
		return &f
	default:
		return nil
	}
}

func toExactInt64(v any) (int64, bool) {
	switch val := v.(type) {
	case int:
		return int64(val), true
	case int8:
		return int64(val), true
	case int16:
		return int64(val), true
	case int32:
		return int64(val), true
	case int64:
		return val, true
	case uint:
		if val > math.MaxInt64 {
			return 0, false
		}
		return int64(val), true
	case uint8:
		return int64(val), true
	case uint16:
		return int64(val), true
	case uint32:
		return int64(val), true
	case uint64:
		if val > math.MaxInt64 {
			return 0, false
		}
		return int64(val), true
	case float64:
		if math.IsNaN(val) || math.IsInf(val, 0) || val != math.Trunc(val) || val < math.MinInt64 || val >= maxInt64AsFloat64 {
			return 0, false
		}
		return int64(val), true
	case float32:
		return toExactInt64(float64(val))
	default:
		return 0, false
	}
}

func toRoundedInt64(v any, round func(float64) float64) (int64, bool) {
	if exact, ok := toExactInt64(v); ok {
		return exact, true
	}

	f := toFloat64(v)
	if f == nil || math.IsNaN(*f) || math.IsInf(*f, 0) || *f < math.MinInt64 || *f >= maxInt64AsFloat64 {
		return 0, false
	}

	return int64(round(*f)), true
}

func (idx *GINIndex) evaluateIntOnlyEQ(ni *NumericIndex, numRGs int, queryInt int64) *RGSet {
	if queryInt < ni.IntGlobalMin || queryInt > ni.IntGlobalMax {
		return NoRGs(numRGs)
	}
	result := MustNewRGSet(numRGs)
	for rgID, stat := range ni.RGStats {
		if stat.HasValue && queryInt >= stat.IntMin && queryInt <= stat.IntMax {
			result.Set(rgID)
		}
	}
	return result
}

func EQ(path string, value any) Predicate {
	return Predicate{Path: path, Operator: OpEQ, Value: value}
}

func NE(path string, value any) Predicate {
	return Predicate{Path: path, Operator: OpNE, Value: value}
}

func GT(path string, value any) Predicate {
	return Predicate{Path: path, Operator: OpGT, Value: value}
}

func GTE(path string, value any) Predicate {
	return Predicate{Path: path, Operator: OpGTE, Value: value}
}

func LT(path string, value any) Predicate {
	return Predicate{Path: path, Operator: OpLT, Value: value}
}

func LTE(path string, value any) Predicate {
	return Predicate{Path: path, Operator: OpLTE, Value: value}
}

func IN(path string, values ...any) Predicate {
	return Predicate{Path: path, Operator: OpIN, Value: values}
}

func NIN(path string, values ...any) Predicate {
	return Predicate{Path: path, Operator: OpNIN, Value: values}
}

func IsNull(path string) Predicate {
	return Predicate{Path: path, Operator: OpIsNull}
}

func IsNotNull(path string) Predicate {
	return Predicate{Path: path, Operator: OpIsNotNull}
}

func Contains(path string, pattern string) Predicate {
	return Predicate{Path: path, Operator: OpContains, Value: pattern}
}

func Regex(path string, pattern string) Predicate {
	return Predicate{Path: path, Operator: OpRegex, Value: pattern}
}

func (o Operator) String() string {
	switch o {
	case OpEQ:
		return "="
	case OpNE:
		return "!="
	case OpGT:
		return ">"
	case OpGTE:
		return ">="
	case OpLT:
		return "<"
	case OpLTE:
		return "<="
	case OpIN:
		return "IN"
	case OpNIN:
		return "NOT IN"
	case OpIsNull:
		return "IS NULL"
	case OpIsNotNull:
		return "IS NOT NULL"
	case OpContains:
		return "CONTAINS"
	case OpRegex:
		return "REGEX"
	default:
		return "UNKNOWN"
	}
}

func (p Predicate) String() string {
	if p.Operator == OpIsNull || p.Operator == OpIsNotNull {
		return fmt.Sprintf("%s %s", p.Path, p.Operator)
	}
	return fmt.Sprintf("%s %s %v", p.Path, p.Operator, p.Value)
}

func (idx *GINIndex) MatchingDocIDs(rgSet *RGSet) []DocID {
	positions := rgSet.ToSlice()
	docIDs := make([]DocID, 0, len(positions))
	for _, pos := range positions {
		if pos < len(idx.DocIDMapping) {
			docIDs = append(docIDs, idx.DocIDMapping[pos])
		} else {
			docIDs = append(docIDs, DocID(pos))
		}
	}
	return docIDs
}
