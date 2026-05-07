package mem

const optCycleDefault uint64 = 10

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
		stats:   &AllocStats{cycle: optCycleDefault},
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

// AllocStats tracks allocation statistics; will be used later.
type AllocStats struct {
	cycle         uint64    // max number of allocations before next optimization must be ran
	lastOptimized uint64    // number of allocations since last optimization
	avgReq        uint64    // rounded average allocation request size
	nLocs         [2]uint64 // number of allocations per general/scratch (since last optimization)
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

// Allocator allocates and manages memory; the current optimization logic is
// extremely basic, and will be built out over time :)
type Allocator struct {
	stats   *AllocStats // allocation statistics
	general *SlabSet    // set for general, non-temporary data
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

// Optimize runs Optimize on the slab sets, also resetting the
// number of requests per location.
func (a *Allocator) Optimize() {
	a.stats.lastOptimized = 0
	a.stats.nLocs[0] = 0
	a.stats.nLocs[1] = 0

	a.general.Optimize()
	// if for some reason, we call Optimize in middle of operation,
	// rather than at the start.
	a.scratch.Optimize()
}

// CheckOptimize checks if `a` needs to run a set of optimizations.
func (a *Allocator) CheckOptimize() {
	if a.stats.lastOptimized > a.stats.cycle {
		a.Optimize()
	}
}

// AllocTemp returns a Data object with a single segment, with the
// implication that the object is temporary and will be freed shortly.
func (a *Allocator) AllocTemp(l int) *Data {
	a.CheckOptimize()
	a.stats.updateState(l, reqScratch)
	// check scratch, then general, then grow scratch if needed
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
	a.CheckOptimize()
	a.stats.updateState(l, reqGeneral)
	g := a.general.ForceSegment(l)

	return MakeDataFromSingleSegment(g)
}

// AllocWithProfile returns a Data object matching the size profile `s`;
// in other words, the Data object will have len(`s`.p) segments, each with
// at least `s`.p[i] bytes of capacity.
func (a *Allocator) AllocWithProfile(p []uint64) *Data {
	a.CheckOptimize()

	firstP := int(p[0])
	a.stats.updateState(firstP, reqGeneral)

	g := a.general.ForceSegment(firstP)
	d := MakeDataFromSingleSegment(g)

	for i := 1; i < len(p); i++ {
		l := int(p[i])
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
func (a *Allocator) AllocTempWithProfile(p []uint64) *Data {
	a.CheckOptimize()

	segs := make([]*Segment, len(p))

	for i, l := range p {
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
