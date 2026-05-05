package mem

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
		stats:    &AllocStats{},
		on:       0,
		capacity: capacity,
		slabs:    slabs,
		pawner:   pawner,
		scratch:  scratch,
	}
}

// allocLoc represents where a segment belongs to.
type allocLoc int

const (
	fromSlabs allocLoc = iota
	fromPawner
	fromScratch
)

// AllocStats tracks allocation statistics.
type AllocStats struct {
	lastOptimized uint64    // number of allocations since last optimization
	avgReq        uint64    // rounded average allocation request size
	nLocs         [3]uint64 // number of allocations per slabs/pawner/scratch
}

// resetLastOptimized sets lastOptimized to zero.
func (a *AllocStats) resetLastOptimized() {
	a.lastOptimized = 0
}

// updateAvgReq updates the average request size.
func (a *AllocStats) updateAvgReq(l int) {
	a.avgReq = (a.avgReq + uint64(l)) >> 1
}

// updateNLocs updates the number of allocations for a given `l` location.
func (a *AllocStats) updateNLocs(l allocLoc) {
	a.nLocs[l] += 1
}

// updateState updates the average request size, and the number
// of allocations-per-location; increments lastOptimized.
func (a *AllocStats) updateState(n int, l allocLoc) {
	a.lastOptimized += 1
	a.updateAvgReq(n)
	a.updateNLocs(l)
}

// Allocator allocates and manages memory.
type Allocator struct {
	stats    *AllocStats // allocation statistics
	on       uint64      // offset into slabs slice
	capacity uint64      // maximum byte capacity FOR SLABS
	slabs    []*Slab     // slabs
	pawner   *Pawner     // slab with fixed 128-byte segments
	scratch  *Scratch    // slab for temporary objects
}

// Clear clears all underlying slabs in `a`.
func (a *Allocator) Clear() {
	for _, v := range a.slabs {
		v.Clear()
	}
	a.on = 0

	if a.PawnerIsValid() {
		a.pawner.Clear()
	}

	if a.ScratchIsValid() {
		a.scratch.Clear()
	}
}

// ClearSlabs clears all slabs in the slab set, excluding the
// pawner and scratch.
func (a *Allocator) ClearSlabs() {
	for _, v := range a.slabs {
		v.Clear()
	}
	a.on = 0
}

// ClearPawner clears the pawner slab, if non-nil.
func (a *Allocator) ClearPawner() {
	if a.PawnerIsValid() {
		a.pawner.Clear()
	}
}

// ClearScratch clears the scratch slab, if non-nil.
func (a *Allocator) ClearScratch() {
	if a.ScratchIsValid() {
		a.scratch.Clear()
	}
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
func (a *Allocator) allocTinySegment() (*Segment, allocLoc) {
	if !a.PawnerIsValid() {
		// if a pawn is needed, then it's probably worth making a small pawner.
		a.pawner = MakePawner(pawnSegmentSize * 5)
	}
	// try pawner, and grow if necessary; if pawner is maxed out, that means
	// a) pawner slab is too small, and/or
	// b) tiny segments are more in use than expected
	// so, the necessary copy and 1280 bytes added is fine.
	g := a.pawner.MakeSegmentForce()
	return g, fromPawner
}

// allocTempSegment allocates a segment that should be temporary,
// and where it was allocated from.
func (a *Allocator) allocTempSegment(l int) (*Segment, allocLoc) {
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
			return g, fromScratch
		}
	}

	// need to allocate from slabs now
	if g, ok := a.allocSegFromCurrentSlabs(l); ok {
		return g, fromSlabs
	}
	a.Grow(l + (l >> 1))
	a.on = uint64(len(a.slabs) - 1)
	// guaranteed to work
	g, _ := a.slabs[a.on].MakeSegment(l)
	return g, fromSlabs

}

// allocSegment returns a segment of at least `l` bytes, along
// with where the data was allocated from.
func (a *Allocator) allocSegment(l int) (*Segment, allocLoc) {
	if l < pawnSegmentSize {
		return a.allocTinySegment()
	}

	// need to allocate from slabs now
	if g, ok := a.allocSegFromCurrentSlabs(l); ok {
		return g, fromSlabs
	}
	a.Grow(l + (l >> 1))
	a.on = uint64(len(a.slabs) - 1)
	// guaranteed to work
	g, _ := a.slabs[a.on].MakeSegment(l)
	return g, fromSlabs
}

// AllocTemp returns a Data object with a single segment, with the
// implication that the object is temporary and will be freed shortly.
func (a *Allocator) AllocTemp(l int) *Data {
	g, loc := a.allocTempSegment(l)
	a.stats.updateState(l, loc)

	return MakeDataFromSingleSegment(g)
}

// Alloc returns a Data object with a single segment of at least `l` bytes.
func (a *Allocator) Alloc(l int) *Data {
	g, loc := a.allocSegment(l)
	a.stats.updateState(l, loc)
	return MakeDataFromSingleSegment(g)
}

// AllocWithProfile returns a Data object matching the size profile `s`;
// in other words, the Data object will have len(`s`.p) segments, each with
// at least `s`.p[i] bytes of capacity.
func (a *Allocator) AllocWithProfile(s SizeProfile) *Data {
	segs := make([]*Segment, len(s.p))
	for i := 0; i < len(s.p); i++ {
		g, l := a.allocSegment(int(s.p[i]))
		segs[i] = g
		a.stats.updateState(int(s.p[i]), l)
	}
	return MakeData(segs)
}
