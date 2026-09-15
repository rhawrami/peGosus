package store

import (
	"unsafe"

	"github.com/rhawrami/peGosus/pkg/mem"
)

// RowSelectionKind identifies a row-selection representation.
type RowSelectionKind uint8

const (
	SelectionInvalid RowSelectionKind = iota
	SelectionBitMap
	SelectionVector
)

// MakeRowSelectionFromBitMap returns a row selection that takes ownership of
// `m`.
func MakeRowSelectionFromBitMap(m *BitMap) *RowSelection {
	if m == nil {
		return nil
	}
	return &RowSelection{kind: SelectionBitMap, vec: m}
}

// MakeRowSelectionFromSelVec returns a row selection that takes ownership of
// `s`.
func MakeRowSelectionFromSelVec(s *SelVec) *RowSelection {
	if s == nil {
		return nil
	}
	return &RowSelection{kind: SelectionVector, vec: (*BitMap)(unsafe.Pointer(s))}
}

// RowSelection is a checked tagged union of bitmap and offset selections.
type RowSelection struct {
	kind RowSelectionKind
	vec  *BitMap
}

// Kind returns the selection representation.
func (s *RowSelection) Kind() RowSelectionKind {
	if s == nil {
		return SelectionInvalid
	}
	return s.kind
}

// Len returns the represented row-domain length.
func (s *RowSelection) Len() int {
	if s == nil || s.vec == nil {
		return 0
	}
	return s.vec.Len()
}

// ActiveLen returns the selected row count.
func (s *RowSelection) ActiveLen() int {
	if s == nil || s.vec == nil {
		return 0
	}
	return s.vec.ViN()
}

// AsBitMap returns the bitmap only when the representation matches.
func (s *RowSelection) AsBitMap() (*BitMap, bool) {
	if s == nil || s.kind != SelectionBitMap {
		return nil, false
	}
	return s.vec, true
}

// AsSelVec returns the selection vector only when the representation matches.
func (s *RowSelection) AsSelVec() (*SelVec, bool) {
	if s == nil || s.kind != SelectionVector {
		return nil, false
	}
	return (*SelVec)(unsafe.Pointer(s.vec)), true
}

// Retain returns an independently releasable selection.
func (s *RowSelection) Retain() *RowSelection {
	if s == nil || s.vec == nil {
		return nil
	}
	if s.kind == SelectionBitMap {
		return MakeRowSelectionFromBitMap(s.vec.Retain())
	}
	v, _ := s.AsSelVec()
	return MakeRowSelectionFromSelVec(v.Retain())
}

// Release releases the selection and clears the wrapper.
func (s *RowSelection) Release() {
	if s == nil || s.vec == nil {
		return
	}
	if s.kind == SelectionBitMap {
		s.vec.Release()
	} else {
		v, _ := s.AsSelVec()
		v.Release()
	}
	s.kind = SelectionInvalid
	s.vec = nil
}

// MakeBitMapTemp copies the selection into a temporary bitmap.
func (s *RowSelection) MakeBitMapTemp(a *mem.Allocator) *BitMap {
	if s == nil || s.vec == nil {
		return nil
	}
	data := a.AllocSegTemp(bitMapByteLength(s.Len()))
	m := MakeBitMapWithKnownNiN(s.Len(), s.Len(), data)
	m.ClearAll()
	if bitmap, ok := s.AsBitMap(); ok {
		copy(m.Bytes(), bitmap.Bytes())
		m.RecalcNiN()
		return m
	}
	selection, _ := s.AsSelVec()
	for _, offset := range selection.Offsets() {
		if int(offset) < m.Len() {
			m.Set(int(offset))
		}
	}
	return m
}

// MakeBatch returns an owner-confined batch that takes ownership of `vectors`
// on success. Every vector must have the same logical length.
func MakeBatch(vectors []Vector) *Batch {
	var length int
	if len(vectors) != 0 {
		length = vectors[0].Len()
	}
	for i := range vectors {
		if vectors[i].Kind() == VectorInvalid || vectors[i].Len() != length {
			return nil
		}
	}
	owned := make([]Vector, len(vectors))
	copy(owned, vectors)
	return &Batch{length: length, vectors: owned}
}

// MakeBatchRetained returns a batch with retained ownership of `vectors`.
func MakeBatchRetained(vectors []Vector) *Batch {
	retained := make([]Vector, len(vectors))
	for i := range vectors {
		retained[i] = vectors[i].Retain()
	}
	b := MakeBatch(retained)
	if b == nil {
		for i := range retained {
			retained[i].Release()
		}
	}
	return b
}

// Batch is an owner-confined set of equal-length physical vectors. Payload,
// validity, and selection mutation require sole ownership because retained
// batches share storage.
type Batch struct {
	length    int
	vectors   []Vector
	selection *RowSelection
}

// Len returns the physical row-domain length.
func (b *Batch) Len() int { return b.length }

// ActiveLen returns the selected row count, or Len when no selection exists.
func (b *Batch) ActiveLen() int {
	if b.selection == nil {
		return b.length
	}
	return b.selection.ActiveLen()
}

// NVectors returns the number of vectors.
func (b *Batch) NVectors() int { return len(b.vectors) }

// VectorAt returns a borrowed vector at `o`.
func (b *Batch) VectorAt(o int) *Vector { return &b.vectors[o] }

// Selection returns the borrowed row selection.
func (b *Batch) Selection() *RowSelection { return b.selection }

// SetSelection takes ownership of a matching selection and returns true.
func (b *Batch) SetSelection(selection *RowSelection) bool {
	if selection != nil && selection.Len() != b.length {
		return false
	}
	if selection == b.selection {
		return true
	}
	if b.selection != nil {
		b.selection.Release()
	}
	b.selection = selection
	return true
}

// Project returns a retained, zero-copy batch containing `offsets`.
func (b *Batch) Project(offsets []int) *Batch {
	for _, offset := range offsets {
		if offset < 0 || offset >= len(b.vectors) {
			return nil
		}
	}
	vectors := make([]Vector, len(offsets))
	for i, offset := range offsets {
		vectors[i] = b.vectors[offset].Retain()
	}
	return &Batch{length: b.length, vectors: vectors, selection: b.selection.Retain()}
}

// Retain returns an independently releasable batch over the same storage.
func (b *Batch) Retain() *Batch {
	if b == nil {
		return nil
	}
	vectors := make([]Vector, len(b.vectors))
	for i := range b.vectors {
		vectors[i] = b.vectors[i].Retain()
	}
	return &Batch{length: b.length, vectors: vectors, selection: b.selection.Retain()}
}

// Release releases all vector and selection references and clears the batch.
func (b *Batch) Release() {
	if b == nil {
		return
	}
	for i := range b.vectors {
		b.vectors[i].Release()
	}
	if b.selection != nil {
		b.selection.Release()
	}
	b.length = 0
	b.vectors = nil
	b.selection = nil
}
