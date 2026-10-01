package mem

import (
	"math/rand/v2"
	"runtime"
	"sync"
	"testing"
)

func TestSlabSetFragmentedReuse(t *testing.T) {
	for _, temporary := range []bool{false, true} {
		a := MakeAllocatorWithProfiles([]int{1024}, []int{1024})
		scope := MakeAllocationScope(a, 1024)
		view := MakeAllocatorWithScope(a, scope)
		allocate := view.AllocSeg
		if temporary {
			allocate = view.AllocSegTemp
		}
		first, second, retained, tail := allocate(256), allocate(256), allocate(256), allocate(256)
		base := first.base
		retained.MemSetU8(37)
		tail.MemSetU8(91)
		retained.Inc()
		retained.Dec()
		before := a.Usage()
		first.Dec()
		second.Dec()
		merged := allocate(400)
		if merged == nil || merged.base != base || a.Usage().GeneralCapacity != before.GeneralCapacity || a.Usage().ScratchCapacity != before.ScratchCapacity {
			t.Fatalf("temporary=%t: adjacent free segments did not satisfy request without growth", temporary)
		}
		merged.MemSetU8(127)
		for _, held := range []*Segment{retained, tail} {
			pattern := byte(37)
			if held == tail {
				pattern = 91
			}
			for _, value := range held.AsBytes() {
				if value != pattern {
					t.Fatal("coalescing overwrote retained storage")
				}
			}
		}
		merged.Dec()
		retained.Dec()
		tail.Dec()
		whole := allocate(1024)
		if whole == nil || whole.base != base || whole.Cap() != 1024 {
			t.Fatal("fully released slab was not reusable as one segment")
		}
		whole.Dec()
		if scope.Live() != 0 || scope.Peak() > scope.Limit() {
			t.Fatal("coalescing changed scope accounting")
		}
	}
}

func TestSlabCoalesceTrailingFreeCursor(t *testing.T) {
	for _, coalesce := range []struct {
		name string
		run  func(*Slab) bool
	}{{"full", (*Slab).FullCoalesce}, {"simple", (*Slab).SimpleCoalesce}} {
		t.Run(coalesce.name, func(t *testing.T) {
			s := MakeSlab(1024)
			guard, _ := s.MakeSegment(64)
			first, _ := s.MakeSegment(128)
			last, _ := s.MakeSegment(128)
			guard.MemSetU8(71)
			first.Dec()
			last.Dec()
			if !coalesce.run(s) || s.on != s.segments[len(s.segments)-1].base {
				t.Fatal("coalescing trailing holes left the allocation cursor behind")
			}
			large, ok := s.MakeSegment(256)
			if !ok || calcOffset(s.base, large.base) != 64 || calcOffset(s.base, s.on) != 320 {
				t.Fatal("allocation after trailing coalesce used the wrong address")
			}
			next, ok := s.MakeSegment(704)
			if !ok || calcOffset(s.base, next.base) != 320 || s.used != s.capacity {
				t.Fatal("trailing storage was not contiguous after coalescing")
			}
			large.MemSetU8(93)
			next.MemSetU8(99)
			for _, value := range guard.AsBytes() {
				if value != 71 {
					t.Fatal("coalescing moved or overwrote live storage")
				}
			}
			guard.Dec()
			large.Dec()
			next.Dec()
		})
	}
}

func TestSlabSetFragmentationRandomized(t *testing.T) {
	const blocks = 64
	rng := rand.New(rand.NewPCG(111, 219))
	set := MakeSlabSet([]int{blocks * alignSize})
	slab := set.slabs[0]
	var occupied [blocks]bool
	type allocation struct {
		segment *Segment
		start   int
		blocks  int
		refs    int
		pattern byte
	}
	var held []allocation
	for iteration := range 4000 {
		if len(held) == 0 || rng.IntN(3) != 0 {
			length := rng.IntN(16*alignSize + 1)
			needed := max(1, (length+alignSize-1)/alignSize)
			available, run := false, 0
			for _, used := range occupied {
				if used {
					run = 0
				} else {
					run++
					available = available || run >= needed
				}
			}
			segment, ok := set.MakeSegment(length)
			if ok != available {
				t.Fatalf("iteration %d: request %d, available=%t allocated=%t", iteration, length, available, ok)
			}
			if ok {
				entry := allocation{segment, calcOffset(slab.base, segment.base) / alignSize, segment.Cap() / alignSize, 1, byte(rng.IntN(255) + 1)}
				if entry.start < 0 || entry.start+entry.blocks > blocks || segment.Len() != length || segment.Cap() < length {
					t.Fatal("allocation outside slab or smaller than requested")
				}
				for i := entry.start; i < entry.start+entry.blocks; i++ {
					if occupied[i] {
						t.Fatal("allocation overlaps a live segment")
					}
					occupied[i] = true
				}
				segment.MemSetU8(entry.pattern)
				if rng.IntN(3) == 0 {
					segment.Inc()
					entry.refs++
				}
				held = append(held, entry)
			}
		} else {
			at := rng.IntN(len(held))
			entry := &held[at]
			entry.segment.Dec()
			entry.refs--
			if entry.refs == 0 {
				for i := entry.start; i < entry.start+entry.blocks; i++ {
					occupied[i] = false
				}
				held[at] = held[len(held)-1]
				held = held[:len(held)-1]
			}
		}
		used := 0
		for _, entry := range held {
			used += entry.blocks * alignSize
			if entry.segment.RefCount() != int64(entry.refs) || calcOffset(slab.base, entry.segment.base) != entry.start*alignSize || entry.segment.Cap() != entry.blocks*alignSize {
				t.Fatal("coalescing changed live metadata")
			}
			for _, value := range entry.segment.AsBytes() {
				if value != entry.pattern {
					t.Fatal("live payload changed")
				}
			}
		}
		offset, holes := 0, 0
		for i, segment := range slab.segments {
			if calcOffset(slab.base, segment.base) != offset {
				t.Fatal("segment layout is not contiguous")
			}
			offset += segment.Cap()
			if i+1 < len(slab.segments) && segment.IsFree() {
				holes++
			}
		}
		if used != slab.used || offset != slab.capacity || holes != slab.holes || slab.on != slab.segments[len(slab.segments)-1].base {
			t.Fatalf("iteration %d: inconsistent slab metadata", iteration)
		}
	}
	for _, entry := range held {
		for range entry.refs {
			entry.segment.Dec()
		}
	}
	whole, ok := set.MakeSegment(blocks * alignSize)
	if !ok || whole.base != slab.base {
		t.Fatal("randomized workload did not reclaim the whole slab")
	}
	whole.Dec()
}

func TestAllocatorFragmentationConcurrentRetained(t *testing.T) {
	a := MakeAllocatorWithProfiles([]int{64 << 10}, []int{64 << 10})
	var workers sync.WaitGroup
	for worker := range 8 {
		workers.Go(func() {
			scope := MakeAllocationScope(a, 16<<10)
			view := MakeAllocatorWithScope(a, scope)
			guard := view.AllocSeg(128)
			guard.MemSetU8(byte(worker + 1))
			guard.Inc()
			guard.Dec()
			for iteration := range 200 {
				var pages [4]*Segment
				for i := range pages {
					pages[i] = view.AllocSegTemp(64 * (1 + (worker+iteration+i*3)%16))
					if pages[i] == nil {
						t.Error("fragmented allocation exhausted scope")
						return
					}
					pages[i].MemSetU8(byte(worker + 11))
				}
				runtime.Gosched()
				for _, page := range pages {
					for _, value := range page.AsBytes() {
						if value != byte(worker+11) {
							t.Error("concurrent coalescing overwrote live page")
						}
					}
					page.Dec()
				}
				for _, value := range guard.AsBytes() {
					if value != byte(worker+1) {
						t.Error("retained payload changed during coalescing")
					}
				}
			}
			guard.Dec()
			if scope.Live() != 0 {
				t.Error("concurrent scope did not return to zero")
			}
		})
	}
	workers.Wait()
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("concurrent allocations leaked: %+v", usage)
	}
}
