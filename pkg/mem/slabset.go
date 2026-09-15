package mem

import (
	"sync"
	"sync/atomic"
)

var nextSlabSetID atomic.Uint64

// MakeSlabSet returns a SlabSet, given a size profile.
func MakeSlabSet(p []int) *SlabSet {
	var capacity int
	slabs := make([]*Slab, len(p))

	for i, v := range p {
		b := MakeSlab(v)
		capacity += b.capacity
		slabs[i] = b
	}

	s := &SlabSet{
		id:       nextSlabSetID.Add(1),
		capacity: capacity,
		on:       0,
		slabs:    slabs,
	}
	for _, slab := range slabs {
		slab.owner.Store(s)
	}
	return s
}

// SlabSet is a set of slabs.
type SlabSet struct {
	mu       sync.Mutex
	id       uint64
	capacity int     // total byte capacity
	on       int     // offset into current slab to pull from
	slabs    []*Slab // set of slabs
}

// Cap returns the total byte capacity of `s`.
func (s *SlabSet) Cap() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.capacity
}

// Clear clears all slabs; it requires no live segments or payload views.
func (s *SlabSet) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, v := range s.slabs {
		v.clearLocked()
	}
	s.on = 0
}

// Nuke nukes `s` and all its underlying slabs; it requires quiescence.
func (s *SlabSet) Nuke() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, v := range s.slabs {
		v.owner.CompareAndSwap(s, nil)
		v.nukeLocked()
	}
	s.slabs = nil
}

// Grow adds a slab with byte capacity equal to the first slab
// in the set.
func (s *SlabSet) Grow() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.growLocked()
}

func (s *SlabSet) growLocked() {
	b := MakeSlab(s.slabs[0].capacity)
	b.owner.Store(s)
	s.capacity += b.capacity
	s.slabs = append(s.slabs, b)
}

// GrowWithSize adds a slab with at least `l` bytes of capacity; also sets
// on to the new slab.
func (s *SlabSet) GrowWithSize(l int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.growWithSizeLocked(l)
}

func (s *SlabSet) growWithSizeLocked(l int) {
	b := MakeSlab(l)
	b.owner.Store(s)
	s.capacity += b.capacity
	s.slabs = append(s.slabs, b)
	s.on = len(s.slabs) - 1
}

// Optimize runs a FullCoalesce on all slabs and updates on to the slab with
// the greatest remaining capacity; it is intended for query boundaries.
func (s *SlabSet) Optimize() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.optimizeLocked()
}

func (s *SlabSet) optimizeLocked() {
	r := s.slabs[s.on].capacity - s.slabs[s.on].used
	o := s.on

	for i, v := range s.slabs {
		_ = v.fullCoalesceLocked()
		if rem := (v.capacity - v.used); rem > r {
			r = rem
			o = i
		}
	}

	s.on = o
}

// Accept takes in `b`, and adds it to the set, also updating `on`.
func (s *SlabSet) Accept(b *Slab) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !b.owner.CompareAndSwap(nil, s) {
		return
	}
	s.acceptLocked(b)
}

func (s *SlabSet) acceptLocked(b *Slab) {
	s.capacity += b.capacity
	s.slabs = append(s.slabs, b)

	s.setOnLocked()
}

// Remove removes the open slab at offset `o`, also updating `on`; it does
// nothing for the final slab, an invalid offset, or a slab with live segments.
func (s *SlabSet) Remove(o int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeLocked(o)
}

func (s *SlabSet) removeLocked(o int) *Slab {
	if len(s.slabs) <= 1 || o < 0 || o >= len(s.slabs) || s.slabs[o].used != 0 {
		return nil
	}
	b := s.slabs[o]
	s.capacity -= s.slabs[o].capacity
	copy(s.slabs[o:], s.slabs[o+1:])
	s.slabs = s.slabs[:len(s.slabs)-1]
	b.owner.CompareAndSwap(s, nil)

	s.setOnLocked()
	return b
}

// SetOn sets on to the slab with the greatest unused capacity.
func (s *SlabSet) SetOn() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setOnLocked()
}

func (s *SlabSet) setOnLocked() {
	var o int
	var r int
	for i, v := range s.slabs {
		if rem := v.capacity - v.used; rem > r {
			o = i
			r = rem
		}
	}
	s.on = o
}

// MakeSegment attempts to allocate a segment with at least `l`
// bytes of capacity; returns false if unable to allocate.
func (s *SlabSet) MakeSegment(l int) (*Segment, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.makeSegmentLocked(l)
}

func (s *SlabSet) makeSegmentLocked(l int) (*Segment, bool) {
	g, ok := s.slabs[s.on].makeSegmentLocked(l)
	if !ok {
		for _, v := range s.slabs {
			g, ok = v.makeSegmentLocked(l)
			if ok {
				break
			}
		}
	}
	return g, ok
}

// ForceSegment allocates a segment with at least `l` bytes; if unable
// to allocate, a slab is added to `s` in order to accomodate.
func (s *SlabSet) ForceSegment(l int) *Segment {
	s.mu.Lock()
	defer s.mu.Unlock()

	g, ok := s.makeSegmentLocked(l)
	if !ok {
		// add a slab by factor of 1.5`l`
		s.growWithSizeLocked(l + (l >> 1))
		// GrowWithSize sets `on` to final element
		g, _ = s.slabs[s.on].makeSegmentLocked(l)
	}
	return g
}

// GrowAndMakeSegment returns a segment with at least `l` bytes, growing the
// set only if no existing slab can satisfy the request.
func (s *SlabSet) GrowAndMakeSegment(l int) *Segment {
	s.mu.Lock()
	defer s.mu.Unlock()
	if g, ok := s.makeSegmentLocked(l); ok {
		return g
	}

	// if l > first slab cap, grow slab by 1.5`l`
	sL := s.slabs[0].capacity
	if sL < l {
		sL = l + (l >> 1)
	}
	s.growWithSizeLocked(sL)
	g, _ := s.slabs[s.on].makeSegmentLocked(l)
	return g
}

// TransferSlab takes the first open slab (e.g., used = 0) from `src`,
// and transfers it to `dst`; does nothing if `src` has no open slabs.
func TransferSlab(dst, src *SlabSet) {
	if dst == src {
		return
	}
	unlock := lockSlabSets(dst, src)
	defer unlock()
	if len(src.slabs) <= 1 {
		return
	}

	var a *Slab

	for i, v := range src.slabs {
		if v.used == 0 && v.owner.CompareAndSwap(src, dst) {
			a = v
			src.capacity -= v.capacity
			copy(src.slabs[i:], src.slabs[i+1:])
			src.slabs = src.slabs[:len(src.slabs)-1]
			break
		}
	}

	if a != nil {
		dst.capacity += a.capacity
		dst.slabs = append(dst.slabs, a)
		src.setOnLocked()
		dst.setOnLocked()
	}
}

// TransferSlabWithOffset takes the slab at offset `o` in `src`, and transfers
// it to `dst`; does nothing if slab at `o` isn't open.
func TransferSlabWithOffset(dst, src *SlabSet, o int) {
	if dst == src {
		return
	}
	unlock := lockSlabSets(dst, src)
	defer unlock()
	if len(src.slabs) <= 1 || o < 0 || o >= len(src.slabs) {
		return
	}

	a := src.slabs[o]
	if a.used != 0 || !a.owner.CompareAndSwap(src, dst) {
		return
	}

	src.capacity -= a.capacity
	copy(src.slabs[o:], src.slabs[o+1:])
	src.slabs = src.slabs[:len(src.slabs)-1]
	dst.capacity += a.capacity
	dst.slabs = append(dst.slabs, a)
	src.setOnLocked()
	dst.setOnLocked()
}

func lockSlabSets(a, b *SlabSet) func() {
	first, second := a, b
	if first.id > second.id {
		first, second = second, first
	}
	first.mu.Lock()
	second.mu.Lock()
	return func() {
		second.mu.Unlock()
		first.mu.Unlock()
	}
}
