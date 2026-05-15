package mem

import (
	"fmt"
	"sync/atomic"
)

const (
	// sets the default capacity for a slab's segment set.
	segmentsSetSizeDefault int = 8
	// sets the default capacity for a slab's segment cache.
	segmentsCacheSizeDefault int = 8
)

// MakeSlab generates a new Slab object with at least `l` bytes.
func MakeSlab(l int) *Slab {
	buff := makeAlignedSlice(l)
	base := &buff[0]
	segments := make([]*Segment, 0, segmentsSetSizeDefault)
	segCache := make([]*Segment, 0, segmentsCacheSizeDefault)

	s := &Slab{
		buff:     buff,
		base:     base,
		on:       base,
		used:     0,
		capacity: len(buff),
		segments: segments,
		segCache: segCache,
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
	used     int        // currently used byte length
	capacity int        // maximum byte capacity
	segments []*Segment // set of segments
	segCache []*Segment // cache of segments
	holes    int        // number of holes present
}

func (s *Slab) String() string {
	share := float64(s.used) / float64(s.capacity) * 100
	size := (14 + // address
		6 + // " Slab["
		7 + // assuming avg slab is 1 MB
		15 + // "B, XXX% used] {"
		2*len(s.segments) + // "[u|f],"
		2) // "|}"

	b := make([]byte, 0, size)
	b = append(b, fmt.Sprintf("%p Slab[%dB, %.00f%% used] {", s, s.capacity, share)...)
	for i := 0; i < len(s.segments)-1; i++ {
		var x byte = 'u'
		if s.segments[i].IsFree() {
			x = 'f'
		}
		b = append(b, x)
		b = append(b, ',')
	}
	if b[len(b)-1] == ',' {
		b = b[:len(b)-1]
	}
	b = append(b, "|f}"...)

	return string(b)
}

// takeSeg attempts to place `x` in the segment cache.
func (s *Slab) takeSeg(x *Segment) {
	if len(s.segCache) < cap(s.segCache) {
		s.segCache = append(s.segCache, x)
	}
}

// makeSeg returns a segment, either from the cache if available,
// or from a new allocation.
func (s *Slab) makeSeg() *Segment {
	if len(s.segCache) > 0 {
		g := s.segCache[len(s.segCache)-1]
		s.segCache = s.segCache[:len(s.segCache)-1]
		return g
	}
	return &Segment{}

}

// Cap returns the capacity of the slab.
func (s *Slab) Cap() int { return s.capacity }

// Used returns the number of bytes used.
func (s *Slab) Used() int { return s.used }

// Clear gives `s` a fresh slate; should be called knowing that all related
// segments will now be undefined.
func (s *Slab) Clear() {
	s.on = s.base
	s.used = 0
	s.holes = 0

	// return segments to cache if possible
	N := cap(s.segCache) - len(s.segCache)
	if len(s.segments) < N {
		N = len(s.segments)
	}
	for i := range N {
		s.takeSeg(s.segments[i])
	}

	s.segments = s.segments[:0]
	s.setUp()
}

// Nuke sets all pointers (including the underlying buffer) to nil.
func (s *Slab) Nuke() {
	s.buff = nil
	s.base = nil
	s.on = nil
	s.segments = nil
}

// Grow grows the underlying buffer to at least `l` bytes; requires copy of
// old to new buffer; does nothing if `l` is less than the current capacity.
func (s *Slab) Grow(l int) {
	if l <= s.capacity {
		return
	}

	newB := makeAlignedSlice(l)
	copy(newB, s.buff)

	oldBase := s.base
	newBase := &newB[0]
	for _, v := range s.segments {
		v.base = incPtr(newBase, calcOffset(oldBase, v.base))
	}

	// add excess capacity to edge
	capDiff := len(newB) - s.capacity
	s.segments[len(s.segments)-1].capacity += capDiff

	s.buff = newB
	s.base = newBase
	s.on = incPtr(newBase, calcOffset(oldBase, s.on))
	s.capacity = len(newB)

}

// FreeSpaceAtEnd returns the byte capacity of the final segment
// (e.g, remaining space at end of slab, ignoring holes).
func (s *Slab) FreeSpaceAtEnd() int {
	return s.segments[len(s.segments)-1].capacity
}

// setUp creates a single Segment belonging to `s`, with length and capacity
// equal to the capacity of `s`.
func (s *Slab) setUp() {
	s.segments = s.segments[:0]
	g := s.makeSeg()
	g.base = s.base
	g.length = 0
	g.capacity = s.capacity
	g.refCount = atomic.Int64{}
	g.slab = s

	s.segments = append(s.segments, g)
}

// update updates `s`'s metadata, given another `l` bytes being used, and a
// previous byte capacity of `p`.
func (s *Slab) update(l, p int) {
	s.on = incPtr(s.on, l)
	s.used += l

	g := s.makeSeg()
	g.base = s.on
	g.length = 0
	g.capacity = p - l
	g.refCount = atomic.Int64{}
	g.slab = s

	s.segments = append(s.segments, g)
}

// SimpleCoalesce attempts to coalesce free adjacent segments
// into one; returns true if at least one coalescence succeeded;
// only two contiguous segments can be coalesced.
func (s *Slab) SimpleCoalesce() bool {
	var yay bool
	if s.holes < 2 {
		return yay
	}

	// # segments can change during loop
	l := len(s.segments) - 1
	for i := 0; i < l; i++ {
		if len(s.segments) < 3 {
			break
		}
		if left := s.segments[i]; left.refCount.Load() == 0 {
			if right := s.segments[i+1]; right.refCount.Load() == 0 {
				left.capacity += right.capacity
				// shift down by one, except final segment
				copy(s.segments[i+1:], s.segments[i+2:])
				s.segments = s.segments[:len(s.segments)-1]
				s.holes -= 1
				l -= 1
				yay = true
			}
		}
	}
	return yay
}

// FullCoalesce attempts to coalesce all adjacent free segments;
// returns true if at least one coalesce attempt was successful.
func (s *Slab) FullCoalesce() bool {
	if s.holes == 0 {
		return false
	}
	// if N holes == N segments - 1 (e.g, all holes) -> just clear
	// this is also safe in the case of N segments == 1 (e.g., just edge)
	if s.holes == len(s.segments)-1 {
		s.Clear()
		return true
	}

	pos := make([]int, 0, s.holes>>1)
	var start, stop, adjust int
	var startCtr bool
	// pass 1: find hole postions and "widths"
	for i, v := range s.segments {
		if v.IsFree() {
			if !startCtr {
				startCtr = true
				start = i
			}
			stop = i
		} else {
			startCtr = false
			if start != stop {
				// append start AND stop pairs
				pos = append(pos, start-adjust, stop-adjust)
				// set adjust for next set of adjacent pairs
				adjust += (stop - start)
				// reset range
				start, stop = i, i
			}
		}
	}
	// if adjacent pairs at end of group
	if start != stop {
		pos = append(pos, start-adjust, stop-adjust)
	}

	if len(pos) == 0 {
		return false
	}

	// pass 2: merge segments
	var on int
	for on < len(pos) {
		start, stop := pos[on], pos[on+1]
		diff := stop - start
		// get new cap
		var c int
		for i := start; i < stop+1; i++ {
			c += s.segments[i].capacity
		}
		s.segments[start].capacity = c
		// shift segments over, set new len
		copy(s.segments[start+1:], s.segments[stop+1:])
		s.segments = s.segments[:len(s.segments)-diff]
		// update hole count
		s.holes -= diff

		on += 2
	}

	return true
}

// MakeSegment returns a Segment with at least `length` bytes; returns
// (nil, false) if `s` cannot support a new segment with `length` bytes
// in its current state.
func (s *Slab) MakeSegment(length int) (*Segment, bool) {
	// ensure length divisible by alignment size
	l := length
	if l < alignSize {
		l = alignSize
	}
	if l&(alignSize-1) != 0 {
		l = alignSize - (length & (alignSize - 1)) + length
	}

	// first check if we can use earlier segment
	if s.holes != 0 {
		for i := 0; i < len(s.segments)-1; i++ {
			v := s.segments[i]
			if v.refCount.Load() == 0 && l <= v.capacity {

				// if we only need 50% or less of the capacity, bisect into
				// two parts (not necessarily equal parts though)
				if l*2 <= v.capacity {
					// append segment, shift segments over by one.
					s.segments = append(s.segments, nil)
					copy(s.segments[i+2:], s.segments[i+1:])

					// keep right side free
					right := s.makeSeg()
					right.base = incPtr(v.base, l)
					right.length = 0
					right.capacity = v.capacity - l
					right.refCount = atomic.Int64{}
					right.slab = s
					s.segments[i+1] = right

					s.holes += 1
					v.capacity = l
				}

				v.length = length
				v.refCount.Store(1)

				s.used += v.capacity
				s.holes -= 1
				return v, true
			}
		}
	}

	// if earlier segment can't be used, check at the end
	if oldCap := s.segments[len(s.segments)-1].capacity; l <= oldCap {
		seg := s.segments[len(s.segments)-1]
		seg.length = length
		seg.capacity = l
		seg.refCount.Store(1)

		s.update(l, oldCap)
		return seg, true
	}

	return nil, false
}

// MakeSegmentWithCoalesce calls MakeSegment, but first attempts
// to coalesce adjacent free segments.
func (s *Slab) MakeSegmentWithCoalesce(length int) (*Segment, bool) {
	if s.holes > 1 {
		_ = s.FullCoalesce()
	}
	return s.MakeSegment(length)
}

// TakeSegment takes a segment, returning it to `s`.
func (s *Slab) TakeSegment(g *Segment) {
	s.used -= g.capacity
	g.length = 0
	g.refCount.Store(0)

	var h int = 1
	// try to avoid hole if `g` is penultimate segment
	if g == s.segments[len(s.segments)-2] {
		edge := s.segments[len(s.segments)-1]
		g.capacity += edge.capacity
		s.on = g.base
		s.segments = s.segments[:len(s.segments)-1]
		h = 0
	}
	s.holes += h
}
