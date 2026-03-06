package array

// Array represents a given array and its metadata.
type Array interface {
	// Type returns the ArrayType of a given array.
	Type() ArrayType
	// Offset returns the offset of an array (0 if no offset).
	Offset() uint64
	// Len returns the element length fo an array.
	Len() uint64
	// Nulls returns the null count of an array.
	Nulls() uint64
	// VBM returns the validity bitmap of an array; nil if no nulls
	VBM() *ValidityBitmap
	// Data returns an array's actual data.
	Data() []byte
	// Data offsets returns the element offsets of an array; always
	// returns nil for fixed size data.
	DataOffsets() []byte
	// IsConsistent returns true if there are no detected inconsistencies
	// in the array; for instance, Offset >= Len would cause IsConsistent()
	// to return false.
	IsConsistent() bool
}
