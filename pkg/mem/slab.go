package mem

import "sync/atomic"

// sets the default capacity for a slab's segment set.
const defaultSegL int = 10

// MakeSlab generates a new Slab object with at least `l` bytes.
func MakeSlab(l int) *Slab {
	buff := makeAlignedSlice(l)
	base := &buff[0]
	segments := make([]*Segment, 0, defaultSegL)

	s := &Slab{
		buff:     buff,
		base:     base,
		on:       base,
		used:     0,
		capacity: uint64(len(buff)),
		segments: segments,
		holes:    0,
	}

	s.setUp()
	return s
}

// Slab represents a contiguous chunk of memory; The underlying memory is
// guaranteed to be at least `alignSize` bytes in length, divisible by `alignSize`
// bytes in length, and divisible by `alignSize` in its base address.
type Slab struct {
	buff     []byte     // underlying buffer
	base     *byte      // base address
	on       *byte      // current address on
	used     uint64     // currently used byte length
	capacity uint64     // maximum byte capacity
	segments []*Segment // set of segments
	holes    uint64     // number of holes present
}

// Clear gives `s` a fresh slate; should be called knowing that all related
// segments will now be undefined.
func (s *Slab) Clear() {
	s.on = s.base
	s.used = 0
	s.holes = 0
	s.segments = s.segments[:0]
	s.setUp()
}

// Nuke sets all pointers (including the underlying buffer) to nil. Not really
// necessary, but I thought a function called `Nuke` was kind of cool :)
func (s *Slab) Nuke() {
	s.buff = nil
	s.base = nil
	s.on = nil
	s.segments = nil
}

// setUp creates a single Segment belonging to `s`, with length and capacity
// equal to the capacity of `s`.
func (s *Slab) setUp() {
	s.segments = s.segments[:0]
	seg := &Segment{
		base:     s.base,
		length:   s.capacity,
		capacity: s.capacity,
		refCount: atomic.Int64{},
		slab:     s,
	}
	s.segments = append(s.segments, seg)
}

// update updates `s`'s metadata, given another `l` bytes being used.
func (s *Slab) update(l uint64) {
	s.on = incPtr(s.on, int(l))
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
	l := len(s.segments) - 1
	for i := 0; i < l; i++ {
		if len(s.segments) <= 2 {
			break
		}
		if left := s.segments[i]; left.refCount.Load() == 0 {
			if right := s.segments[i+1]; right.refCount.Load() == 0 {
				left.capacity += right.capacity
				// shift down by one, except final segment
				for j := i + 1; j < len(s.segments)-1; j++ {
					s.segments[j] = s.segments[j+1]
				}
				s.segments = s.segments[:len(s.segments)-1]
				s.holes -= 1
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
	l := uint64(length)
	if l < uint64(alignSize) {
		l = uint64(alignSize)
	} else {
		if l&(uint64(alignSize)-1) != 0 {
			l = uint64(alignSize - (length & (alignSize - 1)) + length)
		}
	}

	// first check if we can use earlier segment
	if s.holes != 0 {
		for _, v := range s.segments {
			if v.refCount.Load() == 0 && l <= v.capacity {
				v.length = l
				v.refCount.Store(1)

				s.holes -= 1
				return v, true
			}
		}
	}

	// if earlier segment can't be used, check at the end
	if l <= s.segments[len(s.segments)-1].capacity {
		seg := s.segments[len(s.segments)-1]
		seg.length = uint64(length)
		seg.capacity = uint64(length)
		seg.refCount.Store(1)

		s.update(l)
		return seg, true
	}

	return nil, false
}

// MakeSegmentWithCoalesce calls MakeSegment, but first attempts
// to coalesce adjacent free segments.
func (s *Slab) MakeSegmentWithCoalesce(length int) (*Segment, bool) {
	if s.holes != 0 {
		_ = s.coalesce()
	}
	return s.MakeSegment(length)
}

// TakeSegment takes a segment, returning it to `s`.
func (s *Slab) TakeSegment(g *Segment) {
	g.length = 0
	g.refCount.Store(0)

	var h uint64 = 1
	// try to avoid hole if `g` is penultimate segment
	if g == s.segments[len(s.segments)-2] {
		edge := s.segments[len(s.segments)-1]
		g.capacity += edge.capacity
		s.on = g.base
		s.used -= g.capacity
		s.segments = s.segments[:len(s.segments)-1]
		h = 0
	}
	s.holes += h
}
