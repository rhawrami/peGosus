package mem

const pawnSegmentSize int = 64

// MakePawner returns a Pawner with an underlying slab of `l` bytes.
func MakePawner(l int) *Pawner {
	s := MakeSlab(l)
	return &Pawner{s: s}
}

// Pawner represents a slab that only allocates segments of
// length 64 bytes. Segments belonging to a Pawner should be used for small,
// short-lived data. Coalescing will generally be avoided, and full-buffer clearing
// will be preferred. In the future, Pawner will be its own complete type, and Slab/Pawner
// will share some interface.
type Pawner struct {
	s *Slab
}

// MakeSegment returns a 64-byte segment.
func (p *Pawner) MakeSegment() (*Segment, bool) {
	return p.s.MakeSegment(pawnSegmentSize)
}

// TakeSegment takes a 64-byte segment.
func (p *Pawner) TakeSegment(g *Segment) {
	// remove this panic after testing.
	if g.capacity != uint64(pawnSegmentSize) {
		panic("TakeSegment: Pawner can only take segments of capacity 64")
	}
	p.s.TakeSegment(g)
}
