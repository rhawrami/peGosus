package store

import (
	"sort"

	"github.com/rhawrami/peGosus/pkg/mem"
)

// MakeSelVec returns a selection vector that takes ownership of `data`.
func MakeSelVec(rows, count int, data *mem.Segment) *SelVec {
	if rows < 0 || count < 0 || count > rows || data == nil || data.Len()&3 != 0 || count > data.Len()>>2 {
		if data != nil {
			data.Dec()
		}
		return nil
	}
	offsets := data.AsU32T()[:count]
	for i, offset := range offsets {
		if uint64(offset) >= uint64(rows) || (i != 0 && offset <= offsets[i-1]) {
			data.Dec()
			return nil
		}
	}
	return &SelVec{length: rows, nin: rows - count, data: data}
}

// MakeSelVecFromOffsets copies `offsets` into general storage.
func MakeSelVecFromOffsets(a *mem.Allocator, rows int, offsets []uint32) *SelVec {
	return makeSelVecFromOffsets(a, rows, offsets, false)
}

// MakeSelVecFromOffsetsTemp copies `offsets` into temporary storage.
func MakeSelVecFromOffsetsTemp(a *mem.Allocator, rows int, offsets []uint32) *SelVec {
	return makeSelVecFromOffsets(a, rows, offsets, true)
}

// SelVec represents selected row offsets over a logical row domain.
type SelVec struct {
	length int
	nin    int
	data   *mem.Segment
}

// Len returns the represented row-domain length.
func (s *SelVec) Len() int { return s.length }

// ViN returns the selected row count.
func (s *SelVec) ViN() int { return s.length - s.nin }

// NiN returns the unselected row count.
func (s *SelVec) NiN() int { return s.nin }

// Data returns the borrowed backing segment.
func (s *SelVec) Data() *mem.Segment { return s.data }

// Offsets returns the selected row offsets.
func (s *SelVec) Offsets() []uint32 {
	if s == nil || s.data == nil {
		return nil
	}
	return s.data.AsU32T()[:s.ViN()]
}

// Retain returns an independently releasable selection vector over immutable
// shared offsets.
func (s *SelVec) Retain() *SelVec {
	if s == nil || s.data == nil {
		return nil
	}
	s.data.Inc()
	return &SelVec{length: s.length, nin: s.nin, data: s.data}
}

// Release releases the segment reference and clears the wrapper.
func (s *SelVec) Release() {
	if s == nil || s.data == nil {
		return
	}
	s.data.Dec()
	s.length = 0
	s.nin = 0
	s.data = nil
}

// Put releases the selection vector.
func (s *SelVec) Put() { s.Release() }

func makeSelVecFromOffsets(a *mem.Allocator, rows int, offsets []uint32, temporary bool) *SelVec {
	if rows < 0 {
		rows = 0
	}
	cleaned := append([]uint32(nil), offsets...)
	sort.Slice(cleaned, func(i, j int) bool { return cleaned[i] < cleaned[j] })
	on := 0
	for _, offset := range cleaned {
		if uint64(offset) >= uint64(rows) || (on != 0 && offset == cleaned[on-1]) {
			continue
		}
		cleaned[on] = offset
		on++
	}
	cleaned = cleaned[:on]
	count := len(cleaned)
	var data *mem.Segment
	if temporary {
		data = a.AllocSegTemp(count * 4)
	} else {
		data = a.AllocSeg(count * 4)
	}
	copy(data.AsU32T(), cleaned)
	return MakeSelVec(rows, count, data)
}
