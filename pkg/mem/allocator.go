package mem

import (
	"sync"
	"sync/atomic"
)

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
		stats:     &AllocStats{cycle: int64(cfg.cycleN)},
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
		stats:     &AllocStats{cycle: int64(optCycleDefault)},
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
	cycle         int64           // max number of allocations before optimization is recommended
	lastOptimized atomic.Int64    // number of allocations since last optimization
	avgReq        atomic.Int64    // rounded rolling average allocation request size
	nLocs         [2]atomic.Int64 // number of allocations per general/scratch (since last optimization)
}

// updateAvgReq updates the average request size.
func (a *AllocStats) updateAvgReq(l int) {
	for {
		old := a.avgReq.Load()
		r := int64(l)
		// if user requests zero bytes, this function interprets it
		// as start of sequence; will change later.
		if old != 0 {
			r = (old + r) >> 1
		}
		if a.avgReq.CompareAndSwap(old, r) {
			return
		}
	}
}

// updateNLocs updates the number of allocations for a given `l` location.
func (a *AllocStats) updateNLocs(l reqLoc) {
	a.nLocs[l].Add(1)
}

// resetState resets the state of `a`, except the average request count.
func (a *AllocStats) resetState() {
	a.lastOptimized.Store(0)
	a.nLocs[reqGeneral].Store(0)
	a.nLocs[reqScratch].Store(0)
}

// updateState updates the average request size, and the number
// of allocations-per-location; increments lastOptimized.
func (a *AllocStats) updateState(n int, l reqLoc) {
	a.lastOptimized.Add(1)
	a.updateAvgReq(n)
	a.updateNLocs(l)
}

// Allocator allocates and manages memory; allocation and release are safe for
// concurrent use, while Clear and Optimize require a quiescent allocator.
// An Allocator has general slabs for long-use data and scratch slabs for
// temporary data.
type Allocator struct {
	stats       *AllocStats // allocation statistics
	general     *SlabSet    // set for general, non-temporary data
	scratch     *SlabSet    // set for temporary data
	dataCacheMu sync.Mutex
	dataCache   []*Data // cache of data objects to reuse
}

// ClearGeneral clears the general slabs; it requires a quiescent allocator.
func (a *Allocator) ClearGeneral() {
	a.general.Clear()
}

// ClearScratch clears the scratch slabs; it requires a quiescent allocator.
func (a *Allocator) ClearScratch() {
	a.scratch.Clear()
}

// ClearDataCache clears the data cache.
func (a *Allocator) ClearDataCache() {
	a.dataCacheMu.Lock()
	defer a.dataCacheMu.Unlock()
	a.dataCache = a.dataCache[:0]
}

// Clear clears all underlying slabs and the data cache; it requires a
// quiescent allocator.
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
	a.dataCacheMu.Lock()
	defer a.dataCacheMu.Unlock()
	if len(a.dataCache) != cap(a.dataCache) {
		a.dataCache = append(a.dataCache, x)
	}
}

// makeData returns a data object, first attempting to
// pull from the cache; if the cache is empty, a data object
// with segment capacity `hint` is created and returned.
func (a *Allocator) makeData(hint int) *Data {
	a.dataCacheMu.Lock()
	defer a.dataCacheMu.Unlock()

	var x *Data
	if len(a.dataCache) > 0 {
		x = a.dataCache[len(a.dataCache)-1]
		a.dataCache = a.dataCache[:len(a.dataCache)-1]
	} else {
		x = &Data{segments: make([]*Segment, 0, hint)}
	}
	return x
}

// Optimize runs Optimize on the slab sets and resets request counts; it
// requires a quiescent allocator.
func (a *Allocator) Optimize() {
	a.stats.resetState()
	a.general.Optimize()
	a.scratch.Optimize()
}

// AllocSegTemp allocates a segment, with the implication that
// the object is temporary and will be freed shortly.
func (a *Allocator) AllocSegTemp(l int) *Segment {
	a.stats.updateState(l, reqScratch)
	// check scratch, then general, then grow scratch if needed
	g, ok := a.scratch.MakeSegment(l)
	if !ok {
		g, ok = a.general.MakeSegment(l)
	}
	if !ok {
		g = a.scratch.GrowAndMakeSegment(l)
	}

	return g
}

// AllocSeg allocates a segment.
func (a *Allocator) AllocSeg(l int) *Segment {
	a.stats.updateState(l, reqScratch)
	// check scratch, then general, then grow scratch if needed
	g, ok := a.scratch.MakeSegment(l)
	if !ok {
		g, ok = a.general.MakeSegment(l)
	}
	if !ok {
		g = a.scratch.GrowAndMakeSegment(l)
	}

	return g
}

// AllocDataTemp returns a Data object with a single segment, with the
// implication that the object is temporary and will be freed shortly.
func (a *Allocator) AllocDataTemp(l int) *Data {
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

// AllocData returns a Data object with a single segment of at least `l` bytes.
func (a *Allocator) AllocData(l int) *Data {
	a.stats.updateState(l, reqGeneral)

	g := a.general.ForceSegment(l)
	d := a.makeData(1)
	d.AddSegment(g, false)
	return d
}

// AllocDataWithProfile returns a Data object matching the size profile `s`;
// in other words, the Data object will have len(`s`.p) segments, each with
// at least `s`.p[i] bytes of capacity.
func (a *Allocator) AllocDataWithProfile(p []int) *Data {
	d := a.makeData(len(p))

	for i := 0; i < len(p); i++ {
		l := p[i]
		a.stats.updateState(l, reqGeneral)

		g := a.general.ForceSegment(l)
		d.AddSegment(g, false)
	}

	return d
}

// AllocDataTempWithProfile returns a Data object matching the size profile `s`;
// in other words, the Data object will have len(`s`.p) segments, each with
// at least `s`.p[i] bytes of capacity; implied that the object is
// temporary and will be freed shortly.
func (a *Allocator) AllocDataTempWithProfile(p []int) *Data {
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
