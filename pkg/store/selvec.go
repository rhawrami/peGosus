package store

import "github.com/rhawrami/peGosus/pkg/mem"

// SelVec represents a selection vector, identifying the indices
// where elements are considered valid. Offsets are 32-bit unsigned integers.
type SelVec struct {
	length int          // element length represented
	nin    int          // number of elements not valid
	data   *mem.Segment // uint32 data
}

// Len returns the selection vector's element length.
func (s *SelVec) Len() int { return s.length }

// Put returns the underlying data, making the selection vector undefined.
func (s *SelVec) Put() {
	s.length = 0
	s.nin = 0
	s.data.Put()
}

// ViN returns the selection vector's number of valid elements.
func (s *SelVec) ViN() int { return s.length - s.nin }

// NiN returns the selection vector's number of invalid elements.
func (s *SelVec) NiN() int { return s.nin }

// Offsets returns the valid element offsets.
func (s *SelVec) Offsets() []uint32 { return s.data.AsU32T() }
