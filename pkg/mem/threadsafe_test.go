package mem

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAllocatorConcurrentAllocRelease(t *testing.T) {
	const workers = 16
	const iterations = 500

	a := MakeAllocatorWithProfiles([]int{1_024}, []int{1_024})
	errs := make(chan error, workers)
	var wg sync.WaitGroup

	for worker := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pattern := byte(worker + 1)
			for iteration := range iterations {
				size := 65 + (worker+iteration)%256
				var data *Data
				if iteration&1 == 0 {
					data = a.AllocData(size)
				} else {
					data = a.AllocDataTemp(size)
				}

				buffer := data.SegmentAt(0).AsBytes()
				for i := range buffer {
					buffer[i] = pattern
				}
				runtime.Gosched()
				for i, value := range buffer {
					if value != pattern {
						errs <- fmt.Errorf("worker %d: byte %d: got %d, expected %d", worker, i, value, pattern)
						return
					}
				}

				data.PutAll()
				a.TakeData(data)
			}
		}()
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	for _, set := range []*SlabSet{a.general, a.scratch} {
		set.mu.Lock()
		for i, slab := range set.slabs {
			if slab.used != 0 {
				set.mu.Unlock()
				t.Fatalf("slab %d: got %d used bytes, expected 0", i, slab.used)
			}
		}
		set.mu.Unlock()
	}

	want := int64(workers * iterations / 2)
	if got := a.stats.nLocs[reqGeneral].Load(); got != want {
		t.Fatalf("general requests: got %d, expected %d", got, want)
	}
	if got := a.stats.nLocs[reqScratch].Load(); got != want {
		t.Fatalf("scratch requests: got %d, expected %d", got, want)
	}
}

func TestSegmentConcurrentDec(t *testing.T) {
	const references = 64

	a := MakeAllocatorWithProfiles([]int{4_096}, []int{4_096})
	segment := a.AllocSeg(1_024)
	for range references - 1 {
		segment.Inc()
	}

	var released atomic.Int64
	var wg sync.WaitGroup
	for range references {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !segment.Dec() {
				released.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := released.Load(); got != 1 {
		t.Fatalf("final releases: got %d, expected 1", got)
	}
	if got := segment.RefCount(); got != 0 {
		t.Fatalf("reference count: got %d, expected 0", got)
	}
	segment.Inc()
	if got := segment.RefCount(); got != 0 {
		t.Fatalf("resurrected segment: got reference count %d", got)
	}
}

func TestSegmentFinalDecConcurrentReuse(t *testing.T) {
	const rounds = 250

	for range rounds {
		set := MakeSlabSet([]int{1_024})
		capacity := set.slabs[0].capacity
		segment, ok := set.MakeSegment(capacity)
		if !ok {
			t.Fatal("failed to fill slab")
		}

		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			segment.Dec()
		}()
		go func() {
			defer wg.Done()
			<-start
			for {
				reused, allocated := set.MakeSegment(capacity)
				if allocated {
					reused.Put()
					return
				}
				runtime.Gosched()
			}
		}()
		close(start)
		wg.Wait()

		if got := set.slabs[0].Used(); got != 0 {
			t.Fatalf("used bytes: got %d, expected 0", got)
		}
	}
}

func TestSegmentConcurrentIncDec(t *testing.T) {
	const workers = 32
	const iterations = 1_000

	set := MakeSlabSet([]int{1_024})
	segment := set.ForceSegment(128)
	var unexpectedFinal atomic.Int64
	var wg sync.WaitGroup

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iterations {
				segment.Inc()
				if !segment.Dec() {
					unexpectedFinal.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	if got := unexpectedFinal.Load(); got != 0 {
		t.Fatalf("unexpected final releases: got %d", got)
	}
	if got := segment.RefCount(); got != 1 {
		t.Fatalf("reference count: got %d, expected 1", got)
	}
	if segment.Dec() {
		t.Fatal("final decrement did not release segment")
	}
}

func TestSlabSetConcurrentTransfers(t *testing.T) {
	const transfers = 1_000

	a := MakeSlabSet([]int{1_024, 1_024, 1_024, 1_024})
	b := MakeSlabSet([]int{2_048, 2_048, 2_048, 2_048})
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		for range transfers {
			TransferSlab(a, b)
		}
	}()
	go func() {
		defer wg.Done()
		for range transfers {
			TransferSlab(b, a)
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("opposite-direction slab transfers deadlocked")
	}

	total := 0
	for _, set := range []*SlabSet{a, b} {
		set.mu.Lock()
		if len(set.slabs) == 0 {
			set.mu.Unlock()
			t.Fatal("transfer emptied a slab set")
		}
		for _, slab := range set.slabs {
			if slab.owner.Load() != set {
				set.mu.Unlock()
				t.Fatal("slab owner does not match containing set")
			}
			total++
		}
		set.mu.Unlock()
	}
	if total != 8 {
		t.Fatalf("total slabs: got %d, expected 8", total)
	}
}

func TestSlabSetRejectsLiveSlabRemoval(t *testing.T) {
	set := MakeSlabSet([]int{1_024, 1_024})
	segment, ok := set.slabs[0].MakeSegment(128)
	if !ok {
		t.Fatal("failed to allocate segment")
	}

	set.Remove(0)
	if len(set.slabs) != 2 || set.slabs[0].owner.Load() != set {
		t.Fatal("removed slab with a live segment")
	}

	segment.Put()
	set.Remove(0)
	if len(set.slabs) != 1 {
		t.Fatalf("slab count: got %d, expected 1", len(set.slabs))
	}
	if segment.slab.owner.Load() != nil {
		t.Fatal("removed slab retained its owner")
	}
}

func TestAcceptedSlabConcurrentAllocation(t *testing.T) {
	const workers = 8
	const iterations = 250

	set := MakeSlabSet([]int{1_024})
	slab := MakeSlab(1_024)
	set.Accept(slab)
	var wg sync.WaitGroup

	for worker := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range iterations {
				var segment *Segment
				var ok bool
				if worker&1 == 0 {
					segment, ok = slab.MakeSegment(64)
				} else {
					segment, ok = set.MakeSegment(64)
				}
				if ok {
					segment.Put()
				}
			}
		}()
	}
	wg.Wait()

	if got := slab.Used(); got != 0 {
		t.Fatalf("used bytes: got %d, expected 0", got)
	}
}

func TestSlabSetConcurrentOffsetTransfers(t *testing.T) {
	src := MakeSlabSet([]int{1_024, 1_024, 1_024})
	dst := MakeSlabSet([]int{1_024})
	var wg sync.WaitGroup

	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			TransferSlabWithOffset(dst, src, 2)
		}()
	}
	wg.Wait()

	if len(src.slabs)+len(dst.slabs) != 4 {
		t.Fatal("offset transfer lost a slab")
	}
}

func TestSlabConcurrentAccept(t *testing.T) {
	slab := MakeSlab(1_024)
	a := MakeSlabSet([]int{1_024})
	b := MakeSlabSet([]int{1_024})
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		a.Accept(slab)
	}()
	go func() {
		defer wg.Done()
		b.Accept(slab)
	}()
	wg.Wait()

	owner := slab.owner.Load()
	if owner != a && owner != b {
		t.Fatal("accepted slab has no valid owner")
	}
	occurrences := 0
	for _, set := range []*SlabSet{a, b} {
		set.mu.Lock()
		for _, member := range set.slabs {
			if member == slab {
				occurrences++
			}
		}
		set.mu.Unlock()
	}
	if occurrences != 1 {
		t.Fatalf("accepted slab occurrences: got %d, expected 1", occurrences)
	}
}

func TestSlabAccessDuringTransfer(t *testing.T) {
	const iterations = 1_000

	a := MakeSlabSet([]int{1_024})
	b := MakeSlabSet([]int{1_024})
	slab := MakeSlab(1_024)
	a.Accept(slab)
	var wg sync.WaitGroup

	wg.Add(3)
	go func() {
		defer wg.Done()
		for range iterations {
			TransferSlab(a, b)
		}
	}()
	go func() {
		defer wg.Done()
		for range iterations {
			TransferSlab(b, a)
		}
	}()
	go func() {
		defer wg.Done()
		for range iterations {
			if segment, ok := slab.MakeSegment(64); ok {
				segment.Put()
			}
		}
	}()
	wg.Wait()

	if got := slab.Used(); got != 0 {
		t.Fatalf("used bytes: got %d, expected 0", got)
	}
}

func TestAcceptedSlabGrowUpdatesSetCapacity(t *testing.T) {
	set := MakeSlabSet([]int{1_024})
	slab := MakeSlab(1_024)
	set.Accept(slab)
	oldSetCapacity := set.Cap()
	oldSlabCapacity := slab.Cap()

	slab.Grow(oldSlabCapacity * 2)

	want := oldSetCapacity + slab.Cap() - oldSlabCapacity
	if got := set.Cap(); got != want {
		t.Fatalf("set capacity: got %d, expected %d", got, want)
	}
}
