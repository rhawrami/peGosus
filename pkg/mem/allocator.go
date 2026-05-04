package mem

// AllocLoc represents where a segment belongs to.
type AllocLoc int

const (
	FromSlabs AllocLoc = iota
	FromPawner
	FromScratch
)

// MakeAllocConfig returns an AllocConfig object.
func MakeAllocConfig(slabProfile []uint64, pawnerSize, scratchSize uint64) AllocConfig {
	return AllocConfig{
		slabProfile: slabProfile,
		pawnerSize:  pawnerSize,
		scratchSize: scratchSize,
	}
}

// AllocConfig configures an initialized Allocator; if pawnerSize (scratchSize)
// is 0, then the underlying Pawner (Scratch) pointer will be nil.
type AllocConfig struct {
	slabProfile []uint64 // byte capacity per slab
	pawnerSize  uint64   // pawner byte capacity
	scratchSize uint64   // scratch byte capacity
}

// MakeAllocatorWithConfig returns an Allocator based on `cfg`.
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
		capacity: capacity,
		slabs:    slabs,
		pawner:   pawner,
		scratch:  scratch,
	}
}

// Allocator allocates and manages memory.
type Allocator struct {
	on       uint64   // offset into slabs slice
	capacity uint64   // maximum byte capacity FOR SLABS
	slabs    []*Slab  // slabs
	pawner   *Pawner  // slab with fixed 128-byte segments
	scratch  *Scratch // slab for temporary objects
}

// Grow adds another slab with at least `l` bytes to the allocator.
func (a *Allocator) Grow(l int) {
	s := MakeSlab(l)
	a.slabs = append(a.slabs, s)
	a.capacity += s.capacity
}

// PawnerIsValid returns true if `a`'s Pawner object is non-nil.
func (a *Allocator) PawnerIsValid() bool { return a.pawner != nil }

// ScratchIsValid returns true if `a`'s Scratch object is non-nil.
func (a *Allocator) ScratchIsValid() bool { return a.scratch != nil }

// allocSegFromCurrentSlabs attempts to make a segment, with at least `l` bytes,
// from the current slabs in `a` (e.g., will not grow or add slabs); returns false
// if unable to.
func (a *Allocator) allocSegFromCurrentSlabs(l int) (*Segment, bool) {
	if g, ok := a.slabs[a.on].MakeSegmentWithCoalesce(l); ok {
		return g, true
	}

	for i, v := range a.slabs {
		if i != int(a.on) {
			if g, ok := v.MakeSegmentWithCoalesce(l); ok {
				return g, true
			}
		}
	}

	return nil, false
}

// allocTinySegment returns a 128-byte segment, guaranteed to come from
// the pawner slab.
func (a *Allocator) allocTinySegment() (*Segment, AllocLoc) {
	if !a.PawnerIsValid() {
		// if a pawn is needed, then it's probably worth making a small pawner.
		a.pawner = MakePawner(pawnSegmentSize * 5)
	}
	// try pawner, and grow if necessary; if pawner is maxed out, that means
	// a) pawner slab is too small, and/or
	// b) tiny segments are more in use than expected
	// so, the necessary copy and 1280 bytes added is fine.
	g := a.pawner.MakeSegmentForce()
	return g, FromPawner
}

// allocTempSegment allocates a segment that should be temporary,
// and where it was allocated from.
func (a *Allocator) allocTempSegment(l int) (*Segment, AllocLoc) {
	// if small enough, just use the pawner
	if l <= pawnSegmentSize {
		return a.allocTinySegment()
	}

	// if a scratch isn't valid, but this fn is being called,
	// and if `l` isn't ridiculously large (relative to slabs),
	// it's probably worth making a scratch slab (with a bit more space than needed)
	if !a.ScratchIsValid() && (l*2) < int(a.slabs[0].capacity) {
		a.scratch = MakeScratch(l + (l >> 1))
	}

	if a.ScratchIsValid() {
		if g, ok := a.scratch.MakeSegment(l); ok {
			return g, FromScratch
		}
	}

	// need to allocate from slabs now
	if g, ok := a.allocSegFromCurrentSlabs(l); ok {
		return g, FromSlabs
	}
	a.Grow(l + (l >> 1))
	a.on = uint64(len(a.slabs) - 1)
	// guaranteed to work
	g, _ := a.slabs[a.on].MakeSegment(l)
	return g, FromSlabs

}

// allocSegment returns a segment of at least `l` bytes, along
// with where the data was allocated from.
func (a *Allocator) allocSegment(l int) (*Segment, AllocLoc) {
	if l < pawnSegmentSize {
		return a.allocTinySegment()
	}

	// need to allocate from slabs now
	if g, ok := a.allocSegFromCurrentSlabs(l); ok {
		return g, FromSlabs
	}
	a.Grow(l + (l >> 1))
	a.on = uint64(len(a.slabs) - 1)
	// guaranteed to work
	g, _ := a.slabs[a.on].MakeSegment(l)
	return g, FromSlabs
}

// AllocTemp returns a Data object with a single segment, with the
// implication that the object is temporary and will be freed shortly.
func (a *Allocator) AllocTemp(l int) (*Data, AllocLoc) {
	g, loc := a.allocTempSegment(l)
	return MakeDataFromSingleSegment(g), loc
}

// Alloc returns a Data object with a single segment of at least `l` bytes.
func (a *Allocator) Alloc(l int) (*Data, AllocLoc) {
	g, loc := a.allocSegment(l)
	return MakeDataFromSingleSegment(g), loc
}

// AllocWithProfile returns a Data object matching the size profile `s`;
// in other words, the Data object will have len(`s`.p) segments, each with
// at least `s`.p[i] bytes of capacity; also returns slice of allocation locations
func (a *Allocator) AllocWithProfile(s SizeProfile) (*Data, []AllocLoc) {
	segs := make([]*Segment, len(s.p))
	locs := make([]AllocLoc, 0, len(s.p))
	for i := 0; i < len(s.p); i++ {
		segs[i], locs[i] = a.allocSegment(int(s.p[i]))
	}
	return MakeData(segs), locs
}
