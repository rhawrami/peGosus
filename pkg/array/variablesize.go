package array

// VariableSizeArray represents an array with variable size data, that meaning
// data where elements can have variable width. Allowed types here include
// string and binary types.
type VariableSizeArray struct {
	T ArrayType         // array type
	O uint64            // offset (0 if no offset)
	L uint64            // length
	N uint64            // null count
	S *CachedStatistics // cached statistics
	V *ValidityBitmap   // validity bitmap (nil if no nulls)
	D []byte            // data
	F []uint32          // data offsets (will come back to making this dynamic b/w 32/64)
}

func (a *VariableSizeArray) Type() ArrayType { return a.T }

func (a *VariableSizeArray) Offset() uint64 { return a.O }

func (a *VariableSizeArray) Len() uint64 { return a.L }

func (a *VariableSizeArray) Nulls() uint64 { return a.N }

func (a *VariableSizeArray) CachedStats() *CachedStatistics { return a.S }

func (a *VariableSizeArray) VBM() *ValidityBitmap { return a.V }

func (a *VariableSizeArray) Data() []byte { return a.D }

func (a *VariableSizeArray) DataOffsets() []uint32 { return a.F }

// come back to this function
func (a *VariableSizeArray) IsConsistent() bool {
	// offset out of bounds
	if a.O >= a.L {
		return false
	}
	// array has nulls, but nil VBM pointer
	if a.N > 0 && a.V == nil {
		return false
	}
	return true
}
