package mem

// MakeSlabSet returns a SlabSet, given a size profile.
func MakeSlabSet(p SizeProfile) *SlabSet {
	var capacity uint64
	slabs := make([]*Slab, len(p.AsSlice()))

	for i, v := range p.AsSlice() {
		b := MakeSlab(int(v))
		capacity += b.capacity
		slabs[i] = b
	}

	return &SlabSet{
		capacity: capacity,
		on:       0,
		slabs:    slabs,
	}
}

// SlabSet is a set of slabs.
type SlabSet struct {
	capacity uint64  // total byte capacity
	on       int     // offset into current slab to pull from
	slabs    []*Slab // set of slabs
}

// Cap returns the total byte capacity of `s`.
func (s *SlabSet) Cap() uint64 { return s.capacity }

// Clear clears all slabs.
func (s *SlabSet) Clear() {
	for _, v := range s.slabs {
		v.Clear()
	}
	s.on = 0
}

// Grow adds a slab with byte capacity equal to the first slab
// in the set.
func (s *SlabSet) Grow() {
	b := MakeSlab(int(s.slabs[0].capacity))
	s.capacity += b.capacity
	s.slabs = append(s.slabs, b)
}

// GrowWithSize adds a slab with at least `l` bytes of capacity.
func (s *SlabSet) GrowWithSize(l int) {
	b := MakeSlab(l)
	s.capacity += b.capacity
	s.slabs = append(s.slabs, b)
}

// SetOn sets on to the slab with the greatest unused capacity.
func (s *SlabSet) SetOn() {
	var o int
	var r uint64
	for i, v := range s.slabs {
		if rS := v.capacity - v.used; rS > r {
			o = i
		}
	}
	s.on = o
}

// MakeSegment attempts to allocate a segment with at least `l`
// bytes of capacity; returns false if unable to allocate.
func (s *SlabSet) MakeSegment(l int) (*Segment, bool) {
	g, ok := s.slabs[s.on].MakeSegment(l)
	if !ok {
		for _, v := range s.slabs {
			g, ok = v.MakeSegment(l)
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
	g, ok := s.MakeSegment(l)
	if !ok {
		// add a slab by factor of 1.5`l`
		s.GrowWithSize(l + (l >> 1))
		// GrowWithSize sets `on` to final element
		g, _ = s.slabs[s.on].MakeSegment(l)
	}
	return g
}

// GrowAndMakeSegment first grows a slab with at least `l` bytes, then
// returns a segment from that slab.
func (s *SlabSet) GrowAndMakeSegment(l int) *Segment {
	// if l > first slab cap, grow slab by 1.5`l`
	sL := int(s.slabs[0].capacity)
	if sL < l {
		sL = l + (l >> 1)
	}
	s.GrowWithSize(sL)
	g, _ := s.slabs[s.on].MakeSegment(l)
	return g
}
