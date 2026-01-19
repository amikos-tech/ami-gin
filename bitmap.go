package gin

import (
	"github.com/RoaringBitmap/roaring/v2"
	"github.com/pkg/errors"
)

type RGSet struct {
	bitmap *roaring.Bitmap
	NumRGs int
}

type RGSetOption func(*RGSet) error

func NewRGSet(numRGs int, opts ...RGSetOption) (*RGSet, error) {
	if numRGs <= 0 {
		return nil, errors.New("numRGs must be greater than 0")
	}
	rs := &RGSet{
		bitmap: roaring.New(),
		NumRGs: numRGs,
	}
	for _, opt := range opts {
		if err := opt(rs); err != nil {
			return nil, err
		}
	}
	return rs, nil
}

func MustNewRGSet(numRGs int, opts ...RGSetOption) *RGSet {
	rs, err := NewRGSet(numRGs, opts...)
	if err != nil {
		panic(err)
	}
	return rs
}

func RGSetFromRoaring(bitmap *roaring.Bitmap, numRGs int) *RGSet {
	return &RGSet{
		bitmap: bitmap,
		NumRGs: numRGs,
	}
}

func (rs *RGSet) Set(rgID int) {
	if rgID < 0 || rgID >= rs.NumRGs {
		return
	}
	rs.bitmap.Add(uint32(rgID))
}

func (rs *RGSet) Clear(rgID int) {
	if rgID < 0 || rgID >= rs.NumRGs {
		return
	}
	rs.bitmap.Remove(uint32(rgID))
}

func (rs *RGSet) IsSet(rgID int) bool {
	if rgID < 0 || rgID >= rs.NumRGs {
		return false
	}
	return rs.bitmap.Contains(uint32(rgID))
}

func (rs *RGSet) Intersect(other *RGSet) *RGSet {
	result := rs.bitmap.Clone()
	result.And(other.bitmap)
	return &RGSet{
		bitmap: result,
		NumRGs: rs.NumRGs,
	}
}

func (rs *RGSet) Union(other *RGSet) *RGSet {
	result := rs.bitmap.Clone()
	result.Or(other.bitmap)
	maxRGs := rs.NumRGs
	if other.NumRGs > maxRGs {
		maxRGs = other.NumRGs
	}
	return &RGSet{
		bitmap: result,
		NumRGs: maxRGs,
	}
}

func (rs *RGSet) All() *RGSet {
	result := roaring.New()
	result.AddRange(0, uint64(rs.NumRGs))
	return &RGSet{
		bitmap: result,
		NumRGs: rs.NumRGs,
	}
}

func AllRGs(numRGs int) *RGSet {
	bitmap := roaring.New()
	bitmap.AddRange(0, uint64(numRGs))
	return &RGSet{
		bitmap: bitmap,
		NumRGs: numRGs,
	}
}

func NoRGs(numRGs int) *RGSet {
	return MustNewRGSet(numRGs)
}

func (rs *RGSet) IsEmpty() bool {
	return rs.bitmap.IsEmpty()
}

func (rs *RGSet) Count() int {
	return int(rs.bitmap.GetCardinality())
}

func (rs *RGSet) ToSlice() []int {
	vals := rs.bitmap.ToArray()
	result := make([]int, len(vals))
	for i, v := range vals {
		result[i] = int(v)
	}
	return result
}

func (rs *RGSet) Clone() *RGSet {
	return &RGSet{
		bitmap: rs.bitmap.Clone(),
		NumRGs: rs.NumRGs,
	}
}

func (rs *RGSet) Invert() *RGSet {
	all := roaring.New()
	all.AddRange(0, uint64(rs.NumRGs))
	all.AndNot(rs.bitmap)
	return &RGSet{
		bitmap: all,
		NumRGs: rs.NumRGs,
	}
}

func (rs *RGSet) Roaring() *roaring.Bitmap {
	return rs.bitmap
}
