package mem

import (
	"sync"
	"testing"
)

func TestAllocationScopeRetainedOwnership(t *testing.T) {
	a := MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	scope := MakeAllocationScope(a, 128)
	general, ok := scope.AllocSeg(96)
	if !ok {
		t.Fatal("could not reserve general bytes")
	}
	if _, ok = scope.AllocSegTemp(33); ok {
		t.Fatal("exceeded scope budget")
	}
	temp, ok := scope.AllocSegTemp(32)
	if !ok || scope.Live() != 128 || scope.Peak() != 128 {
		t.Fatalf("scope live/peak: %d/%d", scope.Live(), scope.Peak())
	}
	general.Inc()
	general.Dec()
	if scope.Live() != 128 {
		t.Fatal("retained segment was uncharged prematurely")
	}
	general.Dec()
	if scope.Live() != 32 {
		t.Fatalf("general release left %d bytes", scope.Live())
	}
	temp.Dec()
	if scope.Live() != 0 || scope.Peak() != 128 {
		t.Fatalf("scope final live/peak: %d/%d", scope.Live(), scope.Peak())
	}
	if _, ok = scope.AllocSeg(-1); ok {
		t.Fatal("accepted a negative request")
	}
	if _, ok = scope.AllocSeg(129); ok {
		t.Fatal("accepted an oversized request")
	}
}

func TestAllocationScopeConcurrentTransfers(t *testing.T) {
	a := MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	scope := MakeAllocationScope(a, 1024)
	var wait sync.WaitGroup
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for range 200 {
				segment, ok := scope.AllocSegTemp(128)
				if !ok {
					continue
				}
				segment.Dec()
			}
		}()
	}
	wait.Wait()
	if scope.Live() != 0 || scope.Peak() > 1024 {
		t.Fatalf("concurrent live/peak: %d/%d", scope.Live(), scope.Peak())
	}
}

func TestScopedAllocatorView(t *testing.T) {
	a := MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	scope := MakeAllocationScope(a, 64)
	view := MakeAllocatorWithScope(a, scope)
	first := view.AllocSeg(64)
	if first == nil || view.AllocSegTemp(1) != nil || !scope.Exhausted() {
		t.Fatal("allocator view did not enforce scope")
	}
	first.Dec()
	if scope.Live() != 0 {
		t.Fatal("view release did not return budget")
	}
	second := view.AllocSegTemp(64)
	if second == nil {
		t.Fatal("scoped capacity could not be reused")
	}
	second.Dec()
	cleaned := view.AllocSeg(-1)
	if cleaned == nil || cleaned.Len() != 0 {
		t.Fatal("scoped allocator did not clean a negative request")
	}
	cleaned.Dec()
	if scope.Peak() != 64 {
		t.Fatalf("unexpected scope peak: %d", scope.Peak())
	}
	if view.AllocDataWithProfile([]int{40, 40}) != nil || scope.Live() != 0 {
		t.Fatal("partial data allocation leaked scope bytes")
	}
	data := view.AllocDataTemp(16)
	if data == nil || scope.Live() != 16 {
		t.Fatal("temporary data was not charged")
	}
	data.DecAll()
	view.TakeData(data)
	if scope.Live() != 0 {
		t.Fatal("temporary data release leaked scope bytes")
	}
}
