package array

type FixedSizeArray struct {
	T ArrayType         // array type
	O uint64            // offset (0 if no offset)
	L uint64            // length
	N uint64            // null count
	S *CachedStatistics // cached statistics
	V *ValidityBitmap   // validity bitmap (nil if no nulls)
	D []byte            // data
}

func (a *FixedSizeArray) Type() ArrayType { return a.T }

func (a *FixedSizeArray) Offset() uint64 { return a.O }

func (a *FixedSizeArray) Len() uint64 { return a.L }

func (a *FixedSizeArray) Nulls() uint64 { return a.N }

func (a *FixedSizeArray) CachedStats() *CachedStatistics { return a.S }

func (a *FixedSizeArray) VBM() *ValidityBitmap { return a.V }

func (a *FixedSizeArray) Data() []byte { return a.D }

func (a *FixedSizeArray) DataOffsets() []byte { return nil }

// come back to this function
func (a *FixedSizeArray) IsConsistent() bool {
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
