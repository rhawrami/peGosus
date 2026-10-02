package mem

import "sync/atomic"

// MakeAllocationScope returns a fallible, query-local reservation scope over
// an existing allocator. Retained segments stay charged until their final release.
func MakeAllocationScope(a *Allocator, limit int64) *AllocationScope {
	if a == nil || limit < 0 {
		return nil
	}
	if a.scope != nil {
		a = a.scope.allocator
	}
	return &AllocationScope{allocator: a, limit: limit}
}

// MakeAllocatorWithScope returns an allocator view whose segment allocations
// are charged to scope. The parent and view share thread-safe slab sets.
func MakeAllocatorWithScope(parent *Allocator, scope *AllocationScope) *Allocator {
	if parent == nil || scope == nil {
		return nil
	}
	parent = scope.allocator
	return &Allocator{stats: parent.stats, general: parent.general, scratch: parent.scratch, scope: scope}
}

// AllocationScope accounts for live requested bytes independently of slab
// capacity. It is safe for concurrent allocation and release.
type AllocationScope struct {
	allocator *Allocator
	limit     int64
	live      atomic.Int64
	peak      atomic.Int64
	exhausted atomic.Bool
}

// Limit returns the maximum requested bytes that may be live at once.
func (s *AllocationScope) Limit() int64 {
	if s == nil {
		return 0
	}
	return s.limit
}

// Live returns currently reserved requested bytes.
func (s *AllocationScope) Live() int64 {
	if s == nil {
		return 0
	}
	return s.live.Load()
}

// Peak returns the largest live reservation so far.
func (s *AllocationScope) Peak() int64 {
	if s == nil {
		return 0
	}
	return s.peak.Load()
}

// Exhausted reports whether a required allocation was denied by the scope.
func (s *AllocationScope) Exhausted() bool { return s != nil && s.exhausted.Load() }

// AllocSeg reserves general storage, returning false without allocation when
// the requested bytes would exceed the scope limit.
func (s *AllocationScope) AllocSeg(length int) (*Segment, bool) { return s.alloc(length, false, true) }

// AllocSegTemp reserves temporary storage under the same limit.
func (s *AllocationScope) AllocSegTemp(length int) (*Segment, bool) {
	return s.alloc(length, true, true)
}

// TryAllocSeg reserves optional general storage without marking exhaustion
// when the reservation is declined. Successful reservations retain the same accounting.
func (s *AllocationScope) TryAllocSeg(length int) (*Segment, bool) {
	return s.alloc(length, false, false)
}

func (s *AllocationScope) alloc(length int, temporary, markExhausted bool) (*Segment, bool) {
	if s == nil || length < 0 {
		return nil, false
	}
	amount := int64(length)
	for {
		live := s.live.Load()
		if amount > s.limit-live {
			if markExhausted {
				s.exhausted.Store(true)
			}
			return nil, false
		}
		if s.live.CompareAndSwap(live, live+amount) {
			break
		}
	}
	var segment *Segment
	if temporary {
		segment = s.allocator.AllocSegTemp(length)
	} else {
		segment = s.allocator.AllocSeg(length)
	}
	if segment == nil {
		s.live.Add(-amount)
		return nil, false
	}
	segment.scope = s
	segment.reserved = amount
	for {
		peak := s.peak.Load()
		live := s.live.Load()
		if live <= peak || s.peak.CompareAndSwap(peak, live) {
			break
		}
	}
	return segment, true
}
