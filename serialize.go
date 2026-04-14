package gin

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	stderrors "errors"
	"io"
	"math"

	"github.com/RoaringBitmap/roaring/v2"
	"github.com/klauspost/compress/zstd"
	"github.com/pkg/errors"
)

const maxConfigSize = 1 << 20 // 1MB max config size

const (
	// maxRGSetSize limits roaring bitmap deserialization.
	// 16MB covers worst-case bitmaps for millions of row groups.
	maxRGSetSize = 16 << 20

	// maxNumPaths matches PathID's uint16 range.
	maxNumPaths = 65535

	// maxTermsPerPath caps string index terms per path.
	// Default CardinalityThreshold is 10,000; 1M is generous headroom.
	maxTermsPerPath = 1_000_000

	// maxTrigramsPerPath caps trigram entries per path.
	// ASCII ceiling is ~2M (128^3); 10M covers extreme Unicode FTS.
	maxTrigramsPerPath = 10_000_000

	// maxBloomWords caps bloom filter word count.
	// Default is 65536 bits (1024 words). 1M words (~8MB) is generous.
	maxBloomWords = 1 << 20

	// maxHLLRegisters caps HyperLogLog register count.
	// Max precision 16 needs 2^16 = 65536 registers.
	maxHLLRegisters = 1 << 16
)

var (
	// ErrVersionMismatch is returned by Decode when the binary format version
	// does not match the expected version (Version constant).
	ErrVersionMismatch = errors.New("version mismatch")

	// ErrInvalidFormat is returned by Decode when the binary data is structurally
	// invalid: unrecognized magic bytes, oversized allocations, or corrupt fields.
	ErrInvalidFormat = errors.New("invalid format")
)

// CompressionLevel specifies the compression level for index serialization.
type CompressionLevel int

const (
	CompressionNone     CompressionLevel = 0  // No compression
	CompressionFastest  CompressionLevel = 1  // zstd level 1
	CompressionBalanced CompressionLevel = 3  // zstd level 3
	CompressionBetter   CompressionLevel = 9  // zstd level 9
	CompressionBest     CompressionLevel = 15 // zstd level 15 (recommended)
	CompressionMax      CompressionLevel = 19 // zstd level 19 (slow)
)

const (
	uncompressedMagic = "GINu"
	compressedMagic   = "GINc"
)

type SerializedConfig struct {
	BloomFilterSize   uint32            `json:"bloom_filter_size"`
	BloomFilterHashes uint8             `json:"bloom_filter_hashes"`
	EnableTrigrams    bool              `json:"enable_trigrams"`
	TrigramMinLength  int               `json:"trigram_min_length"`
	HLLPrecision      uint8             `json:"hll_precision"`
	PrefixBlockSize   int               `json:"prefix_block_size"`
	FTSPaths          []string          `json:"fts_paths,omitempty"`
	Transformers      []TransformerSpec `json:"transformers,omitempty"`
}

func writeRGSet(w io.Writer, rs *RGSet) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(rs.NumRGs)); err != nil {
		return err
	}
	data, err := rs.Roaring().ToBytes()
	if err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(len(data))); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func readRGSet(r io.Reader, maxRGs uint32) (*RGSet, error) {
	var numRGs uint32
	if err := binary.Read(r, binary.LittleEndian, &numRGs); err != nil {
		return nil, err
	}
	if numRGs > maxRGs {
		return nil, errors.Wrapf(ErrInvalidFormat, "rgset numRGs %d exceeds max %d", numRGs, maxRGs)
	}
	var dataLen uint32
	if err := binary.Read(r, binary.LittleEndian, &dataLen); err != nil {
		return nil, err
	}
	if dataLen > maxRGSetSize {
		return nil, errors.Wrapf(ErrInvalidFormat, "rgset data length %d exceeds max %d", dataLen, maxRGSetSize)
	}
	data := make([]byte, dataLen)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	bitmap := roaring.New()
	if err := bitmap.UnmarshalBinary(data); err != nil {
		return nil, err
	}
	return RGSetFromRoaring(bitmap, int(numRGs)), nil
}

// Encode serializes the index using zstd-15 compression (recommended default).
func Encode(idx *GINIndex) ([]byte, error) {
	return EncodeWithLevel(idx, CompressionBest)
}

// EncodeWithLevel serializes the index with the specified compression level.
// Use CompressionNone (0) for no compression, or 1-19 for zstd compression levels.
func EncodeWithLevel(idx *GINIndex, level CompressionLevel) ([]byte, error) {
	if level < 0 || level > 19 {
		return nil, errors.Errorf("compression level must be 0-19, got %d", level)
	}

	var buf bytes.Buffer

	if len(idx.DocIDMapping) > 0 {
		idx.Header.Flags |= FlagHasDocIDMap
	}

	if err := writeHeader(&buf, idx); err != nil {
		return nil, errors.Wrap(err, "write header")
	}

	if err := writePathDirectory(&buf, idx); err != nil {
		return nil, errors.Wrap(err, "write path directory")
	}

	if err := writeBloomFilter(&buf, idx.GlobalBloom); err != nil {
		return nil, errors.Wrap(err, "write bloom filter")
	}

	if err := writeStringIndexes(&buf, idx); err != nil {
		return nil, errors.Wrap(err, "write string indexes")
	}

	if err := writeStringLengthIndexes(&buf, idx); err != nil {
		return nil, errors.Wrap(err, "write string length indexes")
	}

	if err := writeNumericIndexes(&buf, idx); err != nil {
		return nil, errors.Wrap(err, "write numeric indexes")
	}

	if err := writeNullIndexes(&buf, idx); err != nil {
		return nil, errors.Wrap(err, "write null indexes")
	}

	if err := writeTrigramIndexes(&buf, idx); err != nil {
		return nil, errors.Wrap(err, "write trigram indexes")
	}

	if err := writeHyperLogLogs(&buf, idx); err != nil {
		return nil, errors.Wrap(err, "write hyperloglog")
	}

	if idx.Header.Flags&FlagHasDocIDMap != 0 {
		if err := writeDocIDMapping(&buf, idx.DocIDMapping); err != nil {
			return nil, errors.Wrap(err, "write docid mapping")
		}
	}

	if err := writeConfig(&buf, idx.Config); err != nil {
		return nil, errors.Wrap(err, "write config")
	}

	if level == CompressionNone {
		return append([]byte(uncompressedMagic), buf.Bytes()...), nil
	}

	encoder, err := zstd.NewWriter(nil,
		zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(int(level))))
	if err != nil {
		return nil, errors.Wrap(err, "create zstd encoder")
	}
	defer func() { _ = encoder.Close() }()

	compressed := encoder.EncodeAll(buf.Bytes(), nil)
	return append([]byte(compressedMagic), compressed...), nil
}

// Decode deserializes an index, validates cross-structure path references, and
// canonicalizes supported JSONPath spellings in PathDirectory while rebuilding
// derived lookup state.
func Decode(data []byte) (*GINIndex, error) {
	if len(data) < 4 {
		return nil, errors.Wrap(ErrInvalidFormat, "data too short")
	}

	var decompressed []byte
	magic := string(data[:4])

	switch magic {
	case uncompressedMagic:
		decompressed = data[4:]
	case compressedMagic:
		decoder, err := zstd.NewReader(nil)
		if err != nil {
			return nil, errors.Wrap(err, "create zstd decoder")
		}
		defer decoder.Close()

		decompressed, err = decoder.DecodeAll(data[4:], nil)
		if err != nil {
			return nil, errors.Wrap(err, "decompress data")
		}
	default:
		return nil, errors.Wrapf(ErrInvalidFormat, "unrecognized magic bytes: %q", magic)
	}

	buf := bytes.NewReader(decompressed)
	idx := NewGINIndex()

	if err := readHeader(buf, idx); err != nil {
		return nil, errors.Wrap(err, "read header")
	}

	if err := readPathDirectory(buf, idx); err != nil {
		return nil, errors.Wrap(err, "read path directory")
	}

	bloom, err := readBloomFilter(buf)
	if err != nil {
		return nil, errors.Wrap(err, "read bloom filter")
	}
	idx.GlobalBloom = bloom

	if err := readStringIndexes(buf, idx); err != nil {
		return nil, errors.Wrap(err, "read string indexes")
	}

	if err := readStringLengthIndexes(buf, idx, idx.Header.NumRowGroups); err != nil {
		return nil, errors.Wrap(err, "read string length indexes")
	}

	if err := readNumericIndexes(buf, idx, idx.Header.NumRowGroups); err != nil {
		return nil, errors.Wrap(err, "read numeric indexes")
	}

	if err := readNullIndexes(buf, idx); err != nil {
		return nil, errors.Wrap(err, "read null indexes")
	}

	if err := readTrigramIndexes(buf, idx); err != nil {
		return nil, errors.Wrap(err, "read trigram indexes")
	}

	if err := readHyperLogLogs(buf, idx); err != nil {
		return nil, errors.Wrap(err, "read hyperloglog")
	}

	if idx.Header.Flags&FlagHasDocIDMap != 0 {
		mapping, err := readDocIDMapping(buf, idx.Header.NumDocs)
		if err != nil {
			return nil, errors.Wrap(err, "read docid mapping")
		}
		idx.DocIDMapping = mapping
	}

	cfg, err := readConfig(buf)
	if err != nil {
		return nil, errors.Wrap(err, "read config")
	}
	idx.Config = cfg

	if err := idx.rebuildPathLookup(); err != nil {
		return nil, errors.Wrap(err, "rebuild path lookup")
	}

	return idx, nil
}

func writeHeader(w io.Writer, idx *GINIndex) error {
	if _, err := w.Write(idx.Header.Magic[:]); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, idx.Header.Version); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, idx.Header.Flags); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, idx.Header.NumRowGroups); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, idx.Header.NumDocs); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, idx.Header.NumPaths); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, idx.Header.CardinalityThresh)
}

func readHeader(r io.Reader, idx *GINIndex) error {
	if _, err := io.ReadFull(r, idx.Header.Magic[:]); err != nil {
		return err
	}
	if string(idx.Header.Magic[:]) != MagicBytes {
		return errors.Wrapf(ErrInvalidFormat, "invalid inner magic bytes: %q", string(idx.Header.Magic[:]))
	}
	if err := binary.Read(r, binary.LittleEndian, &idx.Header.Version); err != nil {
		return err
	}
	if idx.Header.Version != Version {
		return errors.Wrapf(ErrVersionMismatch, "got version %d, expected %d", idx.Header.Version, Version)
	}
	if err := binary.Read(r, binary.LittleEndian, &idx.Header.Flags); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &idx.Header.NumRowGroups); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &idx.Header.NumDocs); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &idx.Header.NumPaths); err != nil {
		return err
	}
	return binary.Read(r, binary.LittleEndian, &idx.Header.CardinalityThresh)
}

func writePathDirectory(w io.Writer, idx *GINIndex) error {
	for _, entry := range idx.PathDirectory {
		if err := binary.Write(w, binary.LittleEndian, entry.PathID); err != nil {
			return err
		}
		pathBytes := []byte(entry.PathName)
		if err := binary.Write(w, binary.LittleEndian, uint16(len(pathBytes))); err != nil {
			return err
		}
		if _, err := w.Write(pathBytes); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, entry.ObservedTypes); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, entry.Cardinality); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, entry.Flags); err != nil {
			return err
		}
	}
	return nil
}

func readPathDirectory(r io.Reader, idx *GINIndex) error {
	if idx.Header.NumPaths > maxNumPaths {
		return errors.Wrapf(ErrInvalidFormat, "path count %d exceeds max %d", idx.Header.NumPaths, maxNumPaths)
	}
	for i := uint32(0); i < idx.Header.NumPaths; i++ {
		var entry PathEntry
		if err := binary.Read(r, binary.LittleEndian, &entry.PathID); err != nil {
			return err
		}
		var pathLen uint16
		if err := binary.Read(r, binary.LittleEndian, &pathLen); err != nil {
			return err
		}
		pathBytes := make([]byte, pathLen)
		if _, err := io.ReadFull(r, pathBytes); err != nil {
			return err
		}
		entry.PathName = string(pathBytes)
		if err := binary.Read(r, binary.LittleEndian, &entry.ObservedTypes); err != nil {
			return err
		}
		if err := binary.Read(r, binary.LittleEndian, &entry.Cardinality); err != nil {
			return err
		}
		if err := binary.Read(r, binary.LittleEndian, &entry.Flags); err != nil {
			return err
		}
		idx.PathDirectory = append(idx.PathDirectory, entry)
	}
	return nil
}

func writeBloomFilter(w io.Writer, bf *BloomFilter) error {
	if err := binary.Write(w, binary.LittleEndian, bf.NumBits()); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, bf.NumHashes()); err != nil {
		return err
	}
	bits := bf.Bits()
	if err := binary.Write(w, binary.LittleEndian, uint32(len(bits))); err != nil {
		return err
	}
	for _, word := range bits {
		if err := binary.Write(w, binary.LittleEndian, word); err != nil {
			return err
		}
	}
	return nil
}

func readBloomFilter(r io.Reader) (*BloomFilter, error) {
	var numBits uint32
	if err := binary.Read(r, binary.LittleEndian, &numBits); err != nil {
		return nil, err
	}
	var numHashes uint8
	if err := binary.Read(r, binary.LittleEndian, &numHashes); err != nil {
		return nil, err
	}
	var numWords uint32
	if err := binary.Read(r, binary.LittleEndian, &numWords); err != nil {
		return nil, err
	}
	if numWords > maxBloomWords {
		return nil, errors.Wrapf(ErrInvalidFormat, "bloom filter word count %d exceeds max %d", numWords, maxBloomWords)
	}
	bits := make([]uint64, numWords)
	for i := uint32(0); i < numWords; i++ {
		if err := binary.Read(r, binary.LittleEndian, &bits[i]); err != nil {
			return nil, err
		}
	}
	return BloomFilterFromBits(bits, numBits, numHashes), nil
}

func writeStringIndexes(w io.Writer, idx *GINIndex) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(idx.StringIndexes))); err != nil {
		return err
	}
	for pathID, si := range idx.StringIndexes {
		if err := binary.Write(w, binary.LittleEndian, pathID); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint32(len(si.Terms))); err != nil {
			return err
		}
		for i, term := range si.Terms {
			termBytes := []byte(term)
			if err := binary.Write(w, binary.LittleEndian, uint16(len(termBytes))); err != nil {
				return err
			}
			if _, err := w.Write(termBytes); err != nil {
				return err
			}
			if err := writeRGSet(w, si.RGBitmaps[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func readStringIndexes(r io.Reader, idx *GINIndex) error {
	var numPaths uint32
	if err := binary.Read(r, binary.LittleEndian, &numPaths); err != nil {
		return err
	}
	if numPaths > maxNumPaths {
		return errors.Wrapf(ErrInvalidFormat, "string index path count %d exceeds max %d", numPaths, maxNumPaths)
	}
	for i := uint32(0); i < numPaths; i++ {
		var pathID uint16
		if err := binary.Read(r, binary.LittleEndian, &pathID); err != nil {
			return err
		}
		var numTerms uint32
		if err := binary.Read(r, binary.LittleEndian, &numTerms); err != nil {
			return err
		}
		if numTerms > maxTermsPerPath {
			return errors.Wrapf(ErrInvalidFormat, "terms count %d for path %d exceeds max %d", numTerms, pathID, maxTermsPerPath)
		}
		si := &StringIndex{
			Terms:     make([]string, numTerms),
			RGBitmaps: make([]*RGSet, numTerms),
		}
		for j := uint32(0); j < numTerms; j++ {
			var termLen uint16
			if err := binary.Read(r, binary.LittleEndian, &termLen); err != nil {
				return err
			}
			termBytes := make([]byte, termLen)
			if _, err := io.ReadFull(r, termBytes); err != nil {
				return err
			}
			si.Terms[j] = string(termBytes)

			rgSet, err := readRGSet(r, idx.Header.NumRowGroups)
			if err != nil {
				return err
			}
			si.RGBitmaps[j] = rgSet
		}
		idx.StringIndexes[pathID] = si
	}
	return nil
}

func writeStringLengthIndexes(w io.Writer, idx *GINIndex) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(idx.StringLengthIndexes))); err != nil {
		return err
	}
	for pathID, sli := range idx.StringLengthIndexes {
		if err := binary.Write(w, binary.LittleEndian, pathID); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, sli.GlobalMin); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, sli.GlobalMax); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint32(len(sli.RGStats))); err != nil {
			return err
		}
		for _, stat := range sli.RGStats {
			if err := binary.Write(w, binary.LittleEndian, stat.Min); err != nil {
				return err
			}
			if err := binary.Write(w, binary.LittleEndian, stat.Max); err != nil {
				return err
			}
			hasValue := uint8(0)
			if stat.HasValue {
				hasValue = 1
			}
			if err := binary.Write(w, binary.LittleEndian, hasValue); err != nil {
				return err
			}
		}
	}
	return nil
}

func readStringLengthIndexes(r io.Reader, idx *GINIndex, maxRGs uint32) error {
	var numPaths uint32
	if err := binary.Read(r, binary.LittleEndian, &numPaths); err != nil {
		return err
	}
	if numPaths > maxNumPaths {
		return errors.Wrapf(ErrInvalidFormat, "string length index path count %d exceeds max %d", numPaths, maxNumPaths)
	}
	for i := uint32(0); i < numPaths; i++ {
		var pathID uint16
		if err := binary.Read(r, binary.LittleEndian, &pathID); err != nil {
			return err
		}
		sli := &StringLengthIndex{}
		if err := binary.Read(r, binary.LittleEndian, &sli.GlobalMin); err != nil {
			return err
		}
		if err := binary.Read(r, binary.LittleEndian, &sli.GlobalMax); err != nil {
			return err
		}
		var numRGs uint32
		if err := binary.Read(r, binary.LittleEndian, &numRGs); err != nil {
			return err
		}
		if numRGs > maxRGs {
			return errors.Wrapf(ErrInvalidFormat, "string length index rg count %d for path %d exceeds max %d", numRGs, pathID, maxRGs)
		}
		sli.RGStats = make([]RGStringLengthStat, numRGs)
		for j := uint32(0); j < numRGs; j++ {
			if err := binary.Read(r, binary.LittleEndian, &sli.RGStats[j].Min); err != nil {
				return err
			}
			if err := binary.Read(r, binary.LittleEndian, &sli.RGStats[j].Max); err != nil {
				return err
			}
			var hasValue uint8
			if err := binary.Read(r, binary.LittleEndian, &hasValue); err != nil {
				return err
			}
			sli.RGStats[j].HasValue = hasValue != 0
		}
		idx.StringLengthIndexes[pathID] = sli
	}
	return nil
}

func writeNumericIndexes(w io.Writer, idx *GINIndex) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(idx.NumericIndexes))); err != nil {
		return err
	}
	for pathID, ni := range idx.NumericIndexes {
		if err := binary.Write(w, binary.LittleEndian, pathID); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, ni.ValueType); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, math.Float64bits(ni.GlobalMin)); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, math.Float64bits(ni.GlobalMax)); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint32(len(ni.RGStats))); err != nil {
			return err
		}
		for _, stat := range ni.RGStats {
			if err := binary.Write(w, binary.LittleEndian, math.Float64bits(stat.Min)); err != nil {
				return err
			}
			if err := binary.Write(w, binary.LittleEndian, math.Float64bits(stat.Max)); err != nil {
				return err
			}
			hasValue := uint8(0)
			if stat.HasValue {
				hasValue = 1
			}
			if err := binary.Write(w, binary.LittleEndian, hasValue); err != nil {
				return err
			}
		}
	}
	return nil
}

func readNumericIndexes(r io.Reader, idx *GINIndex, maxRGs uint32) error {
	var numPaths uint32
	if err := binary.Read(r, binary.LittleEndian, &numPaths); err != nil {
		return err
	}
	if numPaths > maxNumPaths {
		return errors.Wrapf(ErrInvalidFormat, "numeric index path count %d exceeds max %d", numPaths, maxNumPaths)
	}
	for i := uint32(0); i < numPaths; i++ {
		var pathID uint16
		if err := binary.Read(r, binary.LittleEndian, &pathID); err != nil {
			return err
		}
		ni := &NumericIndex{}
		if err := binary.Read(r, binary.LittleEndian, &ni.ValueType); err != nil {
			return err
		}
		var minBits, maxBits uint64
		if err := binary.Read(r, binary.LittleEndian, &minBits); err != nil {
			return err
		}
		if err := binary.Read(r, binary.LittleEndian, &maxBits); err != nil {
			return err
		}
		ni.GlobalMin = math.Float64frombits(minBits)
		ni.GlobalMax = math.Float64frombits(maxBits)

		var numRGs uint32
		if err := binary.Read(r, binary.LittleEndian, &numRGs); err != nil {
			return err
		}
		if numRGs > maxRGs {
			return errors.Wrapf(ErrInvalidFormat, "numeric index rg count %d for path %d exceeds max %d", numRGs, pathID, maxRGs)
		}
		ni.RGStats = make([]RGNumericStat, numRGs)
		for j := uint32(0); j < numRGs; j++ {
			if err := binary.Read(r, binary.LittleEndian, &minBits); err != nil {
				return err
			}
			if err := binary.Read(r, binary.LittleEndian, &maxBits); err != nil {
				return err
			}
			var hasValue uint8
			if err := binary.Read(r, binary.LittleEndian, &hasValue); err != nil {
				return err
			}
			ni.RGStats[j] = RGNumericStat{
				Min:      math.Float64frombits(minBits),
				Max:      math.Float64frombits(maxBits),
				HasValue: hasValue != 0,
			}
		}
		idx.NumericIndexes[pathID] = ni
	}
	return nil
}

func writeNullIndexes(w io.Writer, idx *GINIndex) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(idx.NullIndexes))); err != nil {
		return err
	}
	for pathID, ni := range idx.NullIndexes {
		if err := binary.Write(w, binary.LittleEndian, pathID); err != nil {
			return err
		}
		if err := writeRGSet(w, ni.NullRGBitmap); err != nil {
			return err
		}
		if err := writeRGSet(w, ni.PresentRGBitmap); err != nil {
			return err
		}
	}
	return nil
}

func readNullIndexes(r io.Reader, idx *GINIndex) error {
	var numPaths uint32
	if err := binary.Read(r, binary.LittleEndian, &numPaths); err != nil {
		return err
	}
	if numPaths > maxNumPaths {
		return errors.Wrapf(ErrInvalidFormat, "null index path count %d exceeds max %d", numPaths, maxNumPaths)
	}
	for i := uint32(0); i < numPaths; i++ {
		var pathID uint16
		if err := binary.Read(r, binary.LittleEndian, &pathID); err != nil {
			return err
		}
		nullBitmap, err := readRGSet(r, idx.Header.NumRowGroups)
		if err != nil {
			return err
		}
		presentBitmap, err := readRGSet(r, idx.Header.NumRowGroups)
		if err != nil {
			return err
		}
		idx.NullIndexes[pathID] = &NullIndex{
			NullRGBitmap:    nullBitmap,
			PresentRGBitmap: presentBitmap,
		}
	}
	return nil
}

func writeTrigramIndexes(w io.Writer, idx *GINIndex) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(idx.TrigramIndexes))); err != nil {
		return err
	}
	for pathID, ti := range idx.TrigramIndexes {
		if err := binary.Write(w, binary.LittleEndian, pathID); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint32(ti.NumRGs)); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint8(ti.N)); err != nil {
			return err
		}
		padBytes := []byte(ti.Padding)
		if err := binary.Write(w, binary.LittleEndian, uint8(len(padBytes))); err != nil {
			return err
		}
		if len(padBytes) > 0 {
			if _, err := w.Write(padBytes); err != nil {
				return err
			}
		}
		if err := binary.Write(w, binary.LittleEndian, uint32(len(ti.Trigrams))); err != nil {
			return err
		}
		for trigram, rgSet := range ti.Trigrams {
			trigramBytes := []byte(trigram)
			if err := binary.Write(w, binary.LittleEndian, uint8(len(trigramBytes))); err != nil {
				return err
			}
			if _, err := w.Write(trigramBytes); err != nil {
				return err
			}
			if err := writeRGSet(w, rgSet); err != nil {
				return err
			}
		}
	}
	return nil
}

func readTrigramIndexes(r io.Reader, idx *GINIndex) error {
	var numPaths uint32
	if err := binary.Read(r, binary.LittleEndian, &numPaths); err != nil {
		return err
	}
	if numPaths > maxNumPaths {
		return errors.Wrapf(ErrInvalidFormat, "trigram index path count %d exceeds max %d", numPaths, maxNumPaths)
	}
	for i := uint32(0); i < numPaths; i++ {
		var pathID uint16
		if err := binary.Read(r, binary.LittleEndian, &pathID); err != nil {
			return err
		}
		var numRGs uint32
		if err := binary.Read(r, binary.LittleEndian, &numRGs); err != nil {
			return err
		}
		var n uint8
		if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
			return err
		}
		var padLen uint8
		if err := binary.Read(r, binary.LittleEndian, &padLen); err != nil {
			return err
		}
		var padding string
		if padLen > 0 {
			padBytes := make([]byte, padLen)
			if _, err := io.ReadFull(r, padBytes); err != nil {
				return err
			}
			padding = string(padBytes)
		}
		ti, err := NewTrigramIndex(int(numRGs), WithN(int(n)), WithPadding(padding))
		if err != nil {
			return err
		}
		var numTrigrams uint32
		if err := binary.Read(r, binary.LittleEndian, &numTrigrams); err != nil {
			return err
		}
		if numTrigrams > maxTrigramsPerPath {
			return errors.Wrapf(ErrInvalidFormat, "trigram count %d for path %d exceeds max %d", numTrigrams, pathID, maxTrigramsPerPath)
		}
		for j := uint32(0); j < numTrigrams; j++ {
			var trigramLen uint8
			if err := binary.Read(r, binary.LittleEndian, &trigramLen); err != nil {
				return err
			}
			trigramBytes := make([]byte, trigramLen)
			if _, err := io.ReadFull(r, trigramBytes); err != nil {
				return err
			}
			rgSet, err := readRGSet(r, idx.Header.NumRowGroups)
			if err != nil {
				return err
			}
			ti.Trigrams[string(trigramBytes)] = rgSet
		}
		idx.TrigramIndexes[pathID] = ti
	}
	return nil
}

func writeHyperLogLogs(w io.Writer, idx *GINIndex) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(idx.PathCardinality))); err != nil {
		return err
	}
	for pathID, hll := range idx.PathCardinality {
		if err := binary.Write(w, binary.LittleEndian, pathID); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, hll.Precision()); err != nil {
			return err
		}
		registers := hll.Registers()
		if err := binary.Write(w, binary.LittleEndian, uint32(len(registers))); err != nil {
			return err
		}
		if _, err := w.Write(registers); err != nil {
			return err
		}
	}
	return nil
}

func readHyperLogLogs(r io.Reader, idx *GINIndex) error {
	var numPaths uint32
	if err := binary.Read(r, binary.LittleEndian, &numPaths); err != nil {
		return err
	}
	if numPaths > maxNumPaths {
		return errors.Wrapf(ErrInvalidFormat, "hll path count %d exceeds max %d", numPaths, maxNumPaths)
	}
	for i := uint32(0); i < numPaths; i++ {
		var pathID uint16
		if err := binary.Read(r, binary.LittleEndian, &pathID); err != nil {
			return err
		}
		var precision uint8
		if err := binary.Read(r, binary.LittleEndian, &precision); err != nil {
			return err
		}
		var numRegisters uint32
		if err := binary.Read(r, binary.LittleEndian, &numRegisters); err != nil {
			return err
		}
		if numRegisters > maxHLLRegisters {
			return errors.Wrapf(ErrInvalidFormat, "hll register count %d exceeds max %d", numRegisters, maxHLLRegisters)
		}
		registers := make([]uint8, numRegisters)
		if _, err := io.ReadFull(r, registers); err != nil {
			return err
		}
		idx.PathCardinality[pathID] = HyperLogLogFromRegisters(registers, precision)
	}
	return nil
}

func writeDocIDMapping(w io.Writer, mapping []DocID) error {
	if err := binary.Write(w, binary.LittleEndian, uint64(len(mapping))); err != nil {
		return err
	}
	for _, docID := range mapping {
		if err := binary.Write(w, binary.LittleEndian, uint64(docID)); err != nil {
			return err
		}
	}
	return nil
}

func readDocIDMapping(r io.Reader, maxDocs uint64) ([]DocID, error) {
	var numDocs uint64
	if err := binary.Read(r, binary.LittleEndian, &numDocs); err != nil {
		return nil, err
	}
	if numDocs > maxDocs {
		return nil, errors.Wrapf(ErrInvalidFormat, "docid mapping count %d exceeds max %d", numDocs, maxDocs)
	}
	mapping := make([]DocID, numDocs)
	for i := uint64(0); i < numDocs; i++ {
		var docID uint64
		if err := binary.Read(r, binary.LittleEndian, &docID); err != nil {
			return nil, err
		}
		mapping[i] = DocID(docID)
	}
	return mapping, nil
}

func writeConfig(w io.Writer, cfg *GINConfig) error {
	if cfg == nil {
		return binary.Write(w, binary.LittleEndian, uint32(0))
	}

	sc := SerializedConfig{
		BloomFilterSize:   cfg.BloomFilterSize,
		BloomFilterHashes: cfg.BloomFilterHashes,
		EnableTrigrams:    cfg.EnableTrigrams,
		TrigramMinLength:  cfg.TrigramMinLength,
		HLLPrecision:      cfg.HLLPrecision,
		PrefixBlockSize:   cfg.PrefixBlockSize,
		FTSPaths:          cfg.ftsPaths,
	}

	for _, spec := range cfg.transformerSpecs {
		sc.Transformers = append(sc.Transformers, spec)
	}

	data, err := json.Marshal(sc)
	if err != nil {
		return errors.Wrap(err, "marshal config")
	}

	if err := binary.Write(w, binary.LittleEndian, uint32(len(data))); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func readConfig(r io.Reader) (*GINConfig, error) {
	var configLen uint32
	if err := binary.Read(r, binary.LittleEndian, &configLen); err != nil {
		if stderrors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, err
	}

	if configLen == 0 {
		return nil, nil
	}

	if configLen > maxConfigSize {
		return nil, errors.Errorf("config size %d exceeds max %d", configLen, maxConfigSize)
	}

	data := make([]byte, configLen)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}

	var sc SerializedConfig
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, errors.Wrap(err, "unmarshal config")
	}

	cfg := &GINConfig{
		BloomFilterSize:   sc.BloomFilterSize,
		BloomFilterHashes: sc.BloomFilterHashes,
		EnableTrigrams:    sc.EnableTrigrams,
		TrigramMinLength:  sc.TrigramMinLength,
		HLLPrecision:      sc.HLLPrecision,
		PrefixBlockSize:   sc.PrefixBlockSize,
	}

	if len(sc.FTSPaths) > 0 {
		cfg.ftsPaths = make([]string, 0, len(sc.FTSPaths))
		seenFTSPaths := make(map[string]string, len(sc.FTSPaths))
		for _, path := range sc.FTSPaths {
			canonicalPath, err := canonicalizeSupportedPath(path)
			if err != nil {
				return nil, errors.Wrapf(err, "canonicalize FTS path %q", path)
			}
			if firstPath, exists := seenFTSPaths[canonicalPath]; exists {
				return nil, errors.Wrapf(ErrInvalidFormat, "duplicate canonical FTS path %q from %q and %q", canonicalPath, firstPath, path)
			}
			seenFTSPaths[canonicalPath] = path
			cfg.ftsPaths = append(cfg.ftsPaths, canonicalPath)
		}
	}

	if len(sc.Transformers) > 0 {
		cfg.fieldTransformers = make(map[string]FieldTransformer)
		cfg.transformerSpecs = make(map[string]TransformerSpec)
		seenTransformerPaths := make(map[string]string, len(sc.Transformers))
		for _, spec := range sc.Transformers {
			canonicalPath, err := canonicalizeSupportedPath(spec.Path)
			if err != nil {
				return nil, errors.Wrapf(err, "canonicalize transformer path %q", spec.Path)
			}
			if firstPath, exists := seenTransformerPaths[canonicalPath]; exists {
				return nil, errors.Wrapf(ErrInvalidFormat, "duplicate canonical transformer path %q from %q and %q", canonicalPath, firstPath, spec.Path)
			}
			seenTransformerPaths[canonicalPath] = spec.Path
			spec.Path = canonicalPath
			fn, err := ReconstructTransformer(spec.ID, spec.Params)
			if err != nil {
				return nil, errors.Wrapf(err, "reconstruct transformer for path %s", spec.Path)
			}
			cfg.fieldTransformers[canonicalPath] = fn
			cfg.transformerSpecs[canonicalPath] = spec
		}
	}

	return cfg, nil
}
