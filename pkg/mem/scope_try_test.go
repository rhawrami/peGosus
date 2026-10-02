package mem

import "testing"

func TestScopeOptionalReservation(t *testing.T) {
	a := MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	scope := MakeAllocationScope(a, 32)
	segment, ok := scope.TryAllocSeg(24)
	if !ok || segment == nil || scope.Live() != 24 || scope.Peak() != 24 {
		t.Fatal("optional reservation was not charged")
	}
	if declined, ok := scope.TryAllocSeg(16); ok || declined != nil || scope.Exhausted() || scope.Live() != 24 {
		t.Fatal("declined hint changed reservations or exhaustion")
	}
	segment.Dec()
	if scope.Live() != 0 {
		t.Fatal("released hint remained charged")
	}
	if segment, ok := scope.AllocSeg(33); ok || segment != nil || !scope.Exhausted() {
		t.Fatal("mandatory reservation no longer reports exhaustion")
	}
}

func TestScopeConcurrentOptionalReservations(t *testing.T) {
	a := MakeAllocatorWithProfiles([]int{4096}, []int{4096})
	scope := MakeAllocationScope(a, 128)
	start := make(chan struct{})
	results := make(chan *Segment, 32)
	for range 32 {
		go func() {
			<-start
			segment, _ := scope.TryAllocSeg(16)
			results <- segment
		}()
	}
	close(start)
	var admitted []*Segment
	for range 32 {
		if segment := <-results; segment != nil {
			admitted = append(admitted, segment)
		}
	}
	if len(admitted) != 8 || scope.Live() != 128 || scope.Peak() != 128 || scope.Exhausted() {
		t.Fatalf("optional concurrent admission: count=%d live=%d peak=%d exhausted=%v", len(admitted), scope.Live(), scope.Peak(), scope.Exhausted())
	}
	for _, segment := range admitted {
		segment.Dec()
	}
	if scope.Live() != 0 {
		t.Fatal("optional reservations leaked")
	}
	if usage := a.Usage(); usage.GeneralUsed != 0 || usage.ScratchUsed != 0 {
		t.Fatalf("optional reservations leaked: %+v", usage)
	}
}
