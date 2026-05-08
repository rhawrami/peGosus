package mem

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"unsafe"
)

func TestMakeSlab(t *testing.T) {
	N := 50
	slabSizes := make([]int, N)
	for i := 0; i < N; i++ {
		slabSizes[i] = int(rand.Uint64N(50_000_000))
	}

	for _, l := range slabSizes {
		t.Run(fmt.Sprintf("Request %d bytes", l), func(t *testing.T) {
			s := MakeSlab(l)

			// ensure capacity == slice length
			if s.capacity != uint64(len(s.buff)) {
				t.Errorf("capacity %d != slice length %d", s.capacity, len(s.buff))
			}
			// ensure underlying buffer is aligned
			if !isAligned((*byte)(unsafe.Pointer(&s.buff[0])), alignSize) {
				t.Errorf("buffer not aligned to %d bytes", alignSize)
			}
			if !isAligned(s.base, alignSize) {
				t.Errorf("base address not aligned to %d bytes", alignSize)
			}
			if !isAligned((*byte)(unsafe.Pointer(s.on)), alignSize) {
				t.Errorf("starting address not aligned to %d bytes", alignSize)
			}

			// ensure capacity is divisible by alignSize
			if s.capacity%uint64(alignSize) != 0 {
				t.Errorf("capacity %d not divisible by %d", s.capacity, alignSize)
			}

			// ensure capacity >= alignSize && capacity >= length request
			if s.capacity < uint64(alignSize) || s.capacity < uint64(l) {
				t.Errorf("capacity %d not >= %d and request %d", s.capacity, alignSize, l)
			}
		})
	}

}

func TestMakeOneSegment(t *testing.T) {
	size := 1_000
	s := MakeSlab(size)

	// make segment that should work
	req := size >> 1
	g, ok := s.MakeSegment(req)
	if g == nil || !ok {
		t.Errorf("segment request should have worked, but returned %v, %v", g, ok)
	}

	// ensure base aligned to alignSize
	if !isAligned(g.base, alignSize) {
		t.Errorf("segment base %p not aligned to %d bytes", g.base, alignSize)
	}

	// ensure capacity is at least size of request
	if g.Cap() < uint64(req) {
		t.Errorf("segment cap %d < req %d", g.Cap(), req)
	}

	// with one request, ensure that segment base is same as slab base
	if g.base != s.base {
		t.Errorf("segment base %p != slab base %p", g.base, s.base)
	}

	// ensure segment length == request length
	if g.length != uint64(req) {
		t.Errorf("starting segment length %d != request length %d", g.length, req)
	}

	// ensure ref count is 1
	if g.RefCount() != 1 {
		t.Errorf("segment ref count %d != 1", g.RefCount())
	}

}

func TestOneSegmentManagement(t *testing.T) {
	size := 1_000
	s := MakeSlab(size)

	g, ok := s.MakeSegment(size >> 1)
	if !ok || g == nil {
		t.Errorf("segment request should have worked, but returned %v, %v", g, ok)
	}

	// ensure `used` == segment capacity
	if g.capacity != s.used {
		t.Errorf("segment cap %d != slab used %d", g.capacity, s.used)
	}

	g.Inc()
	g.Inc()
	g.Inc()
	if g.RefCount() != 4 {
		t.Errorf("called Inc three times, expected 4, got %d", g.RefCount())
	}

	ok = g.Dec()
	ok = g.Dec()
	ok = g.Dec()
	if !ok || g.RefCount() != 1 {
		t.Errorf("called Dec three times (after calling Inc three times), expected 1, got %d and %v",
			g.RefCount(), ok)
	}

	// return segment, ensure Slab updates `used` correctly.
	g.Put()
	if s.used != 0 {
		t.Errorf("returned segment, but used (%d) != 0", s.used)
	}

	// quick test on penultimate segment merging
	if len(s.segments) != 1 {
		t.Errorf("merged penultimate segment, but did not merge to edge")
	}
	if edge := s.segments[len(s.segments)-1]; edge.Cap() != s.capacity {
		t.Errorf("merged segment cap %d != slab cap %d", edge.Cap(), s.capacity)
	}

}

func TestMakeMultiSegment(t *testing.T) {}

func TestMultiSegmentManagement(t *testing.T) {}

func TestSimpleCoalesce(t *testing.T) {}

func TestFullCoalesce(t *testing.T) {}

func TestMakeWithCoalesce(t *testing.T) {}

func TestGrow(t *testing.T) {}

func TestClear(t *testing.T) {}

func TestHoleReuse(t *testing.T) {}
