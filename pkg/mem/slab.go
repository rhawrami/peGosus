package mem

import "sync/atomic"

const defaultSegL int = 10

// MakeSlab generates a new Slab object with `l` bytes.
func MakeSlab(l int) *Slab {
	buff := makeAlignedSlice(l)
	base := &buff[0]
	segments := make([]*Segment, 0, defaultSegL)

	s := &Slab{
		buff:     buff,
		base:     base,
		on:       base,
		used:     0,
		capacity: uint64(l),
		segments: segments,
		nHoles:   0,
	}
	s.setUp()
	return s
}

// Slab represents a contiguous byte buffer.
type Slab struct {
	buff     []byte     // underlying buffer
	base     *byte      // base address
	on       *byte      // current address on
	used     uint64     // currently used byte length
	capacity uint64     // maximum byte capacity
	segments []*Segment // set of segments
	nHoles   uint64     // number of holes present
}

// setUp creates a single Segment belonging to `s`, with length
// equal to the length of `s`.
func (s *Slab) setUp() {
	s.segments[0] = &Segment{
		base:     s.base,
		length:   s.capacity,
		refCount: atomic.Int64{},
		slab:     s,
	}
}

// update updates `s`'s metadata, given another `l` bytes being used.
func (s *Slab) update(l uint64) {
	s.on = incPtr(s.base, int(l))
	s.used += l

	seg := &Segment{
		base:     s.on,
		length:   0,
		capacity: s.capacity - s.used,
		refCount: atomic.Int64{},
		slab:     s,
	}

	s.segments = append(s.segments, seg)
}

// coalesce attempts to coalesce free adjacent segments
// into one; returns true if at least one coalescence succeeded;
// for now, only two contiguous segments can be coalesced.
func (s *Slab) coalesce() bool {
	var yay bool

	// # segments can change during loop
	l := len(s.segments)
	for i := 0; i < l; i++ {
		if s.segments[i].refCount.Load() == 0 {
			if i != (l-1) && s.segments[i+1].refCount.Load() == 0 {
				a := s.segments[i]
				b := s.segments[i+1]
				a.capacity += b.capacity

				// shift down by one, except final segment
				for j := i + 1; j < len(s.segments)-1; j++ {
					s.segments[j] = s.segments[j+1]
				}
				s.segments = s.segments[:len(s.segments)-1]
				s.nHoles -= 1
				l -= 1
				yay = true
			}
		}
	}
	return yay
}

// MakeSegment returns a Segment with at least `length` bytes; returns
// (nil, false) if `s` cannot support a new segment with `length` bytes
// in its current state.
func (s *Slab) MakeSegment(length int) (*Segment, bool) {
	// ensure length divisible by alignment size
	var l uint64
	if l < uint64(alignSize) {
		l = uint64(alignSize)
	} else {
		l = uint64(alignSize - (length & (alignSize - 1)) + length)
	}

	// first check if we can use earlier segment
	if s.nHoles != 0 {
		for _, v := range s.segments {
			if v.refCount.Load() == 0 && v.capacity <= l {
				v.length = l
				v.refCount.Store(1)

				s.nHoles -= 1
				return v, true
			}
		}
	}

	// if earlier segment can't be used, check at the end
	if l <= s.segments[len(s.segments)-1].capacity {
		seg := s.segments[len(s.segments)-1]
		seg.length = uint64(length)
		seg.refCount.Store(1)

		s.update(l)
		return seg, true
	}

	return nil, false
}

// MakeSegmentWithCoalesce calls MakeSegment, but first attempts
// to coalesce adjacent free segments.
func (s *Slab) MakeSegmentWithCoalesce(length int) (*Segment, bool) {
	if s.nHoles != 0 {
		_ = s.coalesce()
	}
	return s.MakeSegment(length)
}
