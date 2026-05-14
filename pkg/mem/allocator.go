package mem

const (
	dataCacheSizeDefault int = 8
	optCycleDefault      int = 16
)

// MakeAllocConfig returns an AllocConfig object.
func MakeAllocConfig(generalProfile, scratchProfile []int, cycleN, cacheN int) AllocConfig {
	return AllocConfig{
		generalProfile: generalProfile,
		scratchProfile: scratchProfile,
		cycleN:         cycleN,
		cacheN:         cacheN,
	}
}

// AllocConfig configures an Allocator.
type AllocConfig struct {
	generalProfile []int // byte capacity per general slab
	scratchProfile []int // byte capacity per scratch slab
	cycleN         int   // cycle threshold
	cacheN         int   // max data in cache
}

// MakeAllocatorWithConfig returns an Allocator based on `cfg`.
func MakeAllocatorWithConfig(cfg AllocConfig) *Allocator {
	general := MakeSlabSet(cfg.generalProfile)
	scratch := MakeSlabSet(cfg.scratchProfile)
	cache := make([]*Data, 0, cfg.cacheN)

	return &Allocator{
		stats:     &AllocStats{cycle: cfg.cycleN},
		general:   general,
		scratch:   scratch,
		dataCache: cache,
	}
}

// MakeAllocatorWithProfiles returns an Allocator, with a general and scratch slab set
// based on the respective profiles; uses default valus for cycle threshold and cache limit.
func MakeAllocatorWithProfiles(generalProfile, scratchProfile []int) *Allocator {
	general := MakeSlabSet(generalProfile)
	scratch := MakeSlabSet(scratchProfile)
	cache := make([]*Data, 0, dataCacheSizeDefault)

	return &Allocator{
		stats:     &AllocStats{cycle: optCycleDefault},
		general:   general,
		scratch:   scratch,
		dataCache: cache,
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
	cycle         int    // max number of allocations before next optimization must be ran
	lastOptimized int    // number of allocations since last optimization
	avgReq        int    // rounded rolling average allocation request size
	nLocs         [2]int // number of allocations per general/scratch (since last optimization)
}

// updateAvgReq updates the average request size.
func (a *AllocStats) updateAvgReq(l int) {
	r := l
	// if user requests zero bytes, this function interprets it
	// as start of sequence; will change later.
	if a.avgReq != 0 {
		r = (a.avgReq + r) >> 1
	}
	a.avgReq = r
}

// updateNLocs updates the number of allocations for a given `l` location.
func (a *AllocStats) updateNLocs(l reqLoc) {
	a.nLocs[l] += 1
}

// resetState resets the state of `a`, except the average request count.
func (a *AllocStats) resetState() {
	a.lastOptimized = 0
	a.nLocs[reqGeneral] = 0
	a.nLocs[reqScratch] = 0
}

// updateState updates the average request size, and the number
// of allocations-per-location; increments lastOptimized.
func (a *AllocStats) updateState(n int, l reqLoc) {
	a.lastOptimized += 1
	a.updateAvgReq(n)
	a.updateNLocs(l)
}

// Allocator allocates and manages memory; an Allocator comes with a set of
// general slabs meant to allocate long-use data, and a set of scratch slabs
// meant to allocate temporary data (e.g., returned before end of a function);
// the current optimization logic is extremely basic, and will be built out over time :)
type Allocator struct {
	stats     *AllocStats // allocation statistics
	general   *SlabSet    // set for general, non-temporary data
	scratch   *SlabSet    // set for temporary data
	dataCache []*Data     // cache of data objects to reuse
}

// ClearGeneral clears the general slabs.
func (a *Allocator) ClearGeneral() {
	a.general.Clear()
}

// ClearScratch clears the scratch slabs.
func (a *Allocator) ClearScratch() {
	a.scratch.Clear()
}

// ClearDataCache clears the data cache.
func (a *Allocator) ClearDataCache() {
	a.dataCache = a.dataCache[:0]
}

// Clear clears all underlying slabs, and the data cache.
func (a *Allocator) Clear() {
	a.ClearGeneral()
	a.ClearScratch()
	a.ClearDataCache()
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

// TakeData attempts to add `x` to the data cache.
func (a *Allocator) TakeData(x *Data) {
	x.Clear()
	if len(a.dataCache) != cap(a.dataCache) {
		a.dataCache = append(a.dataCache, x)
	}
}

// makeData returns a data object, first attempting to
// pull from the cache; if the cache is empty, a data object
// with segment capacity `hint` is created and returned.
func (a *Allocator) makeData(hint int) *Data {
	var x *Data
	if len(a.dataCache) > 0 {
		x = a.dataCache[len(a.dataCache)-1]
		a.dataCache = a.dataCache[:len(a.dataCache)-1]
	} else {
		x = &Data{segments: make([]*Segment, 0, hint)}
	}
	return x
}

// Optimize runs Optimize on the slab sets, also resetting the
// number of requests per location.
func (a *Allocator) Optimize() {
	a.stats.resetState()
	a.general.Optimize()
	a.scratch.Optimize()
}

// checkOptimize checks if `a` needs to run a set of optimizations.
func (a *Allocator) checkOptimize() {
	if a.stats.lastOptimized > a.stats.cycle {
		a.Optimize()
	}
}

// AllocTemp returns a Data object with a single segment, with the
// implication that the object is temporary and will be freed shortly.
func (a *Allocator) AllocTemp(l int) *Data {
	a.checkOptimize()
	a.stats.updateState(l, reqScratch)
	// check scratch, then general, then grow scratch if needed
	g, ok := a.scratch.MakeSegment(l)
	if !ok {
		g, ok = a.general.MakeSegment(l)
	}
	if !ok {
		g = a.scratch.GrowAndMakeSegment(l)
	}

	d := a.makeData(1)
	d.AddSegment(g, false)
	return d
}

// Alloc returns a Data object with a single segment of at least `l` bytes.
func (a *Allocator) Alloc(l int) *Data {
	a.checkOptimize()
	a.stats.updateState(l, reqGeneral)

	g := a.general.ForceSegment(l)
	d := a.makeData(1)
	d.AddSegment(g, false)
	return d
}

// AllocWithProfile returns a Data object matching the size profile `s`;
// in other words, the Data object will have len(`s`.p) segments, each with
// at least `s`.p[i] bytes of capacity.
func (a *Allocator) AllocWithProfile(p []int) *Data {
	a.checkOptimize()

	d := a.makeData(len(p))

	for i := 0; i < len(p); i++ {
		l := p[i]
		a.stats.updateState(l, reqGeneral)

		g := a.general.ForceSegment(l)
		d.AddSegment(g, false)
	}

	return d
}

// AllocTempWithProfile returns a Data object matching the size profile `s`;
// in other words, the Data object will have len(`s`.p) segments, each with
// at least `s`.p[i] bytes of capacity; implied that the object is
// temporary and will be freed shortly.
func (a *Allocator) AllocTempWithProfile(p []int) *Data {
	a.checkOptimize()

	d := a.makeData(len(p))

	for _, l := range p {
		a.stats.updateState(l, reqScratch)

		g, ok := a.scratch.MakeSegment(l)
		if !ok {
			g, ok = a.general.MakeSegment(l)
		}
		if !ok {
			g = a.scratch.GrowAndMakeSegment(l)
		}

		d.AddSegment(g, false)
	}

	return d
}
