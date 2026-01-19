package gin

// DocID represents an external document identifier.
type DocID uint64

// DocIDCodec encodes/decodes composite information into a single DocID.
type DocIDCodec interface {
	Encode(indices ...int) DocID
	Decode(docID DocID) []int
	Name() string
}

// IdentityCodec treats the position as the DocID (1:1 mapping).
type IdentityCodec struct{}

func NewIdentityCodec() *IdentityCodec {
	return &IdentityCodec{}
}

func (c *IdentityCodec) Encode(indices ...int) DocID {
	if len(indices) == 0 {
		return 0
	}
	return DocID(indices[0])
}

func (c *IdentityCodec) Decode(docID DocID) []int {
	return []int{int(docID)}
}

func (c *IdentityCodec) Name() string {
	return "identity"
}

// RowGroupCodec encodes file index and row group index into a DocID.
// Layout: DocID = fileIndex * rowGroupsPerFile + rgIndex
type RowGroupCodec struct {
	rowGroupsPerFile int
}

func NewRowGroupCodec(rowGroupsPerFile int) *RowGroupCodec {
	if rowGroupsPerFile <= 0 {
		rowGroupsPerFile = 1
	}
	return &RowGroupCodec{rowGroupsPerFile: rowGroupsPerFile}
}

func (c *RowGroupCodec) Encode(indices ...int) DocID {
	if len(indices) < 2 {
		if len(indices) == 1 {
			return DocID(indices[0])
		}
		return 0
	}
	fileIndex, rgIndex := indices[0], indices[1]
	return DocID(fileIndex*c.rowGroupsPerFile + rgIndex)
}

func (c *RowGroupCodec) Decode(docID DocID) []int {
	id := int(docID)
	fileIndex := id / c.rowGroupsPerFile
	rgIndex := id % c.rowGroupsPerFile
	return []int{fileIndex, rgIndex}
}

func (c *RowGroupCodec) Name() string {
	return "rowgroup"
}

func (c *RowGroupCodec) RowGroupsPerFile() int {
	return c.rowGroupsPerFile
}
