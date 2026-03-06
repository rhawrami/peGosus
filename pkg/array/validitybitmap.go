package array

// ValidityBitmap is an Arrow-style bitmap representing the validity
// of any given element in an array. Elements that are not null are stored
// as 1, with null elements stored as 0.
type ValidityBitmap struct {
	Len   uint64 // element length represented by the bitmap
	Nulls uint64 // null count
	Data  []byte // data
}

// IsNA returns true if the element is null
func (v *ValidityBitmap) IsNA(i int) bool {
	return (v.Data[(i)/8]>>byte((i)%8))&byte(1) == 0
}

// NotNA returns true if the element is not null
func (v *ValidityBitmap) NotNA(i int) bool {
	return (v.Data[(i)/8]>>byte((i)%8))&byte(1) == 1
}

// IsNABinary returns 1 if the element is null
func (v *ValidityBitmap) IsNABinary(i int) byte {
	return (v.Data[(i)/8]>>byte((i)%8))&byte(1) ^ byte(1)
}

// NotNABinary returns 1 if the element is not null
func (v *ValidityBitmap) NotNABinary(i int) byte {
	return (v.Data[(i)/8] >> byte((i)%8)) & byte(1)
}
