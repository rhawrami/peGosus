package store

import (
	"unsafe"
)

// MakeBatchValidityFromBitmap returns a BatchValidity from a bitmap.
func MakeBatchValidityFromBitmap(m *BitMap) *BatchValidity {
	return &BatchValidity{
		vec:      m,
		isBitMap: true,
	}
}

// MakeBatchValidityFromSelVec returns a BatchValidity from a selection vector.
func MakeBatchValidityFromSelVec(s *SelVec) *BatchValidity {
	return &BatchValidity{
		vec:      (*BitMap)(unsafe.Pointer(s)),
		isBitMap: false,
	}
}

// BatchValidity represents the elements in a batch that are valid;
// `vec` can be of type BitMap or SelVec.
type BatchValidity struct {
	vec      *BitMap
	isBitMap bool
}

// AsBitMap returns the BatchValidity as a validity bitmap.
func (v *BatchValidity) AsBitMap() *BitMap {
	return v.vec
}

// AsSelVec returns the BatchValidity as a selection vector.
func (v *BatchValidity) AsSelVec() *SelVec {
	return (*SelVec)(unsafe.Pointer(v.vec))
}

// Len returns the represented element length.
func (v *BatchValidity) Len() int {
	return v.vec.Len()
}

// ViN returns how many elements are considered valid in the batch.
func (v *BatchValidity) ViN() int {
	return v.vec.ViN()
}

// NiN returns how many elements are considered invalid in the batch.
func (v *BatchValidity) NiN() int {
	return v.vec.NiN()
}

// Put returns the underlying data, making the validity undefined.
func (v *BatchValidity) Put() {
	v.vec.Put()
}

// Batch is a set of vectors, representing column slices; a batch may have
// a validity object that represents which elements are considered valid.
type Batch struct {
	length   int
	vecs     []Vec
	validity *BatchValidity
}

// HasValidity returns true if a batch has a validity object
func (b *Batch) HasValidity() bool { return b.validity != nil }
