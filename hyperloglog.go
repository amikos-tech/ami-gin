package gin

import (
	"math"

	"github.com/cespare/xxhash/v2"
	"github.com/pkg/errors"
)

// HyperLogLog implements the HyperLogLog algorithm for cardinality estimation.
// It uses 2^precision registers to estimate the number of distinct elements.
type HyperLogLog struct {
	registers []uint8
	precision uint8 // number of bits for register index (4-16)
	m         uint32
	alpha     float64
}

type HyperLogLogOption func(*HyperLogLog) error

// NewHyperLogLog creates a new HyperLogLog with the given precision.
// Precision must be between 4 and 16. Higher precision = more accuracy but more memory.
// Memory usage: 2^precision bytes.
// Standard error: 1.04 / sqrt(m) where m = 2^precision
func NewHyperLogLog(precision uint8, opts ...HyperLogLogOption) (*HyperLogLog, error) {
	if precision < 4 || precision > 16 {
		return nil, errors.Errorf("precision must be between 4 and 16, got %d", precision)
	}

	m := uint32(1) << precision
	alpha := getAlpha(m)

	hll := &HyperLogLog{
		registers: make([]uint8, m),
		precision: precision,
		m:         m,
		alpha:     alpha,
	}
	for _, opt := range opts {
		if err := opt(hll); err != nil {
			return nil, err
		}
	}
	return hll, nil
}

func MustNewHyperLogLog(precision uint8, opts ...HyperLogLogOption) *HyperLogLog {
	hll, err := NewHyperLogLog(precision, opts...)
	if err != nil {
		panic(err)
	}
	return hll
}

func getAlpha(m uint32) float64 {
	switch m {
	case 16:
		return 0.673
	case 32:
		return 0.697
	case 64:
		return 0.709
	default:
		return 0.7213 / (1 + 1.079/float64(m))
	}
}

func (hll *HyperLogLog) Add(data []byte) {
	hash := xxhash.Sum64(data)
	hll.addHash(hash)
}

func (hll *HyperLogLog) AddString(s string) {
	hll.Add([]byte(s))
}

func (hll *HyperLogLog) addHash(hash uint64) {
	idx := hash >> (64 - hll.precision)
	w := hash<<hll.precision | (1 << (hll.precision - 1))
	rho := leadingZeros(w) + 1

	if rho > hll.registers[idx] {
		hll.registers[idx] = rho
	}
}

func leadingZeros(x uint64) uint8 {
	if x == 0 {
		return 64
	}
	var n uint8
	if x&0xFFFFFFFF00000000 == 0 {
		n += 32
		x <<= 32
	}
	if x&0xFFFF000000000000 == 0 {
		n += 16
		x <<= 16
	}
	if x&0xFF00000000000000 == 0 {
		n += 8
		x <<= 8
	}
	if x&0xF000000000000000 == 0 {
		n += 4
		x <<= 4
	}
	if x&0xC000000000000000 == 0 {
		n += 2
		x <<= 2
	}
	if x&0x8000000000000000 == 0 {
		n++
	}
	return n
}

func (hll *HyperLogLog) Estimate() uint64 {
	sum := 0.0
	zeros := 0

	for _, val := range hll.registers {
		sum += math.Pow(2, -float64(val))
		if val == 0 {
			zeros++
		}
	}

	estimate := hll.alpha * float64(hll.m) * float64(hll.m) / sum

	// Small range correction
	if estimate <= 2.5*float64(hll.m) && zeros > 0 {
		estimate = float64(hll.m) * math.Log(float64(hll.m)/float64(zeros))
	}

	// Large range correction (for 32-bit hashes, but we use 64-bit)
	// Not needed for 64-bit hashes

	return uint64(estimate + 0.5)
}

func (hll *HyperLogLog) Merge(other *HyperLogLog) {
	if hll.precision != other.precision {
		return
	}
	for i := range hll.registers {
		if other.registers[i] > hll.registers[i] {
			hll.registers[i] = other.registers[i]
		}
	}
}

func (hll *HyperLogLog) Precision() uint8 {
	return hll.precision
}

func (hll *HyperLogLog) Registers() []uint8 {
	return hll.registers
}

func HyperLogLogFromRegisters(registers []uint8, precision uint8) *HyperLogLog {
	m := uint32(1) << precision
	return &HyperLogLog{
		registers: registers,
		precision: precision,
		m:         m,
		alpha:     getAlpha(m),
	}
}

func (hll *HyperLogLog) Clear() {
	for i := range hll.registers {
		hll.registers[i] = 0
	}
}

func (hll *HyperLogLog) Clone() *HyperLogLog {
	registers := make([]uint8, len(hll.registers))
	copy(registers, hll.registers)
	return &HyperLogLog{
		registers: registers,
		precision: hll.precision,
		m:         hll.m,
		alpha:     hll.alpha,
	}
}
