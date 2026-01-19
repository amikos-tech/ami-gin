package gin

import (
	"github.com/cespare/xxhash/v2"
	"github.com/pkg/errors"
)

type BloomFilter struct {
	bits      []uint64
	numBits   uint32
	numHashes uint8
}

type BloomFilterOption func(*BloomFilter) error

func NewBloomFilter(numBits uint32, numHashes uint8, opts ...BloomFilterOption) (*BloomFilter, error) {
	if numBits == 0 {
		return nil, errors.New("numBits must be greater than 0")
	}
	if numHashes == 0 {
		return nil, errors.New("numHashes must be greater than 0")
	}
	numWords := (numBits + 63) / 64
	bf := &BloomFilter{
		bits:      make([]uint64, numWords),
		numBits:   numBits,
		numHashes: numHashes,
	}
	for _, opt := range opts {
		if err := opt(bf); err != nil {
			return nil, err
		}
	}
	return bf, nil
}

func MustNewBloomFilter(numBits uint32, numHashes uint8, opts ...BloomFilterOption) *BloomFilter {
	bf, err := NewBloomFilter(numBits, numHashes, opts...)
	if err != nil {
		panic(err)
	}
	return bf
}

func (bf *BloomFilter) Add(data []byte) {
	h1 := xxhash.Sum64(data)
	h2 := xxhash.Sum64(append(data, 0xFF))

	for i := uint8(0); i < bf.numHashes; i++ {
		pos := (h1 + uint64(i)*h2) % uint64(bf.numBits)
		bf.bits[pos/64] |= 1 << (pos % 64)
	}
}

func (bf *BloomFilter) AddString(s string) {
	bf.Add([]byte(s))
}

func (bf *BloomFilter) MayContain(data []byte) bool {
	h1 := xxhash.Sum64(data)
	h2 := xxhash.Sum64(append(data, 0xFF))

	for i := uint8(0); i < bf.numHashes; i++ {
		pos := (h1 + uint64(i)*h2) % uint64(bf.numBits)
		if bf.bits[pos/64]&(1<<(pos%64)) == 0 {
			return false
		}
	}
	return true
}

func (bf *BloomFilter) MayContainString(s string) bool {
	return bf.MayContain([]byte(s))
}

func (bf *BloomFilter) NumBits() uint32 {
	return bf.numBits
}

func (bf *BloomFilter) NumHashes() uint8 {
	return bf.numHashes
}

func (bf *BloomFilter) Bits() []uint64 {
	return bf.bits
}

func BloomFilterFromBits(bits []uint64, numBits uint32, numHashes uint8) *BloomFilter {
	return &BloomFilter{
		bits:      bits,
		numBits:   numBits,
		numHashes: numHashes,
	}
}
