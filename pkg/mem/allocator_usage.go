package mem

// AllocatorUsage reports slab capacity and live segment capacity in bytes.
// Requests count allocations by requested location since the last Optimize;
// temporary requests may be served by general slabs.
type AllocatorUsage struct {
	GeneralCapacity int
	ScratchCapacity int
	GeneralUsed     int
	ScratchUsed     int
	GeneralRequests int64
	ScratchRequests int64
}

// Usage returns synchronized observations of each slab set and atomic request
// counters. Concurrent changes may occur between the observations.
func (a *Allocator) Usage() AllocatorUsage {
	if a == nil {
		return AllocatorUsage{}
	}
	var usage AllocatorUsage
	a.general.mu.Lock()
	usage.GeneralCapacity = a.general.capacity
	for _, slab := range a.general.slabs {
		usage.GeneralUsed += slab.used
	}
	a.general.mu.Unlock()
	a.scratch.mu.Lock()
	usage.ScratchCapacity = a.scratch.capacity
	for _, slab := range a.scratch.slabs {
		usage.ScratchUsed += slab.used
	}
	a.scratch.mu.Unlock()
	usage.GeneralRequests = a.stats.nLocs[reqGeneral].Load()
	usage.ScratchRequests = a.stats.nLocs[reqScratch].Load()
	return usage
}
