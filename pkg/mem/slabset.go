package mem

// MakeSlabSet returns a SlabSet, given a size profile.
func MakeSlabSet(p []uint64) *SlabSet {
	var capacity uint64
	slabs := make([]*Slab, len(p))

	for i, v := range p {
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

// Nuke nukes `s` and all its underlying slabs.
func (s *SlabSet) Nuke() {
	for _, v := range s.slabs {
		v.Nuke()
	}
	s.slabs = nil
}

// Grow adds a slab with byte capacity equal to the first slab.
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
	s.on = len(s.slabs) - 1
}

// Optimize runs a FullCoalesce on all slabs, and updates on to the
// slab with the greatest remaining capacity.
func (s *SlabSet) Optimize() {
	r := s.slabs[s.on].capacity - s.slabs[s.on].used
	o := s.on

	for i, v := range s.slabs {
		_ = v.FullCoalesce()
		if rem := (v.capacity - v.used); rem > r {
			r = rem
			o = i
		}
	}

	s.on = o
}

// Accept takes in `b`, and adds it to the set, also updating `on`.
func (s *SlabSet) Accept(b *Slab) {
	s.capacity += b.capacity
	s.slabs = append(s.slabs, b)
	if (s.slabs[s.on].capacity - s.slabs[s.on].used) > (b.capacity - b.used) {
		s.on = len(s.slabs) - 1
	}
}

// Remove removes the slab at offset `o`, also updating `on`.
func (s *SlabSet) Remove(o int) {
	s.capacity -= s.slabs[o].capacity

	var newOn int
	var r uint64
	for i := o; i < len(s.slabs)-1; i++ {
		v := s.slabs[i+1]
		if rem := v.capacity - v.used; rem > r {
			r = rem
			newOn = i
		}
		s.slabs[i] = v
	}

	s.slabs = s.slabs[:len(s.slabs)-1]
	s.on = newOn
}

// SetOn sets on to the slab with the greatest unused capacity.
func (s *SlabSet) SetOn() {
	var o int
	var r uint64
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

// transferSlab takes the first open slab (e.g., used = 0) from `src`,
// and transfers it to `dst`; does nothing if `src` has no open slabs.
func transferSlab(dst, src *SlabSet) {
	var a *Slab
	var ok bool

	for i, v := range src.slabs {
		if v.used == 0 {
			a = v
			ok = true
			src.Remove(i)
			break
		}
	}

	if ok {
		dst.Accept(a)
	}
}

// transferSlabWithOffset takes the slab at offset `o` in `src`, and transfers
// it to `dst`; does nothing if slab at `o` isn't open.
func transferSlabWithOffset(dst, src *SlabSet, o int) {
	a := src.slabs[o]
	if a.used != 0 {
		return
	}

	src.Remove(o)
	dst.Accept(a)
}
