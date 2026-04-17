package gin

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	stderrors "errors"
	"io"
	"math"
	"sort"

	"github.com/RoaringBitmap/roaring/v2"
	"github.com/klauspost/compress/zstd"
	"github.com/pkg/errors"
)

const (
	maxConfigSize                = 1 << 20 // 1MB max config size
	maxRepresentationSectionSize = 1 << 20 // 1MB max representation metadata size
)

const (
	// maxDecodedIndexSize caps zstd.DecodeAll's output buffer to defend against
	// decompression bombs. 64 MiB accommodates multi-TB Parquet catalogs (tested
	// up to ~1M row groups / 100M docs) while bounding per-decode allocation to a
	// value safe on 1 GiB heaps.
	maxDecodedIndexSize = 64 << 20

	// maxRGSetSize limits roaring bitmap deserialization.
	// 16MB covers worst-case bitmaps for millions of row groups.
	maxRGSetSize = 16 << 20

	// maxNumPaths matches PathID's uint16 range.
	maxNumPaths = 65535

	// maxHeaderRowGroups bounds NumRowGroups-driven allocations during Decode.
	// 1M matches the largest observed Parquet catalogs (~1 PiB at 1 GiB per RG);
	// values above this are rejected as corrupt.
	maxHeaderRowGroups = 1_000_000
	// maxHeaderDocs bounds NumDocs-driven allocations (primarily DocIDMapping,
	// which is a []DocID of uint64 entries). 100M caps DocIDMapping at ~800 MiB.
	maxHeaderDocs = 100_000_000

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

	// maxAdaptivePaths reuses maxNumPaths because at most one adaptive section
	// can exist per path, and the uint16 PathID ceiling is the real bound.
	maxAdaptivePaths = maxNumPaths

	// maxAdaptiveTermsPerPath caps promoted exact terms persisted per path.
	// PathEntry summary counters are uint16-backed, so larger values would be ambiguous.
	maxAdaptiveTermsPerPath = 1<<16 - 1

	// maxAdaptiveBucketsPerPath caps fixed bucket fan-out for adaptive paths.
	// Default is 128; 4096 preserves headroom without allowing pathological allocations.
	maxAdaptiveBucketsPerPath = 1 << 12
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

const (
	compactStringModeRaw uint8 = iota
	compactStringModeFrontCoded
)

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
	if err := idx.validatePathReferences(); err != nil {
		return nil, errors.Wrap(err, "validate path references")
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

	if err := writeAdaptiveStringIndexes(&buf, idx); err != nil {
		return nil, errors.Wrap(err, "write adaptive string indexes")
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
	if err := writeRepresentations(&buf, idx); err != nil {
		return nil, errors.Wrap(err, "write representations")
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
		decoder, err := zstd.NewReader(nil,
			zstd.WithDecoderMaxMemory(maxDecodedIndexSize),
			zstd.WithDecoderMaxWindow(maxDecodedIndexSize),
			zstd.WithDecodeAllCapLimit(true),
		)
		if err != nil {
			return nil, errors.Wrap(err, "create zstd decoder")
		}
		defer decoder.Close()

		decompressed, err = decoder.DecodeAll(data[4:], make([]byte, 0, maxDecodedIndexSize))
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

	if err := readAdaptiveStringIndexes(buf, idx); err != nil {
		return nil, errors.Wrap(err, "read adaptive string indexes")
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
	representations, err := readRepresentations(buf)
	if err != nil {
		return nil, errors.Wrap(err, "read representations")
	}
	idx.representations = representations

	if err := idx.rebuildPathLookup(); err != nil {
		return nil, errors.Wrap(err, "rebuild path lookup")
	}
	if err := idx.rebuildRepresentationLookup(); err != nil {
		return nil, errors.Wrap(err, "rebuild representation lookup")
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
	if idx.Header.NumRowGroups > maxHeaderRowGroups {
		return errors.Wrapf(ErrInvalidFormat, "row-group count %d exceeds max %d", idx.Header.NumRowGroups, maxHeaderRowGroups)
	}
	if err := binary.Read(r, binary.LittleEndian, &idx.Header.NumDocs); err != nil {
		return err
	}
	if idx.Header.NumDocs > maxHeaderDocs {
		return errors.Wrapf(ErrInvalidFormat, "doc count %d exceeds max %d", idx.Header.NumDocs, maxHeaderDocs)
	}
	if err := binary.Read(r, binary.LittleEndian, &idx.Header.NumPaths); err != nil {
		return err
	}
	return binary.Read(r, binary.LittleEndian, &idx.Header.CardinalityThresh)
}

func rejectDuplicateSectionPath[T any](kind string, sections map[uint16]T, pathID uint16) error {
	if _, exists := sections[pathID]; exists {
		return errors.Wrapf(ErrInvalidFormat, "duplicate %s for path %d", kind, pathID)
	}
	return nil
}

func orderedStringBlockSize(idx *GINIndex) int {
	if idx != nil && idx.Config != nil && idx.Config.PrefixBlockSize > 0 {
		return idx.Config.PrefixBlockSize
	}
	return defaultPrefixBlockSize
}

func wrapOrderedStringFormatError(context string, err error) error {
	if err == nil {
		return nil
	}
	if stderrors.Is(err, ErrInvalidFormat) {
		return err
	}
	return errors.Wrapf(ErrInvalidFormat, "%s: %v", context, err)
}

func writeOrderedStrings(w io.Writer, values []string, blockSize int) error {
	if blockSize < 1 {
		blockSize = defaultPrefixBlockSize
	}

	rawPayload, err := encodeRawOrderedStrings(values)
	if err != nil {
		return err
	}

	frontPayload, err := encodeFrontCodedOrderedStrings(values, blockSize)
	if err != nil {
		return err
	}

	selected := frontPayload
	if rawPayload.Len() <= frontPayload.Len() {
		selected = rawPayload
	}

	_, err = w.Write(selected.Bytes())
	return err
}

func encodeRawOrderedStrings(values []string) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	if err := buf.WriteByte(compactStringModeRaw); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(len(values))); err != nil {
		return nil, err
	}
	for i, value := range values {
		valueBytes := []byte(value)
		if len(valueBytes) > math.MaxUint16 {
			return nil, errors.Errorf("ordered string %d length %d exceeds max %d", i, len(valueBytes), math.MaxUint16)
		}
		if err := binary.Write(&buf, binary.LittleEndian, uint16(len(valueBytes))); err != nil {
			return nil, err
		}
		if _, err := buf.Write(valueBytes); err != nil {
			return nil, err
		}
	}
	return &buf, nil
}

func encodeFrontCodedOrderedStrings(values []string, blockSize int) (*bytes.Buffer, error) {
	pc, err := NewPrefixCompressor(blockSize)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := buf.WriteByte(compactStringModeFrontCoded); err != nil {
		return nil, err
	}
	if err := WriteCompressedTerms(&buf, pc.CompressInOrder(values)); err != nil {
		return nil, err
	}
	return &buf, nil
}

func readOrderedStrings(r io.Reader, expectedCount uint32) ([]string, error) {
	var mode uint8
	if err := binary.Read(r, binary.LittleEndian, &mode); err != nil {
		return nil, wrapOrderedStringFormatError("read compact string mode", err)
	}

	switch mode {
	case compactStringModeRaw:
		return readRawOrderedStrings(r, expectedCount)
	case compactStringModeFrontCoded:
		blocks, err := ReadCompressedTerms(r)
		if err != nil {
			return nil, wrapOrderedStringFormatError("read front-coded ordered strings", err)
		}
		values := (&PrefixCompressor{}).Decompress(blocks)
		if uint32(len(values)) != expectedCount {
			return nil, errors.Wrapf(ErrInvalidFormat, "ordered string count mismatch: got %d want %d", len(values), expectedCount)
		}
		return values, nil
	default:
		return nil, errors.Wrapf(ErrInvalidFormat, "unknown compact string mode %d", mode)
	}
}

func readRawOrderedStrings(r io.Reader, expectedCount uint32) ([]string, error) {
	var count uint32
	if err := binary.Read(r, binary.LittleEndian, &count); err != nil {
		return nil, wrapOrderedStringFormatError("read raw ordered string count", err)
	}
	if count != expectedCount {
		return nil, errors.Wrapf(ErrInvalidFormat, "ordered string count mismatch: got %d want %d", count, expectedCount)
	}

	values := make([]string, expectedCount)
	for i := uint32(0); i < expectedCount; i++ {
		var valueLen uint16
		if err := binary.Read(r, binary.LittleEndian, &valueLen); err != nil {
			return nil, wrapOrderedStringFormatError("read raw ordered string length", err)
		}
		valueBytes := make([]byte, valueLen)
		if _, err := io.ReadFull(r, valueBytes); err != nil {
			return nil, wrapOrderedStringFormatError("read raw ordered string bytes", err)
		}
		values[i] = string(valueBytes)
	}
	return values, nil
}

func writePathDirectory(w io.Writer, idx *GINIndex) error {
	pathNames := make([]string, len(idx.PathDirectory))
	for i, entry := range idx.PathDirectory {
		pathNames[i] = entry.PathName
	}
	if err := writeOrderedStrings(w, pathNames, orderedStringBlockSize(idx)); err != nil {
		return err
	}

	for _, entry := range idx.PathDirectory {
		if err := binary.Write(w, binary.LittleEndian, entry.PathID); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, entry.ObservedTypes); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, entry.Cardinality); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, entry.Mode); err != nil {
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

	pathNames, err := readOrderedStrings(r, idx.Header.NumPaths)
	if err != nil {
		return err
	}

	for i := uint32(0); i < idx.Header.NumPaths; i++ {
		entry := PathEntry{PathName: pathNames[i]}
		if err := binary.Read(r, binary.LittleEndian, &entry.PathID); err != nil {
			return err
		}
		if err := binary.Read(r, binary.LittleEndian, &entry.ObservedTypes); err != nil {
			return err
		}
		if err := binary.Read(r, binary.LittleEndian, &entry.Cardinality); err != nil {
			return err
		}
		if err := binary.Read(r, binary.LittleEndian, &entry.Mode); err != nil {
			return err
		}
		if !entry.Mode.IsValid() {
			return errors.Wrapf(ErrInvalidFormat, "path %q has unknown mode %d", entry.PathName, entry.Mode)
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
	blockSize := orderedStringBlockSize(idx)
	for _, pathID := range sortedPathIDs(idx.StringIndexes) {
		si := idx.StringIndexes[pathID]
		if err := binary.Write(w, binary.LittleEndian, pathID); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint32(len(si.Terms))); err != nil {
			return err
		}
		if err := writeOrderedStrings(w, si.Terms, blockSize); err != nil {
			return err
		}
		for i := range si.Terms {
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
		if err := rejectDuplicateSectionPath("string index", idx.StringIndexes, pathID); err != nil {
			return err
		}
		var numTerms uint32
		if err := binary.Read(r, binary.LittleEndian, &numTerms); err != nil {
			return err
		}
		if numTerms > maxTermsPerPath {
			return errors.Wrapf(ErrInvalidFormat, "terms count %d for path %d exceeds max %d", numTerms, pathID, maxTermsPerPath)
		}
		terms, err := readOrderedStrings(r, numTerms)
		if err != nil {
			return err
		}
		si := &StringIndex{
			Terms:     terms,
			RGBitmaps: make([]*RGSet, numTerms),
		}
		for j := uint32(0); j < numTerms; j++ {
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

func writeAdaptiveStringIndexes(w io.Writer, idx *GINIndex) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(idx.AdaptiveStringIndexes))); err != nil {
		return err
	}
	blockSize := orderedStringBlockSize(idx)
	for _, pathID := range sortedPathIDs(idx.AdaptiveStringIndexes) {
		adaptive := idx.AdaptiveStringIndexes[pathID]
		if adaptive == nil {
			return errors.Wrapf(ErrInvalidFormat, "adaptive string index for path %d is nil", pathID)
		}
		bucketCount := len(adaptive.BucketRGBitmaps)
		if len(adaptive.Terms) > maxAdaptiveTermsPerPath {
			return errors.Wrapf(ErrInvalidFormat, "adaptive term count %d for path %d exceeds max %d", len(adaptive.Terms), pathID, maxAdaptiveTermsPerPath)
		}
		if len(adaptive.RGBitmaps) != len(adaptive.Terms) {
			return errors.Wrapf(ErrInvalidFormat, "adaptive rgbitmap count %d does not match term count %d for path %d", len(adaptive.RGBitmaps), len(adaptive.Terms), pathID)
		}
		if bucketCount <= 0 {
			return errors.Wrapf(ErrInvalidFormat, "adaptive bucket count %d for path %d must be greater than 0", bucketCount, pathID)
		}
		if bucketCount > maxAdaptiveBucketsPerPath {
			return errors.Wrapf(ErrInvalidFormat, "adaptive bucket count %d for path %d exceeds max %d", bucketCount, pathID, maxAdaptiveBucketsPerPath)
		}
		if !isPowerOfTwo(bucketCount) {
			return errors.Wrapf(ErrInvalidFormat, "adaptive bucket count %d for path %d must be a power of two", bucketCount, pathID)
		}
		if !sort.StringsAreSorted(adaptive.Terms) {
			return errors.Wrapf(ErrInvalidFormat, "adaptive terms for path %d must be sorted", pathID)
		}
		for i, rgSet := range adaptive.RGBitmaps {
			if rgSet == nil {
				return errors.Wrapf(ErrInvalidFormat, "adaptive term bitmap %d for path %d is nil", i, pathID)
			}
		}

		if err := binary.Write(w, binary.LittleEndian, pathID); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint32(len(adaptive.Terms))); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint32(bucketCount)); err != nil {
			return err
		}
		if err := writeOrderedStrings(w, adaptive.Terms, blockSize); err != nil {
			return err
		}
		for i := range adaptive.Terms {
			if err := writeRGSet(w, adaptive.RGBitmaps[i]); err != nil {
				return err
			}
		}
		for bucketID, rgSet := range adaptive.BucketRGBitmaps {
			if rgSet == nil {
				return errors.Wrapf(ErrInvalidFormat, "adaptive bucket %d for path %d is nil", bucketID, pathID)
			}
			if err := writeRGSet(w, rgSet); err != nil {
				return err
			}
		}
	}
	return nil
}

func readAdaptiveStringIndexes(r io.Reader, idx *GINIndex) error {
	var numPaths uint32
	if err := binary.Read(r, binary.LittleEndian, &numPaths); err != nil {
		return err
	}
	if numPaths > maxAdaptivePaths {
		return errors.Wrapf(ErrInvalidFormat, "adaptive path count %d exceeds max %d", numPaths, maxAdaptivePaths)
	}

	for i := uint32(0); i < numPaths; i++ {
		var pathID uint16
		if err := binary.Read(r, binary.LittleEndian, &pathID); err != nil {
			return err
		}
		if int(pathID) >= len(idx.PathDirectory) {
			return errors.Wrapf(ErrInvalidFormat, "adaptive string index path id %d out of range", pathID)
		}
		if idx.PathDirectory[pathID].Mode != PathModeAdaptiveHybrid {
			return errors.Wrapf(ErrInvalidFormat, "adaptive section path %d is missing adaptive mode", pathID)
		}
		if err := rejectDuplicateSectionPath("adaptive section", idx.AdaptiveStringIndexes, pathID); err != nil {
			return err
		}

		var numTerms uint32
		if err := binary.Read(r, binary.LittleEndian, &numTerms); err != nil {
			return err
		}
		if numTerms > maxAdaptiveTermsPerPath {
			return errors.Wrapf(ErrInvalidFormat, "adaptive term count %d for path %d exceeds max %d", numTerms, pathID, maxAdaptiveTermsPerPath)
		}

		var bucketCount uint32
		if err := binary.Read(r, binary.LittleEndian, &bucketCount); err != nil {
			return err
		}
		if bucketCount == 0 {
			return errors.Wrapf(ErrInvalidFormat, "adaptive bucket count for path %d must be greater than 0", pathID)
		}
		if bucketCount > maxAdaptiveBucketsPerPath {
			return errors.Wrapf(ErrInvalidFormat, "adaptive bucket count %d for path %d exceeds max %d", bucketCount, pathID, maxAdaptiveBucketsPerPath)
		}
		if !isPowerOfTwo(int(bucketCount)) {
			return errors.Wrapf(ErrInvalidFormat, "adaptive bucket count %d for path %d must be a power of two", bucketCount, pathID)
		}

		terms, err := readOrderedStrings(r, numTerms)
		if err != nil {
			return err
		}
		rgBitmaps := make([]*RGSet, numTerms)
		bucketBitmaps := make([]*RGSet, bucketCount)
		for j := uint32(0); j < numTerms; j++ {
			rgSet, err := readRGSet(r, idx.Header.NumRowGroups)
			if err != nil {
				return err
			}
			rgBitmaps[j] = rgSet
		}
		for bucketID := uint32(0); bucketID < bucketCount; bucketID++ {
			rgSet, err := readRGSet(r, idx.Header.NumRowGroups)
			if err != nil {
				return err
			}
			bucketBitmaps[bucketID] = rgSet
		}

		adaptive, err := NewAdaptiveStringIndex(terms, rgBitmaps, bucketBitmaps)
		if err != nil {
			return errors.Wrapf(ErrInvalidFormat, "adaptive path %d invalid: %v", pathID, err)
		}

		idx.AdaptiveStringIndexes[pathID] = adaptive
		idx.PathDirectory[pathID].AdaptivePromotedTerms = uint16(numTerms)
		idx.PathDirectory[pathID].AdaptiveBucketCount = uint16(bucketCount)
	}

	for i := range idx.PathDirectory {
		entry := &idx.PathDirectory[i]
		if entry.Mode != PathModeAdaptiveHybrid {
			continue
		}
		if _, ok := idx.AdaptiveStringIndexes[entry.PathID]; !ok {
			return errors.Wrapf(ErrInvalidFormat, "adaptive path %d missing adaptive section", entry.PathID)
		}
	}

	return nil
}

func writeStringLengthIndexes(w io.Writer, idx *GINIndex) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(idx.StringLengthIndexes))); err != nil {
		return err
	}
	for _, pathID := range sortedPathIDs(idx.StringLengthIndexes) {
		sli := idx.StringLengthIndexes[pathID]
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
		if err := rejectDuplicateSectionPath("string length index", idx.StringLengthIndexes, pathID); err != nil {
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
	for _, pathID := range sortedPathIDs(idx.NumericIndexes) {
		ni := idx.NumericIndexes[pathID]
		if err := binary.Write(w, binary.LittleEndian, pathID); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, ni.ValueType); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, ni.IntGlobalMin); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, ni.IntGlobalMax); err != nil {
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
			if err := binary.Write(w, binary.LittleEndian, stat.IntMin); err != nil {
				return err
			}
			if err := binary.Write(w, binary.LittleEndian, stat.IntMax); err != nil {
				return err
			}
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
		if err := rejectDuplicateSectionPath("numeric index", idx.NumericIndexes, pathID); err != nil {
			return err
		}
		ni := &NumericIndex{}
		if err := binary.Read(r, binary.LittleEndian, &ni.ValueType); err != nil {
			return err
		}
		if err := binary.Read(r, binary.LittleEndian, &ni.IntGlobalMin); err != nil {
			return err
		}
		if err := binary.Read(r, binary.LittleEndian, &ni.IntGlobalMax); err != nil {
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
			if err := binary.Read(r, binary.LittleEndian, &ni.RGStats[j].IntMin); err != nil {
				return err
			}
			if err := binary.Read(r, binary.LittleEndian, &ni.RGStats[j].IntMax); err != nil {
				return err
			}
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
			ni.RGStats[j].Min = math.Float64frombits(minBits)
			ni.RGStats[j].Max = math.Float64frombits(maxBits)
			ni.RGStats[j].HasValue = hasValue != 0
		}
		idx.NumericIndexes[pathID] = ni
	}
	return nil
}

func writeNullIndexes(w io.Writer, idx *GINIndex) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(idx.NullIndexes))); err != nil {
		return err
	}
	for _, pathID := range sortedPathIDs(idx.NullIndexes) {
		ni := idx.NullIndexes[pathID]
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
		if err := rejectDuplicateSectionPath("null index", idx.NullIndexes, pathID); err != nil {
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
	for _, pathID := range sortedPathIDs(idx.TrigramIndexes) {
		ti := idx.TrigramIndexes[pathID]
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
		trigrams := make([]string, 0, len(ti.Trigrams))
		for trigram := range ti.Trigrams {
			trigrams = append(trigrams, trigram)
		}
		sort.Strings(trigrams)
		for _, trigram := range trigrams {
			rgSet := ti.Trigrams[trigram]
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
		if err := rejectDuplicateSectionPath("trigram index", idx.TrigramIndexes, pathID); err != nil {
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
	for _, pathID := range sortedPathIDs(idx.PathCardinality) {
		hll := idx.PathCardinality[pathID]
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
		if err := rejectDuplicateSectionPath("hyperloglog", idx.PathCardinality, pathID); err != nil {
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

	transformerPaths := make([]string, 0, len(cfg.representationSpecs))
	for path := range cfg.representationSpecs {
		transformerPaths = append(transformerPaths, path)
	}
	sort.Strings(transformerPaths)
	for _, path := range transformerPaths {
		representations := append([]RepresentationSpec(nil), cfg.representationSpecs[path]...)
		sort.Slice(representations, func(i, j int) bool {
			return representations[i].Alias < representations[j].Alias
		})
		for _, representation := range representations {
			if !representation.Serializable {
				continue
			}
			transformer := representation.Transformer
			transformer.FailureMode = normalizeTransformerFailureMode(transformer.FailureMode)
			sc.Transformers = append(sc.Transformers, transformer)
		}
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
		if stderrors.Is(err, io.EOF) || stderrors.Is(err, io.ErrUnexpectedEOF) {
			return nil, errors.Wrap(ErrInvalidFormat, "missing config length")
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
		if stderrors.Is(err, io.EOF) || stderrors.Is(err, io.ErrUnexpectedEOF) {
			return nil, errors.Wrap(ErrInvalidFormat, "truncated config payload")
		}
		return nil, err
	}

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
		for _, spec := range sc.Transformers {
			canonicalPath, err := canonicalizeSupportedPath(spec.Path)
			if err != nil {
				return nil, errors.Wrapf(err, "canonicalize transformer path %q", spec.Path)
			}
			alias := spec.Alias
			if alias == "" {
				alias = spec.Name
			}
			if alias == "" {
				return nil, errors.Wrapf(ErrInvalidFormat, "missing transformer alias for path %q", spec.Path)
			}
			targetPath := spec.TargetPath
			if targetPath == "" {
				targetPath = representationTargetPath(canonicalPath, alias)
			}
			if targetPath != representationTargetPath(canonicalPath, alias) {
				return nil, errors.Wrapf(ErrInvalidFormat, "transformer target path %q for %s alias %q does not match %q", targetPath, canonicalPath, alias, representationTargetPath(canonicalPath, alias))
			}

			spec.Path = canonicalPath
			spec.Alias = alias
			spec.TargetPath = targetPath
			spec.FailureMode = normalizeTransformerFailureMode(spec.FailureMode)
			fn, err := ReconstructTransformer(spec.ID, spec.Params)
			if err != nil {
				return nil, errors.Wrapf(err, "reconstruct transformer for path %s", spec.Path)
			}
			if err := cfg.addRepresentation(canonicalPath, alias, spec, true, spec.FailureMode, fn); err != nil {
				return nil, errors.Wrapf(ErrInvalidFormat, "register transformer for %s alias %q: %v", canonicalPath, alias, err)
			}
		}
	}

	if err := cfg.validate(); err != nil {
		return nil, errors.Wrap(err, "validate config")
	}

	return cfg, nil
}

func writeRepresentations(w io.Writer, idx *GINIndex) error {
	representations := idx.representations
	if representations == nil {
		// Fallback for hand-constructed GINIndex not produced by Finalize() or Decode().
		representations = collectRepresentationsFromConfig(idx.Config)
	}
	if len(representations) == 0 {
		return binary.Write(w, binary.LittleEndian, uint32(0))
	}

	for _, representation := range representations {
		if !representation.Serializable {
			return errors.Errorf("representation %s on %s is not serializable", representation.Alias, representation.SourcePath)
		}
	}

	data, err := json.Marshal(representations)
	if err != nil {
		return errors.Wrap(err, "marshal representations")
	}
	if len(data) > maxRepresentationSectionSize {
		return errors.Wrapf(ErrInvalidFormat, "representation metadata size %d exceeds max %d", len(data), maxRepresentationSectionSize)
	}

	if err := binary.Write(w, binary.LittleEndian, uint32(len(data))); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func readRepresentations(r io.Reader) ([]RepresentationSpec, error) {
	var sectionLen uint32
	if err := binary.Read(r, binary.LittleEndian, &sectionLen); err != nil {
		if stderrors.Is(err, io.EOF) || stderrors.Is(err, io.ErrUnexpectedEOF) {
			return nil, errors.Wrap(ErrInvalidFormat, "missing representation metadata length")
		}
		return nil, err
	}

	if sectionLen == 0 {
		return []RepresentationSpec{}, nil
	}
	if sectionLen > maxRepresentationSectionSize {
		return nil, errors.Wrapf(ErrInvalidFormat, "representation metadata size %d exceeds max %d", sectionLen, maxRepresentationSectionSize)
	}

	data := make([]byte, sectionLen)
	if _, err := io.ReadFull(r, data); err != nil {
		if stderrors.Is(err, io.EOF) || stderrors.Is(err, io.ErrUnexpectedEOF) {
			return nil, errors.Wrap(ErrInvalidFormat, "truncated representation metadata payload")
		}
		return nil, err
	}

	var representations []RepresentationSpec
	if err := json.Unmarshal(data, &representations); err != nil {
		return nil, errors.Wrap(err, "unmarshal representations")
	}

	seen := make(map[string]map[string]struct{})
	for i := range representations {
		representation := &representations[i]
		canonicalPath, err := canonicalizeSupportedPath(representation.SourcePath)
		if err != nil {
			return nil, errors.Wrapf(ErrInvalidFormat, "canonicalize representation source path %q: %v", representation.SourcePath, err)
		}
		if err := validateRepresentationAlias(representation.Alias); err != nil {
			return nil, errors.Wrapf(ErrInvalidFormat, "invalid representation alias for %s: %v", canonicalPath, err)
		}
		if !representation.Serializable {
			return nil, errors.Wrapf(ErrInvalidFormat, "representation %s on %s is not serializable", representation.Alias, canonicalPath)
		}
		wantTarget := representationTargetPath(canonicalPath, representation.Alias)
		if representation.TargetPath != wantTarget {
			return nil, errors.Wrapf(ErrInvalidFormat, "representation target path %q for %s alias %q does not match %q", representation.TargetPath, canonicalPath, representation.Alias, wantTarget)
		}
		if representation.Transformer.Name == "" {
			return nil, errors.Wrapf(ErrInvalidFormat, "missing representation transformer name for %s alias %q", canonicalPath, representation.Alias)
		}
		if representation.Transformer.Path != canonicalPath {
			return nil, errors.Wrapf(ErrInvalidFormat, "representation transformer path %q for %s alias %q does not match source", representation.Transformer.Path, canonicalPath, representation.Alias)
		}
		representation.Transformer.FailureMode = normalizeTransformerFailureMode(representation.Transformer.FailureMode)
		if err := validateTransformerFailureMode(representation.Transformer.FailureMode); err != nil {
			return nil, errors.Wrapf(ErrInvalidFormat, "invalid representation failure mode for %s alias %q: %v", canonicalPath, representation.Alias, err)
		}

		if seen[canonicalPath] == nil {
			seen[canonicalPath] = make(map[string]struct{})
		}
		if _, exists := seen[canonicalPath][representation.Alias]; exists {
			return nil, errors.Wrapf(ErrInvalidFormat, "duplicate representation alias %q for %s", representation.Alias, canonicalPath)
		}
		seen[canonicalPath][representation.Alias] = struct{}{}
		representation.SourcePath = canonicalPath
	}

	return representations, nil
}
