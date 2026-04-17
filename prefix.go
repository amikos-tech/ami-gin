package gin

import (
	"encoding/binary"
	"io"
	"math"
	"sort"

	"github.com/pkg/errors"
)

// PrefixCompressor implements front-coding compression for sorted string lists.
// Each string is stored as: shared prefix length + suffix.
// This works well for sorted terms that share common prefixes.
type PrefixCompressor struct {
	blockSize int // number of terms per block (first term in block stored fully)
}

type PrefixCompressorOption func(*PrefixCompressor) error

func NewPrefixCompressor(blockSize int, opts ...PrefixCompressorOption) (*PrefixCompressor, error) {
	if blockSize < 1 {
		return nil, errors.New("blockSize must be at least 1")
	}
	// WriteCompressedTerms encodes block entry counts as uint16, so block
	// sizes larger than math.MaxUint16 silently overflow on the wire.
	if blockSize > math.MaxUint16 {
		return nil, errors.Errorf("blockSize %d exceeds max %d", blockSize, math.MaxUint16)
	}
	pc := &PrefixCompressor{blockSize: blockSize}
	for _, opt := range opts {
		if err := opt(pc); err != nil {
			return nil, err
		}
	}
	return pc, nil
}

func MustNewPrefixCompressor(blockSize int, opts ...PrefixCompressorOption) *PrefixCompressor {
	pc, err := NewPrefixCompressor(blockSize, opts...)
	if err != nil {
		panic(err)
	}
	return pc
}

type CompressedTermBlock struct {
	FirstTerm string
	Entries   []PrefixEntry
}

type PrefixEntry struct {
	PrefixLen uint16
	Suffix    string
}

func (pc *PrefixCompressor) Compress(terms []string) []CompressedTermBlock {
	if len(terms) == 0 {
		return nil
	}

	sorted := make([]string, len(terms))
	copy(sorted, terms)
	sort.Strings(sorted)

	return pc.CompressInOrder(sorted)
}

// CompressInOrder applies front coding without reordering the caller-provided
// terms. Use this for serialized sections whose bitmap/layout metadata must
// stay aligned with the original slice position.
func (pc *PrefixCompressor) CompressInOrder(terms []string) []CompressedTermBlock {
	if len(terms) == 0 {
		return nil
	}

	var blocks []CompressedTermBlock
	for i := 0; i < len(terms); i += pc.blockSize {
		end := i + pc.blockSize
		if end > len(terms) {
			end = len(terms)
		}
		block := pc.compressBlock(terms[i:end])
		blocks = append(blocks, block)
	}

	return blocks
}

func (pc *PrefixCompressor) compressBlock(terms []string) CompressedTermBlock {
	block := CompressedTermBlock{
		FirstTerm: terms[0],
		Entries:   make([]PrefixEntry, len(terms)-1),
	}

	prev := terms[0]
	for i := 1; i < len(terms); i++ {
		current := terms[i]
		prefixLen := commonPrefixLen(prev, current)
		block.Entries[i-1] = PrefixEntry{
			PrefixLen: uint16(prefixLen),
			Suffix:    current[prefixLen:],
		}
		prev = current
	}

	return block
}

func (pc *PrefixCompressor) Decompress(blocks []CompressedTermBlock) []string {
	var result []string

	for _, block := range blocks {
		result = append(result, block.FirstTerm)
		prev := block.FirstTerm

		for _, entry := range block.Entries {
			term := prev[:entry.PrefixLen] + entry.Suffix
			result = append(result, term)
			prev = term
		}
	}

	return result
}

func commonPrefixLen(a, b string) int {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return minLen
}

func (pc *PrefixCompressor) BlockSize() int {
	return pc.blockSize
}

func WriteCompressedTerms(w io.Writer, blocks []CompressedTermBlock) error {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(blocks))); err != nil {
		return err
	}

	for _, block := range blocks {
		firstBytes := []byte(block.FirstTerm)
		if err := binary.Write(w, binary.LittleEndian, uint16(len(firstBytes))); err != nil {
			return err
		}
		if _, err := w.Write(firstBytes); err != nil {
			return err
		}

		if err := binary.Write(w, binary.LittleEndian, uint16(len(block.Entries))); err != nil {
			return err
		}

		for _, entry := range block.Entries {
			if err := binary.Write(w, binary.LittleEndian, entry.PrefixLen); err != nil {
				return err
			}
			suffixBytes := []byte(entry.Suffix)
			if err := binary.Write(w, binary.LittleEndian, uint16(len(suffixBytes))); err != nil {
				return err
			}
			if _, err := w.Write(suffixBytes); err != nil {
				return err
			}
		}
	}

	return nil
}

func readCompressedTerms(r io.Reader) ([]CompressedTermBlock, error) {
	var numBlocks uint32
	if err := binary.Read(r, binary.LittleEndian, &numBlocks); err != nil {
		return nil, err
	}

	blocks := make([]CompressedTermBlock, numBlocks)
	for i := uint32(0); i < numBlocks; i++ {
		var firstLen uint16
		if err := binary.Read(r, binary.LittleEndian, &firstLen); err != nil {
			return nil, err
		}
		firstBytes := make([]byte, firstLen)
		if _, err := io.ReadFull(r, firstBytes); err != nil {
			return nil, err
		}
		blocks[i].FirstTerm = string(firstBytes)

		var numEntries uint16
		if err := binary.Read(r, binary.LittleEndian, &numEntries); err != nil {
			return nil, err
		}
		blocks[i].Entries = make([]PrefixEntry, numEntries)

		for j := uint16(0); j < numEntries; j++ {
			if err := binary.Read(r, binary.LittleEndian, &blocks[i].Entries[j].PrefixLen); err != nil {
				return nil, err
			}
			var suffixLen uint16
			if err := binary.Read(r, binary.LittleEndian, &suffixLen); err != nil {
				return nil, err
			}
			suffixBytes := make([]byte, suffixLen)
			if _, err := io.ReadFull(r, suffixBytes); err != nil {
				return nil, err
			}
			blocks[i].Entries[j].Suffix = string(suffixBytes)
		}
	}

	return blocks, nil
}

// CompressionRatio returns the compression ratio for a set of terms.
// Returns (compressed size, original size, ratio).
func CompressionStats(terms []string) (compressed, original int, ratio float64) {
	for _, t := range terms {
		original += len(t)
	}

	pc := MustNewPrefixCompressor(defaultPrefixBlockSize)
	blocks := pc.Compress(terms)

	for _, block := range blocks {
		compressed += len(block.FirstTerm)
		for _, entry := range block.Entries {
			compressed += 2 + len(entry.Suffix) // 2 bytes for prefix len
		}
	}

	if original > 0 {
		ratio = float64(compressed) / float64(original)
	}
	return
}
