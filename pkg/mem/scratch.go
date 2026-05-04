package mem

// MakeScratch returns a Scratch object with at least
// `l` bytes in its underlying slab.
func MakeScratch(l int) *Scratch {
	s := MakeSlab(l)
	return &Scratch{s: s}
}

// Scratch represents a slab that should be used to hold temporary
// objects. `Clear` should be the primary function of reusing segments
// in a Scratch buffer. Generally, Scratch segments should only live
// within a single function call. Scratch only has one underlying slab.
type Scratch struct {
	s *Slab
}

// Cap returns the byte capacity of `c`.
func (c *Scratch) Cap() uint64 {
	return c.s.capacity
}

// Clear clears a Scratch object's buffer and segments
func (c *Scratch) Clear() {
	c.s.Clear()
}

// MakeSegment returns a segment with at least `l` bytes in length;
// returns false if not enough space.
func (c *Scratch) MakeSegment(l int) (*Segment, bool) {
	return c.s.MakeSegment(l)
}

// MakeSegmentForce returns a segment with at least `l` bytes; if underlying
// slab cannot support, slab is grown to accomodate.
func (c *Scratch) MakeSegmentForce(l int) *Segment {
	if g, ok := c.s.MakeSegment(l); ok {
		return g
	}

	// add room for at least two more segments with `l` bytes
	c.s.Grow(int(c.s.capacity) + l*2)
	gF, ok := c.s.MakeSegment(l)
	if !ok {
		// this should not happen
		panic("MakeSegmentForce: slab grew, but still unable to fit")
	}

	return gF
}
