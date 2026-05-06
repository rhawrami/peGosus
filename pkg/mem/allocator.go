package mem

// MakeAllocConfig returns an AllocConfig object.
func MakeAllocConfig(generalProfile, scratchProfile []uint64) AllocConfig {
	return AllocConfig{
		generalProfile: generalProfile,
		scratchProfile: scratchProfile,
	}
}

// AllocConfig configures an Allocator.
type AllocConfig struct {
	generalProfile []uint64 // byte capacity per general slab
	scratchProfile []uint64 // byte capacity per scratch slab
}

// MakeAllocatorWithConfig returns an Allocator based on `cfg`.
func MakeAllocatorWithConfig(cfg AllocConfig) *Allocator {
	general := MakeSlabSet(cfg.generalProfile)
	scratch := MakeSlabSet(cfg.scratchProfile)

	return &Allocator{
		stats:   &AllocStats{},
		general: general,
		scratch: scratch,
	}
}

// reqLoc represents where a segment was initially requested from.
type reqLoc int

const (
	reqGeneral reqLoc = iota
	reqScratch
)

// AllocStats tracks allocation statistics; may be used later.
type AllocStats struct {
	lastOptimized uint64    // number of allocations since last optimization
	avgReq        uint64    // rounded average allocation request size
	nLocs         [2]uint64 // number of allocations per general/scratch
}

// resetLastOptimized sets lastOptimized to zero.
func (a *AllocStats) resetLastOptimized() {
	a.lastOptimized = 0
}

// updateAvgReq updates the average request size.
func (a *AllocStats) updateAvgReq(l int) {
	r := uint64(l)
	if a.avgReq != 0 {
		r = (a.avgReq + uint64(l)) >> 1
	}
	a.avgReq = r
}

// updateNLocs updates the number of allocations for a given `l` location.
func (a *AllocStats) updateNLocs(l reqLoc) {
	a.nLocs[l] += 1
}

// updateState updates the average request size, and the number
// of allocations-per-location; increments lastOptimized.
func (a *AllocStats) updateState(n int, l reqLoc) {
	a.lastOptimized += 1
	a.updateAvgReq(n)
	a.updateNLocs(l)
}

// Allocator allocates and manages memory.
type Allocator struct {
	stats   *AllocStats // allocation statistics
	general *SlabSet    // set for general, non-temporary use
	scratch *SlabSet    // set for temporary data
}

// ClearGeneral clears the general slabs.
func (a *Allocator) ClearGeneral() {
	a.general.Clear()
}

// ClearScratch clears the scratch slabs.
func (a *Allocator) ClearScratch() {
	a.scratch.Clear()
}

// Clear clears all underlying slabs.
func (a *Allocator) Clear() {
	a.ClearGeneral()
	a.ClearScratch()
}

// GrowGeneral adds another slab with at least `l` bytes to the
// general slab set.
func (a *Allocator) GrowGeneral(l int) {
	a.general.GrowWithSize(l)
}

// GrowScratch adds another slab with at least `l` bytes to the
// scratch slab set.
func (a *Allocator) GrowScratch(l int) {
	a.scratch.GrowWithSize(l)
}

// AllocTemp returns a Data object with a single segment, with the
// implication that the object is temporary and will be freed shortly.
func (a *Allocator) AllocTemp(l int) *Data {
	a.stats.updateState(l, reqScratch)
	// check scratch, then general, then grow scratch
	g, ok := a.scratch.MakeSegment(l)
	if !ok {
		g, ok = a.general.MakeSegment(l)
	}
	if !ok {
		g = a.scratch.GrowAndMakeSegment(l)
	}

	return MakeDataFromSingleSegment(g)
}

// Alloc returns a Data object with a single segment of at least `l` bytes.
func (a *Allocator) Alloc(l int) *Data {
	a.stats.updateState(l, reqGeneral)
	g := a.general.ForceSegment(l)

	return MakeDataFromSingleSegment(g)
}

// AllocWithProfile returns a Data object matching the size profile `s`;
// in other words, the Data object will have len(`s`.p) segments, each with
// at least `s`.p[i] bytes of capacity.
func (a *Allocator) AllocWithProfile(p SizeProfile) *Data {
	firstP := int(p.p[0])
	a.stats.updateState(firstP, reqGeneral)

	g := a.general.ForceSegment(firstP)
	d := MakeDataFromSingleSegment(g)

	for i := 1; i < len(p.p); i++ {
		l := int(p.p[i])
		a.stats.updateState(l, reqGeneral)

		g = a.general.ForceSegment(l)
		d.AddSegment(g, false)
	}

	return d
}

// AllocTempWithProfile returns a Data object matching the size profile `s`;
// in other words, the Data object will have len(`s`.p) segments, each with
// at least `s`.p[i] bytes of capacity; implied that the object is
// temporary and will be freed shortly.
func (a *Allocator) AllocTempWithProfile(p SizeProfile) *Data {
	segs := make([]*Segment, len(p.p))

	for i, l := range p.p {
		length := int(l)
		a.stats.updateState(length, reqScratch)

		g, ok := a.scratch.MakeSegment(length)
		if !ok {
			g, ok = a.general.MakeSegment(length)
		}
		if !ok {
			g = a.scratch.GrowAndMakeSegment(length)
		}

		segs[i] = g
	}

	return MakeData(segs)
}
