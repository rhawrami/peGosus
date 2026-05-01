package mem

const pawnSegmentSize int = 64

// Pawner represents a slab that only allocates segments of
// length 64 bytes. Segments belonging to a Pawner should be used for small,
// short-lived data. Coalescing will generally be avoided, and full-buffer clearing
// will be preferred. In the future, Pawner will be its own complete type, and Slab/Pawner
// will share some interface.
type Pawner struct {
	s *Slab
}

// Clear gives `p` a fresh slate.
func (p *Pawner) Clear() {
	p.s.Clear()
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

// MakeScratch returns a Scratch object with at least
// `l` bytes in its underlying slab.
func MakeScratch(l int) *Scratch {
	s := MakeSlab(l)
	return &Scratch{s: s}
}

// Scratch represents a slab that should be used to hold temporary
// objects. While `TakeSegment` is exposed, `Clear` should be the
// primary function of reusing segments in a Scratch buffer. Generally,
// Scratch segments should only live within a single function call.
type Scratch struct {
	s *Slab
}

// MakeSegment returns a segment with at least `l` bytes in length;
// returns false if not enough space.
func (c *Scratch) MakeSegment(l int) (*Segment, bool) {
	return c.s.MakeSegment(l)
}

// Clear clears a Scratch object's buffer and segments
func (c *Scratch) Clear() {
	c.s.Clear()
}

// TakeSegment takes a segment belonging to the Scratch object; should
// be used sparingly.
func (c *Scratch) TakeSegment(g *Segment) {
	c.s.TakeSegment(g)
}

// MakeAllocConfig returns an AllocConfig object.
func MakeAllocConfig(slabProfile []uint64, pawnerSize, scratchSize uint64) AllocConfig {
	return AllocConfig{
		slabProfile: slabProfile,
		pawnerSize:  pawnerSize,
		scratchSize: scratchSize,
	}
}

// AllocConfig configurates an initialized Allocator; if pawnerSize (scratchSize)
// is 0, then the underlying Pawner (Scratch) pointer will be nil.
type AllocConfig struct {
	slabProfile []uint64 // byte capacity per slab
	pawnerSize  uint64   // pawner byte byte capacity
	scratchSize uint64
}

func MakeAllocatorWithConfig(cfg AllocConfig) *Allocator {
	var pawner *Pawner
	var scratch *Scratch
	if cfg.pawnerSize == 0 {
		pawner = nil
	} else {
		pawner = MakePawner(int(cfg.pawnerSize))
	}
	if cfg.scratchSize == 0 {
		scratch = nil
	} else {
		scratch = MakeScratch(int(cfg.scratchSize))
	}

	slabs := make([]*Slab, len(cfg.slabProfile))
	var capacity uint64
	for i := 0; i < len(cfg.slabProfile); i++ {
		s := MakeSlab(int(cfg.slabProfile[i]))
		capacity += s.capacity
		slabs[i] = s
	}

	return &Allocator{
		on:       0,
		used:     0,
		capacity: capacity,
		slabs:    slabs,
		pawner:   pawner,
		scratch:  scratch,
	}
}

// MakePawner returns a Pawner with an underlying slab of `l` bytes.
func MakePawner(l int) *Pawner {
	s := MakeSlab(l)
	return &Pawner{s: s}
}

// Allocator "allocates" and manages memory.
type Allocator struct {
	on       uint64   // offset into slabs slice
	used     uint64   // bytes currently used FOR SLABS
	capacity uint64   // maximum byte capacity FOR SLABS
	slabs    []*Slab  // slabs
	pawner   *Pawner  // slab with fixed 64-byte segments
	scratch  *Scratch // slab for temporary objects
}

// PawnerIsValid returns true if `a`'s Pawner object is non-nil.
func (a *Allocator) PawnerIsValid() bool { return a.pawner != nil }

// ScratchIsValid returns true if `a`'s Scratch object is non-nil.
func (a *Allocator) ScratchIsValid() bool { return a.scratch != nil }
