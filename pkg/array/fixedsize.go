package array

// FixedSizeArray represents an array with fixed size data, that meaning
// data where all elements have the same width. Allowed types here include
// all numeric types, date types and boolean types as well.
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
