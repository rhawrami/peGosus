package mem

const pawnSegmentSize int = 128

// MakePawner returns a Pawner with an underlying slab of `l` bytes.
func MakePawner(l int) *Pawner {
	s := MakeSlab(l)
	return &Pawner{s: s}
}

// Pawner represents a slab that only allocates segments of
// length 128 bytes. Segments belonging to a Pawner should be used for small,
// short-lived data. Coalescing will generally be avoided, and full-buffer clearing
// will be preferred. Pawner only has one underlying slab.
type Pawner struct {
	s *Slab
}

// Cap returns the byte capacity of `p`.
func (p *Pawner) Cap() uint64 {
	return p.s.capacity
}

// Clear gives `p` a fresh slate.
func (p *Pawner) Clear() {
	p.s.Clear()
}

// MakeSegment returns a 128-byte segment; returns false if unable to make segment.
func (p *Pawner) MakeSegment() (*Segment, bool) {
	return p.s.MakeSegment(pawnSegmentSize)
}

// MakeSegmentForce returns a 128-byte segment; if underlying slab cannot support,
// slab is grown to accomodate.
func (p *Pawner) MakeSegmentForce() *Segment {
	if g, ok := p.s.MakeSegment(pawnSegmentSize); ok {
		return g
	}

	// add room for at least five more 128-byte segments
	p.s.Grow(int(p.s.capacity) + pawnSegmentSize*5)
	gF, ok := p.s.MakeSegment(pawnSegmentSize)
	if !ok {
		// this should not happen
		panic("MakeSegmentForce: slab grew, but still unable to fit")
	}

	return gF
}
