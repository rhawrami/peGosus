package mem

import (
	"sync"
	"testing"
)

func TestAllocatorUsageRetainedAndScoped(t *testing.T) {
	a := MakeAllocatorWithProfiles([]int{512}, []int{256})
	initial := a.Usage()
	scope := MakeAllocationScope(a, 1024)
	view := MakeAllocatorWithScope(a, scope)
	general := view.AllocSeg(73)
	scratch := view.AllocSegTemp(35)
	general.Inc()
	usage := a.Usage()
	if usage.GeneralCapacity != initial.GeneralCapacity || usage.ScratchCapacity != initial.ScratchCapacity ||
		usage.GeneralUsed != general.Cap() || usage.ScratchUsed != scratch.Cap() ||
		usage.GeneralRequests != 1 || usage.ScratchRequests != 1 || usage != view.Usage() {
		t.Fatalf("unexpected live usage: %+v", usage)
	}
	general.Dec()
	scratch.Dec()
	if got := a.Usage(); got.GeneralUsed != general.Cap() || got.ScratchUsed != 0 {
		t.Fatalf("retained segment missing from usage: %+v", got)
	}
	general.Dec()
	if got := a.Usage(); got.GeneralUsed != 0 || got.ScratchUsed != 0 || got.GeneralCapacity != initial.GeneralCapacity || got.ScratchCapacity != initial.ScratchCapacity {
		t.Fatalf("released storage should remain reusable: %+v", got)
	}
	a.Optimize()
	if got := a.Usage(); got.GeneralRequests != 0 || got.ScratchRequests != 0 {
		t.Fatalf("Optimize did not reset request counts: %+v", got)
	}
}

func TestAllocatorUsageConcurrent(t *testing.T) {
	a := MakeAllocatorWithProfiles([]int{256}, []int{256})
	var workers sync.WaitGroup
	for range 4 {
		workers.Go(func() {
			for range 200 {
				general := a.AllocSeg(97)
				scratch := a.AllocSegTemp(53)
				usage := a.Usage()
				if usage.GeneralUsed < 0 || usage.GeneralUsed > usage.GeneralCapacity || usage.ScratchUsed < 0 || usage.ScratchUsed > usage.ScratchCapacity {
					t.Errorf("inconsistent slab observations: %+v", usage)
				}
				general.Dec()
				scratch.Dec()
			}
		})
	}
	workers.Wait()
	if got := a.Usage(); got.GeneralUsed != 0 || got.ScratchUsed != 0 || got.GeneralRequests != 800 || got.ScratchRequests != 800 {
		t.Fatalf("unexpected final usage: %+v", got)
	}
}
